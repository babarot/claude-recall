package worktree

import (
	"path/filepath"
	"testing"
)

func TestShortPath(t *testing.T) {
	home := "/Users/me"
	cases := map[string]string{
		"/Users/me/src/github.com/me/repo": "me/repo",
		"/Users/me/.herdr/worktrees/x":     "~/.herdr/worktrees/x",
		"/Users/me":                        "~",
		"/opt/work":                        "/opt/work",
	}
	for in, want := range cases {
		if got := ShortPath(in, home); got != want {
			t.Errorf("ShortPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRemovedWorktree(t *testing.T) {
	home := "/Users/me"
	cases := []struct{ path, repo, name, key string }{
		{"/Users/me/.herdr/worktrees/dotfiles/worktree-brave-stone-cc30", "dotfiles", "brave-stone-cc30", HerdrKey + "dotfiles"},
		{"/Users/me/src/github.com/me/app/.claude/worktrees/fix-login", "me/app", "fix-login", "/Users/me/src/github.com/me/app"},
	}
	for _, c := range cases {
		repo, name, key, ok := RemovedWorktree(c.path, home)
		if !ok || repo != c.repo || name != c.name || key != c.key {
			t.Errorf("RemovedWorktree(%s) = %q %q %q %v", c.path, repo, name, key, ok)
		}
	}
	if _, _, _, ok := RemovedWorktree("/Users/me/src/github.com/me/app", home); ok {
		t.Error("a plain folder is not a worktree")
	}
}

// A linked worktree and its main checkout are one repository; a removed
// herdr worktree is one once SettleKeys matches it by name.
func TestRepo(t *testing.T) {
	main, wt := layout(t)
	r := NewResolver()
	home := filepath.Dir(filepath.Dir(main))

	inMain, inWt := r.Repo(main, home), r.Repo(wt, home)
	if inMain.Key != main || inWt.Key != main || inWt.Worktree != "feature" || inWt.MainRoot != main {
		t.Fatalf("main %+v, worktree %+v", inMain, inWt)
	}
	if got := r.KeyOf(wt); got != main {
		t.Errorf("KeyOf(worktree) = %q, want %q", got, main)
	}

	gone := r.Repo(filepath.Join(home, ".herdr/worktrees/repo/worktree-old"), home)
	if gone.Exists || gone.Key != HerdrKey+"repo" || gone.Name != "repo" || gone.Worktree != "old" {
		t.Fatalf("removed herdr worktree %+v", gone)
	}
	settled := SettleKeys([]string{inMain.Key, gone.Key})
	if settled[gone.Key] != main {
		t.Errorf("SettleKeys = %v, want %s for %s", settled, main, gone.Key)
	}
	if len(SettleKeys([]string{gone.Key})) != 0 {
		t.Error("a placeholder with no checkout of its name stays as it is")
	}
}
