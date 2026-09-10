package main

import (
	"strings"
	"testing"
	"time"
)

// fakeChecker is a SpellChecker backed by an explicit set of correct words and
// canned guesses, so the decision logic can be tested without the system.
type fakeChecker struct {
	ok      map[string]bool
	guesses map[string][]string
}

func (f *fakeChecker) IsCorrect(w string) bool   { return f.ok[w] }
func (f *fakeChecker) Guesses(w string) []string { return f.guesses[w] }

// newRankedDict builds a Dict whose words are ranked in the given order
// (1 = most frequent), mirroring LoadDict on a frequency-ordered file.
func newRankedDict(words ...string) *Dict {
	d := &Dict{
		words:    make(map[string]bool),
		stems:    make(map[string]bool),
		trigrams: make(map[string]bool),
		rank:     make(map[string]int32),
		lang:     "ru",
	}
	for i, w := range words {
		d.words[w] = true
		d.rank[w] = int32(i + 1)
	}
	return d
}

func newTestSpeller() (*Speller, *fakeChecker) {
	dict := newRankedDict(
		"и", "в", "не", "привет", "количество", "инженер", "симпатичный",
		"расстояние", "зашифрованный", "программа", "теплой", "деплой",
		"преет", "програма", // "програма" is a junk entry the checker rejects
	)
	// Push the two "rare" words outside the trusted-rank window.
	dict.rank["деплой"] = spellTrustedRank + 1
	dict.rank["преет"] = spellTrustedRank + 2
	checker := &fakeChecker{
		ok: map[string]bool{
			"привет": true, "количество": true, "инженер": true, "симпатичный": true,
			"расстояние": true, "зашифрованный": true, "зашифрованы": true,
			"программа": true, "теплой": true, "преет": true, "прервет": true,
			"нравица": true, "вообщем": true, "Москва": true, "слово": true,
		},
		guesses: map[string][]string{
			"превет":       {"прервет", "пресет", "преет"},
			"зашифрованый": {"зашифрованный", "зашифрованы"},
			"поресерчить":  {"посеребрить"},
		},
	}
	return NewSpeller(checker, dict), checker
}

func TestSpellFix(t *testing.T) {
	sp, _ := newTestSpeller()
	cases := map[string]struct {
		want   string
		wantOK bool
	}{
		"колличество":  {"количество", true},  // extra letter
		"инжинер":      {"инженер", true},     // wrong letter
		"симпотичный":  {"симпатичный", true}, // wrong letter
		"растояние":    {"расстояние", true},  // missing letter
		"инжеенр":      {"инженер", true},     // swapped letters
		"Инжинер":      {"Инженер", true},     // capitalization preserved
		"превет":       {"привет", true},      // ranked "привет" beats unranked guesses ("преет" is far rarer)
		"зашифрованый": {"зашифрованный", true},
		// left alone
		"нравица":     {"", false}, // deliberate slang the checker accepts
		"вообщем":     {"", false},
		"поресерчить": {"", false}, // anglicism: no 1-edit neighbour
		"ресайз":      {"", false},
		"москва":      {"", false}, // capitalization-only difference
		"ИНЖИНЕР":     {"", false}, // ALL CAPS
		"иНжинер":     {"", false}, // mixed case
		"инжи":        {"", false}, // too short
		"привет":      {"", false}, // correct
		"hello":       {"", false}, // not Cyrillic
		"инжи-нер":    {"", false}, // punctuation
		"програма":    {"", false}, // junk entry, but ranked inside the trusted window → trusted
	}
	for word, c := range cases {
		got, ok := sp.Fix(word)
		if ok != c.wantOK || got != c.want {
			t.Errorf("Fix(%q) = (%q, %v), want (%q, %v)", word, got, ok, c.want, c.wantOK)
		}
	}
}

// "програма" → "программа" must fire even though "програма" is an exact (junk)
// entry of the frequency list, because its rank is outside the trusted window.
func TestSpellJunkEntryStillFixed(t *testing.T) {
	sp, _ := newTestSpeller()
	sp.dict.rank["програма"] = spellTrustedRank + 3
	if got, ok := sp.Fix("програма"); !ok || got != "программа" {
		t.Fatalf("Fix(програма) = (%q, %v), want (программа, true)", got, ok)
	}
}

func TestSpellTrustedRankWins(t *testing.T) {
	sp, checker := newTestSpeller()
	// The checker does not know "деплой" but it is a frequent dictionary
	// entry → trusted, not corrected to "теплой".
	sp.dict.rank["деплой"] = 500
	if sp.Misspelled("деплой") {
		t.Fatal("frequent dictionary entry must be trusted")
	}
	// Outside the trusted window the checker's verdict stands, and "теплой"
	// is the unique 1-edit candidate.
	sp.dict.rank["деплой"] = spellTrustedRank + 1
	delete(checker.ok, "деплой")
	if got, ok := sp.Fix("деплой"); !ok || got != "теплой" {
		t.Fatalf("Fix(деплой) = (%q, %v), want (теплой, true)", got, ok)
	}
}

func TestSpellAmbiguousLeftAlone(t *testing.T) {
	sp, _ := newTestSpeller()
	// Two ranked candidates too close in frequency → no fix.
	sp.dict.rank["преет"] = sp.dict.rank["привет"] + 1
	if got, ok := sp.Fix("превет"); ok {
		t.Fatalf("ambiguous word was fixed to %q", got)
	}
	// Far enough apart → the frequent one wins.
	sp.dict.rank["преет"] = sp.dict.rank["привет"] * spellRankRatio
	if got, ok := sp.Fix("превет"); !ok || got != "привет" {
		t.Fatalf("Fix(превет) = (%q, %v), want (привет, true)", got, ok)
	}
	// Only unranked candidates → nothing to prefer.
	sp.dict.rank = map[string]int32{}
	if got, ok := sp.Fix("зашифрованый"); ok {
		t.Fatalf("unranked-only candidates were resolved to %q", got)
	}
}

func TestIsOneEdit(t *testing.T) {
	yes := [][2]string{
		{"привет", "привет"}, // handled below as "no": identical is not an edit
		{"привет", "првет"},
		{"привет", "приветт"},
		{"привет", "превет"},
		{"привет", "првиет"},
		{"ab", "ba"},
		{"a", ""},
	}
	for _, p := range yes[1:] {
		if !isOneEdit(p[0], p[1]) || !isOneEdit(p[1], p[0]) {
			t.Errorf("isOneEdit(%q, %q) = false, want true", p[0], p[1])
		}
	}
	no := [][2]string{
		{"привет", "привет"},
		{"привет", "прет"},
		{"привет", "пивтер"},
		{"привет", "приветик"},
		{"abc", "cba"},
		{"abcd", "badc"},
	}
	for _, p := range no {
		if isOneEdit(p[0], p[1]) {
			t.Errorf("isOneEdit(%q, %q) = true, want false", p[0], p[1])
		}
	}
}

func TestOneEditsCoverAllKinds(t *testing.T) {
	set := map[string]bool{}
	for _, e := range oneEdits("кот") {
		set[e] = true
	}
	for _, want := range []string{"от", "окт", "кит", "кото", "скот"} {
		if !set[want] {
			t.Errorf("oneEdits(кот) lacks %q", want)
		}
	}
	if set["кот"] {
		t.Error("oneEdits must not yield the word itself")
	}
}

func TestSpellCandidate(t *testing.T) {
	for w, want := range map[string]bool{
		"слово": true, "Слово": true, "слов": false, "СЛОВО": false, "слОво": false,
		"word": false, "сло-во": false, "слово1": false, "": false,
		strings.Repeat("а", spellMaxLen+1): false,
	} {
		if got := spellCandidate(w); got != want {
			t.Errorf("spellCandidate(%q) = %v, want %v", w, got, want)
		}
	}
}

func TestLearnSpellRevert(t *testing.T) {
	s, clock := newTempLearn(t)
	for i := 0; i < 2; i++ {
		if s.RecordSpellRevert("ресайз") {
			t.Fatalf("learned too early on revert %d", i+1)
		}
		tick(clock, 2*time.Minute)
	}
	if !s.RecordSpellRevert("ресайз") {
		t.Fatal("expected personal-dictionary signal on 3rd revert")
	}
	// Spelling entries never become layout rules.
	if _, ok := s.Rule("ресайз"); ok {
		t.Fatal("spell entry must not act as a layout rule")
	}
	// Dedup: two reverts within the window count once.
	if s.RecordSpellRevert("деплой") {
		t.Fatal("first revert must not learn")
	}
	tick(clock, time.Second)
	if s.RecordSpellRevert("деплой") {
		t.Fatal("revert inside the dedup window must not count")
	}
	// Non-candidates (Latin, symbols) are ignored.
	if s.RecordSpellRevert("vscode") {
		t.Fatal("latin word must not be recorded")
	}
	// The entry is namespaced and survives a reload.
	s2, err := NewLearnStore(3)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range s2.List() {
		if e.Word == "ресайз" && e.Direction == learnSpellDir && e.Neg == 3 {
			found = true
		}
	}
	if !found {
		t.Fatal("spell entry not persisted")
	}
	if n, _ := s2.Forget("ресайз"); n != 1 {
		t.Fatalf("Forget removed %d entries, want 1", n)
	}
}
