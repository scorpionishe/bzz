package main

import (
	"testing"
	"time"
)

// newTempLearn returns a store backed by a temp dir with an injectable clock.
// Advance time via *clock = clock.Add(...).
func newTempLearn(t *testing.T) (*LearnStore, *time.Time) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("BZZ_CONFIG_DIR", dir)
	s, err := NewLearnStore(3)
	if err != nil {
		t.Fatalf("NewLearnStore: %v", err)
	}
	cur := time.Now()
	clock := &cur
	s.now = func() time.Time { return *clock }
	return s, clock
}

func tick(clock *time.Time, d time.Duration) {
	*clock = clock.Add(d)
}

func TestLearnPromoteAfterThreshold(t *testing.T) {
	s, clock := newTempLearn(t)
	for i := 0; i < 2; i++ {
		if s.RecordManualFlip("агяя", "fuzz") {
			t.Fatalf("promoted too early on signal %d", i+1)
		}
		tick(clock, 2*time.Minute)
	}
	if _, ok := s.Rule("агяя"); ok {
		t.Fatal("rule must not exist before threshold")
	}
	if !s.RecordManualFlip("агяя", "fuzz") {
		t.Fatal("expected promotion on 3rd signal")
	}
	conv, ok := s.Rule("агяя")
	if !ok || conv != "fuzz" {
		t.Fatalf("Rule = %q, %v; want fuzz, true", conv, ok)
	}
}

func TestLearnRuleCasePreserving(t *testing.T) {
	s, clock := newTempLearn(t)
	for i := 0; i < 3; i++ {
		s.RecordManualFlip("агяя", "fuzz")
		tick(clock, 2*time.Minute)
	}
	conv, ok := s.Rule("Агяя")
	if !ok || conv != "Fuzz" {
		t.Fatalf("Rule(Агяя) = %q, %v; want Fuzz, true", conv, ok)
	}
}

func TestLearnDedupWindow(t *testing.T) {
	s, clock := newTempLearn(t)
	// Three rapid flips within the window = one signal, no rule.
	for i := 0; i < 3; i++ {
		if s.RecordManualFlip("агяя", "fuzz") {
			t.Fatal("rapid flips must not promote")
		}
		tick(clock, 5*time.Second)
	}
	list := s.List()
	if len(list) != 1 || list[0].Pos != 1 {
		t.Fatalf("expected single entry with pos=1, got %+v", list)
	}
}

func TestLearnAntiToggle(t *testing.T) {
	s, clock := newTempLearn(t)
	// Flip, then flip back right away — experimentation, both cancelled.
	s.RecordManualFlip("агяя", "fuzz")
	tick(clock, 2*time.Second)
	s.RecordManualFlip("fuzz", "агяя")
	list := s.List()
	if len(list) != 1 {
		t.Fatalf("expected only the first entry, got %d", len(list))
	}
	if list[0].Pos != 0 {
		t.Fatalf("first signal should be cancelled, pos=%d", list[0].Pos)
	}
}

func TestLearnAntiToggleExpires(t *testing.T) {
	s, clock := newTempLearn(t)
	s.RecordManualFlip("агяя", "fuzz")
	tick(clock, learnToggleWindow+time.Second)
	// Too late to be a toggle — counts as a real (reverse) flip.
	s.RecordManualFlip("fuzz", "агяя")
	list := s.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
	for _, e := range list {
		if e.Pos != 1 {
			t.Fatalf("entry %q pos=%d, want 1", e.Word, e.Pos)
		}
	}
}

func TestLearnableGuards(t *testing.T) {
	cases := []struct {
		word, conv string
		want       bool
	}{
		{"агяя", "fuzz", true},        // genuine layout flip
		{"ghbdtn", "привет", true},    // en→ru
		{"Ж", ":", false},             // single char + punct result
		{"ЖЖ", "::", false},           // punct result
		{"f", "а", false},             // single char
		{"ghbdtn,", "привет,", false}, // trailing punct — not letters-only
		{"n.l.", "т.д.", false},       // known abbreviation
		{"vk.com", "мл.сщь", false},   // URL-ish
		{"foo_bar", "ащщ_ифк", false}, // identifier
		{"мама", "мама", false},       // same script both sides
		{"fooмама", "барfoo", false},  // mixed script
		{"год2", "ujl2", false},       // digit inside
	}
	for _, c := range cases {
		if got := learnable(c.word, c.conv); got != c.want {
			t.Errorf("learnable(%q, %q) = %v, want %v", c.word, c.conv, got, c.want)
		}
	}
}

func TestLearnDemoteDropsRule(t *testing.T) {
	s, clock := newTempLearn(t)
	for i := 0; i < 3; i++ {
		s.RecordManualFlip("агяя", "fuzz")
		tick(clock, 2*time.Minute)
	}
	if _, ok := s.Rule("агяя"); !ok {
		t.Fatal("rule expected")
	}
	// User keeps flipping fuzz back to агяя — three spaced reverts drop the rule.
	for i := 0; i < 2; i++ {
		if s.RecordRevert("агяя", "fuzz") {
			t.Fatalf("demoted too early on revert %d", i+1)
		}
		tick(clock, 2*time.Minute)
	}
	if !s.RecordRevert("агяя", "fuzz") {
		t.Fatal("expected demotion on 3rd revert")
	}
	if _, ok := s.Rule("агяя"); ok {
		t.Fatal("rule must be gone after demotion")
	}
}

func TestLearnRevertOnDictConversion(t *testing.T) {
	// Reverts must also count for words auto-converted by the dictionary
	// (no prior learn entry): 3 reverts → demoted=true → caller adds exception.
	s, clock := newTempLearn(t)
	for i := 0; i < 2; i++ {
		if s.RecordRevert("ghbdtn", "привет") {
			t.Fatalf("demoted too early on revert %d", i+1)
		}
		tick(clock, 2*time.Minute)
	}
	if !s.RecordRevert("ghbdtn", "привет") {
		t.Fatal("expected demotion signal on 3rd revert")
	}
}

func TestLearnPersistReload(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BZZ_CONFIG_DIR", dir)

	s1, err := NewLearnStore(3)
	if err != nil {
		t.Fatal(err)
	}
	cur := time.Now()
	s1.now = func() time.Time { return cur }
	for i := 0; i < 3; i++ {
		s1.RecordManualFlip("агяя", "fuzz")
		cur = cur.Add(2 * time.Minute)
	}

	s2, err := NewLearnStore(3)
	if err != nil {
		t.Fatal(err)
	}
	conv, ok := s2.Rule("агяя")
	if !ok || conv != "fuzz" {
		t.Fatalf("reloaded Rule = %q, %v; want fuzz, true", conv, ok)
	}
	list := s2.List()
	if len(list) != 1 || list[0].Pos != 3 || !list[0].Rule {
		t.Fatalf("reloaded entry mismatch: %+v", list)
	}
}

func TestLearnForget(t *testing.T) {
	s, clock := newTempLearn(t)
	for i := 0; i < 3; i++ {
		s.RecordManualFlip("агяя", "fuzz")
		tick(clock, 2*time.Minute)
	}
	n, err := s.Forget("агяя")
	if err != nil || n != 1 {
		t.Fatalf("Forget = %d, %v; want 1, nil", n, err)
	}
	if _, ok := s.Rule("агяя"); ok {
		t.Fatal("rule must be gone after Forget")
	}
}

func TestLearnNilStoreSafe(t *testing.T) {
	var s *LearnStore
	if s.RecordManualFlip("агяя", "fuzz") {
		t.Fatal("nil store must be a no-op")
	}
	if s.RecordRevert("агяя", "fuzz") {
		t.Fatal("nil store must be a no-op")
	}
	if _, ok := s.Rule("агяя"); ok {
		t.Fatal("nil store must have no rules")
	}
}
