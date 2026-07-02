package main

import "testing"

func TestLookupAbbrev(t *testing.T) {
	cases := map[string]struct {
		want   string
		wantOK bool
	}{
		"n.l.": {"т.д.", true}, // и так далее — the reported case
		"n.g.": {"т.п.", true},
		"n.t.": {"т.е.", true},
		"n.r.": {"т.к.", true},
		"lh.":  {"др.", true},
		"gh.":  {"пр.", true},
		"N.L.": {"т.д.", true}, // case-insensitive
		// non-abbreviations must not match
		"hello":  {"", false},
		"ghbdtn": {"", false},
		"n.l":    {"", false}, // missing trailing dot
		"":       {"", false},
	}
	for word, c := range cases {
		got, ok := lookupAbbrev(word)
		if ok != c.wantOK || got != c.want {
			t.Errorf("lookupAbbrev(%q) = (%q, %v), want (%q, %v)", word, got, ok, c.want, c.wantOK)
		}
	}
}
