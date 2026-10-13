package importer

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/babarot/claude-recall/internal/db"
)

// remoteOf is the stored remote of a session, nil when it is not known.
func remoteOf(t *testing.T, d *db.DB, id string) *string {
	t.Helper()
	var remote *string
	all, err := d.ListSessions(db.ListOptions{Limit: new(int(100))})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range all {
		if s.SessionID == id {
			remote = s.Remote
		}
	}
	return remote
}

func TestImportRecordsRemote(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "config"),
		[]byte("[remote \"origin\"]\n\turl = https://token@github.com/babarot/claude-recall.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	line := func(cwd, uuid, ts string) string {
		b, _ := json.Marshal(map[string]any{
			"type": "user", "uuid": uuid, "sessionId": "sess-001", "timestamp": ts, "cwd": cwd,
			"version": "2.1.87", "gitBranch": "main", "message": map[string]any{"role": "user", "content": "hello"},
		})
		return string(b)
	}

	e := newEnv(t, line(repo, "u1", "2026-03-01T10:00:00.000Z"))
	e.importFile()
	if got := remoteOf(t, e.db, "sess-001"); got == nil || *got != "github.com/babarot/claude-recall" {
		t.Fatalf("remote = %v, want github.com/babarot/claude-recall", got)
	}

	// The directory goes away and the transcript grows: the remote read
	// before stays.
	if err := os.RemoveAll(repo); err != nil {
		t.Fatal(err)
	}
	e.write(line(repo, "u1", "2026-03-01T10:00:00.000Z"), line(repo, "u2", "2026-03-01T10:01:00.000Z"))
	if r := e.importFile(); r.Status != Resynced {
		t.Fatalf("status %v, want Resynced", r.Status)
	}
	if got := remoteOf(t, e.db, "sess-001"); got == nil || *got != "github.com/babarot/claude-recall" {
		t.Fatalf("remote after the directory went = %v, want it kept", got)
	}
}

func TestRunFillsRemotesOfStoredSessions(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "vault.db"), db.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "config"),
		[]byte("[remote \"origin\"]\n\turl = git@github.com:babarot/claude-recall.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Stored before remotes were, and their transcripts are gone: one ran
	// in a directory still on disk, one in a directory since removed.
	for _, s := range []struct{ id, path string }{{"kept", repo}, {"gone", filepath.Join(repo, "..", "gone")}} {
		if err := d.ReplaceSession(db.SessionRow{SessionID: s.id, Project: "p", ProjectPath: s.path, Title: ""}, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
	}

	// No transcripts at all: the remotes are filled in anyway.
	if err := Run(d, Options{ProjectsDirs: []string{filepath.Join(t.TempDir(), "projects")}}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := remoteOf(t, d, "kept"); got == nil || *got != "github.com/babarot/claude-recall" {
		t.Errorf("remote of the session on disk = %v", got)
	}
	if got := remoteOf(t, d, "gone"); got != nil {
		t.Errorf("remote of the session whose directory is gone = %q, want unknown", *got)
	}
}
