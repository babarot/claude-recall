package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeRemote(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"https://github.com/babarot/claude-recall.git", "github.com/babarot/claude-recall"},
		{"https://github.com/babarot/claude-recall", "github.com/babarot/claude-recall"},
		{"https://github.com/babarot/claude-recall/", "github.com/babarot/claude-recall"},
		{"https://x-access-token:ghp_secret@github.com/babarot/claude-recall.git", "github.com/babarot/claude-recall"},
		{"git@github.com:babarot/claude-recall.git", "github.com/babarot/claude-recall"},
		{"github.com:babarot/claude-recall", "github.com/babarot/claude-recall"},
		{"ssh://git@github.com/babarot/claude-recall", "github.com/babarot/claude-recall"},
		{"ssh://git@GitHub.com:22/babarot/claude-recall.git", "github.com/babarot/claude-recall"},
		{"ssh://git@ssh.github.com:443/babarot/claude-recall.git", "github.com/babarot/claude-recall"},
		{"https://gitlab.example.com/group/sub/project.git", "gitlab.example.com/group/sub/project"},
		{"/srv/git/project.git", "/srv/git/project.git"},
		{"file:///srv/git/project.git", "/srv/git/project.git"},
		{"../project", "../project"},
		{"", ""},
	} {
		if got := NormalizeRemote(tt.in); got != tt.want {
			t.Errorf("NormalizeRemote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const originConfig = `[core]
	bare = false
[remote "upstream"]
	url = https://github.com/someone/else.git
[Remote "origin"]
	URL = "git@github.com:babarot/claude-recall.git" ; the fork
	fetch = +refs/heads/*:refs/remotes/origin/*
`

func TestRemote(t *testing.T) {
	root := t.TempDir()

	main := filepath.Join(root, "main")
	writeFile(t, filepath.Join(main, ".git", "config"), originConfig)
	sub := filepath.Join(main, "internal", "db")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	// A linked worktree reads the config of the common dir.
	wt := filepath.Join(root, "wt")
	admin := filepath.Join(main, ".git", "worktrees", "wt")
	writeFile(t, filepath.Join(wt, ".git"), "gitdir: "+admin+"\n")
	writeFile(t, filepath.Join(admin, "commondir"), "../..\n")

	// A submodule's git dir has its own config and no commondir.
	mod := filepath.Join(root, "mod")
	modGit := filepath.Join(main, ".git", "modules", "mod")
	writeFile(t, filepath.Join(mod, ".git"), "gitdir: "+modGit+"\n")
	writeFile(t, filepath.Join(modGit, "config"), "[remote \"origin\"]\n\turl = https://github.com/babarot/mod\n")

	noOrigin := filepath.Join(root, "no-origin")
	writeFile(t, filepath.Join(noOrigin, ".git", "config"), "[core]\n\tbare = false\n")

	noConfig := filepath.Join(root, "no-config")
	if err := os.MkdirAll(filepath.Join(noConfig, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	plain := filepath.Join(root, "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name, dir, want string
		ok              bool
	}{
		{"checkout", main, "github.com/babarot/claude-recall", true},
		{"directory in a checkout", sub, "github.com/babarot/claude-recall", true},
		{"linked worktree", wt, "github.com/babarot/claude-recall", true},
		{"submodule", mod, "github.com/babarot/mod", true},
		{"no origin", noOrigin, "", true},
		{"unreadable config", noConfig, "", false},
		{"removed directory", filepath.Join(root, "gone"), "", false},
		{"no directory", "", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Remote(tt.dir)
			if got != tt.want || ok != tt.ok {
				t.Errorf("Remote(%q) = %q, %v, want %q, %v", tt.dir, got, ok, tt.want, tt.ok)
			}
		})
	}
	// Outside git: known, with no remote. The temp dir may sit inside a
	// checkout on some machines, so only check that it is known.
	if _, ok := Remote(plain); !ok {
		t.Errorf("Remote(%q) not known", plain)
	}
}
