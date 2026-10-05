package main

import (
	"testing"
	"time"
)

// A word flipped with the hotkey mid-way stays one word in the buffer: the
// rest is appended to the flipped part and the boundary emits it whole,
// marked with what was typed before the flip.
func TestBufferSeedKeepsFlippedWordWhole(t *testing.T) {
	var word, orig string
	b := NewBuffer(func(w string, _ rune, o string) { word, orig = w, o })
	for _, r := range "щер" {
		b.Add(r, 0)
	}
	pending, codes, flip := b.FlushFlipped()
	if pending != "щер" || flip != "" {
		t.Fatalf("FlushFlipped = %q, %q; want щер, \"\"", pending, flip)
	}
	b.Seed("oth", codes, pending)
	for _, r := range "er " {
		b.Add(r, 0)
	}
	if word != "other" || orig != "щер" {
		t.Fatalf("emitted %q (flipOrig %q); want other (щер)", word, orig)
	}

	// The mark lives only as long as the word.
	b.Seed("oth", nil, "щер")
	b.Clear()
	for _, r := range "er " {
		b.Add(r, 0)
	}
	if word != "er" || orig != "" {
		t.Fatalf("after Clear emitted %q (flipOrig %q); want er, no mark", word, orig)
	}
	b.Seed("o", nil, "щ")
	b.Backspace()
	for _, r := range "er " {
		b.Add(r, 0)
	}
	if orig != "" {
		t.Fatalf("mark survived backspacing the word away: %q", orig)
	}
}

func TestFinishFlippedWord(t *testing.T) {
	cases := []struct{ word, orig, want string }{
		{"other", "щер", "other"},   // switch mode: rest typed on the new layout
		{"othук", "щер", "other"},   // rest typed on the old layout
		{"cleфк", "сду", "clear"},   // the "cleфк" from the report
		{"Othук!", "Щер", "Other!"}, // case and trailing "!" kept
		{"приdtn", "ghb", "привет"}, // EN → RU the same way
		{"привет", "ghb", "привет"},
	}
	for _, c := range cases {
		if got := finishFlippedWord(c.word, c.orig); got != c.want {
			t.Errorf("finishFlippedWord(%q, %q) = %q, want %q", c.word, c.orig, got, c.want)
		}
	}
}

// Learning sees the finished word, not the fragment flipped mid-way: three
// times "сду" + hotkey + "ar" teach "сдуфк" → "clear", never "сду" → "cle".
func TestLearnFlippedWordLearnsWholeWord(t *testing.T) {
	s, clock := withLearnGlobals(t)
	for i := 0; i < 3; i++ {
		learnFlippedWord("сду", "clear")
		tick(clock, 2*time.Minute)
	}
	if conv, ok := s.Rule("сдуфк"); !ok || conv != "clear" {
		t.Fatalf("Rule(сдуфк) = %q, %v; want clear, true", conv, ok)
	}
	if _, ok := s.Rule("сду"); ok {
		t.Fatal("the fragment сду must not become a rule")
	}
	// Mixed script (rest left unconverted) teaches nothing.
	learnFlippedWord("щер", "othук")
	for _, e := range s.List() {
		if e.Word == "щерук" || e.Word == "othук" || e.Word == "щер" {
			t.Fatalf("unexpected entry %+v", e)
		}
	}
}
