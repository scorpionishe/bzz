package main

import (
	"sync"
	"unicode"
)

// Buffer collects keystrokes and emits words at boundaries
type Buffer struct {
	mu      sync.Mutex
	chars   []rune
	codes   []uint16 // keycode that produced each rune in chars (same index)
	onWord  func(word string)
}

func NewBuffer(onWord func(string)) *Buffer {
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
	var emit string
	if isWordBoundary(r) {
		if universalPunct[r] && len(b.chars) > 0 {
			b.chars = append(b.chars, r)
			emit = string(b.chars)
			b.chars = b.chars[:0]
			b.codes = b.codes[:0]
		} else if len(b.chars) > 0 {
			emit = string(b.chars)
			b.chars = b.chars[:0]
			b.codes = b.codes[:0]
		}
	} else {
		b.chars = append(b.chars, r)
		b.codes = append(b.codes, keycode)
	}
	b.mu.Unlock()

	// Call onWord synchronously AFTER releasing the mutex to avoid deadlock
	// (callback may call buf.Clear() which needs the same mutex).
	// Synchronous call also prevents race conditions on shared Detector state.
	if emit != "" && b.onWord != nil {
		b.onWord(emit)
	}
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
}

func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.chars = b.chars[:0]
	b.codes = b.codes[:0]
}

// FlushWord returns the current buffered word plus the keycodes that produced
// each rune, and clears the buffer.
func (b *Buffer) FlushWord() (string, []uint16) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.chars) == 0 {
		return "", nil
	}
	word := string(b.chars)
	codes := append([]uint16(nil), b.codes...)
	b.chars = b.chars[:0]
	b.codes = b.codes[:0]
	return word, codes
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
