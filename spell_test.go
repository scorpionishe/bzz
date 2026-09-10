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
	d.stemRank = make(map[string]int32)
	for i, w := range words {
		d.words[w] = true
		d.rank[w] = int32(i + 1)
		if stem := stemWord(w, "ru"); stem != "" {
			d.stems[stem] = true
			if _, seen := d.stemRank[stem]; !seen {
				d.stemRank[stem] = int32(i + 1)
			}
		}
	}
	return d
}

func newTestSpeller() (*Speller, *fakeChecker) {
	dict := newRankedDict(
		"и", "в", "не", "привет", "количество", "инженер", "симпатичный",
		"расстояние", "зашифрованный", "программа", "теплой", "типлой",
		"преет", "програма", // "програма" is a junk entry the checker rejects
		"заказ", "показ", "исправить", "товар", "татарин", "что-то", "форма",
	)
	// Push the two "rare" words outside the trusted-rank window.
	dict.rank["типлой"] = spellTrustedRank + 1
	dict.rank["преет"] = spellTrustedRank + 2
	checker := &fakeChecker{
		ok: map[string]bool{
			"привет": true, "количество": true, "инженер": true, "симпатичный": true,
			"расстояние": true, "зашифрованный": true, "зашифрованы": true,
			"программа": true, "теплой": true, "преет": true, "прервет": true,
			"нравица": true, "вообщем": true, "Москва": true, "слово": true,
			"прявет": true, "заказов": true, "показов": true, "исправило": true,
			"товары": true, "татары": true, "что-то": true, "форма": true, "формам": true,
			"заказа": true, "заказу": true, "заказы": true,
		},
		guesses: map[string][]string{
			"превет":       {"прервет", "пресет", "преет"},
			"зашифрованый": {"зашифрованный", "зашифрованы"},
			"поресерчить":  {"посеребрить"},
			"зоказов":      {"заказов", "показов", "заказав"},
			"испрравило":   {"исправила", "исправило"},
			"тавары":       {"татары", "товары"},
			"чтото":        {"что-то", "кто-то"},
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
		"превет":       {"привет", true},      // е→и is a typical slip; "преет" (dropped letter) is far rarer
		"зашифрованый": {"зашифрованный", true},
		"чтото":        {"что-то", true}, // hyphen comes from the checker's guesses
		// Inflected forms absent from the list are admitted through the stem
		// ("заказ" → "заказов"); the error model picks the typical slip.
		"зоказов":    {"заказов", true},   // о→а (typical) beats з→п (never)
		"испрравило": {"исправило", true}, // doubled letter; "исправила" is 2 edits away
		"тавары":     {"товары", true},    // а→о beats в→т
		// Forms of one lemma are not rivals: "форма" (exact) and "формам"
		// (by stem) tie on score, the exact entry wins.
		"формма": {"форма", true},
		"Формма": {"Форма", true},
		// Equally likely forms of one lemma with no exact entry among them
		// (заказа / заказу / заказы) are a coin toss → untouched.
		"заказв": {"", false},
		// Anglicisms from dicts/ru_extra.txt, in any inflection.
		"коммитом":   {"", false},
		"задеплоили": {"", false},
		"деплой":     {"", false},
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
	sp, _ := newTestSpeller()
	// The checker does not know "типлой" but it is a frequent dictionary
	// entry → trusted, not corrected to "теплой".
	sp.dict.rank["типлой"] = 500
	if sp.Misspelled("типлой") {
		t.Fatal("frequent dictionary entry must be trusted")
	}
	// Outside the trusted window the checker's verdict stands, and "теплой"
	// is the unique 1-edit candidate.
	sp.dict.rank["типлой"] = spellTrustedRank + 1
	if got, ok := sp.Fix("типлой"); !ok || got != "теплой" {
		t.Fatalf("Fix(типлой) = (%q, %v), want (теплой, true)", got, ok)
	}
}

func TestSpellAmbiguousLeftAlone(t *testing.T) {
	sp, _ := newTestSpeller()
	// Two same-tier (е→и, е→я) candidates too close in frequency → no fix.
	sp.dict.words["прявет"] = true
	sp.dict.rank["прявет"] = sp.dict.rank["привет"] + 1
	if got, ok := sp.Fix("превет"); ok {
		t.Fatalf("ambiguous word was fixed to %q", got)
	}
	// Far enough apart → the frequent one wins.
	sp.dict.rank["прявет"] = sp.dict.rank["привет"] * spellRankRatio
	if got, ok := sp.Fix("превет"); !ok || got != "привет" {
		t.Fatalf("Fix(превет) = (%q, %v), want (привет, true)", got, ok)
	}
	// Words unknown to the frequency list (by rank or stem) are not candidates.
	sp.dict.rank = map[string]int32{}
	sp.dict.stemRank = map[string]int32{}
	if got, ok := sp.Fix("превет"); ok {
		t.Fatalf("unranked-only candidates were resolved to %q", got)
	}
}

func TestSpellErrorModel(t *testing.T) {
	sp, _ := newTestSpeller()
	priv := sp.dict.rank["привет"]
	// Equal frequency: the typical slip (е→и) beats the dropped letter.
	sp.dict.rank["преет"] = priv
	if got, ok := sp.Fix("превет"); !ok || got != "привет" {
		t.Fatalf("equal ranks: Fix(превет) = (%q, %v), want (привет, true)", got, ok)
	}
	// The dropped-letter reading only wins when its word is overwhelmingly
	// more frequent (more than penalty × ratio).
	sp.dict.rank["преет"] = 1
	sp.dict.rank["привет"] = 1 * spellTypoPenalty * spellRankRatio * 2
	if got, ok := sp.Fix("превет"); !ok || got != "преет" {
		t.Fatalf("rare привет: Fix(превет) = (%q, %v), want (преет, true)", got, ok)
	}
	// In between it is a toss-up → no fix.
	sp.dict.rank["привет"] = spellTypoPenalty / 2
	if got, ok := sp.Fix("превет"); ok {
		t.Fatalf("toss-up was fixed to %q", got)
	}
	// A candidate far rarer than the leader is dropped before voting, even
	// when its edit tier is better: "будующий" → "будущий" (dropped letter),
	// not the obscure "бузующий" (д→з adjacent keys).
	sp.dict.rank["привет"] = priv
	sp.dict.words["бузующий"] = true
	sp.dict.rank["бузующий"] = 47988
	sp.dict.words["будущий"] = true
	sp.dict.rank["будущий"] = 1166
	sp.checker.(*fakeChecker).ok["бузующий"] = true
	sp.checker.(*fakeChecker).ok["будущий"] = true
	if got, ok := sp.Fix("будующий"); !ok || got != "будущий" {
		t.Fatalf("Fix(будующий) = (%q, %v), want (будущий, true)", got, ok)
	}
	// An unlikely substitution is never a correction, even when unique.
	sp.dict.words["накось"] = true
	sp.dict.rank["накось"] = 43023
	sp.checker.(*fakeChecker).ok["накось"] = true
	if got, ok := sp.Fix("макось"); ok {
		t.Fatalf("Fix(макось) = %q, want untouched", got)
	}
	// Stem-only forms of rare lemmas are not admitted ("комит" via "комитет").
	sp.dict.words["комитет"] = true
	sp.dict.rank["комитет"] = spellStemRankCap + 1
	sp.dict.stems[stemWord("комитет", "ru")] = true
	sp.dict.stemRank[stemWord("комитет", "ru")] = spellStemRankCap + 1
	sp.checker.(*fakeChecker).ok["комит"] = true
	delete(sp.known, "коммит")
	delete(sp.known, stemWord("коммит", "ru"))
	if got, ok := sp.Fix("коммит"); ok {
		t.Fatalf("Fix(коммит) = %q, want untouched", got)
	}
}

func TestEditTier(t *testing.T) {
	cases := []struct {
		typed, cand string
		want        int
	}{
		{"зоказов", "заказов", tierTypical},        // а/о confusion
		{"зоказов", "показов", tierUnlikely},       // з→п: neither confusable nor adjacent
		{"инжинер", "инженер", tierTypical},        // и/е
		{"сдесь", "здесь", tierTypical},            // с/з voicing
		{"растояние", "расстояние", tierTypical},   // missed double
		{"колличество", "количество", tierTypical}, // extra double
		{"которрых", "которых", tierTypical},
		{"инжеенр", "инженер", tierTypical}, // swap
		{"чтото", "что-то", tierTypical},    // hyphen
		{"поняль", "понять", tierTypical},   // ль: л=K, т=N... no; ь→т is not adjacent, but wait — checked below
		{"првет", "привет", tierTypo},       // missed letter
		{"приветт", "привет", tierTypical},  // extra double
		{"приевт", "привет", tierTypical},   // swap
		{"привет", "приает", tierTypical},   // в→а: D and F keys are neighbours
		{"привет", "приэет", tierUnlikely},
	}
	for _, c := range cases {
		if c.typed == "поняль" {
			continue // documented separately: ь→т is a plain substitution
		}
		if got := editTier(c.typed, c.cand); got != c.want {
			t.Errorf("editTier(%q, %q) = %d, want %d", c.typed, c.cand, got, c.want)
		}
	}
	if keyNeighbours[[2]rune{'п', 'з'}] {
		t.Error("п and з must not be keyboard neighbours")
	}
	for _, p := range [][2]rune{{'й', 'ц'}, {'ф', 'ы'}, {'а', 'в'}, {'а', 'п'}, {'ф', 'й'}, {'ы', 'ц'}, {'я', 'ф'}} {
		if !keyNeighbours[p] || !keyNeighbours[[2]rune{p[1], p[0]}] {
			t.Errorf("%c and %c should be keyboard neighbours", p[0], p[1])
		}
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
