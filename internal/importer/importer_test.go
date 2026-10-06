package importer

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/babarot/claude-recall/internal/db"
)

func userLine(text, uuid, ts string) string {
	b, _ := json.Marshal(map[string]any{
		"type": "user", "uuid": uuid, "sessionId": "sess-001", "timestamp": ts, "cwd": "/home/user/project",
		"version": "2.1.87", "gitBranch": "main", "message": map[string]any{"role": "user", "content": text},
	})
	return string(b)
}

func assistantLine(text, uuid, ts string) string {
	b, _ := json.Marshal(map[string]any{
		"type": "assistant", "uuid": uuid, "sessionId": "sess-001", "timestamp": ts,
		"message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": text}}},
	})
	return string(b)
}

type env struct {
	t    *testing.T
	path string
	db   *db.DB
}

func newEnv(t *testing.T, lines ...string) *env {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "my-project")
	os.MkdirAll(dir, 0o755)
	d, err := db.Open(filepath.Join(t.TempDir(), "vault.db"), db.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	e := &env{t: t, path: filepath.Join(dir, "sess-001.jsonl"), db: d}
	e.write(lines...)
	return e
}

// write replaces the file and moves its mtime forward, so change detection
// sees the edit even within one filesystem timestamp tick.
func (e *env) write(lines ...string) {
	e.t.Helper()
	if err := os.WriteFile(e.path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		e.t.Fatal(err)
	}
	if fi, err := os.Stat(e.path); err == nil {
		next := fi.ModTime().Add(time.Second)
		os.Chtimes(e.path, next, next)
	}
}

func (e *env) importFile() *Result {
	e.t.Helper()
	r, err := ImportFile(e.db, e.path, nil)
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}

func (e *env) messageCount() int {
	e.t.Helper()
	fi, err := e.db.GetFileInfo("sess-001")
	if err != nil || fi == nil {
		e.t.Fatalf("file info %v %v", fi, err)
	}
	return fi.MessageCount
}

func TestImportNewSession(t *testing.T) {
	e := newEnv(t, userLine("hello", "u1", "2026-01-01T00:00:00Z"), assistantLine("hi", "a1", "2026-01-01T00:00:01Z"))
	r := e.importFile()
	if r.Status != New || r.TotalMessages != 2 || r.SessionID != "sess-001" || r.Project != "my-project" {
		t.Fatalf("%+v", r)
	}
}

func TestImportUnchangedFile(t *testing.T) {
	e := newEnv(t, userLine("hello", "u1", "2026-01-01T00:00:00Z"))
	e.importFile()
	if r := e.importFile(); r.Status != Unchanged || r.TotalMessages != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestImportAppendedMessages(t *testing.T) {
	e := newEnv(t, userLine("hello", "u1", "2026-01-01T00:00:00Z"))
	e.importFile()
	e.write(userLine("hello", "u1", "2026-01-01T00:00:00Z"), assistantLine("hi", "a1", "2026-01-01T00:00:01Z"))
	if r := e.importFile(); r.Status != Resynced || e.messageCount() != 2 {
		t.Fatalf("%+v", r)
	}
}

func TestImportMirrorsShrunkFile(t *testing.T) {
	e := newEnv(t, userLine("a", "u1", "2026-01-01T00:00:00Z"), userLine("b", "u2", "2026-01-01T00:00:01Z"))
	e.importFile()
	e.write(userLine("c", "u3", "2026-01-01T00:00:02Z"))
	e.importFile()
	if n := e.messageCount(); n != 1 {
		t.Fatalf("message count %d, want 1", n)
	}
	r, _ := e.db.Search("a", db.SearchOptions{})
	if len(r) != 0 {
		t.Fatalf("rows of the removed uuid are still searchable: %+v", r)
	}
}

func TestImportMissingFile(t *testing.T) {
	d, _ := db.Open(filepath.Join(t.TempDir(), "vault.db"), db.Options{})
	defer d.Close()
	r, err := ImportFile(d, filepath.Join(t.TempDir(), "p", "nope.jsonl"), nil)
	if r != nil || err != nil {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestRunSummary(t *testing.T) {
	e := newEnv(t, userLine("hello", "u1", "2026-01-01T00:00:00Z"))
	projects := filepath.Dir(filepath.Dir(e.path))
	os.WriteFile(filepath.Join(filepath.Dir(e.path), "broken.jsonl"), []byte("nope\n"), 0o644)

	var out bytes.Buffer
	if err := Run(e.db, Options{ProjectsDirs: []string{projects}}, &out); err != nil {
		t.Fatal(err)
	}
	want := "Syncing 2 sessions...\nImported 1 sessions (1 messages). Skipped 1 unreadable files.\n"
	if out.String() != want {
		t.Fatalf("got %q", out.String())
	}
}

func TestAtob(t *testing.T) {
	for in, want := range map[string]string{"aGVsbG8=": "hello", "aGVsbG8": "hello", "aGVs\nbG8=": "hello", "": ""} {
		got, err := atob(in)
		if err != nil || string(got) != want {
			t.Errorf("atob(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := atob("a"); err == nil {
		t.Error("atob(\"a\") should fail")
	}
}

func TestImportFillsTitleOfOlderRows(t *testing.T) {
	title, _ := json.Marshal(map[string]any{"type": "ai-title", "aiTitle": "Fix the login bug", "sessionId": "sess-001"})
	e := newEnv(t, userLine("hello", "u1", "2026-01-01T00:00:00Z"), string(title))
	e.importFile()
	// Simulate a row written before titles were stored.
	if _, err := e.db.Exec(`UPDATE sessions SET title = NULL`); err != nil {
		t.Fatal(err)
	}
	if r := e.importFile(); r.Status != Resynced {
		t.Fatalf("an unchanged file with a NULL title must be imported again: %+v", r)
	}
	if r := e.importFile(); r.Status != Unchanged {
		t.Fatalf("second import: %+v", r)
	}
	s, err := e.db.Sessions()
	if err != nil || len(s) != 1 || s[0].Title != "Fix the login bug" {
		t.Fatalf("%+v %v", s, err)
	}
}

// Every Claude Code session runs its own `recall mcp`, and each imports
// everything on startup. Imports from separate connections must wait for
// each other instead of failing with SQLITE_BUSY.
func TestConcurrentImportsFromSeparateConnections(t *testing.T) {
	projects := t.TempDir()
	for p := range 4 {
		dir := filepath.Join(projects, "project-"+string(rune('a'+p)))
		os.MkdirAll(dir, 0o755)
		for s := range 15 {
			var lines []string
			for m := range 200 {
				lines = append(lines, userLine(strings.Repeat("words ", 50), "u"+string(rune('A'+m%26))+strings.Repeat("x", m), "2026-01-01T00:00:00Z"))
			}
			id := "sess-" + string(rune('a'+p)) + "-" + string(rune('a'+s))
			body := strings.ReplaceAll(strings.Join(lines, "\n"), "sess-001", id)
			os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte(body+"\n"), 0o644)
		}
	}
	path := filepath.Join(t.TempDir(), "vault.db")
	if d, err := db.Open(path, db.Options{}); err != nil {
		t.Fatal(err)
	} else {
		d.Close()
	}

	const workers = 8
	errs := make(chan error, workers)
	for range workers {
		go func() {
			d, err := db.Open(path, db.Options{})
			if err != nil {
				errs <- err
				return
			}
			defer d.Close()
			errs <- Run(d, Options{ProjectsDirs: []string{projects}}, io.Discard)
		}()
	}
	for range workers {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent import failed: %v", err)
		}
	}
	d, _ := db.Open(path, db.Options{})
	defer d.Close()
	s, err := d.Sessions()
	if err != nil || len(s) != 60 {
		t.Fatalf("sessions %d %v", len(s), err)
	}
}

// tree writes transcripts into a projects directory: project/id.jsonl of
// one user message, text, written at mtime.
type tree string

func (tr tree) put(t *testing.T, project, id, text string, mtime time.Time) {
	t.Helper()
	dir := filepath.Join(string(tr), project)
	os.MkdirAll(dir, 0o755)
	b, _ := json.Marshal(map[string]any{
		"type": "user", "uuid": id + "-u", "sessionId": id, "timestamp": "2026-01-01T00:00:00Z", "cwd": "/w",
		"message": map[string]any{"role": "user", "content": text},
	})
	path := filepath.Join(dir, id+".jsonl")
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(path, mtime, mtime)
}

func (tr tree) index(t *testing.T, project string, entries ...map[string]any) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"entries": entries})
	os.WriteFile(filepath.Join(string(tr), project, "sessions-index.json"), b, 0o644)
}

func firstPrompts(t *testing.T, d *db.DB) map[string]string {
	t.Helper()
	ss, err := d.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, s := range ss {
		out[s.ID] = s.FirstPrompt
	}
	return out
}

// Sessions of every tree are imported, each project with its own tree's
// index, and of a session in two trees the copy written last.
func TestRunTrees(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	primary, extra := tree(filepath.Join(home, "primary")), tree(filepath.Join(home, "extra"))
	old, recent := time.Now().Add(-time.Hour), time.Now()
	primary.put(t, "-p", "s1", "host only", old)
	extra.put(t, "-p", "s2", "container only", old)
	extra.index(t, "-p", map[string]any{"sessionId": "s2", "firstPrompt": "from the container's index"})
	primary.put(t, "-p", "s3", "stale copy", old)
	extra.put(t, "-p", "s3", "continued in the container", recent)

	d, _ := db.Open(filepath.Join(t.TempDir(), "vault.db"), db.Options{})
	defer d.Close()
	dirs := []string{string(primary), string(extra), filepath.Join(home, "missing")}

	// --session takes the copy the full import would.
	var out bytes.Buffer
	if err := Run(d, Options{ProjectsDirs: dirs, Session: "s3"}, &out); err != nil {
		t.Fatal(err)
	}
	if got := firstPrompts(t, d)["s3"]; got != "continued in the container" {
		t.Fatalf("s3: %q", got)
	}

	out.Reset()
	if err := Run(d, Options{ProjectsDirs: dirs}, &out); err != nil {
		t.Fatal(err)
	}
	want := "~/primary: 2 session files\n~/extra: 2 session files\n~/missing: not found\n" +
		"Syncing 3 sessions...\nImported 2 sessions (2 messages). 1 unchanged.\n"
	if out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
	got := firstPrompts(t, d)
	if got["s1"] != "host only" || got["s2"] != "from the container's index" || got["s3"] != "continued in the container" {
		t.Fatalf("%v", got)
	}

	out.Reset()
	Run(nil, Options{ProjectsDirs: dirs, DryRun: true}, &out)
	for _, line := range []string{"  s1 (-p)\n", "  s2 (-p)  in ~/extra\n", "  s3 (-p)  in ~/extra, chosen over the copy in ~/primary\n"} {
		if !strings.Contains(out.String(), line) {
			t.Errorf("dry run lacks %q:\n%s", line, out.String())
		}
	}
}
