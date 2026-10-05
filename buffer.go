package main

import (
	"sync"
	"unicode"
)

// Buffer collects keystrokes and emits words at boundaries. onWord receives
// the word and the boundary rune that ended it (space, hyphen, bracket, …)
// so a replacement can retype that same character instead of a space, plus
// the word's flipOrig (see below).
type Buffer struct {
	mu    sync.Mutex
	chars []rune
	codes []uint16 // keycode that produced each rune in chars (same index)
	// flipOrig is what the user had typed before flipping the word in
	// progress with the hotkey, "" when it was not flipped. chars then hold
	// the flipped text (Seed) and the rest of the word is appended to it, so
	// the word reaches onWord whole: "щер" + hotkey + "er" → "other", not
	// a stray tail "er".
	flipOrig string
	onWord   func(word string, boundary rune, flipOrig string)
}

func NewBuffer(onWord func(word string, boundary rune, flipOrig string)) *Buffer {
	return &Buffer{
		chars:  make([]rune, 0, 64),
		codes:  make([]uint16, 0, 64),
		onWord: onWord,
	}
}

func (b *Buffer) Add(r rune, keycode uint16) {
	// Trace special chars only in verbose mode.
	if !('a' <= r && r <= 'z') && !('A' <= r && r <= 'Z') && !('а' <= r && r <= 'я') && !('А' <= r && r <= 'Я') && r != ' ' {
		vlog("Buffer.Add special char: %q (U+%04X)", string(r), r)
	}

	b.mu.Lock()
	var emit, flipOrig string
	if isWordBoundary(r) {
		if universalPunct[r] && len(b.chars) > 0 {
			b.chars = append(b.chars, r)
			emit = string(b.chars)
		} else if len(b.chars) > 0 {
			emit = string(b.chars)
		}
		flipOrig = b.flipOrig
		b.chars = b.chars[:0]
		b.codes = b.codes[:0]
		b.flipOrig = ""
	} else {
		b.chars = append(b.chars, r)
		b.codes = append(b.codes, keycode)
	}
	b.mu.Unlock()

	// Call onWord synchronously AFTER releasing the mutex to avoid deadlock
	// (callback may call buf.Clear() which needs the same mutex).
	// Synchronous call also prevents race conditions on shared Detector state.
	if emit != "" && b.onWord != nil {
		b.onWord(emit, r, flipOrig)
	}
}

// Seed replaces the word in progress with text — the hotkey's flip of it,
// typed by the same keys (codes) — and remembers orig, what the user had
// typed, until the word ends.
func (b *Buffer) Seed(text string, codes []uint16, orig string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.chars = append(b.chars[:0], []rune(text)...)
	b.codes = append(b.codes[:0], codes...)
	b.flipOrig = orig
}

func (b *Buffer) Backspace() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.chars) > 0 {
		b.chars = b.chars[:len(b.chars)-1]
	}
	if len(b.codes) > 0 {
		b.codes = b.codes[:len(b.codes)-1]
	}
	if len(b.chars) == 0 {
		b.flipOrig = ""
	}
}

func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.chars = b.chars[:0]
	b.codes = b.codes[:0]
	b.flipOrig = ""
}

// FlushWord returns the current buffered word plus the keycodes that produced
// each rune, and clears the buffer.
func (b *Buffer) FlushWord() (string, []uint16) {
	word, codes, _ := b.FlushFlipped()
	return word, codes
}

// FlushFlipped is FlushWord that also returns the word's flipOrig.
func (b *Buffer) FlushFlipped() (word string, codes []uint16, flipOrig string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	flipOrig = b.flipOrig
	b.flipOrig = ""
	if len(b.chars) == 0 {
		return "", nil, ""
	}
	word = string(b.chars)
	codes = append([]uint16(nil), b.codes...)
	b.chars = b.chars[:0]
	b.codes = b.codes[:0]
	return word, codes, flipOrig
}

func isWordBoundary(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	// QWERTY punctuation that maps to Russian letters — NOT boundaries
	if qwertyRuPunct[r] {
		return false
	}
	// Shifted number keys that map to Russian punctuation — NOT boundaries
	if _, ok := shiftedRuPunct[r]; ok {
		return false
	}
	// Russian-layout-only symbols (№ from Shift+3) — NOT boundaries, they stay
	// in the pending word so the manual hotkey can flip them (№ → #).
	if _, ok := ruLayoutFlips[r]; ok {
		return false
	}
	return true
}
