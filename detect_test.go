package main

import (
	"fmt"
	"testing"
)

func TestDetection(t *testing.T) {
	ruDict, _ := LoadDict("ru")
	enDict, _ := LoadDict("en")
	det := NewDetector(ruDict, enDict)

	cases := []struct {
		input    string
		wantFix  bool
		wantText string
	}{
		// Basic words
		{"ghbdtn", true, "привет"},
		{"vbh", true, "мир"},
		{"ckjdj", true, "слово"},
		{"rfr", true, "как"},
		{"xnj", true, "что"},
		{"'nj", true, "это"},
		{"rjulf", true, "когда"},
		{"gjxtve", true, "почему"},
		{"ujyznm", true, "гонять"},
		{"hf,jnftn", true, "работает"},
		
		// Morphological forms  
		{"nsryek", true, "тыкнул"},
		{"jnht;m", true, "отрежь"},
		{",eltv", true, "будем"},
		{"cltkfk", true, "сделал"},
		{"gbitn", true, "пишет"},
		{"xbnftn", true, "читает"},
		
		// Single letter Russian words
		{"f", true, "а"},
		{"d", true, "в"},
		{"j", true, "о"},
		{"r", true, "к"},
		{"c", true, "с"},
		{"b", true, "и"},
		
		// Trailing punctuation: word + comma (corrected = just word, punct handled by caller)
		{"ckjdj,", true, "слово"},
		{"vbh.", true, "мир"},
		
		// Punctuation as letter (б, ю)
		{"ct,z", true, "себя"},
		{",skj", true, "было"},
		{",eltv", true, "будем"},
		
		// Should NOT convert (English words)
		// These depend on layout state, skip for unit test
		
		// Cyrillic already correct — don't touch
		{"привет", false, ""},
		{"мир", false, ""},
		{"работает", false, ""},
		
		// Common phrases (word by word)
		{"ghbdtn", true, "привет"},
		{"lj,hjt", true, "доброе"},
		{"enhj", true, "утро"},
		{"cgfcb,j", true, "спасибо"},
		{"gj;fkeqcnf", true, "пожалуйста"},
		{"bpdbyb", true, "извини"},
		{"cjukfcty", true, "согласен"},
		{"rjytxyj", true, "конечно"},
		{"yfghbvth", true, "например"},
		{"djpvj;yj", true, "возможно"},
		
		// Tech words
		{"rjvgm.nth", true, "компьютер"},
		{"bynthytn", true, "интернет"},
		{"ghjuhfvvf", true, "программа"},
		{"cnhfybwf", true, "страница"},

		// ── Apostrophe ' = э ──
		{"'nj", true, "это"},
		{"gjl]tpl", true, "подъезд"},

		// ── Semicolon ; = ж ──
		{"yj;", true, "нож"},
		{"vj;tn", true, "может"},
		{"ke;f", true, "лужа"},
		{"lj;lm", true, "дождь"},

		// ── Square brackets [ ] = х ъ ──
		{"[jhjibq", true, "хороший"},
		{"gj[j;t", true, "похоже"},

		// ── Backtick ` = ё ──
		{"`krf", true, "ёлка"},

		// ── Comma , = б ──
		{",tp", true, "без"},
		{",jkmijq", true, "большой"},
		{",hfn", true, "брат"},
		{"j,kfrj", true, "облако"},
		{"cj,frf", true, "собака"},
		{"hf,jnf", true, "работа"},
		{"j,", true, "об"},

		// ── Period . = ю ──
		{"k.,jdm", true, "любовь"},
		{"k.lb", true, "люди"},
		{"rk.x", true, "ключ"},

		// ── Mixed punct-letters ──
		{",k.lj", true, "блюдо"},
		{".yjcnm", true, "юность"},
		{".,rf", true, "юбка"},

		// ── Trailing comma (punctuation, NOT б) — corrected = word only ──
		{"ghbdtn,", true, "привет"},
		{"vbh,", true, "мир"},
		{"ckjdf,", true, "слова"},

		// ── Trailing period (punctuation, NOT ю) ──
		{"ghbdtn.", true, "привет"},
		{"ckjdf.", true, "слова"},

		// ── Trailing semicolon (punctuation, NOT ж) ──
		{"ghbdtn;", true, "привет"},

		// ── Words ending with ь (m on QWERTY) ──
		{"gjvjom", true, "помощь"},
		{"kj;m", true, "ложь"},
		{"hjkm", true, "роль"},

		// ── Cyrillic should NOT be touched ──
		{"собака", false, ""},
		{"компьютер", false, ""},
		{"ёлка", false, ""},
		{"большой", false, ""},
		{"люди", false, ""},

		// ── Backslash \ = ё (Mac keyboard) ──
		{"\\krf", true, "ёлка"},
		{"\\;br", true, "ёжик"},
		{"\\km", true, "ёль"},

		// ── Double punct-letters — nonsense, should NOT convert ──
		{",,.p", false, ""},

		// ── Profanity (in 100k dict) ──
		{",kznm", true, "блять"},
		{"[eq", true, "хуй"},
		{"gbpltw", true, "пиздец"},

		// ── Numbers mixed — should not convert ──
		{"123", false, ""},
		{"test123", false, ""},

		// ── Long words ──
		{"ghjuhfvvbhjdfybt", true, "программирование"},
		{"ljrevtynjj,jhjn", true, "документооборот"},
		{"ghtlghbybvfntkmcndj", true, "предпринимательство"},

		// ── Words with ь (soft sign = m) ──
		{"gjvjom", true, "помощь"},
		{";bpym", true, "жизнь"},
		{"k.,jdm", true, "любовь"},
		{"jcnfyjdbnm", true, "остановить"},
		{"gjyznm", true, "понять"},
		{"dcnhtnbnm", true, "встретить"},

		// ── Words with ъ (hard sign = ]) ──
		{"j,]tv", true, "объем"},
		{"j,]zdktybt", true, "объявление"},
		{"c]tcnm", true, "съесть"},

		// ── Words with э (= ') ──
		{"'rcrfdfnjh", true, "экскаватор"},
		{"'rcgthbvtyn", true, "эксперимент"},
		{"'ktvtyn", true, "элемент"},
		{"'ythubb", true, "энергии"},

		// ── Words with ё (= \ on Mac) ──
		{"\\k", false, ""},     // too short, single letter ё + л

		// ── Common typing mistakes ──
		{"ghbdtn vbh", false, ""},  // space inside — buffer splits, each word separate

		// ── Words starting with х (= [) ──
		{"[dfnbn", true, "хватит"},
		{"[jntnm", true, "хотеть"},
		{"[jlbnm", true, "ходить"},
		{"[kt,", true, "хлеб"},

		// ── Sentence fragments (individual words) ──
		{"rfr", true, "как"},
		{"ltkf", true, "дела"},
		{"ctujlyz", true, "сегодня"},
		{"dpznm", true, "взять"},

		// ── Trailing punct edge cases ──
		{"lf;", true, "даж"},     // "да" + ж → full "даж" wins (short trim)
		{"yt;", true, "неж"},     // same logic
		{"vjq.", true, "мой"},    // "мой" is 3+ chars → trim wins
		{"dfv,", true, "вам"},    // "вам" is 3+ chars → trim wins
		{"yfc;", true, "нас"},    // "нас" is 3+ chars → trim wins

		// ── Already correct cyrillic ──
		{"привет", false, ""},
		{"ёжик", false, ""},
		{"съесть", false, ""},
		{"объявление", false, ""},
		{"экскаватор", false, ""},
		{"хватит", false, ""},
		{"взять", false, ""},
		{"жизнь", false, ""},

		// ── Shifted Russian punct (Shift+6=^ → comma, Shift+7=& → period) ──
		{"ckjdj^", true, "слово"},   // Shift+6 = Russian comma → trailingPunct = ','
		{"ckjdj&", true, "слово"},   // Shift+7 = Russian period → trailingPunct = '.'
		{"ghbdtn^", true, "привет"}, // привет,
		{"ghbdtn&", true, "привет"}, // привет.
		{"vbh$", true, "мир"},       // Shift+4 = Russian semicolon
		{"lf@", true, "да"},         // Shift+2 = Russian quote

		// ── Universal punct (! ?) — same key on both layouts ──
		{"ghbdtn!", true, "привет"}, // привет!
		{"ghbdtn?", true, "привет"}, // привет?
		{"ckjdj!", true, "слово"},   // слово!
		{"ckjdj?", true, "слово"},   // слово?
		{"rfr?", true, "как"},       // как?

		// ── First person verbs (suffix у/ю) ──
		{"uhe;e", true, "гружу"},
		{"gbie", true, "пишу"},
		{"crf;e", true, "скажу"},
		{"[j;e", true, "хожу"},
		{"db;e", true, "вижу"},
		{"cb;e", true, "сижу"},
		{"kt;e", true, "лежу"},
		{",the", true, "беру"},
		{"yfqle", true, "найду"},

		// ── Gerunds / participles ──
		{"ltkfz", true, "делая"},
		{"xbnfz", true, "читая"},
		{"blz", true, "идя"},

		// ── Adjectives (various cases) ──
		{"[jhjituj", true, "хорошего"},
		{",jkmijve", true, "большому"},
		{"yjdjq", true, "новой"},
		{"cnfhjuj", true, "старого"},
		{"rhfcbdsq", true, "красивый"},
		{"vfktymrbq", true, "маленький"},

		// ── Nouns (various cases) ──
		{"rybub", true, "книги"},
		{"ljvjd", true, "домов"},
		{"ltntq", true, "детей"},
		{"lheptq", true, "друзей"},
		{"cnjkjv", true, "столом"},

		// ── Reflexive verbs (-ся, -сь) — complex morphology, skip for now ──

		// ── Past tense (all genders) ──
		{"ltkfk", true, "делал"},
		{"ltkfkf", true, "делала"},
		{"ltkfkb", true, "делали"},
		{"ltkfkj", true, "делало"},
		{"crf;tn", true, "скажет"},
		{"crfpfk", true, "сказал"},
		{"crfpfkf", true, "сказала"},

		// ── Imperative ──
		{"ltkfq", true, "делай"},
		{"crf;b", true, "скажи"},
		{"bib", true, "иши"},
		{"cvjnhb", true, "смотри"},
		{"gjcvjnhb", true, "посмотри"},  // may need prefix handling

		// ── Common chat/slang ──
		{"cgg", false, ""},          // nonsense
		{"ghbdtn", true, "привет"},
		{"rfr", true, "как"},
		{"xj", true, "чо"},         // slang, in dict
		{"jrt", true, "оке"},
		{"kflyj", true, "ладно"},
		{"gjrf", true, "пока"},
		{"cgfcb,j", true, "спасибо"},
		{"ljhjuj", true, "дорого"},
		{"ltitdj", true, "дешево"},

		// ── Words with double letters ──
		{"ghjuhfvvf", true, "программа"},
		{"heccrbq", true, "русский"},
		{"rkfcc", true, "класс"},

		// ── Prepositions & particles (short) ──
		{"yf", true, "на"},
		{"yj", true, "но"},
		{"yt", true, "не"},
		// {"pf", true, "за"},   // "pf" is in EN dict → ambiguous, skipped
		{"lj", true, "до"},
		{",s", true, "бы"},
		{";t", true, "же"},
		// {"kb", true, "ли"},   // "kb" is in EN dict → ambiguous, skipped

		// ── Numbers + text — should not crash ──
		{"123", false, ""},
		{"ntcn123", false, ""},

		// ── Universal punct with various words ──
		{"cjukfcty!", true, "согласен"},
		{"ytghfdbkmyj?", true, "неправильно"},
		{"ghbrjkmyj!", true, "прикольно"},
		{"pfxtv?", true, "зачем"},

		// ── Shifted punct with various words ──
		{"ckjdj^", true, "слово"},     // слово,
		{"gjyznyj&", true, "понятно"}, // понятно.
		{"rfr$", true, "как"},         // как;

		// ── Mixed: word with ё ──
		{"\\;br", true, "ёжик"},
		{"\\krf", true, "ёлка"},

		// ── Fuzzy matching (typo in wrong layout, 1 edit distance) ──
		{"gjljk;bv", true, "продолжим"},   // "подолжим" → fuzzy → "продолжим"
		{"ghjuhfvf", true, "програма"},   // "програма" matches via stemmer
		{"bynthytn", true, "интернет"},   // exact match still works
		{"ghbdn", false, ""},             // "привт" — too short (5 chars), no fuzzy

		// ── Edge: single letters ──
		{"f", true, "а"},
		{"d", true, "в"},
		{"j", true, "о"},
		{"r", true, "к"},
		{"c", true, "с"},
		{"z", true, "я"},
		{"b", true, "и"},
		{"e", true, "у"},
	}

	passed, failed := 0, 0
	for _, tc := range cases {
		det.lastLangRu = true  // simulate context
		det.initialized = true
		wrong, corrected := det.Check(tc.input)
		
		if wrong != tc.wantFix {
			fmt.Printf("FAIL %q: got wrong=%v want=%v (corrected=%q)\n", tc.input, wrong, tc.wantFix, corrected)
			failed++
		} else if wrong && corrected != tc.wantText {
			fmt.Printf("FAIL %q: got %q want %q\n", tc.input, corrected, tc.wantText)
			failed++
		} else {
			passed++
		}
	}
	
	fmt.Printf("\n=== Results: %d passed, %d failed out of %d ===\n", passed, failed, passed+failed)
	if failed > 0 {
		t.Errorf("%d tests failed", failed)
	}
}
