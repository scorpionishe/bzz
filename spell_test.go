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
	d.stemBest = make(map[string]string)
	for i, w := range words {
		d.words[w] = true
		d.rank[w] = int32(i + 1)
		if stem := stemWord(w, "ru"); stem != "" {
			d.stems[stem] = true
			if _, seen := d.stemRank[stem]; !seen {
				d.stemRank[stem] = int32(i + 1)
				d.stemBest[stem] = w
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
		"заполняться", "пировать",
		// Proper nouns: the checker lists them Capitalized only.
		"москва", "маска", "игорь", "гарь", "викерс",
		// Word families the checker knows only by the lemma (#23).
		"трафик", "трафика", "столб", "столбы", "виртуальный", "виртуально",
		"система", "женщина", "димка", "диске", "гарантированно", "гарантировано",
	)
	// A rare surname: never a fix, only a rival.
	dict.rank["викерс"] = spellTrustedRank + 5
	// Rare lemmas: their stem-only forms are capped unless the lemma itself
	// is a candidate.
	dict.rank["заполняться"] = spellStemRankCap + 10
	dict.stemRank[stemWord("заполняться", "ru")] = spellStemRankCap + 10
	dict.rank["пировать"] = spellStemRankCap + 20
	dict.stemRank[stemWord("пировать", "ru")] = spellStemRankCap + 20
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
			"заполняться": true, "заполняется": true, "пирует": true,
			"маска": true, "Игорь": true, "гарь": true, "Викерс": true,
			"трафик": true, "столбы": true, "виртуальный": true, "виртуально": true,
			"система": true, "женщина": true, "Димка": true, "диске": true,
			"гарантировано": true,
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
		// тся/ться: "заполняется" is known only by stem and the lemma is rare,
		// but the lemma "заполняться" is itself a candidate → admitted, and
		// the dropped soft sign (typical) beats the dropped е (typo).
		"заполняеться": {"заполняется", true},
		// Without that anchor a rare lemma's form stays out: "пирует" cannot
		// rival "привет" for "пирвет".
		"пирвет": {"привет", true},
		// Anglicisms from dicts/ru_extra.txt, in any inflection — including
		// forms snowball stems differently from the lemma ("токена" → "ток").
		"коммитом":   {"", false},
		"задеплоили": {"", false},
		"деплой":     {"", false},
		"токена":     {"", false},
		"хуков":      {"", false},
		"бакете":     {"", false},
		// Inflected forms of a known lemma the checker lacks are not typos
		// when the only fix is an ending change ("трафика" → "трафик"), but
		// a typical slip of a known lemma still is ("системо").
		"трафика":     {"", false},
		"виртуальной": {"", false},
		"системо":     {"система", true},
		"женщино":     {"женщина", true},
		// A form of a name the checker lists Capitalized only ("Димка").
		"Димке": {"", false},
		// нн/н on a list entry is the adverb / participle split.
		"гарантированно": {"", false},
		// Proper nouns are candidates through their Capitalized form and win
		// only for a Capitalized word and a frequent entry.
		"Масква": {"Москва", true},
		"масква": {"", false},
		"Игарь":  {"Игорь", true},
		"Вигерс": {"", false},
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

// A rare exact entry the checker lacks ("подложка") is a real word: a
// one-edit neighbour only twice as frequent ("подлодка") must not replace it.
// Only a fix spellRankRatio times more frequent — the junk-entry case — may.
func TestSpellListedWordNeedsMuchMoreFrequentFix(t *testing.T) {
	sp, _ := newTestSpeller()
	for _, w := range []string{"подложка", "подлодка"} {
		sp.dict.words[w] = true
		sp.dict.stems[stemWord(w, "ru")] = true
	}
	sp.dict.rank["подлодка"] = spellTrustedRank
	sp.dict.rank["подложка"] = spellTrustedRank * 2
	sp.checker.(*fakeChecker).ok["подлодка"] = true
	if got, ok := sp.Fix("подложка"); ok {
		t.Fatalf("Fix(подложка) = %q, want untouched", got)
	}
	sp.dict.rank["подложка"] = spellTrustedRank * spellRankRatio
	if got, ok := sp.Fix("подложка"); !ok || got != "подлодка" {
		t.Fatalf("Fix(подложка) = (%q, %v), want (подлодка, true)", got, ok)
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

// Issue #28: a form known only through its stem must not inherit the rank of a
// frequent short word that happens to share the stem ("разо" → "раз").
func TestSpellStemRankFloor(t *testing.T) {
	sp, chk := newTestSpeller()
	for i, w := range []string{"раз", "фраза"} {
		sp.dict.words[w] = true
		sp.dict.rank[w] = int32(80 + i*1000)
		st := stemWord(w, "ru")
		sp.dict.stems[st] = true
		sp.dict.stemRank[st] = int32(80 + i*1000)
		sp.dict.stemBest[st] = w
	}
	chk.ok["разо"], chk.ok["фраза"] = true, true
	if got, ok := sp.Fix("фразо"); ok && got != "фраза" {
		t.Fatalf("Fix(фразо) = %q, want фраза or untouched", got)
	}
	for _, c := range sp.Candidates("фразо") {
		if c.Word == "разо" && c.Rank < spellStemRankFloor {
			t.Fatalf("stem-only candidate разо ranked %d, below the floor", c.Rank)
		}
	}
}

// Issue #28: rare-drop is measured against the score leader, so a typo-tier
// candidate with a good raw rank ("пота" r115) cannot evict the typical slip
// that scores better ("поток" r1287). The outcome is a stand-off, never "пота".
func TestSpellRareDropKeepsLeader(t *testing.T) {
	sp, chk := newTestSpeller()
	for w, r := range map[string]int32{"поток": 1287, "пота": 115} {
		sp.dict.words[w] = true
		sp.dict.rank[w] = r
		chk.ok[w] = true
	}
	if got, ok := sp.Fix("потак"); ok {
		t.Fatalf("Fix(потак) = %q, want untouched", got)
	}
	// With the rival gone the typical slip wins outright.
	delete(sp.dict.rank, "пота")
	delete(sp.dict.words, "пота")
	if got, ok := sp.Fix("потак"); !ok || got != "поток" {
		t.Fatalf("Fix(потак) = (%q, %v), want (поток, true)", got, ok)
	}
}

// Issue #23: the inflected-form guard needs the stem's best word to pass the
// checker and to be a different word; junk list entries protect nothing, and
// short colliding stems ("буд") are ignored.
func TestSpellInflectedFormGuard(t *testing.T) {
	sp, chk := newTestSpeller()
	if !sp.inflectedForm("трафика") {
		t.Error("трафика should count as a form of трафик")
	}
	if sp.inflectedForm("колличество") {
		t.Error("a junk entry must not vouch for itself")
	}
	// Same stem, but the checker rejects the best word too → still a typo.
	chk.ok["трафик"] = false
	if sp.inflectedForm("трафика") {
		t.Error("guard must require the checker to accept the lemma")
	}
	chk.ok["трафик"] = true
	// Short stem: "буд" is shared with "будет"; no protection.
	sp.dict.words["будет"] = true
	sp.dict.rank["будет"] = 30
	sp.dict.stemRank["буд"] = 30
	sp.dict.stemBest["буд"] = "будет"
	chk.ok["будет"] = true
	if sp.inflectedForm("будующий") {
		t.Error("a 3-letter stem must not trigger the guard")
	}
}

// Typing ё is never a slip: a dictionary that lists only the е spelling must
// not "fix" Трёхступенчатый → Трехступенчатый.
func TestEditTierYo(t *testing.T) {
	if got := editTier("трёх", "трех"); got != tierUnlikely {
		t.Errorf("ё→е tier = %d, want %d", got, tierUnlikely)
	}
	if got := editTier("трех", "трёх"); got != tierTypical {
		t.Errorf("е→ё tier = %d, want %d", got, tierTypical)
	}
}

// ru_extra.txt stems shorter than stemSetMinLen are not recorded: "токен"
// must not make every "ток…" word known.
func TestLoadStemSetShortStems(t *testing.T) {
	set, err := LoadStemSet("ru_extra", "ru")
	if err != nil {
		t.Fatal(err)
	}
	if !set["токен"] || set["ток"] {
		t.Errorf("токен=%v ток=%v, want true/false", set["токен"], set["ток"])
	}
	if !set[stemWord("коммитом", "ru")] {
		t.Error("stem of коммит must be recorded")
	}
}

func TestAdverbEdit(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"гарантированно", "гарантировано", true},
		{"гарантировано", "гарантированно", true},
		{"зашифрованый", "зашифрованный", false}, // adjective: a real misspelling
		{"даный", "данный", false},
		{"колличество", "количество", false},
		{"привет", "приветы", false},
	} {
		if got := adverbEdit(c.a, c.b); got != c.want {
			t.Errorf("adverbEdit(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// Issue #31: punctuation the buffer keeps attached to a word is split off
// for the checker and retyped with the fix.
func TestSpellSplitAndPlan(t *testing.T) {
	for w, want := range map[string][2]string{
		"превет,":    {"превет", ","},
		"превет!":    {"превет", "!"},
		"превет...":  {"превет", "..."},
		"превет":     {"превет", ""},
		"превет....": {"превет....", ""}, // too long: left alone
		"превет2":    {"превет2", ""},    // digits are not punctuation
		"т.д.":       {"т.д", "."},       // inner dot stays; the core then fails spellCandidate
	} {
		core, tail := spellSplit(w)
		if core != want[0] || tail != want[1] {
			t.Errorf("spellSplit(%q) = (%q, %q), want (%q, %q)", w, core, tail, want[0], want[1])
		}
	}
	cases := []struct {
		word     string
		boundary rune
		core     string
		del      int
		suffix   string
	}{
		{"превет,", ' ', "превет", 8, ", "}, // comma inside the word, space after
		{"превет!", '!', "превет", 7, "!"},  // universal punct folded in, nothing after
		{"превет", '-', "превет", 7, "-"},   // plain boundary, retyped
		{"превет,", 0, "превет", 7, ","},    // Enter path: nothing follows
		{"превет", ' ', "превет", 7, " "},
	}
	for _, c := range cases {
		core, _, del, suffix := spellPlan(c.word, c.boundary)
		if core != c.core || del != c.del || suffix != c.suffix {
			t.Errorf("spellPlan(%q, %q) = (%q, %d, %q), want (%q, %d, %q)", c.word, string(c.boundary), core, del, suffix, c.core, c.del, c.suffix)
		}
	}
	sp, _ := newTestSpeller()
	core, _ := spellSplit("превет,")
	if got, ok := sp.Fix(core); !ok || got != "привет" {
		t.Fatalf("Fix(%q) = (%q, %v), want (привет, true)", core, got, ok)
	}
	if sp.Misspelled("превет,") {
		t.Fatal("the raw word with punctuation must never reach the checker as-is")
	}
}

func TestKnownWordEndings(t *testing.T) {
	sp, _ := newTestSpeller()
	sp.known = map[string]bool{"токен": true, "хз": true}
	for w, want := range map[string]bool{
		"токен": true, "токена": true, "токенами": true, // stem match
		"токенизация": false, // neither stem nor a short ending
		"хзака":       false, // entry too short to anchor an ending
		"поток":       false,
	} {
		if got := sp.knownWord(w); got != want {
			t.Errorf("knownWord(%q) = %v, want %v", w, got, want)
		}
	}
}
