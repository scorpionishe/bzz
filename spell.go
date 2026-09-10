package main

// spell.go — Russian spelling correction for Cyrillic words the layout
// detector left alone.
//
// The decision "is this word misspelled?" is delegated to a SpellChecker (the
// macOS system checker, NSSpellChecker, on darwin — see spell_darwin.go). It
// knows every inflected form, honours the user's system-wide "Learn Spelling"
// list, and costs nothing in binary size or memory. Its *suggestions*, however,
// are unreliable ("превет" → "прервет", "деплой" → "теплой"), so candidates are
// generated here instead and only confirmed by the checker.
//
// A correction fires only when it is as close to certain as we can get without
// a language model:
//   - the word is letters-only Cyrillic, 4–24 runes, not ALL CAPS;
//   - the checker rejects it AND it is not a frequent exact entry of the
//     embedded frequency dictionary (guards against gaps in the system list);
//   - among the words within ONE edit (missing / extra / wrong / swapped
//     letter) that exist in the embedded dictionary or in the checker's own
//     guesses, and that the checker accepts, there is exactly one — or one
//     that is spellRankRatio times more frequent than any other.
//
// Anything ambiguous is left alone: no correction beats a wrong one. Words the
// checker accepts — including deliberate slang like "нравица" — are never
// touched, and neologisms with no close dictionary neighbour ("поресерчить",
// "ресайз") fall through with no candidates.

import (
	"sort"
	"strings"
	"unicode"
)

// SpellChecker is the oracle for "is this a real word" plus optional guesses.
// Implemented by the system checker (darwin) and by a fake in tests.
type SpellChecker interface {
	// IsCorrect reports whether word is spelled correctly (known to the
	// checker or learned by the user).
	IsCorrect(word string) bool
	// Guesses returns the checker's own suggestions for a misspelled word.
	// May be empty; only 1-edit guesses are used.
	Guesses(word string) []string
}

const (
	spellMinLen = 5  // shorter words have too many 1-edit neighbours and too much slang ("фейл")
	spellMaxLen = 24 // longer "words" are pasted junk, not typing
	// An exact frequency-dictionary entry ranked better than this is trusted
	// as correct even when the system checker flags it.
	spellTrustedRank = 20000
	// With several candidates the best one wins only when the runner-up is at
	// least this many times rarer (an unranked runner-up counts as infinitely
	// rare). Ranks are positions in the frequency list, so "rarer" = larger.
	// 10 keeps "растояние" → "расстояние" (1692) over "настояние" (20756).
	spellRankRatio = 10
	// Guesses beyond this count are ignored — the system returns them in its
	// own confidence order and the tail is noise.
	spellMaxGuesses = 8
)

const cyrillicAlphabet = "абвгдеёжзийклмнопрстуфхцчшщъыьэюя"

// Speller combines the system checker with the embedded frequency dictionary.
type Speller struct {
	checker SpellChecker
	dict    *Dict // Russian dictionary with ranks
}

func NewSpeller(checker SpellChecker, dict *Dict) *Speller {
	if checker == nil || dict == nil {
		return nil
	}
	return &Speller{checker: checker, dict: dict}
}

// spellCandidate reports whether word is something the speller would even look
// at: Cyrillic letters only, sane length, lowercase or Capitalized (ALL CAPS
// and mixed case are skipped — abbreviations, deliberate emphasis).
func spellCandidate(word string) bool {
	r := []rune(word)
	if len(r) < spellMinLen || len(r) > spellMaxLen {
		return false
	}
	for _, c := range r {
		if !unicode.IsLetter(c) || !unicode.Is(unicode.Cyrillic, c) {
			return false
		}
	}
	for _, c := range r[1:] {
		if unicode.IsUpper(c) {
			return false
		}
	}
	return true
}

// capitalize upper-cases the first rune of s.
func capitalize(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// Misspelled is the cheap pre-check run synchronously at the word boundary:
// ~0.1–0.4 ms for a correct word (the common case). It returns true only when
// the system checker rejects the word and the frequency dictionary does not
// vouch for it. A true result does not yet mean a fix exists — see Suggest.
func (s *Speller) Misspelled(word string) bool {
	if s == nil || !spellCandidate(word) {
		return false
	}
	lower := strings.ToLower(word)
	if rank, ok := s.dict.Rank(lower); ok && rank <= spellTrustedRank {
		return false
	}
	// Try the word as typed, lowercase, and Capitalized: "москва" is only a
	// capitalization slip, not a spelling error we should rewrite.
	if s.checker.IsCorrect(word) || s.checker.IsCorrect(lower) || s.checker.IsCorrect(capitalize(lower)) {
		return false
	}
	return true
}

// Suggest returns the single confident correction for a word that Misspelled()
// already flagged. Case of the first letter is preserved. This is the slower
// path (candidate generation + several checker calls, typically 5–50 ms) and is
// meant to run off the event-tap thread.
func (s *Speller) Suggest(word string) (string, bool) {
	if s == nil {
		return "", false
	}
	lower := strings.ToLower(word)

	type cand struct {
		word string
		rank int32 // 0 = not in the frequency dictionary
	}
	seen := map[string]bool{lower: true}
	var cands []cand

	// 1. One-edit neighbours that are exact entries of the frequency dictionary.
	for _, c := range oneEdits(lower) {
		if seen[c] {
			continue
		}
		seen[c] = true
		if rank, ok := s.dict.Rank(c); ok {
			cands = append(cands, cand{c, rank})
		}
	}
	// Only keep neighbours the checker also accepts — the frequency list has
	// junk entries ("програма") that must not become "corrections".
	kept := cands[:0]
	for _, c := range cands {
		if s.checker.IsCorrect(c.word) {
			kept = append(kept, c)
		}
	}
	cands = kept

	// 2. The checker's own guesses, if they are one edit away. These cover
	// inflected forms missing from the frequency list; they are correct by
	// definition, so no second IsCorrect call.
	for i, g := range s.checker.Guesses(lower) {
		if i >= spellMaxGuesses {
			break
		}
		gl := strings.ToLower(g)
		if seen[gl] || !isOneEdit(lower, gl) {
			continue
		}
		seen[gl] = true
		rank, _ := s.dict.Rank(gl)
		cands = append(cands, cand{gl, rank})
	}

	if len(cands) == 0 {
		return "", false
	}
	var best cand
	if len(cands) == 1 {
		best = cands[0]
	} else {
		// Ranked (known-frequency) candidates first, most frequent first;
		// unranked ones last.
		sort.SliceStable(cands, func(i, j int) bool {
			ri, rj := cands[i].rank, cands[j].rank
			if ri == 0 {
				return false
			}
			if rj == 0 {
				return true
			}
			return ri < rj
		})
		best = cands[0]
		second := cands[1]
		if best.rank == 0 {
			return "", false // nothing to prefer between unranked candidates
		}
		if second.rank != 0 && second.rank < best.rank*spellRankRatio {
			return "", false // too close to call
		}
	}

	fixed := best.word
	if unicode.IsUpper([]rune(word)[0]) {
		fixed = capitalize(fixed)
	}
	if fixed == word {
		return "", false
	}
	return fixed, true
}

// Fix is Misspelled + Suggest in one call, for the CLI and tests.
func (s *Speller) Fix(word string) (string, bool) {
	if !s.Misspelled(word) {
		return "", false
	}
	return s.Suggest(word)
}

// oneEdits generates every string within one Damerau edit of word over the
// Cyrillic alphabet: deletions, adjacent transpositions, substitutions and
// insertions. Duplicates are possible; callers dedupe.
func oneEdits(word string) []string {
	r := []rune(word)
	n := len(r)
	alpha := []rune(cyrillicAlphabet)
	out := make([]string, 0, n*(2*len(alpha)+2)+len(alpha))

	// deletions
	for i := 0; i < n; i++ {
		out = append(out, string(r[:i])+string(r[i+1:]))
	}
	// transpositions
	for i := 0; i+1 < n; i++ {
		if r[i] == r[i+1] {
			continue
		}
		t := append([]rune(nil), r...)
		t[i], t[i+1] = t[i+1], t[i]
		out = append(out, string(t))
	}
	// substitutions
	for i := 0; i < n; i++ {
		for _, a := range alpha {
			if a == r[i] {
				continue
			}
			t := append([]rune(nil), r...)
			t[i] = a
			out = append(out, string(t))
		}
	}
	// insertions
	for i := 0; i <= n; i++ {
		for _, a := range alpha {
			out = append(out, string(r[:i])+string(a)+string(r[i:]))
		}
	}
	return out
}

// isOneEdit reports whether b is exactly one Damerau edit (insert, delete,
// substitute, adjacent transpose) away from a.
func isOneEdit(a, b string) bool {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	switch {
	case la == lb:
		diff := -1
		for i := 0; i < la; i++ {
			if ra[i] != rb[i] {
				if diff == -1 {
					diff = i
					continue
				}
				// Second difference: only an adjacent swap is allowed, and
				// everything after it must match.
				if i != diff+1 || ra[diff] != rb[i] || ra[i] != rb[diff] {
					return false
				}
				for j := i + 1; j < la; j++ {
					if ra[j] != rb[j] {
						return false
					}
				}
				return true
			}
		}
		return diff != -1
	case la+1 == lb:
		return isDeletion(rb, ra)
	case lb+1 == la:
		return isDeletion(ra, rb)
	}
	return false
}

// isDeletion reports whether short equals long with exactly one rune removed.
func isDeletion(long, short []rune) bool {
	i, j := 0, 0
	skipped := false
	for i < len(long) {
		if j < len(short) && long[i] == short[j] {
			i++
			j++
			continue
		}
		if skipped {
			return false
		}
		skipped = true
		i++
	}
	return j == len(short)
}
