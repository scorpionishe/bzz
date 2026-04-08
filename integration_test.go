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
