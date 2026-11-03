// Package repos tells which repository each session directory of the
// archive belongs to, the way the TUI groups them, for the CLI and the MCP
// server.
package repos

import (
	"os"
	"path/filepath"

	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/worktree"
)

// Index is the repository of every directory sessions ran in. A removed
// herdr worktree, which names only its repository, belongs to the checkout
// of that name when the archive has exactly one.
type Index struct {
	byPath  map[string]worktree.Repo
	settled map[string]string // a removed herdr worktree's placeholder key to its checkout's
	// remotes is the git remote of each key, as its latest session that
	// recorded one recorded it.
	remotes  map[string]string
	home     string
	resolver *worktree.Resolver
}

// Load resolves every session directory in d, and the git remote its
// sessions recorded.
func Load(d *db.DB) (*Index, error) {
	prs, err := d.PathRemotes()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var paths []string
	for _, p := range prs {
		if !seen[p.Path] {
			seen[p.Path] = true
			paths = append(paths, p.Path)
		}
	}
	x := New(paths, home())
	x.setRemotes(prs)
	return x, nil
}

// setRemotes records the remotes the sessions of each directory recorded,
// for Scope to tell the remote of a directory since removed.
func (x *Index) setRemotes(prs []db.PathRemote) {
	members := make([]worktree.Member, len(prs))
	for i, p := range prs {
		members[i] = worktree.Member{Key: x.Of(p.Path).Key, Remote: p.Remote, Ended: p.Ended}
	}
	x.remotes = worktree.LatestRemotes(members)
}

// New resolves paths against the disk, home shortening how they show.
func New(paths []string, home string) *Index {
	x := &Index{byPath: map[string]worktree.Repo{}, home: home, resolver: worktree.NewResolver()}
	keys := make([]string, 0, len(paths))
	names := map[string]string{}
	for _, p := range paths {
		r := x.resolver.Repo(p, home)
		x.byPath[p] = r
		keys = append(keys, r.Key)
		names[r.Key] = r.Name
	}
	x.settled = worktree.SettleKeys(keys)
	for p, r := range x.byPath {
		if key, ok := x.settled[r.Key]; ok {
			r.Key, r.Name = key, names[key]
			x.byPath[p] = r
		}
	}
	return x
}

// Of is the repository of a session directory. A directory the index does
// not hold is resolved on its own, a removed herdr worktree settled as the
// archive's are; a session with no directory has none.
func (x *Index) Of(path string) worktree.Repo {
	if path == "" {
		return worktree.Repo{}
	}
	if r, ok := x.byPath[path]; ok {
		return r
	}
	r := x.resolver.Repo(path, x.home)
	if key, ok := x.settled[r.Key]; ok {
		r.Key = key
		r.Name = worktree.ShortPath(key, x.home)
	}
	return r
}

// PathsIn returns the session directories of the repository dir is in:
// its checkout, its worktrees, removed ones included, whether dir is the
// checkout, a worktree, a directory inside one, or a worktree since
// removed. Never nil, so an empty result narrows a search to nothing.
func (x *Index) PathsIn(dir string) []string {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	key := x.Of(dir).Key
	out := []string{}
	for p, r := range x.byPath {
		if r.Key == key {
			out = append(out, p)
		}
	}
	return out
}

// Scope is the repository a --repo value names, to narrow sessions to (see
// db.Scope). A git remote (https://github.com/owner/repo,
// git@github.com:owner/repo.git, github.com/owner/repo) names the
// repository wherever it is checked out. A directory names the sessions of
// PathsIn, and its origin: the one on disk now, or for a directory since
// removed, the one its sessions recorded. So two checkouts of one
// repository are one, and one with no remote stays its directories.
func (x *Index) Scope(value string) db.Scope {
	if remote, ok := worktree.RemoteArg(value); ok {
		return db.Scope{Remote: remote}
	}
	dir := value
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	sc := db.Scope{Paths: x.PathsIn(dir)}
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		if remote, ok := worktree.Remote(dir); ok {
			sc.Remote = remote
			return sc
		}
	}
	sc.Remote = x.remotes[x.Of(dir).Key]
	return sc
}

func home() string {
	h, _ := os.UserHomeDir()
	return h
}
