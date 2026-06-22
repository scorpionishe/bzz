package main

import "strings"

// CGEventFlags modifier masks (the four general modifier bits).
const (
	flagShift   int64 = 1 << 17
	flagControl int64 = 1 << 18
	flagAlt     int64 = 1 << 19
	flagCommand int64 = 1 << 20
	// modAll masks just the modifier bits so we can compare the pressed
	// modifiers exactly, ignoring device/caps/fn bits in the raw flags.
	modAll = flagShift | flagControl | flagAlt | flagCommand
)

// hotkeyKeycodes maps key names (as written in config) to macOS virtual keycodes.
var hotkeyKeycodes = map[string]uint16{
	// letters
	"a": 0x00, "b": 0x0B, "c": 0x08, "d": 0x02, "e": 0x0E, "f": 0x03,
	"g": 0x05, "h": 0x04, "i": 0x22, "j": 0x26, "k": 0x28, "l": 0x25,
	"m": 0x2E, "n": 0x2D, "o": 0x1F, "p": 0x23, "q": 0x0C, "r": 0x0F,
	"s": 0x01, "t": 0x11, "u": 0x20, "v": 0x09, "w": 0x0D, "x": 0x07,
	"y": 0x10, "z": 0x06,
	// digits
	"0": 0x1D, "1": 0x12, "2": 0x13, "3": 0x14, "4": 0x15,
	"5": 0x17, "6": 0x16, "7": 0x1A, "8": 0x1C, "9": 0x19,
	// function keys (f18/f19 are common "dedicated" targets — no physical key)
	"f1": 0x7A, "f2": 0x78, "f3": 0x63, "f4": 0x76, "f5": 0x60, "f6": 0x61,
	"f7": 0x62, "f8": 0x64, "f9": 0x65, "f10": 0x6D, "f11": 0x67, "f12": 0x6F,
	"f13": 0x69, "f14": 0x6B, "f15": 0x71, "f16": 0x6A, "f17": 0x40,
	"f18": 0x4F, "f19": 0x50, "f20": 0x5A,
	// misc
	"space": 0x31, "tab": 0x30, "return": 0x24, "enter": 0x24,
	"escape": 0x35, "esc": 0x35, "delete": 0x33,
}

// parseHotkey parses a string like "cmd+shift+x", "ctrl+space" or "f18" into a
// keycode plus an exact modifier mask. Modifier aliases: cmd/command,
// ctrl/control, opt/option/alt, shift. Returns ok=false on an unknown key.
func parseHotkey(s string) (keycode uint16, mods int64, ok bool) {
	for _, raw := range strings.Split(strings.ToLower(strings.TrimSpace(s)), "+") {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		switch p {
		case "cmd", "command", "meta", "super":
			mods |= flagCommand
		case "shift":
			mods |= flagShift
		case "ctrl", "control":
			mods |= flagControl
		case "opt", "option", "alt":
			mods |= flagAlt
		default:
			kc, found := hotkeyKeycodes[p]
			if !found {
				return 0, 0, false
			}
			keycode = kc
			ok = true
		}
	}
	return keycode, mods, ok
}
