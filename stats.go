package main

// stats.go — the "time saved" counter shown in the tray menu and the About
// panel.
//
// Every automatic fix (wrong layout, spelling, abbreviation, learned rule)
// spares the user the manual round trip, estimated per word as
//
//	notice the garbage and select the word        statsNoticeSeconds
//	switch the layout there and back              statsLayoutSeconds
//	retype the word                               statsCharSeconds × runes
//
// minus what Bzz's own fix costs (the retype the user watches at the word
// boundary, statsAutoSeconds). A five-letter word comes out at ≈3 s saved.
// The estimate accumulates from the first launch that carried the counter.
// A fix the user rolls back (hotkey revert, retyping the original) is
// subtracted with the same estimate: it cost time instead of saving it.
//
// Storage: stats.json next to exceptions.json, same atomic-write pattern,
// written on every change (a few times a minute at most). Thread-safe.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	statsFileName      = "stats.json"
	statsNoticeSeconds = 1.5  // spot the wrong word, move the caret / double-click it
	statsLayoutSeconds = 0.5  // switch the layout and back
	statsCharSeconds   = 0.25 // retype one letter (≈4 chars/s)
	statsAutoSeconds   = 0.3  // Bzz's own retype at the word boundary
)

type statsFile struct {
	Since        time.Time `json:"since"`
	Fixes        int64     `json:"fixes"`
	Reverts      int64     `json:"reverts"`
	SavedSeconds float64   `json:"saved_seconds"`
}

// savedByFix estimates the seconds one automatic fix of word saved.
func savedByFix(word string) float64 {
	return statsNoticeSeconds + statsLayoutSeconds + statsCharSeconds*float64(len([]rune(word))) - statsAutoSeconds
}

// Stats counts automatic fixes and their reverts. A nil *Stats is a no-op.
type Stats struct {
	mu   sync.Mutex
	path string
	data statsFile
	now  func() time.Time
}

// NewStats loads the counter from the config dir, starting a fresh one dated
// now when the file is missing or unreadable.
func NewStats() (*Stats, error) {
	dir, err := defaultConfigDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Stats{path: filepath.Join(dir, statsFileName), now: time.Now}
	raw, err := os.ReadFile(s.path)
	switch {
	case err == nil:
		if json.Unmarshal(raw, &s.data) != nil || s.data.Since.IsZero() {
			s.data = statsFile{}
		}
	case !errors.Is(err, os.ErrNotExist):
		return nil, err
	}
	if s.data.Since.IsZero() {
		s.data.Since = s.now().UTC()
		if err := s.persistLocked(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Fixed records one automatic fix of word (as the user typed it).
func (s *Stats) Fixed(word string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Fixes++
	s.data.SavedSeconds += savedByFix(word)
	s.persistLocked()
}

// Reverted records that the user undid an automatic fix of word.
func (s *Stats) Reverted(word string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Reverts++
	s.data.SavedSeconds -= savedByFix(word)
	if s.data.SavedSeconds < 0 {
		s.data.SavedSeconds = 0
	}
	s.persistLocked()
}

// Snapshot returns the start date, the net number of fixes and the time saved.
func (s *Stats) Snapshot() (since time.Time, fixes int64, saved time.Duration) {
	if s == nil {
		return time.Time{}, 0, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fixes = s.data.Fixes - s.data.Reverts
	if fixes < 0 {
		fixes = 0
	}
	return s.data.Since, fixes, time.Duration(s.data.SavedSeconds * float64(time.Second))
}

// Summary is the line shown to the user: "с 01.01.2026 — сэкономлено 2 ч 15 мин".
func (s *Stats) Summary() string {
	since, _, saved := s.Snapshot()
	if since.IsZero() {
		return ""
	}
	return fmt.Sprintf("с %s — сэкономлено %s", since.Local().Format("02.01.2006"), formatSaved(saved))
}

// formatSaved renders a duration in whole days, hours and minutes, dropping
// leading zero units; under a minute it says so.
func formatSaved(d time.Duration) string {
	minutes := int64(d / time.Minute)
	if minutes < 1 {
		return "меньше минуты"
	}
	days, hours, mins := minutes/(24*60), minutes%(24*60)/60, minutes%60
	switch {
	case days > 0:
		return fmt.Sprintf("%d д %d ч %d мин", days, hours, mins)
	case hours > 0:
		return fmt.Sprintf("%d ч %d мин", hours, mins)
	}
	return fmt.Sprintf("%d мин", mins)
}

func (s *Stats) persistLocked() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
