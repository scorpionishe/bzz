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

func TestIsPotentialSingleLetterRu(t *testing.T) {
	in := map[string]bool{
		"z": true, "f": true, "d": true, "j": true,
		"r": true, "c": true, "b": true, "e": true,
		// not mapped
		"a": false, "q": false, "x": false, "": false, "zz": false,
		// digits / punct
		"1": false, "?": false,
	}
	for word, want := range in {
		if got := isPotentialSingleLetterRu(word); got != want {
			t.Errorf("isPotentialSingleLetterRu(%q) = %v, want %v", word, got, want)
		}
	}
}
