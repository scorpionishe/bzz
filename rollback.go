package main

// rollback.go — Rollback detection state machine.
//
// Flow:
//   1. detector converts X -> Y and calls tracker.OnConversion(X, Y, app).
//   2. tracker enters TRACKING, records how many runes must be deleted (len(Y)).
//   3. hook calls tracker.ObserveKey(ev) for every subsequent key event.
//      The key stream is interpreted as char insertions and backspaces.
//      Any non-text event (layout switch, app switch, timeout) resets to IDLE.
//   4. When backspace count reaches len(Y), tracker enters WAIT_RETYPE.
//   5. Typed chars are appended; if they remain a prefix of X, keep waiting;
//      once equal to X → ROLLBACK_DETECTED → store.Add(app, X).
//   6. Any mismatch (wrong char, app switch, timeout, new conversion) → IDLE.
//
// Thread-safety: all public methods acquire the mutex. Intended to be called
// from the hook goroutine, but safe from any goroutine.

import (
	"strings"
	"sync"
	"time"
)

const (
	rollbackWindowSec    = 10
	rollbackConfirmCount = 1 // number of rollbacks before exception is persisted
)

type rollbackPhase int

const (
	phaseIdle rollbackPhase = iota
	phaseTracking
	phaseWaitRetype
)

type RollbackTracker struct {
	mu    sync.Mutex
	store *ExceptionStore

	phase     rollbackPhase
	original  string    // word before our conversion (what the user typed)
	replaced  string    // what we replaced it with
	app       string    // frontmost bundle id at the time of conversion
	expiresAt time.Time // deadline for rollback detection

	bsSeen    int    // backspaces observed since conversion
	typedBack string // chars typed after full backspace (for WAIT_RETYPE)
}

// NewRollbackTracker constructs a tracker. store may be nil in tests.
func NewRollbackTracker(store *ExceptionStore) *RollbackTracker {
	return &RollbackTracker{store: store, phase: phaseIdle}
}

// OnConversion must be called AFTER the replacer has successfully inserted
// the replaced text. original is what the user typed, replaced is what we
// inserted in its place. app is the frontmost bundle id at that moment;
// pass "" for global.
func (t *RollbackTracker) OnConversion(original, replaced, app string) {
	if original == "" || replaced == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	t.phase = phaseTracking
	t.original = original
	t.replaced = replaced
	t.app = app
	t.expiresAt = time.Now().Add(time.Duration(rollbackWindowSec) * time.Second)
	t.bsSeen = 0
	t.typedBack = ""
}

// KeyKind classifies the event for the tracker. hook_darwin.go/hook_windows.go
// should map their native events to one of these and call ObserveKey.
type KeyKind int

const (
	KeyKindChar      KeyKind = iota // printable character (pass Rune)
	KeyKindBackspace                // single backspace
	KeyKindOther                    // arrow, enter, tab, etc. — resets
	KeyKindLayout                   // system layout switched — resets
	KeyKindAppSwitch                // frontmost app changed — resets
)

type KeyObservation struct {
	Kind KeyKind
	Rune rune   // for KeyKindChar
	App  string // for KeyKindAppSwitch (new app); optional otherwise
}

// Result is what ObserveKey returns to the caller.
type Result struct {
	RollbackDetected bool
	Word             string // the original word we should remember as exception
	App              string
}

func (t *RollbackTracker) ObserveKey(obs KeyObservation) Result {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.phase == phaseIdle {
		return Result{}
	}
	if time.Now().After(t.expiresAt) {
		t.reset()
		return Result{}
	}

	switch obs.Kind {
	case KeyKindLayout:
		t.reset()
		return Result{}
	case KeyKindAppSwitch:
		if obs.App != "" && obs.App != t.app {
			t.reset()
		}
		return Result{}
	case KeyKindOther:
		t.reset()
		return Result{}
	case KeyKindBackspace:
		switch t.phase {
		case phaseTracking:
			t.bsSeen++
			if t.bsSeen >= len([]rune(t.replaced)) {
				t.phase = phaseWaitRetype
				t.typedBack = ""
			}
		case phaseWaitRetype:
			// extra backspaces after full deletion kill the chain
			if t.typedBack != "" {
				t.reset()
			}
		}
		return Result{}
	case KeyKindChar:
		switch t.phase {
		case phaseTracking:
			// user typed more of the replaced text — they're accepting, not rolling back
			t.reset()
			return Result{}
		case phaseWaitRetype:
			t.typedBack += string(obs.Rune)
			if !strings.HasPrefix(t.original, t.typedBack) {
				t.reset()
				return Result{}
			}
			if t.typedBack == t.original {
				word := t.original
				app := t.app
				t.reset()
				if t.store != nil {
					_ = t.store.Add(app, word)
				}
				return Result{RollbackDetected: true, Word: word, App: app}
			}
		}
	}
	return Result{}
}

// CurrentPhase exposes state for tests / debug.
func (t *RollbackTracker) CurrentPhase() rollbackPhase {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.phase
}

func (t *RollbackTracker) reset() {
	t.phase = phaseIdle
	t.original = ""
	t.replaced = ""
	t.app = ""
	t.bsSeen = 0
	t.typedBack = ""
}
