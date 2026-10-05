//go:build darwin

package main

import "testing"

// Keys typed while a manual flip switches the layout were captured with the
// old layout's chars; the replay retypes them on the new one.
func TestReplayChar(t *testing.T) {
	cases := []struct {
		ch    rune
		flags int64
		sw    int32
		want  rune
	}{
		{'ф', 0, replayToEnglish, 'a'}, // "сду" + hotkey + "ar" → "clear", not "cleфк"
		{'к', 0, replayToEnglish, 'r'},
		{'Ф', flagShift, replayToEnglish, 'A'},
		{'a', 0, replayToRussian, 'ф'},
		{',', 0, replayToRussian, 'б'},
		{' ', 0, replayToEnglish, ' '},           // not a layout key
		{'ф', 0, replayNoSwitch, 'ф'},            // no switch → untouched
		{'с', flagCommand, replayToEnglish, 'с'}, // shortcut keeps its char
		{'с', flagControl, replayToEnglish, 'с'},
		{0, 0, replayToEnglish, 0}, // control key, no override
	}
	for _, c := range cases {
		if got := replayChar(c.ch, c.flags, c.sw); got != c.want {
			t.Errorf("replayChar(%q, %#x, %d) = %q, want %q", c.ch, c.flags, c.sw, got, c.want)
		}
	}
}
