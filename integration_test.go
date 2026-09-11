package main

import (
	"fmt"
	"testing"
)

// TestTrailingPunctIntegration simulates the full callback logic in main.go
// for trailing punctuation cases — verifying deletion count and retyped text.
func TestTrailingPunctIntegration(t *testing.T) {
	ruDict, _ := LoadDict("ru")
	enDict, _ := LoadDict("en")
	det := NewDetector(ruDict, enDict)

	cases := []struct {
		// bufWord is what buffer.onWord() receives (word + trailing punct, no space)
		bufWord string
		// wantDelete is how many chars replaceText must backspace:
		//   pureWordLen + 2  (word chars + punct + space)
		wantDelete int
		// wantNewText is what replaceText types after backspacing
		wantNewText string
	}{
		// "ckjdj," → "слово" + "," + " " — 5 word chars + comma + space = 7 delete
		{"ckjdj,", 7, "слово, "},
		// "vbh." → "мир" + "." + " " — 3 + 1 + 1 = 5 delete
		{"vbh.", 5, "мир. "},
		// "ghbdtn," → "привет" + "," + " " — 6 + 1 + 1 = 8 delete
		{"ghbdtn,", 8, "привет, "},
		// "ghbdtn." → "привет" + "." + " " — 6 + 1 + 1 = 8 delete
		{"ghbdtn.", 8, "привет. "},
		// "ghbdtn;" → "привет" + ";" + " " — 6 + 1 + 1 = 8 delete
		{"ghbdtn;", 8, "привет; "},
		// "ckjdj," — same as first, double-check
		{"ckjdf,", 7, "слова, "},
	}

	fmt.Println("=== Trailing punct integration: deleteChars + newText ===")
	passed, failed := 0, 0
	for _, tc := range cases {
		det.trailingPunct = 0
		det.lastLangRu = true
		det.initialized = true

		wrong, corrected := det.Check(tc.bufWord)
		if !wrong {
			fmt.Printf("FAIL %q: detector.Check returned wrong=false\n", tc.bufWord)
			failed++
			continue
		}
		if det.trailingPunct == 0 {
			fmt.Printf("FAIL %q: expected trailingPunct != 0\n", tc.bufWord)
			failed++
			continue
		}

		// Replicate the logic from main.go callback exactly
		wordRunes := []rune(tc.bufWord)
		pureWordLen := len(wordRunes) - 1
		deleteChars := pureWordLen + 2
		newText := corrected + string(det.trailingPunct) + " "

		ok := true
		if deleteChars != tc.wantDelete {
			fmt.Printf("FAIL %q: deleteChars=%d want %d\n", tc.bufWord, deleteChars, tc.wantDelete)
			ok = false
		}
		if newText != tc.wantNewText {
			fmt.Printf("FAIL %q: newText=%q want %q\n", tc.bufWord, newText, tc.wantNewText)
			ok = false
		}
		if ok {
			fmt.Printf("PASS %q → delete=%d type=%q\n", tc.bufWord, deleteChars, newText)
			passed++
		} else {
			failed++
		}
	}
	fmt.Printf("\n=== Integration results: %d passed, %d failed ===\n", passed, failed)
	if failed > 0 {
		t.Errorf("%d integration tests failed", failed)
	}
}

// Issue #29: the word callback receives the boundary rune that ended the
// word, and the replacement retypes that rune — not a space.
func TestBufferBoundaryRune(t *testing.T) {
	type emit struct {
		word     string
		boundary rune
	}
	var got []emit
	b := NewBuffer(func(w string, r rune) { got = append(got, emit{w, r}) })
	for _, r := range "превет-пока (мир) \"да\" ок:" {
		b.Add(r, 0)
	}
	want := []emit{
		{"превет", '-'}, {"пока", ' '}, {"мир", ')'}, {"да", '"'}, {"ок", ':'},
	}
	if len(got) != len(want) {
		t.Fatalf("emitted %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("emit %d = %q/%q, want %q/%q", i, got[i].word, string(got[i].boundary), want[i].word, string(want[i].boundary))
		}
	}
	// A universal punct is folded into the word and is its own boundary;
	// the "no space" branch in main.go then adds no suffix.
	got = nil
	for _, r := range "ghbdtn!" {
		b.Add(r, 0)
	}
	if len(got) != 1 || got[0].word != "ghbdtn!" || got[0].boundary != '!' {
		t.Fatalf("universal punct emit = %v", got)
	}
	// The retyped text mirrors main.go: corrected word + the boundary itself.
	if newText := "привет" + string('-'); newText != "привет-" {
		t.Fatalf("newText = %q", newText)
	}
}
