package main

import (
	"strings"
	"unicode"
)

// Common single-char Russian words on QWERTY
var singleLetterRu = map[string]string{
	"f": "а", "d": "в", "j": "о", "r": "к",
	"c": "с", "z": "я", "b": "и", "e": "у",
}

// Detector determines if text was typed in the wrong layout
type Detector struct {
	ruDict        *Dict
	enDict        *Dict
	lastLangRu    bool
	initialized   bool
	trailingPunct rune // set when last Check() stripped trailing punct
}

func NewDetector(ruDict, enDict *Dict) *Detector {
	return &Detector{ruDict: ruDict, enDict: enDict}
}

// Check returns (true, corrected) if wrong layout detected
func (d *Detector) Check(text string) (wrong bool, corrected string) {
	d.trailingPunct = 0
	runes := []rune(text)

	// Single-letter words: use context + known Russian letters
	if len(runes) == 1 {
		if conv, ok := singleLetterRu[text]; ok && (d.lastLangRu || !d.initialized) {
			d.lastLangRu = true
			d.initialized = true
			return true, conv
		}
		return false, ""
	}

	script := detectScript(text)

	switch script {
	case "latin":
		converted := QWERTYToRussian(text)
		inRu := d.ruDict.Has(converted)
		// Only check EXACT match in English (not stemmed) to avoid false positives
		inEnExact := d.enDict.words[strings.ToLower(text)]

		// Trailing punct: two types
		// 1. QWERTY punct-as-letter: "," "." ";" "'" → could be б ю ж э
		// 2. Shifted number keys: "^" "&" "$" "@" → Russian , . ; "
		if len(runes) > 2 {
			lastChar := runes[len(runes)-1]
			trimmed := string(runes[:len(runes)-1])
			trimConv := QWERTYToRussian(trimmed)

			// Type 2: Shifted Russian punct (^ & $ @)
			// These are ALWAYS punctuation, never letters
			if ruPunct, ok := shiftedRuPunct[lastChar]; ok {
				if d.ruDict.Has(trimConv) {
					d.lastLangRu = true
					d.initialized = true
					d.trailingPunct = ruPunct
					return true, trimConv
				}
			}

			// Type 3: Universal punct (! ?) — same key on both layouts
			if universalPunct[lastChar] {
				if d.ruDict.Has(trimConv) {
					d.lastLangRu = true
					d.initialized = true
					d.trailingPunct = lastChar // keep as-is (! = !)
					return true, trimConv
				}
			}

			// Type 1: QWERTY punct-as-letter (, . ; ')
			// Could be letter (себя) or punctuation (слово,)
			// Prefer trim if word is valid and longer than 2 chars
			if qwertyRuPunct[lastChar] {
				if d.ruDict.Has(trimConv) && (!inRu || len([]rune(trimmed)) <= 2) {
					if !inRu {
						d.lastLangRu = true
						d.initialized = true
						d.trailingPunct = lastChar
						return true, trimConv
					}
				} else if d.ruDict.Has(trimConv) && len([]rune(trimmed)) > 2 {
					d.lastLangRu = true
					d.initialized = true
					d.trailingPunct = lastChar
					return true, trimConv
				}
			}
		}

		if inRu {
			if inEnExact && !IsRussianLayout() {
				// Real English word typed on EN layout — don't touch
				// e.g. "if", "the", "and", "no"
				return false, ""
			}
			// Russian word — convert
			d.lastLangRu = true
			d.initialized = true
			vlog("Fix: %q → %q (enExact=%v ruLayout=%v)", text, converted, inEnExact, IsRussianLayout())
			return true, converted
		}

		// Fuzzy: if converted word is 1 edit away from a known Russian word,
		// likely a typo in wrong layout. Only for 6+ rune words to avoid false positives.
		if len([]rune(converted)) >= 6 && !inEnExact {
			if corrected, ok := d.ruDict.FuzzyFind(converted); ok {
				d.lastLangRu = true
				d.initialized = true
				vlog("Fix (fuzzy): %q → %q (was %q)", text, corrected, converted)
				return true, corrected
			}
		}

	case "cyrillic":
		// Already cyrillic and in Russian dict → never touch
		if d.ruDict.Has(text) {
			d.lastLangRu = true
			d.initialized = true
			return false, ""
		}
		// Check if it should be English
		converted := RussianToQWERTY(text)
		if d.enDict.Has(converted) {
			d.lastLangRu = false
			d.initialized = true
			return true, converted
		}
	}

	return false, ""
}

func detectScript(text string) string {
	for _, r := range text {
		if unicode.Is(unicode.Cyrillic, r) {
			return "cyrillic"
		}
		if unicode.Is(unicode.Latin, r) {
			return "latin"
		}
	}
	return "unknown"
}
