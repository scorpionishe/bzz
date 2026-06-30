package main

import (
	"bufio"
	"embed"
	"strings"

	"github.com/kljensen/snowball/english"
	"github.com/kljensen/snowball/russian"
)

//go:embed dicts/*.txt
var dictsFS embed.FS

// Dict holds words and their stems for fast lookup
type Dict struct {
	words map[string]bool
	stems map[string]bool
	lang  string
}

func LoadDict(lang string) (*Dict, error) {
	path := "dicts/" + lang + "_freq.txt"
	file, err := dictsFS.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	d := &Dict{
		words: make(map[string]bool, 100000),
		stems: make(map[string]bool, 50000),
		lang:  lang,
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if word == "" || strings.HasPrefix(word, "#") {
			continue
		}
		d.words[word] = true
		if stem := stemWord(word, lang); stem != "" {
			d.stems[stem] = true
		}
	}
	return d, scanner.Err()
}

// Russian verb/adj suffixes to strip for fuzzy matching
var ruSuffixes = []string{
	// past tense
	"нул", "нула", "нули", "нуло",
	"ал", "ала", "али", "ало",
	"ел", "ела", "ели", "ело",
	"ил", "ила", "или", "ило",
	"ял", "яла", "яли", "яло",
	// present/future
	"ает", "яет", "ует", "ёт", "ет",
	"аю", "яю", "ую", "жу", "шу", "чу", "щу",
	"у",
	"ают", "яют", "уют",
	"аешь", "яешь", "уешь", "ешь", "ишь",
	// imperative
	"ай", "яй", "уй", "ь", "и", "ите",
	// short forms
	"ешь", "ишь",
	// participle/gerund
	"ая", "яя",
	// adjective
	"ого", "его",
	"ому", "ему",
	"ой", "ей", "ий", "ый",
	"ые", "ие",
	"ых", "их",
	// noun cases
	"ом", "ем", "ам", "ям",
	"ов", "ев",
	"ах", "ях",
	"ами", "ями",
}

// Russian consonant alternations: ж↔з, ш↔с, ч↔к, щ↔ст, д↔ж, т↔ч
var consonantAlts = map[string][]string{
	"ж": {"з", "г", "д"},
	"ш": {"с", "х"},
	"ч": {"к", "ц"},
	"щ": {"ст", "ск"},
	"з": {"ж"},
	"г": {"ж"},
	"д": {"ж"},
	"с": {"ш"},
	"к": {"ч"},
	"ц": {"ч"},
}

// alternateConsonants returns the original root plus variants with swapped trailing consonant
func alternateConsonants(root string) []string {
	runes := []rune(root)
	if len(runes) < 2 {
		return []string{root}
	}
	results := []string{root}
	last := string(runes[len(runes)-1])
	prefix := string(runes[:len(runes)-1])

	if alts, ok := consonantAlts[last]; ok {
		for _, alt := range alts {
			results = append(results, prefix+alt)
		}
	}
	// Also try 2-char ending for щ↔ст
	if len(runes) >= 3 {
		last2 := string(runes[len(runes)-2:])
		prefix2 := string(runes[:len(runes)-2])
		if alts, ok := consonantAlts[last2]; ok {
			for _, alt := range alts {
				results = append(results, prefix2+alt)
			}
		}
	}
	return results
}

// Has checks exact match, stem match, or fuzzy suffix match
func (d *Dict) Has(word string) bool {
	lower := strings.ToLower(word)
	if d.words[lower] {
		return true
	}
	// Snowball stem
	if stem := stemWord(lower, d.lang); stem != "" && d.stems[stem] {
		return true
	}
	// Fuzzy: strip common suffixes and check if root+ть or root exists
	if d.lang == "ru" {
		for _, suf := range ruSuffixes {
			if strings.HasSuffix(lower, suf) {
				root := lower[:len(lower)-len(suf)]
				if len([]rune(root)) < 2 {
					continue
				}
				// Try root + infinitive endings, including consonant alternations
				roots := alternateConsonants(root)
				for _, r := range roots {
					for _, inf := range []string{"ть", "ать", "ять", "ить", "еть", "уть", "нуть", "овать", "зать", "сать"} {
						if d.words[r+inf] {
							return true
						}
					}
					// Check root as noun/adj base
					if d.words[r] || d.words[r+"а"] || d.words[r+"о"] || d.words[r+"ь"] || d.words[r+"ка"] {
						return true
					}
				}
			}
		}
	}
	return false
}

// solidWord reports whether word is a confident dictionary match: an exact entry
// or a snowball-stem match. It deliberately excludes Has()'s looser suffix-root +
// consonant-alternation heuristic, which combined with 1-edit guessing turned
// real Latin words into nonsense Cyrillic — e.g. "vscode"→"мысщву"→"мысщу", where
// "мысщу" only "matched" by стрипая "щу" and alternating с→ш onto "мышь".
func (d *Dict) solidWord(word string) bool {
	lower := strings.ToLower(word)
	if d.words[lower] {
		return true
	}
	if stem := stemWord(lower, d.lang); stem != "" && d.stems[stem] {
		return true
	}
	return false
}

// FuzzyFind returns the closest solid dictionary word within 1 edit distance.
// It accepts only solidWord() matches (exact or stem), not Has()'s loose suffix
// heuristic — see solidWord. Returns ("", false) if none.
func (d *Dict) FuzzyFind(word string) (string, bool) {
	runes := []rune(word)
	n := len(runes)
	alphabet := []rune("абвгдеёжзийклмнопрстуфхцчшщъыьэюя")

	// 1. Insertions first (most common typo = missed a key)
	for i := 0; i <= n; i++ {
		prefix := string(runes[:i])
		suffix := string(runes[i:])
		for _, ch := range alphabet {
			candidate := prefix + string(ch) + suffix
			if d.solidWord(candidate) {
				return candidate, true
			}
		}
	}

	// 2. Substitutions: wrong key pressed
	for i := 0; i < n; i++ {
		orig := runes[i]
		prefix := string(runes[:i])
		suffix := string(runes[i+1:])
		for _, ch := range alphabet {
			if ch == orig {
				continue
			}
			candidate := prefix + string(ch) + suffix
			if d.solidWord(candidate) {
				return candidate, true
			}
		}
	}

	// 3. Deletions last: extra key pressed
	for i := 0; i < n; i++ {
		candidate := string(runes[:i]) + string(runes[i+1:])
		if d.solidWord(candidate) {
			return candidate, true
		}
	}

	return "", false
}

func stemWord(word, lang string) string {
	switch lang {
	case "ru":
		return russian.Stem(word, true)
	case "en":
		return english.Stem(word, true)
	}
	return ""
}
