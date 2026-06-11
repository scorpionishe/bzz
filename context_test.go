package main

import "testing"

func TestLooksLikeContext(t *testing.T) {
	cases := []struct {
		in   string
		want bool
		why  string
	}{
		// Should skip (looks like URL/email/path/identifier)
		{"https://example.com", true, "URL with :/"},
		{"user@gmail.com", true, "email"},
		{"/var/log", true, "path"},
		{"C:\\Users", true, "windows path"},
		{"github.com/zlopixatel/bzz", true, "slash + dots"},
		{"user_name", true, "snake_case"},
		{"my_var_1", true, "snake_case digit"},
		{"version1.2", true, "digit + dot"},
		{"node_modules/lodash", true, "underscore + slash"},
		{"google.com.au", true, "two dots tld"},
		{"v1.2.3", true, "version triplet"},

		// Should NOT skip (normal word, even with one hyphen or single dot at end)
		{"", false, "empty"},
		{"hello", false, "plain word"},
		{"ghbdtn", false, "plain QWERTY-RU candidate"},
		{"co-op", false, "single hyphen"},
		{"don't", false, "apostrophe inside"},
		{"hello,", false, "trailing comma (handled elsewhere)"},
	}
	for _, c := range cases {
		got := looksLikeContext(c.in)
		if got != c.want {
			t.Errorf("looksLikeContext(%q) = %v, want %v (%s)", c.in, got, c.want, c.why)
		}
	}
}

// TestShouldSkipWord_SingleLetterBypass verifies that single-letter QWERTY
// codes mapped to Russian letters bypass MinWordLength=2.
func TestShouldSkipWord_SingleLetterBypass(t *testing.T) {
	cfg := &Config{MinWordLength: 2}

	cases := map[string]bool{ // word → expect skip
		// single letters that map to Russian (bypass MinWordLength)
		"z": false, "f": false, "d": false, "j": false,
		"r": false, "c": false, "b": false, "e": false,
		// single letters that don't map (no bypass → skipped by MinWordLength)
		"a": true, "q": true, "x": true,
		// empty / longer words: empty is shorter than 2 → skipped
		"": true,
		// digits / punct: not in singleLetterRu → skipped
		"1": true, "?": true,
	}
	for word, want := range cases {
		got, _ := shouldSkipWord(cfg, nil, word)
		if got != want {
			t.Errorf("shouldSkipWord(%q) skip=%v, want %v", word, got, want)
		}
	}
}
