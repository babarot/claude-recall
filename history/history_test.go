package history_test

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/babarot/claude-recall/history"
	"github.com/babarot/claude-recall/internal/config"
	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/fixture"
)

func open(t *testing.T) (*history.History, string) {
	t.Helper()
	path := fixture.Archive(t)
	h, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	return h, path
}

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got, want := history.DefaultPath(), config.DefaultDBPath(); got != want {
		t.Errorf("DefaultPath() = %q, recall uses %q", got, want)
	}
}

func TestSessions(t *testing.T) {
	h, _ := open(t)
	ctx := context.Background()

	all, err := h.Sessions(ctx, at("2026-01-01T00:00:00Z"), at("2027-01-01T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].ID != fixture.AppSession || all[1].ID != fixture.APISession {
		t.Fatalf("Sessions = %+v, want the app session then the api one", all)
	}
	app := all[0]
	if app.ProjectPath != "/work/app" || app.GitBranch != "main" || app.Repository != "/work/app" ||
		app.RepositoryKey != "/work/app" || app.Worktree != "" || app.MessageCount != 4 {
		t.Errorf("app session = %+v", app)
	}
	if want := "How do I configure the terraform module for staging?"; app.Title != want || app.FirstPrompt != want {
		t.Errorf("Title, FirstPrompt = %q, %q, want %q", app.Title, app.FirstPrompt, want)
	}
	if !app.StartedAt.Equal(at("2026-03-01T10:00:00Z")) || !app.EndedAt.Equal(at("2026-03-01T10:01:00Z")) {
		t.Errorf("StartedAt, EndedAt = %v, %v", app.StartedAt, app.EndedAt)
	}

	// A range holds a session by its messages' timestamps, from inclusive
	// and to exclusive, in whatever time zone they are given.
	jst := time.FixedZone("JST", 9*60*60)
	for _, tt := range []struct {
		name     string
		from, to time.Time
		want     []string
	}{
		{"the app session's day", at("2026-03-01T00:00:00Z"), at("2026-03-02T00:00:00Z"), []string{fixture.AppSession}},
		{"from its last message", at("2026-03-01T10:01:05Z"), at("2026-03-02T00:00:00Z"), []string{fixture.AppSession}},
		{"to its first message", at("2026-02-01T00:00:00Z"), at("2026-03-01T10:00:00Z"), nil},
		{"the api session's day in JST", time.Date(2026, 4, 2, 0, 0, 0, 0, jst), time.Date(2026, 4, 3, 0, 0, 0, 0, jst), []string{fixture.APISession}},
		{"before the api session in JST", time.Date(2026, 4, 1, 0, 0, 0, 0, jst), time.Date(2026, 4, 2, 18, 0, 0, 0, jst), nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := h.Sessions(ctx, tt.from, tt.to)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, s := range got {
				ids = append(ids, s.ID)
			}
			if len(ids) != len(tt.want) || (len(ids) > 0 && ids[0] != tt.want[0]) {
				t.Errorf("Sessions = %v, want %v", ids, tt.want)
			}
		})
	}
}

func TestMessages(t *testing.T) {
	h, _ := open(t)
	ctx := context.Background()

	var got []history.Message
	for m, err := range h.Messages(ctx, "", at("2026-01-01T00:00:00Z"), at("2027-01-01T00:00:00Z")) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, m)
	}
	// Every session's messages, by session, then in conversation order.
	type key struct {
		session, uuid, kind string
		block               int
	}
	want := []key{
		{fixture.AppSession, "a1", "text", 0},
		{fixture.AppSession, "a2", "text", 0},
		{fixture.AppSession, "a3", "text", 0},
		{fixture.AppSession, "a4", "text", 0},
		{fixture.APISession, "b1", "text", 0},
		{fixture.APISession, "b2", "tool_use", 0},
		{fixture.APISession, "b2", "text", 1},
	}
	if len(got) != len(want) {
		t.Fatalf("Messages yielded %d, want %d: %+v", len(got), len(want), got)
	}
	for i, m := range got {
		if k := (key{m.SessionID, m.UUID, m.Kind, m.Block}); k != want[i] {
			t.Errorf("message %d = %+v, want %+v", i, k, want[i])
		}
	}
	use := got[5]
	if use.Role != "assistant" || use.ToolName != "Bash" || !bytes.Contains([]byte(use.ToolInput), []byte("go test ./auth/...")) {
		t.Errorf("tool_use = %+v", use)
	}
	if !use.Timestamp.Equal(at("2026-04-02T09:00:10Z")) {
		t.Errorf("tool_use Timestamp = %v", use.Timestamp)
	}

	// One session, and only the messages in the range.
	var uuids []string
	for m, err := range h.Messages(ctx, fixture.AppSession, at("2026-03-01T10:00:05Z"), at("2026-03-01T10:01:05Z")) {
		if err != nil {
			t.Fatal(err)
		}
		uuids = append(uuids, m.UUID)
	}
	if len(uuids) != 2 || uuids[0] != "a2" || uuids[1] != "a3" {
		t.Errorf("app session's messages in range = %v, want [a2 a3]", uuids)
	}
}

// Sessions can be called while Messages is being ranged over, and ending
// the loop early lets go of the query.
func TestMessagesWhileRanging(t *testing.T) {
	h, _ := open(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	from, to := at("2026-01-01T00:00:00Z"), at("2027-01-01T00:00:00Z")

	for range 3 {
		for _, err := range h.Messages(ctx, "", from, to) {
			if err != nil {
				t.Fatal(err)
			}
			if _, err := h.Sessions(ctx, from, to); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
}

func TestMessagesCanceled(t *testing.T) {
	h, _ := open(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var gotErr error
	for _, err := range h.Messages(ctx, "", at("2026-01-01T00:00:00Z"), at("2027-01-01T00:00:00Z")) {
		gotErr = err
	}
	if gotErr == nil {
		t.Error("Messages with a canceled context yielded no error")
	}
}

func TestOpenDoesNotWrite(t *testing.T) {
	path := fixture.Archive(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Sessions(context.Background(), at("2026-01-01T00:00:00Z"), at("2027-01-01T00:00:00Z")); err != nil {
		t.Fatal(err)
	}
	h.Close()
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("the database changed by being read")
	}
	if _, err := history.Open(path + ".missing"); err == nil {
		t.Error("Open made a database that did not exist")
	}
}

func TestSessionRemote(t *testing.T) {
	path := fixture.Archive(t)
	d, err := db.Open(path, db.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`UPDATE sessions SET remote = 'github.com/babarot/claude-recall' WHERE session_id = ?`, fixture.AppSession); err != nil {
		t.Fatal(err)
	}
	d.Close()
	h, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ss, err := h.Sessions(context.Background(), at("2026-01-01T00:00:00Z"), at("2027-01-01T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range ss {
		got[s.ID] = s.Remote
	}
	if got[fixture.AppSession] != "github.com/babarot/claude-recall" || got[fixture.APISession] != "" {
		t.Errorf("Remote = %v", got)
	}
}

// A database recall has not migrated yet, opened read-only, reads with
// no remote.
func TestSessionsBeforeRemoteMigration(t *testing.T) {
	path := fixture.Archive(t)
	d, err := db.Open(path, db.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`ALTER TABLE sessions DROP COLUMN remote`); err != nil {
		t.Fatal(err)
	}
	d.Close()
	h, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ss, err := h.Sessions(context.Background(), at("2026-01-01T00:00:00Z"), at("2027-01-01T00:00:00Z"))
	if err != nil || len(ss) != 2 || ss[0].Remote != "" {
		t.Errorf("Sessions = %+v, %v", ss, err)
	}
}
