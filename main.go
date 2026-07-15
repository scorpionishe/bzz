package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
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

// lastConvState remembers the most recent auto-conversion (non-consuming,
// unlike undo) so a manual hotkey flip of the converted word can be recognized
// as a REVERT — a negative learning signal — rather than a fresh manual flip.
type lastConvState struct {
	mu       sync.Mutex
	original string // what the user typed (wrong-layout form)
	replaced string // what bzz inserted (without trailing space/punct)
	at       time.Time
}

const lastConvWindow = 30 * time.Second

func (l *lastConvState) Save(original, replaced string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.original = original
	l.replaced = strings.TrimRight(replaced, " ")
	l.at = time.Now()
}

// Match reports whether flipping selected → converted undoes the last
// auto-conversion, and returns the original wrong-layout form.
func (l *lastConvState) Match(selected, converted string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.original == "" || time.Since(l.at) > lastConvWindow {
		return "", false
	}
	if strings.EqualFold(selected, l.replaced) && strings.EqualFold(converted, l.original) {
		return l.original, true
	}
	return "", false
}

var lastConv lastConvState

// switchLayoutEnabled mirrors Config.SwitchLayout (1 = on). Toggled from the tray
// thread and read from replace goroutines, so it's an atomic int32. When on,
// maybeSwitchLayout() moves the system input source to match a correction.
var switchLayoutEnabled int32

// Shared runtime state the tray settings submenu mutates. activeCfg is the live
// config (persisted on change), activeDetector lets the Context toggle take
// effect immediately, and pendingExclude is the frontmost app captured when the
// menu opens (used by the exclude-app action). cfgMu guards config mutation.
var (
	activeCfg      *Config
	activeStore    *ExceptionStore
	activeDetector *Detector
	activeLearn    *LearnStore
	pendingExclude string
	cfgMu          sync.Mutex
)

// learnManualFlip records a positive learning signal after a manual hotkey
// conversion. On promotion the word becomes an auto-convert rule; any learned
// exception for it is dropped so the rule can actually fire.
func learnManualFlip(word, converted string) {
	if activeLearn == nil {
		return
	}
	if activeLearn.RecordManualFlip(word, converted) {
		log.Printf("Learned rule: %q → %q (will auto-convert)", word, converted)
		if activeStore != nil {
			if n, _ := activeStore.Forget(word); n > 0 {
				log.Printf("Learned rule: dropped %d exception(s) for %q", n, word)
			}
		}
	}
}

// learnRevert records a negative learning signal: the user manually flipped
// back a word bzz had auto-converted. On demotion the rule (if any) is gone
// and the word is persisted as a global exception.
func learnRevert(original, replaced string) {
	if activeLearn == nil {
		return
	}
	if activeLearn.RecordRevert(original, replaced) {
		if activeStore != nil {
			if err := activeStore.Add("", original); err == nil {
				log.Printf("Learned exclusion: %q — reverted too often, added to exceptions", original)
			}
		}
	}
}

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
				vlog("SKIP %q: too short (len=%d < min=%d)", word, n, cfg.MinWordLength)
				return true, ""
			}
		}
	}
	if looksLikeContext(word) {
		vlog("SKIP %q: looksLikeContext (url/email/path/identifier)", word)
		return true, ""
	}
	app = FrontmostAppID()
	if cfg.IsAppExcluded(app) {
		vlog("SKIP %q: app excluded (%s)", word, app)
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

// lastToken returns the last whitespace-delimited token of s (s itself if no
// whitespace). Used to inspect the final word of a converted selection.
func lastToken(s string) string {
	if i := strings.LastIndexAny(s, " \t\n"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// typedOnRussianLayout decides the manual-flip direction for a pending word:
// true → the word was typed on the Russian layout, flip RU→EN. Evidence is
// either a Russian-only char (Cyrillic letter, №) or a physical-key signature —
// a char plus the keycode that can only produce it on the Russian layout
// (';' from the 8 key, ']' from the key left of 1). The keycode check is what
// distinguishes ';' typed as Shift+8 on Russian (user wanted '*') from ';'
// typed on the semicolon key in EN layout (flips to 'ж' as before).
func typedOnRussianLayout(pending string, codes []uint16) bool {
	for i, r := range []rune(pending) {
		if r >= 'а' && r <= 'я' || r >= 'А' && r <= 'Я' || r == 'ё' || r == 'Ё' || r == '№' {
			return true
		}
		if i < len(codes) && isRuLayoutEvidence(r, codes[i]) {
			return true
		}
	}
	return false
}

// convertPendingWord converts the word currently being typed (still in bzz's
// buffer, no selection needed) to the other layout, in place. It backspaces the
// typed chars and types the converted form — the only approach that works
// reliably across apps (terminals can't replace a selection; some editors copy
// the whole line on an empty Cmd+C). Layout is left unchanged on purpose.
func convertPendingWord(pending string, codes []uint16) {
	atomic.StoreInt32(&replacing, 1)
	clearModifiers()
	time.Sleep(20 * time.Millisecond)

	var converted string
	if typedOnRussianLayout(pending, codes) {
		converted = RussianToQWERTY(pending)
	} else {
		converted = QWERTYToRussian(pending)
	}

	for range []rune(pending) {
		sendBackspaceKey()
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)
	for _, ch := range converted {
		sendChar(ch)
		time.Sleep(5 * time.Millisecond)
	}

	clearModifiers()
	finishReplacing()
	log.Printf("Manual convert (word): %q → %q", pending, converted)

	// Positive learning signal: bzz left this word alone, the user flipped it.
	learnManualFlip(pending, converted)
}

// revertReplacement flips the last auto-conversion back: deletes the inserted
// text and retypes what the user originally typed. Triggered by a bare
// manual-convert hotkey press with no pending word and no selection, within
// the undo window. Counts as a negative learning signal — learn_threshold
// reverts of the same word drop its rule and persist an exception. The caller
// must hold replacing=1 with modifiers cleared.
func revertReplacement(original, replaced string) {
	log.Printf("Revert (hotkey): %q → %q", replaced, original)
	for range []rune(replaced) {
		sendBackspaceKey()
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)
	for _, ch := range original {
		sendChar(ch)
		time.Sleep(5 * time.Millisecond)
	}
	// Switch layout back only in switch-mode; neutral mode never switched.
	maybeSwitchLayout(original)
	time.Sleep(30 * time.Millisecond)
	learnRevert(original, strings.TrimRight(replaced, " "))
}

// convertSelection reads the current selection, converts it QWERTY↔Cyrillic and
// pastes it back over the selection. Runs in a goroutine because it synthesizes
// keystrokes and touches the clipboard, which should not block the event tap.
//
// Selection is read two ways, in order:
//  1. Accessibility API (axSelectedText) — clean, no clipboard, works in native
//     apps. Returns "" in Electron apps (VSCode) that don't expose it.
//  2. Guarded Cmd+C fallback — for apps where AX gave nothing. The trap here is
//     VSCode's editor.emptySelectionClipboard: an empty-selection copy grabs the
//     WHOLE LINE (this was the original garbage bug). We reject that by the one
//     reliable tell — a whole-line copy ends with the line's trailing newline,
//     while a real in-line selection never does. A sentinel also distinguishes
//     "copy did nothing" from a genuinely empty clipboard.
func convertSelection(detector *Detector, buf *Buffer) {
	atomic.StoreInt32(&replacing, 1)
	defer finishReplacing()
	// Release any modifiers still held from the triggering hotkey (esp. a
	// synthetic one from Karabiner) so they can't leak into our Cmd+C / Cmd+V.
	clearModifiers()
	time.Sleep(20 * time.Millisecond)

	savedClipboard := readClipboard()
	selected := axSelectedText()
	if selected == "" {
		// AX exposed nothing — fall back to a guarded Cmd+C.
		const sentinel = "\x00bzz-nosel\x00"
		writeClipboard(sentinel)
		time.Sleep(20 * time.Millisecond)
		sendCopy()
		time.Sleep(120 * time.Millisecond)
		got := readClipboard()
		buf.Clear() // drop the stray 'c' our synthetic Cmd+C left in the buffer
		if savedClipboard == sentinel {
			savedClipboard = ""
		}
		if got == sentinel || got == "" || strings.HasSuffix(got, "\n") {
			// Nothing selected, or VSCode's whole-line copy (trailing newline) —
			// do NOT convert; that was the source of the garbage.
			writeClipboard(savedClipboard)
			clearModifiers()
			// Bare hotkey right after an auto-conversion = revert it (this
			// replaced the old Cmd+Z undo; Cmd+Z stays with the app).
			if original, replaced, ok := undo.Get(); ok {
				revertReplacement(original, replaced)
				return
			}
			log.Printf("Manual convert: nothing to convert (no selection)")
			return
		}
		selected = got
	}

	// Convert the entire selection in whichever direction fits.
	// Direction is decided by the MAJORITY script, not by "any Cyrillic present".
	// Selections are often mixed (the user's text already has some words
	// auto-corrected to Cyrillic next to a still-Latin word, or the selection
	// grabbed one adjacent char). Picking by majority and letting the conversion
	// pass already-correct chars through avoids the flip-flop where a mostly-Latin
	// word with one trailing Cyrillic letter got mangled (e.g. "geyrnjvп" →
	// "geyrnjvg") and needed a second f18 press.
	var converted string
	latin, cyr := 0, 0
	for _, r := range selected {
		switch {
		// '№' exists only on the Russian layout, so it counts as Cyrillic
		// evidence — a bare "№"/"№5" selection flips RU→EN (№ → #).
		case r >= 'а' && r <= 'я' || r >= 'А' && r <= 'Я' || r == 'ё' || r == 'Ё' || r == '№':
			cyr++
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
			latin++
		}
	}
	if cyr > latin {
		converted = RussianToQWERTY(selected)
	} else {
		converted = QWERTYToRussian(selected)
		// Trailing punct-as-letter heuristic (mirror of the auto-detector): the
		// last char of a latin selection could be a letter ("," = б) or real
		// punctuation ("дела,"). Keep it literal when the last word without it is
		// a solid match (valid & >2 chars) AND either exact, or the full
		// punct-as-letter word is nonsense ("делаб") — then "," is a comma.
		if r := []rune(selected); len(r) >= 2 && qwertyRuPunct[r[len(r)-1]] {
			trimConv := QWERTYToRussian(string(r[:len(r)-1]))
			lastWord := lastToken(trimConv)
			fullLastWord := lastToken(converted)
			if len([]rune(lastWord)) > 2 && detector.ruDict.Has(lastWord) &&
				(detector.ruDict.words[strings.ToLower(lastWord)] || !detector.ruDict.Has(fullLastWord)) {
				converted = trimConv + string(r[len(r)-1])
			}
		}
	}

	// Put converted text into clipboard and paste it over the selection.
	writeClipboard(converted)
	time.Sleep(30 * time.Millisecond)
	sendPaste()
	time.Sleep(150 * time.Millisecond)

	// Restore original clipboard so we don't pollute the user's copy/paste state.
	writeClipboard(savedClipboard)

	// Deliberately do NOT switch the system input source. bzz stays layout-
	// neutral (pure Punto-style text fixer): it converts the selected word in
	// place and leaves the active layout alone. Switching it to match the
	// converted word's language disrupts the common case of a single foreign
	// word inside a sentence — after fixing it the user keeps typing in their
	// original language, which a layout switch would derail (every following
	// word then comes out in the wrong script and flickers as auto-correct
	// fixes it). Auto-correction already handles continued typing.

	// Release modifiers again so the Cmd left over from our Cmd+V paste can't
	// turn the user's next Space into Cmd+Space (Spotlight).
	clearModifiers()

	log.Printf("Manual convert (selection): %q → %q", selected, converted)

	// Learning signals — single-word selections only (multi-word selections are
	// too ambiguous to learn from). Flipping back the word bzz just auto-converted
	// is a negative signal; anything else is a positive one.
	if !strings.ContainsAny(selected, " \t\n") {
		if orig, ok := lastConv.Match(selected, converted); ok {
			learnRevert(orig, selected)
		} else {
			learnManualFlip(selected, converted)
		}
	}
}

func main() {
	// CLI flags for exceptions store management — handled before tray/hook init
	var (
		flagListExceptions  = flag.Bool("list-exceptions", false, "print learned exceptions and exit")
		flagForget          = flag.String("forget", "", "remove exceptions for a word and exit")
		flagForgetApp       = flag.String("forget-app", "", "remove all exceptions for an app bundle id and exit")
		flagClearExceptions = flag.Bool("clear-exceptions", false, "remove all exceptions and exit")
		flagListLearned     = flag.String("list-learned", "", "print learned rules/candidates and exit (pass 'all' or 'rules')")
		flagForgetLearned   = flag.String("forget-learned", "", "remove a learned rule/candidate for a word and exit")
		flagClearLearned    = flag.Bool("clear-learned", false, "remove all learned rules/candidates and exit")
		flagVerbose         = flag.Bool("verbose", false, "enable verbose per-keystroke logging")
	)
	flag.Parse()
	setVerbose(*flagVerbose)

	if *flagListLearned != "" || *flagForgetLearned != "" || *flagClearLearned {
		ls, err := NewLearnStore(0)
		if err != nil {
			log.Fatalf("learn store: %v", err)
		}
		switch {
		case *flagListLearned != "":
			entries := ls.List()
			shown := 0
			for _, e := range entries {
				if *flagListLearned == "rules" && !e.Rule {
					continue
				}
				state := "candidate"
				if e.Rule {
					state = "RULE"
				}
				fmt.Printf("%-9s  %-30s  %s  pos=%d neg=%d applied=%d  last=%s\n",
					state, e.Word, e.Direction, e.Pos, e.Neg, e.Applied, e.LastEvent.Format("2006-01-02"))
				shown++
			}
			if shown == 0 {
				fmt.Println("(no learned entries)")
			}
		case *flagForgetLearned != "":
			n, err := ls.Forget(*flagForgetLearned)
			if err != nil {
				log.Fatalf("forget-learned: %v", err)
			}
			fmt.Printf("forgot %d learned entries for word %q\n", n, *flagForgetLearned)
		case *flagClearLearned:
			if err := ls.Clear(); err != nil {
				log.Fatalf("clear-learned: %v", err)
			}
			fmt.Println("learned entries cleared")
		}
		return
	}

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

	// Adaptive learning store (learned rules from manual hotkey flips).
	// Failures are non-fatal, and cfg can disable the whole mechanism.
	var learn *LearnStore
	if cfg.Learn {
		learn, err = NewLearnStore(cfg.LearnThreshold)
		if err != nil {
			log.Printf("Learn store warning: %v — running without adaptive learning", err)
			learn = nil
		}
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
	detector.contextAware = cfg.ContextAware

	// Layout-switch mode (classic Punto "switch"): when enabled, corrections also
	// move the macOS input source to match. Default off = layout-neutral.
	if cfg.SwitchLayout {
		atomic.StoreInt32(&switchLayoutEnabled, 1)
	}

	// Publish live state for the tray settings submenu.
	activeCfg = cfg
	activeStore = store
	activeDetector = detector
	activeLearn = learn

	// doReplace performs replacement, saves undo state, and arms the rollback tracker.
	doReplace := func(buf *Buffer, word string, corrected string, deleteChars int, newText string) {
		undo.Save(word, newText)
		lastConv.Save(word, newText)
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
		vlog("WORD %q (app=%s ruLayout=%v)", word, FrontmostAppID(), IsRussianLayout())

		// Known Russian abbreviation typed on EN layout (n.l. → т.д.). Handled
		// before shouldSkipWord because looksLikeContext() skips anything with two
		// dots as a URL/version. Excluded-app and learned-exception rules still apply.
		if conv, ok := lookupAbbrev(word); ok {
			app := FrontmostAppID()
			if cfg.IsAppExcluded(app) || (store != nil && store.IsException(app, word)) {
				return
			}
			log.Printf("Fix (abbrev): %q → %q", word, conv)
			doReplace(buf, word, conv, len([]rune(word))+1, conv+" ")
			return
		}

		if skip, _ := shouldSkipWord(cfg, store, word); skip {
			return
		}
		wrong, corrected := detector.Check(word)
		if !wrong {
			// Learned rule: the user manually flipped this word enough times —
			// force-convert even though the detector says it's fine.
			if conv, ok := learn.Rule(word); ok {
				wrong, corrected = true, conv
				log.Printf("Fix (learned): %q → %q", word, conv)
			} else {
				vlog("NOFIX %q (script=%s ruHas(qwerty→ru)=%v)", word, detectScript(word), detector.ruDict.Has(QWERTYToRussian(word)))
				return
			}
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

	// Parse the manual-convert hotkey from config (default cmd+shift+x).
	hotkeyCode, hotkeyMods, hkOK := parseHotkey(cfg.Hotkey)
	if !hkOK {
		log.Printf("invalid hotkey %q, falling back to cmd+shift+x", cfg.Hotkey)
		hotkeyCode, hotkeyMods = macX, kCGEventFlagMaskCommand|kCGEventFlagMaskShift
	}
	log.Printf("Manual convert hotkey: %q (keycode=0x%02X mods=0x%X)", cfg.Hotkey, hotkeyCode, hotkeyMods)

	// Set up key event handler — called synchronously from CGEventTap
	onKeyEvent = func(keycode uint16, char rune, flags int64) bool {
		if !cfg.Enabled || !isTrayEnabled() {
			return false
		}

		// Manual convert selected text (killer feature) — configurable hotkey.
		// Exact modifier match (flags&modAll == hotkeyMods) so e.g. a bare F18
		// fires only with no modifiers held.
		if keycode == hotkeyCode && (flags&modAll) == hotkeyMods {
			log.Printf("Manual convert hotkey (%s)", cfg.Hotkey)
			// Punto-style: if a word is being typed (buffer non-empty), convert
			// THAT last word in place with backspaces — reliable in every app.
			// FlushWord also clears the buffer so a later space can't re-fire
			// auto-correction on stale letters.
			//
			// Otherwise (word already finished by a space, buffer empty), read the
			// real selection via the Accessibility API and convert it. Whole-line
			// grabs from an empty-selection Cmd+C (VSCode) are rejected inside
			// convertSelection. When nothing is selected either, a bare press
			// reverts the last auto-conversion (the Cmd+Z replacement) — or
			// no-ops if there is none.
			if pending, codes := buf.FlushWord(); pending != "" {
				go convertPendingWord(pending, codes)
			} else {
				go convertSelection(detector, buf)
			}
			return true // suppress the hotkey
		}

		// Any other key clears the revert window (user moved on). Cmd+Z is NOT
		// intercepted: it belongs to the app's own undo; a bzz correction is
		// reverted with the manual-convert hotkey (bare press, no selection,
		// right after the correction).
		if keycode != macBackspace && char != 0 {
			// Don't clear on modifier-only keys
			if (flags & kCGEventFlagMaskCommand) == 0 {
				undo.mu.Lock()
				undo.original = ""
				undo.mu.Unlock()
			}
		}

		// Keystrokes with Command or Control held are shortcuts (Cmd+C/V/A/X,
		// Cmd+←, Ctrl+A …), never text input. They MUST NOT feed the word buffer:
		// the stray 'c' from Cmd+C or 'v' from Cmd+V used to linger there and get
		// "corrected" on the next boundary — paste a value into a rename field, hit
		// Enter, and it turned into "с". They also move the cursor or change the
		// clipboard/selection, so any pending word is stale. Drop the buffer and
		// let the shortcut pass through untouched. (The manual-convert hotkey is
		// handled above and never reaches here.)
		if flags&(flagCommand|flagControl) != 0 {
			buf.Clear()
			return false
		}

		// Backspace
		if keycode == macBackspace {
			buf.Backspace()
			if tracker != nil {
				tracker.ObserveKey(KeyObservation{Kind: KeyKindBackspace})
			}
			return false
		}

		// Skip null chars (modifier-only / dead keys — keep the buffer so Shift
		// before a capital letter doesn't reset the word).
		if char == 0 || char == 0x08 {
			return false
		}

		// Enter/Return — check word BEFORE letting Enter through
		if keycode == macReturn || keycode == macEnter || char == '\r' || char == '\n' {
			word, _ := buf.FlushWord()
			if word == "" {
				if tracker != nil {
					tracker.ObserveKey(KeyObservation{Kind: KeyKindOther})
				}
				return false
			}

			// Abbreviation (n.l. → т.д.) before shouldSkipWord for the same reason
			// as the space path. Convert in place, then let the Enter through.
			if conv, ok := lookupAbbrev(word); ok {
				app := FrontmostAppID()
				if cfg.IsAppExcluded(app) || (store != nil && store.IsException(app, word)) {
					return false
				}
				go func() {
					log.Printf("Fix (abbrev, enter): %q → %q", word, conv)
					atomic.StoreInt32(&replacing, 1)
					buf.Clear()
					for range []rune(word) {
						sendBackspaceKey()
						time.Sleep(5 * time.Millisecond)
					}
					time.Sleep(10 * time.Millisecond)
					for _, ch := range conv {
						sendChar(ch)
						time.Sleep(5 * time.Millisecond)
					}
					undo.Save(word, conv)
					lastConv.Save(word, conv)
					if tracker != nil {
						tracker.OnConversion(word, conv, FrontmostAppID())
					}
					maybeSwitchLayout(conv)
					time.Sleep(10 * time.Millisecond)
					sendEnter()
					finishReplacing()
				}()
				return true
			}

			if skip, _ := shouldSkipWord(cfg, store, word); skip {
				return false
			}

			wrong, corrected := detector.Check(word)
			if !wrong {
				if conv, ok := learn.Rule(word); ok {
					wrong, corrected = true, conv
					log.Printf("Fix (learned, enter): %q → %q", word, conv)
				} else {
					return false
				}
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
				lastConv.Save(word, newText)
				if tracker != nil {
					tracker.OnConversion(word, newText, FrontmostAppID())
				}

				// Only switch the system layout in switch-mode; otherwise stay
				// neutral (the old code cycled unconditionally, which landed on the
				// wrong source when >2 input sources were installed).
				maybeSwitchLayout(newText)
				time.Sleep(30 * time.Millisecond)

				time.Sleep(10 * time.Millisecond)
				sendEnter()
				finishReplacing()
			}()
			return true
		}

		// Navigation / control keys. Arrow keys arrive as control chars
		// (U+001C–U+001F), other navigation keys as low control codes. They move
		// the cursor, so any pending word is now stale. CLEAR the buffer instead
		// of feeding it: otherwise buffer.Add treats the control char as a word
		// boundary and fires a replace (backspaces) at the new cursor position,
		// deleting selected/adjacent text — e.g. the word vanishes while
		// selecting it with Shift+Option+Arrow. (Enter and Backspace are handled
		// above, so they never reach here.)
		if char < 0x20 {
			buf.Clear()
			return false
		}

		// Regular char
		buf.Add(char, keycode)
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

	// Install keyboard-layout-change observer + paint the initial flag icon.
	installLayoutObserver()

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
