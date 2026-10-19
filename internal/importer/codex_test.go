package importer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/parser"
)

const codexThread = "019a1b2c-3d4e-7f80-9abc-def012345678"

func codexLine(ts, kind string, payload any) string {
	b, _ := json.Marshal(map[string]any{"timestamp": ts, "type": kind, "payload": payload})
	return string(b)
}

func codexUser(ts, text string) string {
	return codexLine(ts, "response_item", map[string]any{"type": "message", "role": "user",
		"content": []any{map[string]any{"type": "input_text", "text": text}}})
}

func codexRollout(cwd string, prompts ...string) string {
	lines := []string{codexLine("2026-05-07T17:24:21.100Z", "session_meta", map[string]any{
		"id": codexThread, "timestamp": "2026-05-07T17:24:21.000Z", "cwd": cwd, "cli_version": "0.112.0",
		"source": "cli", "git": map[string]any{"branch": "main"}})}
	for i, p := range prompts {
		lines = append(lines, codexUser("2026-05-07T17:25:0"+string(rune('0'+i))+".000Z", p))
	}
	return strings.Join(lines, "\n") + "\n"
}

// codexHome writes a Codex home: sessions/2026/05/07/rollout-...jsonl and
// session_index.jsonl with the thread's name.
func codexHome(t *testing.T, name string, content string) (home, path string) {
	t.Helper()
	home = t.TempDir()
	dir := filepath.Join(home, "sessions", "2026", "05", "07")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(home, "session_index.jsonl"),
		[]byte(`{"id":"`+codexThread+`","thread_name":"Login redirect","updated_at":"2026-05-07T17:30:00Z"}`+"\n"), 0o644)
	return home, path
}

func TestImportCodexRollout(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "vault.db"), db.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	home, path := codexHome(t, "rollout-2026-05-07T17-24-21-"+codexThread+".jsonl", codexRollout("/work/app", "Fix the login redirect"))
	tree := filepath.Join(home, "sessions")

	r, err := ImportFile(d, path, nil, []string{tree})
	if err != nil {
		t.Fatal(err)
	}
	if r == nil || r.Status != New || r.SessionID != codexThread || r.Project != "-work-app" || r.TotalMessages != 1 {
		t.Fatalf("result %+v", r)
	}
	sessions, err := d.Sessions()
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions %v %v", sessions, err)
	}
	s := sessions[0]
	if s.Agent != parser.AgentCodex || s.ProjectPath != "/work/app" || s.GitBranch != "main" || s.ClaudeVersion != "0.112.0" ||
		s.Title != "Login redirect" || s.FirstPrompt != "Fix the login redirect" {
		t.Errorf("session %+v", s)
	}
	// Unchanged until the rollout grows.
	if r, _ = ImportFile(d, path, nil, []string{tree}); r == nil || r.Status != Unchanged {
		t.Fatalf("second import %+v", r)
	}
	os.WriteFile(path, []byte(codexRollout("/work/app", "Fix the login redirect", "And the logout one")), 0o644)
	if r, _ = ImportFile(d, path, nil, []string{tree}); r == nil || r.Status != Resynced || r.TotalMessages != 2 || r.AddedMessages != 1 {
		t.Fatalf("third import %+v", r)
	}

	// With no trees given, the rollout's own tree is used for the check.
	if r, err = ImportFile(d, path, nil, nil); err != nil || r == nil || r.Status != Unchanged {
		t.Fatalf("own tree: %+v %v", r, err)
	}

	// The list and the search see the agent.
	listed, err := d.ListSessions(db.ListOptions{Agent: parser.AgentCodex})
	if err != nil || len(listed) != 1 || listed[0].Agent != parser.AgentCodex {
		t.Fatalf("list codex: %+v %v", listed, err)
	}
	if listed, _ = d.ListSessions(db.ListOptions{Agent: parser.AgentClaude}); len(listed) != 0 {
		t.Fatalf("list claude: %+v", listed)
	}
	hits, err := d.Search("logout", db.SearchOptions{})
	if err != nil || len(hits) != 1 || hits[0].Agent != parser.AgentCodex {
		t.Fatalf("search: %+v %v", hits, err)
	}
}

func TestImportCompressedCodexRollout(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "vault.db"), db.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	enc, _ := zstd.NewWriter(nil)
	home, path := codexHome(t, "rollout-2026-05-07T17-24-21-"+codexThread+".jsonl.zst",
		string(enc.EncodeAll([]byte(codexRollout("/work/app", "Fix the login redirect")), nil)))
	r, err := ImportFile(d, path, nil, []string{filepath.Join(home, "sessions")})
	if err != nil || r == nil || r.Status != New || r.SessionID != codexThread || r.TotalMessages != 1 {
		t.Fatalf("result %+v %v", r, err)
	}
}

// Run imports a Claude Code tree and a Codex tree together.
func TestRunImportsBothAgents(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "vault.db"), db.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	claudeTree := t.TempDir()
	os.MkdirAll(filepath.Join(claudeTree, "-work-app"), 0o755)
	os.WriteFile(filepath.Join(claudeTree, "-work-app", "sess-001.jsonl"),
		[]byte(userLine("hello", "u1", "2026-01-01T00:00:00.000Z")+"\n"+assistantLine("hi", "a1", "2026-01-01T00:00:01.000Z")+"\n"), 0o644)
	home, _ := codexHome(t, "rollout-2026-05-07T17-24-21-"+codexThread+".jsonl", codexRollout("/work/app", "Fix the login redirect"))

	var out strings.Builder
	if err := Run(d, Options{ProjectsDirs: []string{claudeTree, filepath.Join(home, "sessions")}}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Imported 2 sessions (3 messages).") {
		t.Fatalf("output:\n%s", out.String())
	}
	sessions, _ := d.Sessions()
	agents := map[string]string{}
	for _, s := range sessions {
		agents[s.ID] = s.Agent
	}
	if agents["sess-001"] != parser.AgentClaude || agents[codexThread] != parser.AgentCodex {
		t.Fatalf("agents %v", agents)
	}
}
