package main

import (
	"fmt"
	"testing"
)

func TestShiftedPunct(t *testing.T) {
	ruDict, _ := LoadDict("ru")
	enDict, _ := LoadDict("en")
	det := NewDetector(ruDict, enDict)

	cases := []struct {
		input     string
		wantPunct rune
	}{
		{"ckjdj^", ','},   // Shift+6 → comma
		{"ckjdj&", '.'},   // Shift+7 → period
		{"vbh$", ';'},     // Shift+4 → semicolon
		{"ghbdtn,", ','},  // regular comma stays as comma
		{"ghbdtn.", '.'},  // regular period stays as period
	}

	for _, tc := range cases {
		det.lastLangRu = true
		det.initialized = true
		wrong, corrected := det.Check(tc.input)
		if !wrong {
			fmt.Printf("FAIL %q not detected\n", tc.input)
			t.Fail()
			continue
		}
		if det.trailingPunct != tc.wantPunct {
			fmt.Printf("FAIL %q: trailingPunct=%c want=%c (corrected=%q)\n", tc.input, det.trailingPunct, tc.wantPunct, corrected)
			t.Fail()
		} else {
			fmt.Printf("OK %q → %q + %c\n", tc.input, corrected, det.trailingPunct)
		}
	}
}
