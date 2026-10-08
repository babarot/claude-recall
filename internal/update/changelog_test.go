package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func TestParseChangelog(t *testing.T) {
	src := `# Changelog

## [1.7.2](https://github.com/babarot/claude-recall/compare/1.7.1...1.7.2) - 2026-10-07
### Bug fixes
- Do not let an import replace a newer one with stale content by @babarot in https://github.com/babarot/claude-recall/pull/71
A stray line someone wrote by hand.

## [1.7.0](https://github.com/babarot/claude-recall/compare/1.6.2...1.7.0) - 2026-10-06
### New Features
- Support importing transcripts from more than one directory by @babarot in https://github.com/babarot/claude-recall/pull/67
### Improvements
- Add the first prompt to search results by @babarot in https://github.com/babarot/claude-recall/pull/64

## [0.1.0](https://github.com/babarot/claude-recall/commits/0.1.0) - 2026-09-01
- Old entry without a heading by @someone in https://github.com/babarot/agent-recall/pull/3
`
	want := []Release{
		{"1.7.2", "2026-10-07", []Category{{"Bug fixes", []string{"Do not let an import replace a newer one with stale content (#71)"}}}},
		{"1.7.0", "2026-10-06", []Category{
			{"New Features", []string{"Support importing transcripts from more than one directory (#67)"}},
			{"Improvements", []string{"Add the first prompt to search results (#64)"}},
		}},
		{"0.1.0", "2026-09-01", []Category{{"Changes", []string{"Old entry without a heading (#3)"}}}},
	}
	if got := ParseChangelog(src); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

// The repository's own CHANGELOG.md parses into every release it has. A
// release may list nothing (0.2.0 does); the newest lists something.
func TestParseRealChangelog(t *testing.T) {
	src, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	notes := ParseChangelog(string(src))
	if len(notes) < 10 {
		t.Fatalf("only %d releases", len(notes))
	}
	for _, r := range notes {
		if _, err := parse(r.Version); err != nil || r.Date == "" {
			t.Errorf("release %+v", r)
		}
	}
	if len(notes[0].Categories) == 0 || len(notes[0].Categories[0].Entries) == 0 {
		t.Errorf("the newest release lists nothing: %+v", notes[0])
	}
}

func TestChangelog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/1.7.2/CHANGELOG.md" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("## [1.7.2](x) - 2026-10-07\n### Bug fixes\n- Fix it by @a in https://x/pull/1\n"))
	}))
	defer srv.Close()
	c := &Client{}
	notes, err := c.Changelog(context.Background(), srv.URL, "1.7.2")
	if err != nil || len(notes) != 1 || notes[0].Categories[0].Entries[0] != "Fix it (#1)" {
		t.Fatalf("got %+v, %v", notes, err)
	}
	if _, err := c.Changelog(context.Background(), srv.URL, "9.9.9"); err == nil {
		t.Error("a missing tag is an error")
	}
}
