package main

// learn.go — adaptive learning from manual hotkey conversions.
//
// Positive signal: the user manually flips a word bzz left untouched (hotkey
// on the pending word or on a single-word selection). After learnThreshold
// distinct signals the word becomes a RULE: it is force-converted at word
// boundaries even though the detector said "not wrong".
//
// Negative signal: the user manually flips BACK a word bzz just auto-converted
// (a selection flip that matches the last conversion). After learnThreshold
// reverts the rule is dropped and the caller is told to add an exception.
//
// False-positive protection (the "Ж → :" class of problems):
//   - learnable() gate: >= 2 runes, letters-only on BOTH sides, each side a
//     single script and the scripts differ, no URLs/identifiers, no known
//     abbreviations. Flips whose result is punctuation/symbols never learn.
//   - Dedup window: repeated signals for the same word within learnDedupWindow
//     count once — mashing the hotkey is not "3 repeats".
//   - Anti-toggle: flip A→B immediately followed by B→A is experimentation;
//     the first signal is cancelled and the second is not recorded.
//
// Storage: learned.json next to exceptions.json, same atomic-write pattern.
// Thread-safe: all public methods acquire the mutex.

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	learnFileName     = "learned.json"
	learnSchemaVer    = 1
	learnHardCap      = 5000
	learnPruneMinAge  = 30 * 24 * time.Hour
	learnDedupWindow  = 60 * time.Second
	learnToggleWindow = 10 * time.Second
	learnDefThreshold = 3
)

type LearnEntry struct {
	Word      string    `json:"word"`      // lowercase wrong-layout form as typed
	Direction string    `json:"direction"` // "en→ru" | "ru→en" (derived from script, kept for CLI/UI)
	Pos       int       `json:"pos"`       // manual-flip signals seen
	Neg       int       `json:"neg"`       // revert signals seen
	Rule      bool      `json:"rule"`      // active auto-convert rule
	Added     time.Time `json:"added"`
	LastEvent time.Time `json:"last_event"`
	Applied   int       `json:"applied"` // times the rule fired
}

type learnFile struct {
	Version int          `json:"version"`
	Updated time.Time    `json:"updated"`
	Entries []LearnEntry `json:"entries"`
}

// learnSignal remembers the last recorded flip so an immediate reverse flip
// (A→B then B→A) can cancel it instead of feeding the opposite counter.
type learnSignal struct {
	from, to string // lowercase, the flip as performed on screen
	entryKey string // which entry the signal touched
	positive bool   // which counter was incremented
	counted  bool   // false when the signal was swallowed by the dedup window
	at       time.Time
	valid    bool
}

type LearnStore struct {
	mu        sync.Mutex
	path      string
	index     map[string]*LearnEntry // key = lowercase word
	entries   []*LearnEntry          // ordered for persistence
	threshold int
	last      learnSignal
	now       func() time.Time // injectable for tests
}

// NewLearnStore loads the store from disk, creating parent dir if needed.
// Missing file → empty store; corrupt file → renamed to .corrupt.bak.
func NewLearnStore(threshold int) (*LearnStore, error) {
	if threshold <= 0 {
		threshold = learnDefThreshold
	}
	dir, err := defaultConfigDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &LearnStore{
		path:      filepath.Join(dir, learnFileName),
		index:     make(map[string]*LearnEntry),
		threshold: threshold,
		now:       time.Now,
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *LearnStore) load() error {
	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var file learnFile
	if err := json.Unmarshal(data, &file); err != nil {
		_ = os.Rename(s.path, s.path+".corrupt.bak")
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range file.Entries {
		e := file.Entries[i]
		if e.Word == "" {
			continue
		}
		ptr := &e
		s.entries = append(s.entries, ptr)
		s.index[strings.ToLower(e.Word)] = ptr
	}
	return nil
}

// learnable is the gate that keeps garbage out of the store. Both the typed
// form and the converted form must be letters-only words (>= 2 runes) of a
// single script each, with DIFFERENT scripts — i.e. a genuine layout flip.
// Punctuation/symbol output ("Ж" → ":", "ЖЖ" → "::"), digits, URLs,
// identifiers and known abbreviations are all rejected.
func learnable(word, converted string) bool {
	if len([]rune(word)) < 2 {
		return false
	}
	if _, isAbbrev := lookupAbbrev(word); isAbbrev {
		return false
	}
	if looksLikeContext(word) {
		return false
	}
	ws, cs := lettersScript(word), lettersScript(converted)
	return ws != "" && cs != "" && ws != cs
}

// lettersScript returns "latin" or "cyrillic" when ALL runes of s are letters
// of that one script; otherwise "" (mixed scripts, digits, punctuation…).
func lettersScript(s string) string {
	script := ""
	for _, r := range s {
		var cur string
		switch {
		case unicode.Is(unicode.Latin, r):
			cur = "latin"
		case unicode.Is(unicode.Cyrillic, r):
			cur = "cyrillic"
		default:
			return ""
		}
		if script == "" {
			script = cur
		} else if script != cur {
			return ""
		}
	}
	return script
}

// flipWord converts a word to the other layout, direction chosen by script.
func flipWord(word string) string {
	if detectScript(word) == "cyrillic" {
		return RussianToQWERTY(word)
	}
	return QWERTYToRussian(word)
}

func directionOf(word string) string {
	if detectScript(word) == "cyrillic" {
		return "ru→en"
	}
	return "en→ru"
}

// RecordManualFlip registers a positive signal: the user converted word →
// converted by hotkey and bzz had left it alone. Returns true when the entry
// just crossed the threshold and became an active rule.
func (s *LearnStore) RecordManualFlip(word, converted string) (promoted bool) {
	if s == nil || !learnable(word, converted) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.ToLower(word)
	if s.cancelToggleLocked(key, strings.ToLower(converted)) {
		return false
	}
	e := s.getOrCreateLocked(key, directionOf(word))
	sig := learnSignal{
		from: key, to: strings.ToLower(converted),
		entryKey: key, positive: true, at: s.now(), valid: true,
	}
	if s.now().Sub(e.LastEvent) < learnDedupWindow {
		s.last = sig // counted=false: suppresses a reverse flip, cancels nothing
		vlog("LEARN dedup: %q within window, not counted", word)
		return false
	}
	e.Pos++
	e.LastEvent = s.now()
	sig.counted = true
	s.last = sig
	if !e.Rule && e.Pos >= s.threshold {
		e.Rule = true
		e.Neg = 0
		promoted = true
	}
	s.persistLocked()
	return promoted
}

// RecordRevert registers a negative signal: the user flipped replaced back to
// original, undoing an auto-conversion original → replaced. Returns true when
// the entry crossed the threshold — the rule is dropped and the caller should
// persist an exception for original.
func (s *LearnStore) RecordRevert(original, replaced string) (demoted bool) {
	if s == nil || !learnable(original, replaced) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.ToLower(original)
	// The on-screen flip was replaced → original.
	if s.cancelToggleLocked(strings.ToLower(replaced), key) {
		return false
	}
	e := s.getOrCreateLocked(key, directionOf(original))
	sig := learnSignal{
		from: strings.ToLower(replaced), to: key,
		entryKey: key, positive: false, at: s.now(), valid: true,
	}
	if s.now().Sub(e.LastEvent) < learnDedupWindow {
		s.last = sig
		vlog("LEARN dedup: revert %q within window, not counted", original)
		return false
	}
	e.Neg++
	e.LastEvent = s.now()
	sig.counted = true
	s.last = sig
	if e.Neg >= s.threshold {
		e.Rule = false
		e.Pos = 0
		demoted = true
	}
	s.persistLocked()
	return demoted
}

// cancelToggleLocked detects the reverse of the immediately preceding flip
// (experimentation: A→B, then B→A within learnToggleWindow). It rolls back
// the previous signal's counter and reports true — the current flip must not
// be recorded either.
func (s *LearnStore) cancelToggleLocked(from, to string) bool {
	l := s.last
	if !l.valid || s.now().Sub(l.at) > learnToggleWindow {
		return false
	}
	if l.from != to || l.to != from {
		return false
	}
	s.last.valid = false
	if l.counted {
		if e, ok := s.index[l.entryKey]; ok {
			if l.positive && e.Pos > 0 {
				e.Pos--
			} else if !l.positive && e.Neg > 0 {
				e.Neg--
			}
			s.persistLocked()
		}
	}
	vlog("LEARN toggle: %q ↔ %q cancelled", from, to)
	return true
}

// Rule returns the forced conversion for word when an active rule exists.
// Case-preserving: the conversion is recomputed from the word as typed.
func (s *LearnStore) Rule(word string) (converted string, ok bool) {
	if s == nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.index[strings.ToLower(word)]
	if !found || !e.Rule {
		return "", false
	}
	e.Applied++ // in-memory only; persisted on the next signal write
	return flipWord(word), true
}

func (s *LearnStore) getOrCreateLocked(key, direction string) *LearnEntry {
	if e, ok := s.index[key]; ok {
		return e
	}
	s.maybePruneLocked()
	e := &LearnEntry{Word: key, Direction: direction, Added: s.now().UTC()}
	s.entries = append(s.entries, e)
	s.index[key] = e
	return e
}

// Forget removes the entry (candidate or rule) for word. Returns count removed.
func (s *LearnStore) Forget(word string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	kept := s.entries[:0]
	for _, e := range s.entries {
		if strings.EqualFold(e.Word, word) {
			delete(s.index, strings.ToLower(e.Word))
			removed++
			continue
		}
		kept = append(kept, e)
	}
	s.entries = kept
	if removed == 0 {
		return 0, nil
	}
	return removed, s.persistLocked()
}

// Clear removes every entry.
func (s *LearnStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
	s.index = make(map[string]*LearnEntry)
	return s.persistLocked()
}

// List returns a copy of all entries, for CLI/UI.
func (s *LearnStore) List() []LearnEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]LearnEntry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, *e)
	}
	return out
}

// maybePruneLocked drops stale candidates (never promoted, no recent signals)
// once the store grows past the hard cap. Rules are never pruned.
func (s *LearnStore) maybePruneLocked() {
	if len(s.entries) < learnHardCap {
		return
	}
	cutoff := s.now().Add(-learnPruneMinAge)
	kept := s.entries[:0]
	for _, e := range s.entries {
		if !e.Rule && e.LastEvent.Before(cutoff) {
			delete(s.index, strings.ToLower(e.Word))
			continue
		}
		kept = append(kept, e)
	}
	s.entries = kept
}

func (s *LearnStore) persistLocked() error {
	out := learnFile{
		Version: learnSchemaVer,
		Updated: s.now().UTC(),
		Entries: make([]LearnEntry, len(s.entries)),
	}
	for i, e := range s.entries {
		out.Entries[i] = *e
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
