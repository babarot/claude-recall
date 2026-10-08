package update

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// DefaultRawURL serves the repository's files at a tag.
const DefaultRawURL = "https://raw.githubusercontent.com/babarot/claude-recall"

// Release is one version's section of CHANGELOG.md.
type Release struct {
	Version, Date string
	Categories    []Category
}

// Category is a heading of a release's section (New Features, Bug fixes)
// and its entries, each a pull request's title and number: "Add x (#75)".
type Category struct {
	Name    string
	Entries []string
}

var (
	releaseLine = regexp.MustCompile(`^## \[([^\]]+)\]\([^)]*\)(?: - (\S+))?`)
	entryLine   = regexp.MustCompile(`^- (.+?) by @\S+ in \S+/pull/(\d+)\s*$`)
)

// ParseChangelog reads CHANGELOG.md as tagpr writes it, newest first:
//
//	## [1.7.2](https://github.com/.../compare/1.7.1...1.7.2) - 2026-10-07
//	### Bug fixes
//	- Do not let ... by @babarot in https://github.com/.../pull/71
//
// Lines of another shape are skipped, so a hand edit does not break it.
func ParseChangelog(src string) []Release {
	var out []Release
	sc := bufio.NewScanner(strings.NewReader(src))
	for sc.Scan() {
		l := sc.Text()
		if m := releaseLine.FindStringSubmatch(l); m != nil {
			out = append(out, Release{Version: m[1], Date: m[2]})
			continue
		}
		if len(out) == 0 {
			continue
		}
		r := &out[len(out)-1]
		switch m := entryLine.FindStringSubmatch(l); {
		case strings.HasPrefix(l, "### "):
			r.Categories = append(r.Categories, Category{Name: strings.TrimSpace(l[4:])})
		case m != nil:
			if len(r.Categories) == 0 {
				r.Categories = append(r.Categories, Category{Name: "Changes"})
			}
			c := &r.Categories[len(r.Categories)-1]
			c.Entries = append(c.Entries, m[1]+" (#"+m[2]+")")
		}
	}
	return out
}

// Changelog fetches CHANGELOG.md as of tag, which has every release up to
// it, and parses it.
func (c *Client) Changelog(ctx context.Context, rawURL, tag string) ([]Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL+"/"+tag+"/CHANGELOG.md", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CHANGELOG.md of %s: %s", tag, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	notes := ParseChangelog(string(b))
	if len(notes) == 0 {
		return nil, fmt.Errorf("CHANGELOG.md of %s has no releases", tag)
	}
	return notes, nil
}
