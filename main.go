package main

import (
	"flag"
	"fmt"
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
	macX         = 0x07 // X key
	macC         = 0x08 // C key
	macV         = 0x09 // V key

	kCGEventFlagMaskShift = 1 << 17
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

// shouldSkipWord runs the common pre-check pipeline for both the space-boundary
// and Enter-boundary callbacks: minimum word length, URL/email/path filter,
// excluded-app rule, and learned-exception lookup. Returns the resolved
// frontmost app id so callers can avoid querying it twice.
//
// Single-letter QWERTY keys that map to real Russian words (z=я, b=и, e=у, ...)
// bypass MinWordLength — the detector still applies its own context guard.
func shouldSkipWord(cfg *Config, store *ExceptionStore, word string) (skip bool, app string) {
	if cfg.MinWordLength > 0 {
		n := len([]rune(word))
		if n < cfg.MinWordLength {
			if _, isSingleLetterRu := singleLetterRu[word]; n != 1 || !isSingleLetterRu {
				return true, ""
			}
		}
	}
	if looksLikeContext(word) {
		return true, ""
	}
	app = FrontmostAppID()
	if cfg.IsAppExcluded(app) {
		return true, app
	}
	if store != nil && store.IsException(app, word) {
		log.Printf("Exception skip: %q in %q", word, app)
		return true, app
	}
	return false, app
}

// looksLikeContext returns true if the word looks like a URL, email, file path,
// or identifier that should NOT be auto-converted. Heuristics are conservative —
// we'd rather miss a conversion than mangle a URL.
func looksLikeContext(word string) bool {
	if word == "" {
		return false
	}
	hasDigit := false
	dots := 0
	for _, r := range word {
		switch r {
		case '@', '/', '\\', ':':
			return true // definitely URL, email, path, or namespaced identifier
		case '_':
			return true // snake_case identifier (common in code)
		case '-':
			// Could be a hyphenated word. Only skip if multiple hyphens or mixed with dots.
			dots++
		case '.':
			dots++
		}
		if r >= '0' && r <= '9' {
			hasDigit = true
		}
	}
	// "word.word" or "word.word.word" — likely URL/domain/filename
	if dots >= 1 && hasDigit {
		return true
	}
	if dots >= 2 {
		return true
	}
	return false
}

// convertSelectedText: copy selected text, convert QWERTY↔Cyrillic, paste back.
// Triggered by Cmd+Shift+X. Runs in a goroutine because it involves keystroke
// synthesis and clipboard I/O that should not block the event tap callback.
func convertSelectedText(detector *Detector) {
	// Save current clipboard so we can restore it afterwards.
	savedClipboard := readClipboard()

	// Copy selection into clipboard and wait for the OS to populate it.
	atomic.StoreInt32(&replacing, 1)
	// Release any modifiers still held from the triggering Cmd+Shift+X hotkey
	// (esp. a synthetic one from Karabiner), otherwise the Shift/Cmd leaks into
	// our Cmd+C and the copy fails ("no selection detected").
	clearModifiers()
	time.Sleep(20 * time.Millisecond)
	sendCopy()
	time.Sleep(100 * time.Millisecond)

	selected := readClipboard()
	if selected == "" || selected == savedClipboard {
		// Nothing selected, or copy didn't update the clipboard.
		atomic.StoreInt32(&replacing, 0)
		if savedClipboard != "" {
			writeClipboard(savedClipboard)
		}
		log.Printf("Manual convert: no selection detected")
		return
	}

	// Convert the entire selection in whichever direction fits.
	// Heuristic: if it contains Cyrillic letters, convert RU→QWERTY; else QWERTY→RU.
	var converted string
	hasCyrillic := false
	for _, r := range selected {
		if r >= 'а' && r <= 'я' || r >= 'А' && r <= 'Я' || r == 'ё' || r == 'Ё' {
			hasCyrillic = true
			break
		}
	}
	if hasCyrillic {
		converted = RussianToQWERTY(selected)
	} else {
		converted = QWERTYToRussian(selected)
	}

	// Put converted text into clipboard and paste it over the selection.
	writeClipboard(converted)
	time.Sleep(30 * time.Millisecond)
	sendPaste()
	time.Sleep(150 * time.Millisecond)

	// Restore original clipboard so we don't pollute the user's copy/paste state.
	if savedClipboard != "" {
		writeClipboard(savedClipboard)
	}

	// Switch to the layout matching the converted text so the user can keep
	// typing correctly. We target the layout directly (Russian vs ASCII)
	// instead of cycling to the next source like switchLang() — cycling
	// misbehaves with >2 sources (e.g. ABC + Russian + Character Viewer),
	// landing on the wrong one and breaking every other word.
	// converted Cyrillic (hasCyrillic==false) -> want Russian;
	// converted to QWERTY (hasCyrillic==true) -> want English/ASCII.
	selectLayout(!hasCyrillic)
	time.Sleep(30 * time.Millisecond)

	// Release modifiers again so the Cmd left over from our Cmd+V paste can't
	// turn the user's next Space into Cmd+Space (Spotlight).
	clearModifiers()
	atomic.StoreInt32(&replacing, 0)

	log.Printf("Manual convert: %q → %q", selected, converted)
}

func main() {
	// CLI flags for exceptions store management — handled before tray/hook init
	var (
		flagListExceptions  = flag.Bool("list-exceptions", false, "print learned exceptions and exit")
		flagForget          = flag.String("forget", "", "remove exceptions for a word and exit")
		flagForgetApp       = flag.String("forget-app", "", "remove all exceptions for an app bundle id and exit")
		flagClearExceptions = flag.Bool("clear-exceptions", false, "remove all exceptions and exit")
		flagVerbose         = flag.Bool("verbose", false, "enable verbose per-keystroke logging")
	)
	flag.Parse()
	setVerbose(*flagVerbose)

	if *flagListExceptions || *flagForget != "" || *flagForgetApp != "" || *flagClearExceptions {
		store, err := NewExceptionStore()
		if err != nil {
			log.Fatalf("exceptions store: %v", err)
		}
		switch {
		case *flagListExceptions:
			entries := store.List()
			if len(entries) == 0 {
				fmt.Println("(no exceptions)")
				return
			}
			for _, e := range entries {
				fmt.Printf("%-40s  %-30s  %d hits  added=%s\n",
					e.App, e.Word, e.HitCount, e.Added.Format("2006-01-02"))
			}
		case *flagForget != "":
			n, err := store.Forget(*flagForget)
			if err != nil {
				log.Fatalf("forget: %v", err)
			}
			fmt.Printf("forgot %d entries for word %q\n", n, *flagForget)
		case *flagForgetApp != "":
			n, err := store.ForgetApp(*flagForgetApp)
			if err != nil {
				log.Fatalf("forget-app: %v", err)
			}
			fmt.Printf("forgot %d entries for app %q\n", n, *flagForgetApp)
		case *flagClearExceptions:
			if err := store.Clear(); err != nil {
				log.Fatalf("clear: %v", err)
			}
			fmt.Println("exceptions cleared")
		}
		return
	}

	log.Println("Bzz starting...")

	// Load config
	cfg, err := LoadConfig()
	if err != nil {
		log.Printf("Config warning: %v", err)
	}
	if !cfg.Enabled {
		log.Println("Disabled in config, exiting")
		return
	}

	// Exceptions store + rollback tracker — learning from user corrections.
	// Store failures are non-fatal: we fall back to no-learning mode.
	store, err := NewExceptionStore()
	if err != nil {
		log.Printf("Exceptions store warning: %v — running without learning", err)
	}
	tracker := NewRollbackTracker(store)

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

	// doReplace performs replacement, saves undo state, and arms the rollback tracker.
	doReplace := func(buf *Buffer, word string, corrected string, deleteChars int, newText string) {
		undo.Save(word, newText)
		if tracker != nil {
			tracker.OnConversion(word, newText, FrontmostAppID())
		}
		replaceText(buf, deleteChars, newText)
	}

	// Create buffer with word callback (for space and other non-Enter boundaries)
	var buf *Buffer
	buf = NewBuffer(func(word string) {
		if !cfg.Enabled || atomic.LoadInt32(&replacing) == 1 || !isTrayEnabled() {
			return
		}
		if skip, _ := shouldSkipWord(cfg, store, word); skip {
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

		// Cmd+Shift+X — manually convert selected text (killer feature)
		if keycode == macX &&
			(flags&kCGEventFlagMaskCommand) != 0 &&
			(flags&kCGEventFlagMaskShift) != 0 {
			log.Printf("Manual convert hotkey (Cmd+Shift+X)")
			// Clear the auto-correction buffer: the selected word is being
			// replaced via clipboard, so the keystrokes still accumulated here
			// are stale. Without this, the next space re-fires auto-correction
			// on the old letters and double-converts (e.g. "привет" -> "привета").
			buf.Clear()
			go convertSelectedText(detector)
			return true // suppress the hotkey
		}

		// Cmd+Z — undo last replacement (within 5 seconds)
		if keycode == macZ && (flags&kCGEventFlagMaskCommand) != 0 {
			original, replaced, ok := undo.Get()
			if !ok {
				return false // no recent replacement, let Cmd+Z pass to app
			}
			// Explicit user rejection — learn this as an exception.
			if store != nil {
				app := FrontmostAppID()
				if err := store.Add(app, original); err == nil {
					log.Printf("Learned exception (Cmd+Z): %q in %q", original, app)
				}
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
			if tracker != nil {
				tracker.ObserveKey(KeyObservation{Kind: KeyKindBackspace})
			}
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
				if tracker != nil {
					tracker.ObserveKey(KeyObservation{Kind: KeyKindOther})
				}
				return false
			}

			if skip, _ := shouldSkipWord(cfg, store, word); skip {
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
				if tracker != nil {
					tracker.OnConversion(word, newText, FrontmostAppID())
				}

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
		if tracker != nil {
			res := tracker.ObserveKey(KeyObservation{Kind: KeyKindChar, Rune: char})
			if res.RollbackDetected {
				log.Printf("Learned exception (retype): %q in %q", res.Word, res.App)
			}
		}
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

	// Install NSWorkspace observer for thread-safe frontmost app detection
	// (must be called after startTray() initializes NSApplication).
	installFrontmostObserver()

	log.Println("Bzz ready")

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
