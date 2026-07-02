package main

import "strings"

// abbreviations maps a Russian abbreviation typed by mistake on the US/QWERTY
// layout to its intended Cyrillic form. Keys are the literal characters the
// keyboard produces for that abbreviation on QWERTY (letters swapped, dots kept
// as dots); values are the intended Russian abbreviation.
//
// These are matched as a whole buffered word and MUST be handled before
// looksLikeContext(), which otherwise skips anything containing two dots
// (treating it as a URL/version string). They deliberately preserve the dots
// verbatim — a plain per-letter conversion would turn "." into "ю".
var abbreviations = map[string]string{
	"n.l.": "т.д.", // (и) так далее
	"n.g.": "т.п.", // тому подобное
	"n.t.": "т.е.", // то есть
	"n.r.": "т.к.", // так как
	"lh.":  "др.",  // другое / другие
	"gh.":  "пр.",  // прочее
}

// lookupAbbrev returns the intended Russian abbreviation for a QWERTY-typed word
// and true when word (matched case-insensitively) is a known abbreviation.
func lookupAbbrev(word string) (string, bool) {
	v, ok := abbreviations[strings.ToLower(word)]
	return v, ok
}
