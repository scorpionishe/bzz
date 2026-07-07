package main

import "testing"

// RU→EN positional flips of Russian-layout-only symbols (macOS "Russian"):
// Shift+3 = № / #, Shift+8 = ; / *, the key left of 1 = ] / `.
func TestRuLayoutFlipsRuToEn(t *testing.T) {
	cases := map[string]string{
		"№":        "#",
		";":        "*",
		"]":        "`",
		"№123":     "#123",
		";;":       "**",
		"заказ №5": "pfrfp #5",
		"привет":   "ghbdtn", // untouched by the new mappings
	}
	for in, want := range cases {
		if got := RussianToQWERTY(in); got != want {
			t.Errorf("RussianToQWERTY(%q) = %q, want %q", in, got, want)
		}
	}
}

// The EN→RU direction must be unaffected: ';' and ']' keep their letter
// meaning, '`' stays ё, '#' passes through.
func TestFlipsDoNotLeakIntoEnToRu(t *testing.T) {
	cases := map[string]string{
		";": "ж",
		"]": "ъ",
		"`": "ё",
		"#": "#",
	}
	for in, want := range cases {
		if got := QWERTYToRussian(in); got != want {
			t.Errorf("QWERTYToRussian(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTypedOnRussianLayout(t *testing.T) {
	const (
		kc8         = 0x1C // the "8" key (Shift+8 → ';' on Russian)
		kcGrave     = 0x32 // ANSI key left of 1 (→ ']' on Russian)
		kcISO       = 0x0A // ISO key left of 1
		kcSemicolon = 0x29 // the ';'/ж key
		kcRBracket  = 0x1E // the ']'/ъ key
	)
	cases := []struct {
		pending string
		codes   []uint16
		want    bool
	}{
		{"№", nil, true},                       // № is Russian-only, no keycode needed
		{";", []uint16{kc8}, true},             // Shift+8 on Russian → user wanted '*'
		{";", []uint16{kcSemicolon}, false},    // ';' key on EN → flips to 'ж' as before
		{"]", []uint16{kcGrave}, true},         // key left of 1 on Russian → wanted '`'
		{"]", []uint16{kcISO}, true},           // same key on ISO keyboards
		{"]", []uint16{kcRBracket}, false},     // ']' key on EN → flips to 'ъ' as before
		{";12", []uint16{kc8, 0, 0}, true},     // signature anywhere in the word counts
		{"привет", nil, true},                  // Cyrillic is Russian evidence by itself
		{"ghbdtn", []uint16{4, 11, 2}, false},  // plain Latin, no signatures
	}
	for _, c := range cases {
		if got := typedOnRussianLayout(c.pending, c.codes); got != c.want {
			t.Errorf("typedOnRussianLayout(%q, %v) = %v, want %v", c.pending, c.codes, got, c.want)
		}
	}
}

// № must not be a word boundary: it has to stay in the pending buffer so the
// manual hotkey can flip it.
func TestNumeroSignStaysInBuffer(t *testing.T) {
	var emitted string
	b := NewBuffer(func(w string) { emitted = w })
	b.Add('№', 0x14)
	word, codes := b.FlushWord()
	if word != "№" {
		t.Fatalf("FlushWord word = %q, want %q", word, "№")
	}
	if len(codes) != 1 || codes[0] != 0x14 {
		t.Fatalf("FlushWord codes = %v, want [0x14]", codes)
	}
	if emitted != "" {
		t.Fatalf("unexpected word emit %q", emitted)
	}
}

// Backspace must keep chars and keycodes in sync.
func TestBufferBackspaceKeepsCodesInSync(t *testing.T) {
	b := NewBuffer(nil)
	b.Add(';', 0x1C)
	b.Add('1', 0x12)
	b.Backspace()
	word, codes := b.FlushWord()
	if word != ";" {
		t.Fatalf("FlushWord word = %q, want %q", word, ";")
	}
	if len(codes) != 1 || codes[0] != 0x1C {
		t.Fatalf("FlushWord codes = %v, want [0x1C]", codes)
	}
}
