package main

import (
	"testing"
	"time"
)

func newTestTracker(t *testing.T) (*RollbackTracker, *ExceptionStore) {
	t.Helper()
	store := newTempStore(t)
	return NewRollbackTracker(store), store
}

func feed(t *RollbackTracker, s string) Result {
	var last Result
	for _, r := range s {
		last = t.ObserveKey(KeyObservation{Kind: KeyKindChar, Rune: r})
		if last.RollbackDetected {
			return last
		}
	}
	return last
}

func feedBackspaces(t *RollbackTracker, n int) {
	for i := 0; i < n; i++ {
		t.ObserveKey(KeyObservation{Kind: KeyKindBackspace})
	}
}

func TestTrackerIdleByDefault(t *testing.T) {
	tr, _ := newTestTracker(t)
	r := tr.ObserveKey(KeyObservation{Kind: KeyKindChar, Rune: 'a'})
	if r.RollbackDetected {
		t.Error("idle tracker should not detect rollback")
	}
}

func TestTrackerRollbackHappyPath(t *testing.T) {
	tr, store := newTestTracker(t)
	tr.OnConversion("ru-en", "ку-ут", "com.jetbrains.WebStorm")

	feedBackspaces(tr, len([]rune("ку-ут")))
	r := feed(tr, "ru-en")

	if !r.RollbackDetected {
		t.Fatalf("expected rollback, got %+v", r)
	}
	if r.Word != "ru-en" {
		t.Errorf("expected word=ru-en, got %q", r.Word)
	}
	if !store.IsException("com.jetbrains.WebStorm", "ru-en") {
		t.Error("exception should be persisted")
	}
}

func TestTrackerPartialBackspaceNotRollback(t *testing.T) {
	tr, _ := newTestTracker(t)
	tr.OnConversion("ru-en", "ку-ут", "app")
	feedBackspaces(tr, 2) // not enough
	r := feed(tr, "ru-en")
	if r.RollbackDetected {
		t.Error("partial backspace + retype should NOT be rollback")
	}
}

func TestTrackerTypeMismatchResets(t *testing.T) {
	tr, store := newTestTracker(t)
	tr.OnConversion("ru-en", "ку-ут", "app")
	feedBackspaces(tr, len([]rune("ку-ут")))
	r := feed(tr, "hello") // different
	if r.RollbackDetected {
		t.Error("mismatched retype should not be rollback")
	}
	if store.IsException("app", "ru-en") {
		t.Error("no exception should be added on mismatch")
	}
}

func TestTrackerLayoutSwitchResets(t *testing.T) {
	tr, _ := newTestTracker(t)
	tr.OnConversion("ru-en", "ку-ут", "app")
	tr.ObserveKey(KeyObservation{Kind: KeyKindLayout})
	feedBackspaces(tr, 5)
	r := feed(tr, "ru-en")
	if r.RollbackDetected {
		t.Error("layout switch should reset tracking")
	}
}

func TestTrackerAppSwitchResets(t *testing.T) {
	tr, _ := newTestTracker(t)
	tr.OnConversion("ru-en", "ку-ут", "app1")
	tr.ObserveKey(KeyObservation{Kind: KeyKindAppSwitch, App: "app2"})
	feedBackspaces(tr, 5)
	r := feed(tr, "ru-en")
	if r.RollbackDetected {
		t.Error("app switch should reset tracking")
	}
}

func TestTrackerOtherKeyResets(t *testing.T) {
	tr, _ := newTestTracker(t)
	tr.OnConversion("hello", "руддщ", "app")
	tr.ObserveKey(KeyObservation{Kind: KeyKindOther}) // arrow/enter
	feedBackspaces(tr, 5)
	r := feed(tr, "hello")
	if r.RollbackDetected {
		t.Error("non-text key should reset tracking")
	}
}

func TestTrackerConversionReplacesOld(t *testing.T) {
	tr, store := newTestTracker(t)
	tr.OnConversion("foo", "аоо", "app")
	tr.OnConversion("bar", "ифк", "app")
	feedBackspaces(tr, 3)
	r := feed(tr, "bar")
	if !r.RollbackDetected {
		t.Error("most recent conversion should be trackable")
	}
	if store.IsException("app", "foo") {
		t.Error("superseded conversion should not have exception")
	}
}

func TestTrackerWindowExpires(t *testing.T) {
	tr, _ := newTestTracker(t)
	tr.OnConversion("ru-en", "ку-ут", "app")
	tr.expiresAt = time.Now().Add(-time.Second)
	feedBackspaces(tr, 5)
	r := feed(tr, "ru-en")
	if r.RollbackDetected {
		t.Error("expired window should not trigger rollback")
	}
}

func TestTrackerTypingMoreAcceptsConversion(t *testing.T) {
	tr, store := newTestTracker(t)
	tr.OnConversion("ru-en", "ку-ут", "app")
	// user didn't backspace — just kept typing
	tr.ObserveKey(KeyObservation{Kind: KeyKindChar, Rune: ' '})
	feedBackspaces(tr, 5) // now try to fake a rollback
	r := feed(tr, "ru-en")
	if r.RollbackDetected {
		t.Error("typing more before backspace should mark conversion as accepted")
	}
	if store.IsException("app", "ru-en") {
		t.Error("accepted conversion should not become exception")
	}
}
