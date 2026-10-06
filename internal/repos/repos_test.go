package repos

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A checkout and a removed herdr worktree of it are one repository, so
// narrowing to the checkout keeps the sessions of both.
func TestIndex(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	checkout := filepath.Join(home, "src/github.com/me/repo")
	other := filepath.Join(home, "src/github.com/me/other")
	for _, d := range []string{checkout, other} {
		if err := os.MkdirAll(filepath.Join(d, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	removed := filepath.Join(home, ".herdr/worktrees/repo/worktree-old")

	x := New([]string{checkout, removed, other}, home)
	if r := x.Of(removed); r.Key != checkout || r.Name != "me/repo" || r.Worktree != "old" {
		t.Fatalf("removed worktree: %+v", r)
	}
	if r := x.Of(checkout); r.Name != "me/repo" || r.Worktree != "" {
		t.Fatalf("checkout: %+v", r)
	}
	got, want := x.PathsIn(checkout), []string{checkout, removed}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("PathsIn = %v, want the checkout and the removed worktree %v", got, want)
	}
	if p := x.PathsIn(filepath.Join(home, "elsewhere")); p == nil || len(p) != 0 {
		t.Fatalf("a directory with no sessions narrows to nothing, got %v", p)
	}
}
