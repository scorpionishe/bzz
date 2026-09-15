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
		"зоказов":     "заказов",
		"будующий":    "будущий",
		"Масква":      "Москва", // proper noun: the checker lists it Capitalized only (#27)
		"Игарь":       "Игорь",
		// #28: the stem-rank floor keeps "разо"/"пота" from outranking the real word
		"фразо": "фраза",
		"потак": "поток",
	}
	for w, want := range fixes {
		if got, ok := sp.Fix(w); !ok || got != want {
			t.Errorf("Fix(%q) = (%q, %v), want (%q, true)", w, got, ok, want)
		}
	}
	untouched := []string{"привет", "сделаю", "нравится", "нравица", "поресерчить", "ресайз", "москва", "Москва", "СССР", "кот",
		// #23: inflected forms the system dictionary lacks
		"трафика", "виртуальной", "серверных", "Димке", "гарантированно", "таймауты", "столбцы",
		// #24: anglicisms from dicts/ru_extra.txt
		"бакет", "промта", "токена", "хуков", "чеклист",
		// #28: ranking stand-offs must not pick the wrong word
		"дайти", "юнаша",
		// names the checker does not know and no confident fix exists
		"Наташка", "Дудь", "Трёхступенчатый",
		// #27: a proper noun never wins for a lowercase word or a rare entry
		"масква", "Вигерс", "алиас",
		// a rare list entry the system dictionary lacks; "подлодка" is only twice as frequent
		"подложка",
	}
	for _, w := range untouched {
		if got, ok := sp.Fix(w); ok {
			t.Errorf("Fix(%q) = %q, want untouched", w, got)
		}
	}
}
