package filter

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in        string
		words, at []string
	}{
		{"login bug", []string{"login", "bug"}, nil},
		{"in:Dotfiles nix", []string{"nix"}, []string{"dotfiles"}},
		{"in:a in:b", nil, []string{"a", "b"}},
		{"in: http://x", []string{"http://x"}, nil}, // an empty in: is nothing, other keys are words
		{"folder:Dotfiles nix", []string{"nix"}, []string{"dotfiles"}},
		{"folder:a in:b", nil, []string{"a", "b"}}, // in: is folder: for short
	}
	for _, c := range cases {
		q := Parse(c.in)
		if !reflect.DeepEqual(q.Words, c.words) || !reflect.DeepEqual(q.In, c.at) {
			t.Errorf("Parse(%q) = %q %q", c.in, q.Words, q.In)
		}
	}
}

func TestBodyWords(t *testing.T) {
	got := Parse("a retry text:x title:t").BodyWords()
	if want := []string{"retry", "x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("BodyWords = %q, want %q", got, want)
	}
}

func TestMatch(t *testing.T) {
	f := Fields{Title: "Fix the Parser", Folder: "babarot/claude-recall", Worktree: "silver-valley",
		Branch: "feat/web", ID: "AbCd1234-5678", Search: "fix the parser babarot/claude-recall silver-valley feat/web abcd1234-5678"}
	said := map[string]bool{"retry": true}
	cases := map[string]bool{
		"":                      true,
		"parser":                true,
		"retry":                 true, // in the conversation only
		"nowhere":               false,
		"text:retry":            true,
		"text:parser":           false, // text: looks in the conversation only
		"title:parser":          true,
		"title:x title:fix":     true, // either title
		"branch:web":            true,
		"branch:main":           false,
		"worktree:valley":       true,
		"id:abcd":               true,
		"id:5678":               false, // the start of the ID only
		"in:crec":               true,  // folders match fuzzily
		"in:dotfiles":           false,
		"in:crec title:nothing": false,
	}
	for q, want := range cases {
		if got := Match(Parse(q), f, func(w string) bool { return said[w] }); got != want {
			t.Errorf("Match(%q) = %v, want %v", q, got, want)
		}
	}
}
