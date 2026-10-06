// Package watcher keeps the archive in step with ~/.claude/projects, and the
// other transcript trees the config file lists, while the web UI or the MCP
// server runs (see docs/adr/002).
//
// It polls file sizes and modification times instead of using filesystem
// notifications: on macOS, fsnotify's kqueue backend needs an open file
// descriptor for every transcript, and it does not watch directories
// recursively. A scan of a thousand transcripts costs a few milliseconds.
package watcher

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/importer"
	"github.com/babarot/claude-recall/internal/parser"
)

const (
	// DefaultDebounce is how long a file must stay unchanged before it is
	// imported, so a burst of writes leads to one import.
	DefaultDebounce = 300 * time.Millisecond
	pollInterval    = 250 * time.Millisecond
)

// Status is the watcher state reported by /api/status.
type Status struct {
	Enabled bool `json:"enabled"`
	Running bool `json:"running"`
	// ProjectsDir is the primary tree; ProjectsDirs every tree watched, the
	// primary first, and MissingDirs those not found at the last look.
	ProjectsDir  string   `json:"projectsDir"`
	ProjectsDirs []string `json:"projectsDirs"`
	MissingDirs  []string `json:"missingDirs,omitempty"`
	DebounceMs   int      `json:"debounceMs"`
	LastEventAt  string   `json:"lastEventAt,omitempty"`
	LastImportAt string   `json:"lastImportAt,omitempty"`
	LastError    string   `json:"lastError,omitempty"`
	LastErrorAt  string   `json:"lastErrorAt,omitempty"`
}

// Watcher imports changed transcripts.
type Watcher struct {
	DB *db.DB
	// ProjectsDirs are the transcript trees, the primary one first.
	ProjectsDirs []string
	Debounce     time.Duration
	// OnImport is called after a new or changed session was imported.
	OnImport func(*importer.Result)
	Log      io.Writer

	mu     sync.Mutex
	status Status
}

// Status returns a snapshot of the watcher state.
func (w *Watcher) Status() Status {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status
}

func (w *Watcher) update(f func(*Status)) {
	w.mu.Lock()
	f(&w.status)
	w.mu.Unlock()
}

func now() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") }

func (w *Watcher) fail(msg string) {
	w.update(func(s *Status) { s.LastError, s.LastErrorAt = msg, now() })
}

type fileState struct {
	size  int64
	mtime time.Time
}

// scan lists the transcripts of every tree, by path, as Discover finds
// them, so symlinks are followed as the importer follows them.
func scan(dirs []string) (map[string]fileState, []parser.File) {
	files := parser.Discover(dirs...)
	out := make(map[string]fileState, len(files))
	for _, f := range files {
		out[f.Path] = fileState{size: f.Size, mtime: f.ModTime}
	}
	return out, files
}

// missing are the trees of dirs that are not there.
func missing(dirs []string) []string {
	var out []string
	for _, d := range dirs {
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			out = append(out, d)
		}
	}
	return out
}

// Run watches until ctx is canceled. Files present when it starts are
// taken as already imported.
func (w *Watcher) Run(ctx context.Context) {
	if w.Debounce == 0 {
		w.Debounce = DefaultDebounce
	}
	if w.Log == nil {
		w.Log = os.Stderr
	}
	dirs := w.ProjectsDirs
	primary := ""
	if len(dirs) > 0 {
		primary = dirs[0]
	}
	w.update(func(s *Status) {
		*s = Status{Enabled: true, ProjectsDir: primary, ProjectsDirs: dirs, DebounceMs: int(w.Debounce / time.Millisecond)}
	})

	// A tree that is not there yet, as a container's before it first runs,
	// is looked for again on every tick; with none there, nothing is.
	gone := missing(dirs)
	switch {
	case len(dirs) == 1 && len(gone) == 1:
		if _, err := os.Stat(primary); err != nil {
			w.fail(primary + " not found")
			fmt.Fprintf(w.Log, "[watcher] %s not found; watcher disabled.\n", primary)
		} else {
			w.fail(primary + " is not a directory")
			fmt.Fprintf(w.Log, "[watcher] %s is not a directory; watcher disabled.\n", primary)
		}
		return
	case len(gone) == len(dirs):
		w.fail("none of " + strings.Join(dirs, ", ") + " found")
		fmt.Fprintf(w.Log, "[watcher] none of %s found; watcher disabled.\n", strings.Join(dirs, ", "))
		return
	}
	for _, d := range gone {
		fmt.Fprintf(w.Log, "[watcher] %s not found; watching the others.\n", d)
	}

	w.update(func(s *Status) { s.Running, s.MissingDirs = true, gone })
	defer w.update(func(s *Status) { s.Running = false })

	seen, _ := scan(dirs)
	pending := map[string]time.Time{} // path -> time of the last change
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		current, files := scan(dirs)
		if m := missing(dirs); !slices.Equal(m, gone) {
			gone = m
			w.update(func(s *Status) { s.MissingDirs = gone })
		}
		changed := false
		for path, st := range current {
			if old, ok := seen[path]; !ok || old != st {
				pending[path] = time.Now()
				changed = true
			}
		}
		seen = current
		if changed {
			w.update(func(s *Status) { s.LastEventAt = now() })
		}
		for path, at := range pending {
			if time.Since(at) < w.Debounce {
				continue
			}
			delete(pending, path)
			w.importSession(path, files)
		}
	}
}

// importSession imports the session of the transcript at path that
// changed: its copy the importer would choose, when it is in more than one
// tree, so a change to an older copy never replaces a newer one.
func (w *Watcher) importSession(path string, files []parser.File) {
	i := slices.IndexFunc(files, func(f parser.File) bool { return f.Path == path })
	if i < 0 {
		return // removed since
	}
	id := files[i].SessionID
	var copies []parser.File
	for _, f := range files {
		if f.SessionID == id {
			copies = append(copies, f)
		}
	}
	w.importFile(parser.Choose(copies)[0].Path)
}

func (w *Watcher) importFile(path string) {
	r, err := importer.ImportFile(w.DB, path, nil)
	if err != nil {
		w.fail(err.Error())
		fmt.Fprintf(w.Log, "[watcher] import failed for %s: %v\n", path, err)
		return
	}
	if r == nil {
		return
	}
	w.update(func(s *Status) { s.LastImportAt, s.LastError, s.LastErrorAt = now(), "", "" })
	if r.Status != importer.Unchanged && w.OnImport != nil {
		w.OnImport(r)
	}
}
