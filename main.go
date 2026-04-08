package main

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func init() {
	// macOS requires NSApp and status bar to run on the actual OS main thread.
	runtime.LockOSThread()
}

const (
	macBackspace = 0x33
	macReturn    = 0x24
	macEnter     = 0x4C // numpad enter
	macZ         = 0x06 // Z key
)

// lastReplace stores the last replacement for undo
type undoState struct {
	mu        sync.Mutex
	original  string // what was on screen before replacement (QWERTY text)
	replaced  string // what we typed instead
	timestamp time.Time
}

var undo undoState

func (u *undoState) Save(original, replaced string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.original = original
	u.replaced = replaced
	u.timestamp = time.Now()
}

func (u *undoState) Get() (original, replaced string, ok bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.original == "" || time.Since(u.timestamp) > 5*time.Second {
		return "", "", false
	}
	orig, repl := u.original, u.replaced
	u.original = "" // consume — one undo only
	return orig, repl, true
}

func main() {
	log.Println("RuSwitch starting...")

	// Load config
	cfg, err := LoadConfig()
	if err != nil {
		log.Printf("Config warning: %v", err)
	}
	if !cfg.Enabled {
		log.Println("Disabled in config, exiting")
		return
	}

	// Load dictionaries
	ruDict, err := LoadDict("ru")
	if err != nil {
		log.Fatalf("Cannot load Russian dict: %v", err)
	}
	log.Printf("Russian dict: %d words", len(ruDict.words))

	enDict, err := LoadDict("en")
	if err != nil {
		log.Fatalf("Cannot load English dict: %v", err)
	}
	log.Printf("English dict: %d words", len(enDict.words))

	// Init detector
	detector := NewDetector(ruDict, enDict)

	// doReplace performs replacement and saves undo state
	doReplace := func(buf *Buffer, word string, corrected string, deleteChars int, newText string) {
		undo.Save(word, newText)
		replaceText(buf, deleteChars, newText)
	}

	// Create buffer with word callback (for space and other non-Enter boundaries)
	var buf *Buffer
	buf = NewBuffer(func(word string) {
		if !cfg.Enabled || atomic.LoadInt32(&replacing) == 1 || !isTrayEnabled() {
			return
		}

		wrong, corrected := detector.Check(word)
		if !wrong {
			return
		}

		if detector.trailingPunct != 0 {
			wordRunes := []rune(word)
			pureWordLen := len(wordRunes) - 1
			lastChar := wordRunes[len(wordRunes)-1]
			if universalPunct[lastChar] {
				log.Printf("Fix (trail %c, no space): %q → %q", detector.trailingPunct, word, corrected)
				doReplace(buf, word, corrected, pureWordLen+1, corrected+string(detector.trailingPunct))
			} else {
				log.Printf("Fix (trail %c): %q → %q", detector.trailingPunct, word, corrected)
				doReplace(buf, word, corrected, pureWordLen+2, corrected+string(detector.trailingPunct)+" ")
			}
		} else {
			log.Printf("Fix: %q → %q", word, corrected)
			doReplace(buf, word, corrected, len([]rune(word))+1, corrected+" ")
		}
	})

	// Set up key event handler — called synchronously from CGEventTap
	onKeyEvent = func(keycode uint16, char rune, flags int64) bool {
		if !cfg.Enabled || !isTrayEnabled() {
			return false
		}

		// Cmd+Z — undo last replacement (within 5 seconds)
		if keycode == macZ && (flags&kCGEventFlagMaskCommand) != 0 {
			original, replaced, ok := undo.Get()
			if !ok {
				return false // no recent replacement, let Cmd+Z pass to app
			}
			log.Printf("Undo: reverting %q → %q", replaced, original)
			go func() {
				atomic.StoreInt32(&replacing, 1)
				buf.Clear()

				// Delete the replaced text
				for i := 0; i < len([]rune(replaced)); i++ {
					sendBackspaceKey()
					time.Sleep(5 * time.Millisecond)
				}
				time.Sleep(10 * time.Millisecond)

				// Type original text back
				for _, ch := range original {
					sendChar(ch)
					time.Sleep(5 * time.Millisecond)
				}

				// Switch layout back
				switchLang()
				time.Sleep(30 * time.Millisecond)
				atomic.StoreInt32(&replacing, 0)
			}()
			return true // suppress Cmd+Z
		}

		// Any other key clears undo window (user moved on)
		if keycode != macBackspace && char != 0 {
			// Don't clear on modifier-only keys
			if (flags & kCGEventFlagMaskCommand) == 0 {
				undo.mu.Lock()
				undo.original = ""
				undo.mu.Unlock()
			}
		}

		// Backspace
		if keycode == macBackspace {
			buf.Backspace()
			return false
		}

		// Skip null chars
		if char == 0 || char == 0x08 {
			return false
		}

		// Enter/Return — check word BEFORE letting Enter through
		if keycode == macReturn || keycode == macEnter || char == '\r' || char == '\n' {
			word := buf.FlushWord()
			if word == "" {
				return false
			}

			wrong, corrected := detector.Check(word)
			if !wrong {
				return false
			}

			go func() {
				log.Printf("Fix (enter): %q → %q", word, corrected)
				atomic.StoreInt32(&replacing, 1)
				buf.Clear()

				wordRunes := []rune(word)
				for i := 0; i < len(wordRunes); i++ {
					sendBackspaceKey()
					time.Sleep(5 * time.Millisecond)
				}
				time.Sleep(10 * time.Millisecond)

				newText := corrected
				for _, ch := range corrected {
					sendChar(ch)
					time.Sleep(5 * time.Millisecond)
				}

				undo.Save(word, newText)

				switchLang()
				time.Sleep(30 * time.Millisecond)
				atomic.StoreInt32(&replacing, 0)

				time.Sleep(10 * time.Millisecond)
				sendEnter()
			}()
			return true
		}

		// Regular char
		buf.Add(char)
		return false
	}

	// Start keyboard hook
	err = startHook()
	if err != nil {
		log.Fatalf("Hook error: %v", err)
	}
	log.Println("Keyboard hook started")

	// Start tray icon
	startTray()
	log.Println("RuSwitch ready")

	// Handle signals in background
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("Shutting down")
		os.Exit(0)
	}()

	// Run NSApp loop on main thread
	runAppLoop()
}
