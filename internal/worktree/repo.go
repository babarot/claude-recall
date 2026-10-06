package worktree

import (
	"cmp"
	"path/filepath"
	"regexp"
	"strings"
)

// HerdrKey marks the key of a removed herdr worktree, whose path names only
// the repository; SettleKeys replaces it with that repository's checkout
// when the other sessions show which one it is.
const HerdrKey = "herdr:"

var (
	// herdr: ~/.herdr/worktrees/<repo>/worktree-<name>
	herdrWorktree = regexp.MustCompile(`/\.herdr/worktrees/([^/]+)/(?:worktree-)?([^/]+)$`)
	// Claude Code: <repo>/.claude/worktrees/<name>
	claudeWorktree = regexp.MustCompile(`^(.+)/\.claude/worktrees/([^/]+)$`)
)

// Repo is the repository a session directory belongs to, as the TUI, the
// CLI and the MCP server all show it.
type Repo struct {
	Exists bool // the directory is still on disk
	// MainRoot is the main checkout of a linked worktree, empty otherwise.
	MainRoot string
	// Folder is where the session ran, shortened: the main checkout for a
	// worktree, the repository's name for a removed one.
	Folder string
	// Worktree is the worktree's name, empty outside a linked worktree.
	Worktree string
	// Key is what the sessions of one repository share: the main checkout's
	// real path, the checkout itself, the directory outside git, or HerdrKey
	// and the repository's name for a removed herdr worktree.
	Key string
	// Name is how the repository shows: Key shortened, or the name of a
	// removed herdr worktree's repository.
	Name string
}

// Repo resolves the repository of a session directory. A removed worktree,
// which git can no longer resolve, is recognized by where herdr or Claude
// Code put it.
func (r *Resolver) Repo(dir, home string) Repo {
	info := r.Resolve(dir)
	out := Repo{Exists: info.Exists, Key: RealPath(dir)}
	switch {
	case info.IsWorktree():
		out.MainRoot = info.MainRoot
		out.Folder = ShortPath(info.MainRoot, home)
		out.Worktree = Name(info.Root)
		out.Key = RealPath(info.MainRoot)
	case !info.Exists:
		if repo, name, key, ok := RemovedWorktree(dir, home); ok {
			out.Folder, out.Worktree, out.Key = repo, name, key
			break
		}
		out.Folder = ShortPath(dir, home)
	default:
		out.Folder = ShortPath(dir, home)
		if info.Root != "" {
			out.Key = info.Root
		}
	}
	out.Name = ShortPath(out.Key, home)
	if repo, ok := strings.CutPrefix(out.Key, HerdrKey); ok {
		out.Name = repo
	}
	return out
}

// KeyOf is the Key of the repository dir is in, for narrowing to it: the
// same key Repo gives the sessions that ran there.
func (r *Resolver) KeyOf(dir string) string {
	info := r.Resolve(dir)
	return cmp.Or(RealPath(info.MainRoot), info.Root, RealPath(dir))
}

// SettleKeys maps each HerdrKey placeholder among keys to the one other key
// whose directory has that repository's name. A placeholder that matches
// none, or more than one, is left out and stays as it is.
func SettleKeys(keys []string) map[string]string {
	byBase := map[string][]string{}
	seen := map[string]bool{}
	for _, k := range keys {
		if !strings.HasPrefix(k, HerdrKey) && !seen[k] {
			seen[k] = true
			byBase[filepath.Base(k)] = append(byBase[filepath.Base(k)], k)
		}
	}
	out := map[string]string{}
	for _, k := range keys {
		if repo, ok := strings.CutPrefix(k, HerdrKey); ok {
			if found := byBase[repo]; len(found) == 1 {
				out[k] = found[0]
			}
		}
	}
	return out
}

// RemovedWorktree guesses the repository and worktree name of a worktree
// directory that no longer exists, from where herdr or Claude Code put it,
// and the key it belongs to. herdr's path holds only the repository's name,
// not its owner, so its key is a placeholder SettleKeys resolves.
func RemovedWorktree(path, home string) (repo, name, key string, ok bool) {
	if m := claudeWorktree.FindStringSubmatch(path); m != nil {
		return ShortPath(m[1], home), m[2], m[1], true
	}
	if m := herdrWorktree.FindStringSubmatch(path); m != nil {
		return m[1], m[2], HerdrKey + m[1], true
	}
	return "", "", "", false
}

// ShortPath drops the ~/src/github.com/ prefix that ghq-style checkouts
// share, and otherwise shortens $HOME to ~.
func ShortPath(path, home string) string {
	if path == "" {
		return "?"
	}
	if home != "" {
		if rest, ok := strings.CutPrefix(path, home+"/src/github.com/"); ok {
			return rest
		}
		if path == home {
			return "~"
		}
		if rest, ok := strings.CutPrefix(path, home+"/"); ok {
			return "~/" + rest
		}
	}
	return path
}

// RealPath resolves symlinks in an existing path, so one folder reached two
// ways is one key; other paths stay as they are.
func RealPath(p string) string {
	if p == "" {
		return ""
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return p
}

// Name is a worktree directory's name without the "worktree-" prefix some
// tools add.
func Name(root string) string {
	return strings.TrimPrefix(filepath.Base(root), "worktree-")
}
