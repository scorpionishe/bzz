//go:build !darwin

package main

import "errors"

// newSystemSpellChecker is darwin-only (NSSpellChecker); other platforms run
// without spelling correction.
func newSystemSpellChecker(lang string) (SpellChecker, error) {
	return nil, errors.New("system spell checker not available on this platform")
}
