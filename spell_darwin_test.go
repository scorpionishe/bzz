//go:build darwin

package main

import "testing"

// TestSpellSystemChecker exercises the real NSSpellChecker + embedded
// dictionary. Results depend on the macOS Russian dictionary, so only the
// stable cases are asserted.
func TestSpellSystemChecker(t *testing.T) {
	checker, err := newSystemSpellChecker("ru")
	if err != nil {
		t.Skip(err)
	}
	dict, err := LoadDict("ru")
	if err != nil {
		t.Fatal(err)
	}
	sp := NewSpeller(checker, dict)
	fixes := map[string]string{
		"колличество": "количество",
		"инжинер":     "инженер",
		"симпотичный": "симпатичный",
		"растояние":   "расстояние",
		"Инжинер":     "Инженер",
	}
	for w, want := range fixes {
		if got, ok := sp.Fix(w); !ok || got != want {
			t.Errorf("Fix(%q) = (%q, %v), want (%q, true)", w, got, ok, want)
		}
	}
	untouched := []string{"привет", "сделаю", "нравится", "нравица", "поресерчить", "ресайз", "москва", "Москва", "СССР", "кот"}
	for _, w := range untouched {
		if got, ok := sp.Fix(w); ok {
			t.Errorf("Fix(%q) = %q, want untouched", w, got)
		}
	}
}
