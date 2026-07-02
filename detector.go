package main

import (
	"strings"
	"unicode"
)

// Common single-char Russian words on QWERTY.
// "z"→"я" is deliberately omitted: a lone "z" is common in English/code (axis,
// loop var) and auto-converting it to "я" was a frequent false positive. "я"
// can still be fixed manually with the f18 hotkey.
var singleLetterRu = map[string]string{
	"f": "а", "d": "в", "j": "о", "r": "к",
	"c": "с", "b": "и", "e": "у",
}

// Detector determines if text was typed in the wrong layout
type Detector struct {
	ruDict        *Dict
	enDict        *Dict
	lastLangRu    bool
	initialized   bool
	trailingPunct rune // set when last Check() stripped trailing punct
	// contextAware enables recent-word bias + the impossible-in-English combo
	// fallback (mirrors Config.ContextAware). Set at construction.
	contextAware bool
	// recentRu is a small running tally of the language of recent words:
	// +1 per Russian decision, -1 per English, clamped to [-recentClamp, recentClamp].
	// A positive value means "the user has been writing Russian lately" and is
	// used to gate the borderline impossible-combo conversion.
	recentRu int
}

const recentClamp = 3

func NewDetector(ruDict, enDict *Dict) *Detector {
	return &Detector{ruDict: ruDict, enDict: enDict, contextAware: true}
}

// noteLang records the language of a resolved word into the recent-context tally.
func (d *Detector) noteLang(ru bool) {
	if ru {
		if d.recentRu < recentClamp {
			d.recentRu++
		}
	} else {
		if d.recentRu > -recentClamp {
			d.recentRu--
		}
	}
}

// wrongLayoutByCombo is the context-aware fallback: it fires only after the
// normal dictionary/fuzzy paths failed, for a Latin word whose Russian
// conversion is fully plausible Russian while the word itself is impossible in
// English. Example: "ddj" → "вво" ("dj" is legal English in "adjust", but the
// triple "ddj" never occurs, and "вво" is an attested Russian run as in "ввод").
//
// Guardrails keep false positives near zero:
//   - Latin only, length >= 3.
//   - At least half the word's English trigrams are absent from the English dict.
//   - EVERY trigram of the Russian conversion is attested in the Russian dict,
//     and the conversion contains a vowel.
//   - We're not in an English streak (recentRu >= 0).
func (d *Detector) wrongLayoutByCombo(text string) (string, bool) {
	if !d.contextAware || d.recentRu < 0 {
		return "", false
	}
	runes := []rune(strings.ToLower(text))
	// Restrict to short fragments (3–4 letters). Longer wrong-layout words are
	// reliably caught by the dictionary/fuzzy paths; opening the combo heuristic
	// to them only invites false positives on proper nouns / code identifiers.
	if len(runes) < 3 || len(runes) > 4 {
		return "", false
	}
	for _, r := range runes {
		if r < 'a' || r > 'z' {
			return "", false
		}
	}

	total := len(runes) - 2
	impossible := 0
	for i := 0; i+3 <= len(runes); i++ {
		if !d.enDict.hasTrigram(runes[i], runes[i+1], runes[i+2]) {
			impossible++
		}
	}
	if impossible == 0 || impossible*2 < total {
		return "", false
	}

	conv := QWERTYToRussian(string(runes))
	cr := []rune(conv)
	hasVowel := false
	for _, r := range cr {
		if strings.ContainsRune("аеёиоуыэюя", r) {
			hasVowel = true
			break
		}
	}
	if !hasVowel {
		return "", false
	}
	for i := 0; i+3 <= len(cr); i++ {
		if !d.ruDict.hasTrigram(cr[i], cr[i+1], cr[i+2]) {
			return "", false
		}
	}
	return conv, true
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
			// The last char is ambiguous: a letter ("." = ю → печатаю) or real
			// punctuation (слово.). Strip it as punctuation when the trimmed word
			// is a solid match (valid & >2 chars) AND either:
			//   - it's an EXACT dictionary word (привет. → привет), or
			//   - the FULL word with punct-as-letter isn't valid at all
			//     (ltkf, → "делаб" is nonsense, so "," is a comma → дела,).
			// Keep the full word only when punct-as-letter forms a valid word and
			// the trimmed form is just a stem (gtxfnf. → печатаю).
			if qwertyRuPunct[lastChar] && len([]rune(trimmed)) > 2 && d.ruDict.Has(trimConv) {
				if d.ruDict.words[strings.ToLower(trimConv)] || !inRu {
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
				d.lastLangRu = false
				d.initialized = true
				d.noteLang(false)
				return false, ""
			}
			// Russian word — convert
			d.lastLangRu = true
			d.initialized = true
			d.noteLang(true)
			vlog("Fix: %q → %q (enExact=%v ruLayout=%v)", text, converted, inEnExact, IsRussianLayout())
			return true, converted
		}

		// Fuzzy: if converted word is 1 edit away from a known Russian word,
		// likely a typo in wrong layout. Only for 6+ rune words to avoid false positives.
		if len([]rune(converted)) >= 6 && !inEnExact {
			if corrected, ok := d.ruDict.FuzzyFind(converted); ok {
				d.lastLangRu = true
				d.initialized = true
				d.noteLang(true)
				vlog("Fix (fuzzy): %q → %q (was %q)", text, corrected, converted)
				return true, corrected
			}
		}

		// Context-aware fallback: short fragment impossible in English but
		// plausible Russian (e.g. "ddj" → "вво"). Gated by recent-word context.
		if !inEnExact {
			if conv, ok := d.wrongLayoutByCombo(text); ok {
				d.lastLangRu = true
				d.initialized = true
				d.noteLang(true)
				vlog("Fix (combo): %q → %q", text, conv)
				return true, conv
			}
		}

	case "cyrillic":
		// Already cyrillic and in Russian dict → never touch
		if d.ruDict.Has(text) {
			d.lastLangRu = true
			d.initialized = true
			d.noteLang(true)
			return false, ""
		}
		// Check if it should be English
		converted := RussianToQWERTY(text)
		if d.enDict.Has(converted) {
			d.lastLangRu = false
			d.initialized = true
			d.noteLang(false)
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
