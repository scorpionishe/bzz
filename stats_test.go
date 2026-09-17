package main

import (
	"testing"
	"time"
)

func TestStatsCountsAndPersists(t *testing.T) {
	t.Setenv("BZZ_CONFIG_DIR", t.TempDir())
	s, err := NewStats()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		s.Fixed("ghbdtn") // 6 runes: 1.5 + 0.5 + 1.5 - 0.3 = 3.2 s
	}
	s.Reverted("ghbdtn")
	since, fixes, saved := s.Snapshot()
	if since.IsZero() || fixes != 99 {
		t.Fatalf("snapshot = %v, %d; want dated, 99", since, fixes)
	}
	if want := time.Duration(99 * 3.2 * float64(time.Second)); (saved - want).Abs() > time.Millisecond {
		t.Fatalf("saved = %v, want %v", saved, want)
	}
	s2, err := NewStats()
	if err != nil {
		t.Fatal(err)
	}
	since2, fixes2, saved2 := s2.Snapshot()
	if !since2.Equal(since) || fixes2 != 99 || saved2 != saved {
		t.Fatalf("reload = %v, %d, %v; want %v, 99, %v", since2, fixes2, saved2, since, saved)
	}
}

func TestStatsNilAndFormat(t *testing.T) {
	var s *Stats
	s.Fixed("ghbdtn")
	s.Reverted("ghbdtn")
	if s.Summary() != "" {
		t.Fatal("nil stats must have an empty summary")
	}
	cases := map[time.Duration]string{
		30 * time.Second:             "меньше минуты",
		15 * time.Minute:             "15 мин",
		2*time.Hour + 15*time.Minute: "2 ч 15 мин",
		26*time.Hour + 3*time.Minute: "1 д 2 ч 3 мин",
		49 * time.Hour:               "2 д 1 ч 0 мин",
	}
	for d, want := range cases {
		if got := formatSaved(d); got != want {
			t.Errorf("formatSaved(%v) = %q, want %q", d, got, want)
		}
	}
}
