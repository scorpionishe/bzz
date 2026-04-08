package main

import (
	"log"
	"sync"
	"unicode"
)

// Buffer collects keystrokes and emits words at boundaries
type Buffer struct {
	mu      sync.Mutex
	chars   []rune
	onWord  func(word string)
}

func NewBuffer(onWord func(string)) *Buffer {
	return &Buffer{
		chars:  make([]rune, 0, 64),
		onWord: onWord,
	}
}

func (b *Buffer) Add(r rune) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Debug: log non-letter chars to understand what hook sends
	if !('a' <= r && r <= 'z') && !('A' <= r && r <= 'Z') && !('а' <= r && r <= 'я') && !('А' <= r && r <= 'Я') && r != ' ' {
		log.Printf("Buffer.Add special char: %q (U+%04X)", string(r), r)
	}

	if isWordBoundary(r) {
		// If it's ! or ? right after a word, attach it to the word
		if universalPunct[r] && len(b.chars) > 0 {
			b.chars = append(b.chars, r)
			word := string(b.chars)
			b.chars = b.chars[:0]
			if b.onWord != nil {
				go b.onWord(word)
			}
			return
		}
		if len(b.chars) > 0 {
			word := string(b.chars)
			b.chars = b.chars[:0]
			if b.onWord != nil {
				go b.onWord(word)
			}
		}
		return
	}
	b.chars = append(b.chars, r)
}

func (b *Buffer) Backspace() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.chars) > 0 {
		b.chars = b.chars[:len(b.chars)-1]
	}
}

func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.chars = b.chars[:0]
}

// FlushWord returns the current buffered word and clears the buffer.
func (b *Buffer) FlushWord() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.chars) == 0 {
		return ""
	}
	word := string(b.chars)
	b.chars = b.chars[:0]
	return word
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
	return true
}
