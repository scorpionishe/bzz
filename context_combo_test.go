package main

import "testing"

// TestWrongLayoutByCombo checks the context-aware impossible-in-English fallback.
func TestWrongLayoutByCombo(t *testing.T) {
	ruDict, _ := LoadDict("ru")
	enDict, _ := LoadDict("en")
	det := NewDetector(ruDict, enDict)
	det.recentRu = 2 // simulate a recent Russian streak

	// Positive: "ddj" is impossible in English, "вво" is an attested Russian run.
	if conv, ok := det.wrongLayoutByCombo("ddj"); !ok || conv != "вво" {
		t.Errorf("wrongLayoutByCombo(ddj) = (%q, %v), want (вво, true)", conv, ok)
	}

	// Negatives that must NOT fire:
	neg := []string{
		"cgg",  // conversion "спп" has no vowel
		"the",  // legal English trigram
		"json", // >4 chars anyway, and conversion implausible
		"cat",  // legal English
		"ab",   // too short
		"test", // legal English word
	}
	for _, w := range neg {
		if conv, ok := det.wrongLayoutByCombo(w); ok {
			t.Errorf("wrongLayoutByCombo(%q) fired unexpectedly → %q", w, conv)
		}
	}

	// Context gate: in an English streak the fallback stays silent.
	det.recentRu = -1
	if _, ok := det.wrongLayoutByCombo("ddj"); ok {
		t.Errorf("wrongLayoutByCombo(ddj) fired during an English streak")
	}
}

// TestContextTallyClamp verifies recentRu tracking stays bounded.
func TestContextTallyClamp(t *testing.T) {
	det := NewDetector(nil, nil)
	for i := 0; i < 10; i++ {
		det.noteLang(true)
	}
	if det.recentRu != recentClamp {
		t.Errorf("recentRu = %d, want clamp %d", det.recentRu, recentClamp)
	}
	for i := 0; i < 20; i++ {
		det.noteLang(false)
	}
	if det.recentRu != -recentClamp {
		t.Errorf("recentRu = %d, want -clamp %d", det.recentRu, -recentClamp)
	}
}
