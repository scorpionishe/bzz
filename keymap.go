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

func init() {
	ruToEn = make(map[rune]rune, len(enToRu))
	for en, ru := range enToRu {
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
