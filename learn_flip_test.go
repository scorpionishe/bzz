package main

import (
	"testing"
	"time"
)

// withLearnGlobals wires the package globals learnManualFlip reads to a temp
// store and a fresh detector for the duration of the test.
func withLearnGlobals(t *testing.T) (*LearnStore, *time.Time) {
	t.Helper()
	s, clock := newTempLearn(t)
	ruDict, _ := LoadDict("ru")
	enDict, _ := LoadDict("en")
	prevLearn, prevDet, prevStore := activeLearn, activeDetector, activeStore
	activeLearn, activeDetector, activeStore = s, NewDetector(ruDict, enDict), nil
	t.Cleanup(func() { activeLearn, activeDetector, activeStore = prevLearn, prevDet, prevStore })
	return s, clock
}

func TestDetectorPeekKeepsContext(t *testing.T) {
	ruDict, _ := LoadDict("ru")
	enDict, _ := LoadDict("en")
	det := NewDetector(ruDict, enDict)
	det.Check("ghbdtn") // Russian context: lastLangRu, recentRu = 1
	type ctx struct {
		lastLangRu, initialized bool
		trailingPunct           rune
		recentRu                int
	}
	snap := func() ctx { return ctx{det.lastLangRu, det.initialized, det.trailingPunct, det.recentRu} }
	before := snap()
	wrong, conv := det.Peek("ns")
	if !wrong || conv != "ты" {
		t.Fatalf("Peek(ns) = %v, %q; want true, ты", wrong, conv)
	}
	if after := snap(); after != before {
		t.Fatalf("Peek changed detector context: %+v → %+v", before, after)
	}
}

func TestLearnManualFlipUndoingDetectorIsRevert(t *testing.T) {
	// The detector turns "ns" into "ты"; the user flipping "ты" → "ns" three
	// times is rejecting that, not asking for a "ты" → "ns" rule (which would
	// then fire on every real "ты").
	s, clock := withLearnGlobals(t)
	for i := 0; i < 3; i++ {
		learnManualFlip("ты", "ns")
		tick(clock, 2*time.Minute)
	}
	if _, ok := s.Rule("ты"); ok {
		t.Fatal("a rule for the real word ты must never be learned")
	}
	var ns *LearnEntry
	for _, e := range s.List() {
		e := e
		if e.Word == "ns" {
			ns = &e
		}
	}
	if ns == nil || ns.Neg != 3 || ns.Rule {
		t.Fatalf("expected three revert signals on ns, got %+v", ns)
	}
}

func TestLearnManualFlipUndoingRuleDemotesIt(t *testing.T) {
	s, clock := withLearnGlobals(t)
	for i := 0; i < 3; i++ {
		s.RecordManualFlip("ты", "ns") // the rule as it was learned before the fix
		tick(clock, 2*time.Minute)
	}
	if _, ok := s.Rule("ты"); !ok {
		t.Fatal("setup: rule ты → ns expected")
	}
	for i := 0; i < 3; i++ {
		learnManualFlip("ns", "ты")
		tick(clock, 2*time.Minute)
	}
	if _, ok := s.Rule("ты"); ok {
		t.Fatal("flipping the rule's output back must demote the rule")
	}
	if _, ok := s.Rule("ns"); ok {
		t.Fatal("no opposite rule ns → ты must appear")
	}
}
