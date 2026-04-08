package main

import "time"

// KeyEvent represents a keyboard event from the hook
type KeyEvent struct {
	KeyCode uint16
	Char    rune
	Time    time.Time
}
