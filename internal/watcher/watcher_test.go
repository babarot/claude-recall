package watcher

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/importer"
)

func userLine(sessionID, uuid, text string) string {
	b, _ := json.Marshal(map[string]any{"type": "user", "uuid": uuid, "sessionId": sessionID,
		"timestamp": "2026-01-01T00:00:00Z", "message": map[string]any{"role": "user", "content": text}})
	return string(b) + "\n"
}

type recorder struct {
	mu   sync.Mutex
	seen []importer.Result
}

func (r *recorder) add(res *importer.Result) {
	r.mu.Lock()
	r.seen = append(r.seen, *res)
	r.mu.Unlock()
}

func (r *recorder) wait(t *testing.T, n int) []importer.Result {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		if len(r.seen) >= n {
			out := append([]importer.Result(nil), r.seen...)
			r.mu.Unlock()
			return out
		}
		r.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d imports", n)
	return nil
}

func TestWatcherImportsNewAndChangedTranscripts(t *testing.T) {
	projects := t.TempDir()
	d, err := db.Open(filepath.Join(t.TempDir(), "vault.db"), db.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	rec := &recorder{}
	w := &Watcher{DB: d, ProjectsDirs: []string{projects}, Debounce: 50 * time.Millisecond, OnImport: rec.add}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	for !w.Status().Running {
		time.Sleep(10 * time.Millisecond)
	}

	// A project directory that did not exist when the watcher started.
	dir := filepath.Join(projects, "-home-user-new")
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "s1.jsonl")
	os.WriteFile(path, []byte(userLine("s1", "u1", "hello")), 0o644)
	got := rec.wait(t, 1)
	if got[0].Status != importer.New || got[0].SessionID != "s1" || got[0].Project != "-home-user-new" {
		t.Fatalf("%+v", got[0])
	}

	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(userLine("s1", "u2", "more"))
	f.Close()
	got = rec.wait(t, 2)
	if got[1].Status != importer.Resynced || got[1].TotalMessages != 2 {
		t.Fatalf("%+v", got[1])
	}

	st := w.Status()
	if st.LastEventAt == "" || st.LastImportAt == "" || st.LastError != "" || st.DebounceMs != 50 {
		t.Fatalf("status %+v", st)
	}
}

func TestWatcherMissingDirectory(t *testing.T) {
	w := &Watcher{ProjectsDirs: []string{filepath.Join(t.TempDir(), "nope")}, Log: os.Stderr}
	w.Run(context.Background())
	st := w.Status()
	if st.Running || st.LastError == "" || !st.Enabled {
		t.Fatalf("status %+v", st)
	}
}

// A tree that is not there does not stop the others being watched, and is
// imported from once it appears, as a container's is after its first run.
func TestWatcherTrees(t *testing.T) {
	primary, extra := t.TempDir(), filepath.Join(t.TempDir(), "extra")
	d, err := db.Open(filepath.Join(t.TempDir(), "vault.db"), db.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	rec := &recorder{}
	w := &Watcher{DB: d, ProjectsDirs: []string{primary, extra}, Debounce: 50 * time.Millisecond, OnImport: rec.add, Log: io.Discard}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	for !w.Status().Running {
		time.Sleep(10 * time.Millisecond)
	}
	if st := w.Status(); st.ProjectsDir != primary || !slices.Equal(st.ProjectsDirs, []string{primary, extra}) || !slices.Equal(st.MissingDirs, []string{extra}) {
		t.Fatalf("status %+v", st)
	}

	dir := filepath.Join(extra, "-workspace")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(userLine("s1", "u1", "in the container")), 0o644)
	if got := rec.wait(t, 1); got[0].SessionID != "s1" || got[0].Status != importer.New {
		t.Fatalf("%+v", got[0])
	}
	if st := w.Status(); len(st.MissingDirs) != 0 {
		t.Fatalf("status %+v", st)
	}

	// An older copy of s1 in the primary tree changing does not replace the
	// newer one in the archive.
	stale := filepath.Join(primary, "-workspace", "s1.jsonl")
	os.MkdirAll(filepath.Dir(stale), 0o755)
	os.WriteFile(stale, []byte(userLine("s1", "u0", "stale")), 0o644)
	past := time.Now().Add(-time.Hour)
	os.Chtimes(stale, past, past)
	time.Sleep(400 * time.Millisecond)
	msgs, err := d.SessionMessages("s1")
	if err != nil || len(msgs) != 1 || msgs[0].Content != "in the container" {
		t.Fatalf("%+v %v", msgs, err)
	}
}

func TestWatcherNoTrees(t *testing.T) {
	var log strings.Builder
	w := &Watcher{ProjectsDirs: []string{filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")}, Log: &log}
	w.Run(context.Background())
	if st := w.Status(); st.Running || !strings.HasPrefix(st.LastError, "none of ") || !strings.Contains(log.String(), "watcher disabled") {
		t.Fatalf("status %+v, log %q", st, log.String())
	}
}

// Started comes after Run has taken the files present as imported, so a
// file that changes from there on, as during the catch-up import Started
// begins, is imported by the watcher.
func TestWatcherStartedAfterSnapshot(t *testing.T) {
	projects := t.TempDir()
	dir := filepath.Join(projects, "-p")
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "s1.jsonl")
	os.WriteFile(path, []byte(userLine("s1", "u1", "hello")), 0o644)
	d, err := db.Open(filepath.Join(t.TempDir(), "vault.db"), db.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	rec := &recorder{}
	w := &Watcher{DB: d, ProjectsDirs: []string{projects}, Debounce: 50 * time.Millisecond, OnImport: rec.add,
		Started: func() {
			f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			f.WriteString(userLine("s1", "u2", "more"))
			f.Close()
			later := time.Now().Add(time.Second)
			os.Chtimes(path, later, later)
		}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	if got := rec.wait(t, 1); got[0].TotalMessages != 2 {
		t.Fatalf("%+v; want the file as changed in Started", got[0])
	}
}

// Started is called when Run gives up for want of a tree, so the catch-up
// import runs as it did before it waited for Run.
func TestWatcherStartedWithoutTree(t *testing.T) {
	called := false
	w := &Watcher{ProjectsDirs: []string{filepath.Join(t.TempDir(), "nope")}, Log: io.Discard, Started: func() { called = true }}
	w.Run(context.Background())
	if !called {
		t.Fatal("Started was not called")
	}
}
