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
//   - candidates pass an error model (editTier): a typical Russian slip —
//     unstressed-vowel or voicing confusion (а/о, е/и, д/т), a swap, a
//     doubled or missed double letter, a neighbouring key — counts at face
//     value, a missed or extra letter is weighed as spellTypoPenalty times
//     less likely, and a substitution nobody makes by accident is never a
//     correction ("зоказов" is "заказов", not "показов"). Inflected forms the
//     frequency list lacks ("заказов", "товары") are admitted and ranked
//     through their stem, capped at spellStemRankCap so obscure inflections
//     of rare words cannot sneak in.
//   - IT anglicisms and chat slang from dicts/ru_extra.txt (matched by stem)
//     are never touched, whatever the system checker thinks of them.
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
	// A missed or extra letter (tierTypo) is this many times less likely than
	// a typical slip (tierTypical) when candidates are compared by frequency.
	spellTypoPenalty = 12
	// Inflected forms known only through their stem must have a stem this
	// frequent; rarer lemmas produce too many accidental forms ("комит").
	spellStemRankCap = 5000
	// Candidates this many times rarer than the most frequent one are dropped
	// before the error model votes ("бузующий" cannot outvote "будущий").
	spellRareDrop = spellRankRatio
)

const cyrillicAlphabet = "абвгдеёжзийклмнопрстуфхцчшщъыьэюя"

// Orthographic confusions: letters Russians mix up because they sound alike in
// the given position. Both directions are typical.
var confusablePairs = map[[2]rune]bool{}

func init() {
	pairs := []string{"ао", "еи", "ея", "иы", "еэ", "ёе", "ёо", "бп", "вф", "гк", "дт", "жш", "зс", "ьъ", "цт", "чш", "щш"}
	for _, p := range pairs {
		r := []rune(p)
		confusablePairs[[2]rune{r[0], r[1]}] = true
		confusablePairs[[2]rune{r[1], r[0]}] = true
	}
	rows := []string{"qwertyuiop[]", "asdfghjkl;'", "zxcvbnm,./"}
	ru := make([][]rune, len(rows))
	for i, row := range rows {
		for _, k := range row {
			ru[i] = append(ru[i], enToRu[k])
		}
	}
	add := func(a, b rune) {
		keyNeighbours[[2]rune{a, b}] = true
		keyNeighbours[[2]rune{b, a}] = true
	}
	for r, row := range ru {
		for i := range row {
			if i+1 < len(row) {
				add(row[i], row[i+1])
			}
			// The next row is shifted half a key to the right, so key i sits
			// between keys i-1 and i of the row below.
			if r+1 < len(ru) {
				for _, j := range []int{i - 1, i} {
					if j >= 0 && j < len(ru[r+1]) {
						add(row[i], ru[r+1][j])
					}
				}
			}
		}
	}
}

// keyNeighbours holds physically adjacent keys of the ЙЦУКЕН layout.
var keyNeighbours = map[[2]rune]bool{}

// Edit tiers, lower = more typical for a Russian typist.
const (
	tierTypical  = 1 // confusable letters, adjacent keys, swap, doubled/missed double, ь/ъ, hyphen
	tierTypo     = 2 // a missed or an extra letter
	tierUnlikely = 3 // a substitution nobody makes by accident
)

// editTier classifies the single edit turning typed into cand.
func editTier(typed, cand string) int {
	a, b := []rune(typed), []rune(cand)
	switch {
	case len(a) == len(b):
		i := 0
		for i < len(a) && a[i] == b[i] {
			i++
		}
		if i >= len(a) {
			return tierUnlikely
		}
		if i+1 < len(a) && a[i] == b[i+1] && a[i+1] == b[i] {
			return tierTypical // transposition
		}
		if confusablePairs[[2]rune{a[i], b[i]}] || keyNeighbours[[2]rune{a[i], b[i]}] {
			return tierTypical
		}
		return tierUnlikely
	case len(a) == len(b)+1:
		// typed has an extra rune at i
		i := 0
		for i < len(b) && a[i] == b[i] {
			i++
		}
		x := a[i]
		if x == 'ь' || x == 'ъ' || (i > 0 && a[i-1] == x) || (i+1 < len(a) && a[i+1] == x) {
			return tierTypical // doubled letter or stray soft sign
		}
		return tierTypo
	case len(b) == len(a)+1:
		// typed lacks the rune b[i]
		i := 0
		for i < len(a) && a[i] == b[i] {
			i++
		}
		y := b[i]
		if y == 'ь' || y == 'ъ' || y == '-' || (i > 0 && b[i-1] == y) || (i+1 < len(b) && b[i+1] == y) {
			return tierTypical // missed double letter, soft sign or hyphen
		}
		return tierTypo
	}
	return tierUnlikely
}

// spellCandidate describes one correction candidate, for Suggest and Explain.
type spellCand struct {
	Word string
	Rank int32 // exact or stem rank, 0 = unknown
	Tier int
}

// Speller combines the system checker with the embedded frequency dictionary.
type Speller struct {
	checker SpellChecker
	dict    *Dict           // Russian dictionary with ranks
	known   map[string]bool // stems of dicts/ru_extra.txt — never corrected
}

func NewSpeller(checker SpellChecker, dict *Dict) *Speller {
	if checker == nil || dict == nil {
		return nil
	}
	known, err := LoadStemSet("ru_extra", "ru")
	if err != nil {
		known = map[string]bool{}
	}
	return &Speller{checker: checker, dict: dict, known: known}
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
	if s.known[lower] || s.known[stemWord(lower, "ru")] {
		return false // anglicism / slang the user types on purpose
	}
	// Try the word as typed, lowercase, and Capitalized: "москва" is only a
	// capitalization slip, not a spelling error we should rewrite.
	if s.checker.IsCorrect(word) || s.checker.IsCorrect(lower) || s.checker.IsCorrect(capitalize(lower)) {
		return false
	}
	return true
}

// Candidates lists every admissible one-edit correction for a (lowercase)
// word — dictionary neighbours (exact or by stem) confirmed by the checker,
// plus the checker's hyphenated guesses — sorted by likelihood: frequency rank
// with the tierTypo penalty applied (see Suggest).
func (s *Speller) Candidates(lower string) []spellCand {
	seen := map[string]bool{lower: true}
	var cands []spellCand

	// 1. One-edit neighbours known to the frequency dictionary — exactly, or
	// through their stem so inflected forms the list lacks ("заказов",
	// "товары") still count — and accepted by the checker (the list has junk
	// entries like "програма" that must not become "corrections"). Ranked
	// through the stem as well.
	for _, c := range oneEdits(lower) {
		if seen[c] {
			continue
		}
		seen[c] = true
		tier := editTier(lower, c)
		if tier == tierUnlikely {
			continue
		}
		rank, ok := s.dict.RankLoose(c)
		if !ok {
			continue
		}
		if _, exact := s.dict.Rank(c); !exact && rank > spellStemRankCap {
			continue
		}
		if !s.checker.IsCorrect(c) {
			continue
		}
		cands = append(cands, spellCand{c, rank, tier})
	}

	// 2. The checker's own guesses, only for edits oneEdits cannot produce —
	// a missing hyphen ("чтото" → "что-то"). Letter-edit guesses are skipped
	// on purpose: they are full of obscure words ("тавары" → "авары",
	// "конфиг" → "контиг") that our dictionary gate is there to exclude.
	for i, g := range s.checker.Guesses(lower) {
		if i >= spellMaxGuesses {
			break
		}
		gl := strings.ToLower(g)
		if seen[gl] || !strings.ContainsRune(gl, '-') || !isOneEdit(lower, gl) {
			continue
		}
		seen[gl] = true
		rank, _ := s.dict.RankLoose(gl)
		cands = append(cands, spellCand{gl, rank, editTier(lower, gl)})
	}

	sort.SliceStable(cands, func(i, j int) bool {
		si, sj := cands[i].score(), cands[j].score()
		if si == 0 {
			return false
		}
		if sj == 0 {
			return true
		}
		return si < sj
	})
	return cands
}

// score is the candidate's effective rank: the frequency rank, penalised for
// a plain typo. 0 = unranked (only hyphenated guesses can be).
func (c spellCand) score() int64 {
	if c.Rank == 0 {
		return 0
	}
	if c.Tier == tierTypo {
		return int64(c.Rank) * spellTypoPenalty
	}
	return int64(c.Rank)
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
	cands := s.Candidates(lower)
	if len(cands) == 0 {
		return "", false
	}
	best := cands[0]
	if len(cands) > 1 {
		if best.Rank == 0 {
			return "", false // nothing to prefer between unranked candidates
		}
		// Drop candidates far rarer than the most frequent one, then require
		// the leader to be spellRankRatio times more likely than the runner-up.
		var minRank int32
		for _, c := range cands {
			if c.Rank != 0 && (minRank == 0 || c.Rank < minRank) {
				minRank = c.Rank
			}
		}
		var live []spellCand
		for _, c := range cands {
			if c.Rank != 0 && c.Rank <= minRank*spellRareDrop {
				live = append(live, c)
			}
		}
		best = live[0]
		if len(live) > 1 && live[1].score() < best.score()*spellRankRatio {
			return "", false // too close to call
		}
	}

	fixed := best.Word
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
