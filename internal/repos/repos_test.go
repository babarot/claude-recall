package repos

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/babarot/claude-recall/internal/db"
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

	// The removed worktree itself, or another removed one of the same
	// repository, narrows to the repository as the checkout does.
	for _, dir := range []string{removed, filepath.Join(home, ".herdr/worktrees/repo/worktree-never-seen")} {
		got := x.PathsIn(dir)
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("PathsIn(%s) = %v, want %v", dir, got, want)
		}
	}

	// A session with no directory has no repository, not a placeholder name.
	if r := x.Of(""); r.Name != "" || r.Key != "" {
		t.Errorf(`Of("") = %+v`, r)
	}
}

func writeOrigin(t *testing.T, gitDir, url string) {
	t.Helper()
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "[remote \"origin\"]\n\turl = " + url + "\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A directory names its repository by its origin as well: a worktree of
// another clone, with no sessions of its own yet, reaches the sessions of
// the user's checkout. A remote names it directly, and a directory since
// removed by the remote its sessions recorded.
func TestScope(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	checkout := filepath.Join(home, "src/github.com/acme/api")
	writeOrigin(t, filepath.Join(checkout, ".git"), "git@github.com:acme/api.git")
	clone := filepath.Join(home, ".cache/triage/repos/acme/api")
	writeOrigin(t, filepath.Join(clone, ".git"), "https://github.com/acme/api.git")
	wt := filepath.Join(home, ".cache/triage/worktrees/acme/api/42")
	admin := filepath.Join(clone, ".git/worktrees/42")
	if err := os.MkdirAll(admin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	for f, body := range map[string]string{filepath.Join(wt, ".git"): "gitdir: " + admin + "\n", filepath.Join(admin, "commondir"): "../..\n"} {
		if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plain := filepath.Join(home, "notes")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(home, "src/github.com/acme/web")

	x := New([]string{checkout, plain, gone}, home)
	x.setRemotes([]db.PathRemote{
		{Path: checkout, Remote: "github.com/acme/api", Ended: time.Unix(1, 0)},
		{Path: gone, Remote: "github.com/acme/web-old", Ended: time.Unix(1, 0)},
		{Path: gone, Remote: "github.com/acme/web", Ended: time.Unix(2, 0)},
		{Path: plain, Remote: "", Ended: time.Unix(1, 0)},
	})

	for _, tt := range []struct {
		value string
		want  db.Scope
	}{
		{wt, db.Scope{Remote: "github.com/acme/api", Paths: []string{}}},
		{checkout, db.Scope{Remote: "github.com/acme/api", Paths: []string{checkout}}},
		{"https://github.com/acme/api", db.Scope{Remote: "github.com/acme/api"}},
		{"github.com/acme/api", db.Scope{Remote: "github.com/acme/api"}},
		{gone, db.Scope{Remote: "github.com/acme/web", Paths: []string{gone}}},
		{plain, db.Scope{Paths: []string{plain}}},
	} {
		got := x.Scope(tt.value)
		if got.Remote != tt.want.Remote || !slices.Equal(got.Paths, tt.want.Paths) || (got.Paths == nil) != (tt.want.Paths == nil) {
			t.Errorf("Scope(%s) = %+v, want %+v", tt.value, got, tt.want)
		}
	}
}

// A pin is a remote, or a directory on disk whose origin is read now; one
// with no origin holds to the directory's own repository, not to a removed
// worktree only guessed to belong to it.
func TestPin(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	withOrigin := filepath.Join(home, "src/acme/api")
	writeOrigin(t, filepath.Join(withOrigin, ".git"), "git@github.com:acme/api.git")
	noOrigin := filepath.Join(home, "src/local/repo")
	if err := os.MkdirAll(filepath.Join(noOrigin, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(noOrigin, ".git", "config"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		value string
		want  Pin
	}{
		{"https://github.com/acme/api", Pin{Remote: "github.com/acme/api"}},
		{withOrigin, Pin{Remote: "github.com/acme/api"}},
		{filepath.Join(withOrigin, ".git"), Pin{Remote: "github.com/acme/api"}},
		{noOrigin, Pin{Dir: noOrigin}},
	} {
		if got, err := NewPin(tt.value); err != nil || got != tt.want {
			t.Errorf("NewPin(%s) = %+v, %v, want %+v", tt.value, got, err, tt.want)
		}
	}
	for _, bad := range []string{"", " ", filepath.Join(home, "gone"), filepath.Join(home, ".herdr/worktrees/repo/worktree-old")} {
		if p, err := NewPin(bad); err == nil {
			t.Errorf("NewPin(%q) = %+v, want an error", bad, p)
		}
	}

	removed := filepath.Join(home, ".herdr/worktrees/repo/worktree-old")
	x := New([]string{noOrigin, removed, withOrigin}, home)
	got := x.PinScope(Pin{Dir: noOrigin})
	if got.Remote != "" || !slices.Equal(got.Paths, []string{noOrigin}) {
		t.Errorf("PinScope = %+v, want the checkout alone, not the removed worktree", got)
	}
	if x.Of(removed).Key != noOrigin {
		t.Fatalf("the removed worktree should be guessed to be the checkout's for the test to mean anything: %+v", x.Of(removed))
	}
}
