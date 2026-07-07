package main

// QWERTY → Russian keyboard mapping
var enToRu = map[rune]rune{
	'q': 'й', 'w': 'ц', 'e': 'у', 'r': 'к', 't': 'е',
	'y': 'н', 'u': 'г', 'i': 'ш', 'o': 'щ', 'p': 'з',
	'[': 'х', ']': 'ъ', 'a': 'ф', 's': 'ы', 'd': 'в',
	'f': 'а', 'g': 'п', 'h': 'р', 'j': 'о', 'k': 'л',
	'l': 'д', ';': 'ж', '\'': 'э', 'z': 'я', 'x': 'ч',
	'c': 'с', 'v': 'м', 'b': 'и', 'n': 'т', 'm': 'ь',
	',': 'б', '.': 'ю', '`': 'ё', '\\': 'ё',
	'Q': 'Й', 'W': 'Ц', 'E': 'У', 'R': 'К', 'T': 'Е',
	'Y': 'Н', 'U': 'Г', 'I': 'Ш', 'O': 'Щ', 'P': 'З',
	'{': 'Х', '}': 'Ъ', 'A': 'Ф', 'S': 'Ы', 'D': 'В',
	'F': 'А', 'G': 'П', 'H': 'Р', 'J': 'О', 'K': 'Л',
	'L': 'Д', ':': 'Ж', '"': 'Э', 'Z': 'Я', 'X': 'Ч',
	'C': 'С', 'V': 'М', 'B': 'И', 'N': 'Т', 'M': 'Ь',
	'<': 'Б', '>': 'Ю', '~': 'Ё',
}

// Russian → QWERTY mapping (built from reverse)
var ruToEn map[rune]rune

// QWERTY keys that map to Russian letters — NOT word boundaries
var qwertyRuPunct = map[rune]bool{
	',': true, '.': true, ';': true, '\'': true,
	'[': true, ']': true, '`': true, '\\': true,
}

// Shifted number keys: what you get on EN layout when you meant Russian punctuation
// User thinks they're on Russian layout and presses Shift+N
// Standard Russian PC keyboard layout:
//   Shift+1 = !   → EN Shift+1 = !  (same, no mapping needed)
//   Shift+2 = "   → EN Shift+2 = @
//   Shift+3 = №   → EN Shift+3 = #
//   Shift+4 = ;   → EN Shift+4 = $
//   Shift+5 = %   → EN Shift+5 = %  (same)
//   Shift+6 = :   → EN Shift+6 = ^
//   Shift+7 = ?   → EN Shift+7 = &
//   Shift+8 = *   → EN Shift+8 = *  (same)
var shiftedRuPunct = map[rune]rune{
	'^': ',',  // Shift+6: Russian запятая (Russian-PC layout)
	'&': '.',  // Shift+7: Russian точка (Russian-PC layout)
	'$': ';',  // Shift+4: Russian точка с запятой
	'@': '"',  // Shift+2: Russian кавычки
	'#': '№',  // Shift+3: Russian знак номера
}

// Punctuation that is the SAME key on both layouts
// These are word boundaries but should be preserved as trailing punct
var universalPunct = map[rune]bool{
	'!': true,  // Shift+1: same on both layouts
	'?': true,  // Shift+/: same on both layouts
}

// Symbols produced by the macOS "Russian" layout whose EN-layout counterpart
// sits on the SAME physical key: Shift+3 = № / #, Shift+8 = ; / *, the key
// left of 1 (grave / ISO §) = ] / `. Merged into ruToEn so an RU→EN flip
// retypes the same physical keys in the other layout. enToRu stays untouched:
// in the EN→RU direction ';' and ']' keep their letter meaning (ж, ъ).
var ruLayoutFlips = map[rune]rune{
	'№': '#',
	';': '*',
	']': '`',
}

// Physical-key signatures proving a char was typed on the RUSSIAN layout: each
// of these chars comes out of that keycode only when the Russian layout is
// active (Shift+8 → ';', the key left of 1 → ']'). '№' needs no keycode — it
// exists only on the Russian layout.
var ruFlipKeycodes = map[rune][]uint16{
	';': {0x1C},       // the "8" key
	']': {0x32, 0x0A}, // kVK_ANSI_Grave / kVK_ISO_Section — the key left of 1
}

// isRuLayoutEvidence reports whether rune r typed with keycode kc proves the
// Russian layout was active when it was typed.
func isRuLayoutEvidence(r rune, kc uint16) bool {
	if r == '№' {
		return true
	}
	for _, want := range ruFlipKeycodes[r] {
		if kc == want {
			return true
		}
	}
	return false
}

func init() {
	ruToEn = make(map[rune]rune, len(enToRu)+len(ruLayoutFlips))
	for en, ru := range enToRu {
		ruToEn[ru] = en
	}
	for ru, en := range ruLayoutFlips {
		ruToEn[ru] = en
	}
}

// QWERTYToRussian converts a string typed on QWERTY to Russian layout
func QWERTYToRussian(s string) string {
	runes := make([]rune, 0, len(s))
	for _, r := range s {
		if mapped, ok := enToRu[r]; ok {
			runes = append(runes, mapped)
		} else {
			runes = append(runes, r)
		}
	}
	return string(runes)
}

// RussianToQWERTY converts a string typed on Russian layout to QWERTY
func RussianToQWERTY(s string) string {
	runes := make([]rune, 0, len(s))
	for _, r := range s {
		if mapped, ok := ruToEn[r]; ok {
			runes = append(runes, mapped)
		} else {
			runes = append(runes, r)
		}
	}
	return string(runes)
}
