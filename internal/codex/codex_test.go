package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/babarot/claude-recall/internal/parser"
)

func line(ts, kind string, payload any) string {
	b, err := json.Marshal(map[string]any{"timestamp": ts, "type": kind, "payload": payload})
	if err != nil {
		panic(err)
	}
	return string(b)
}

const threadID = "019a1b2c-3d4e-7f80-9abc-def012345678"

var sessionMetaLine = line("2026-05-07T17:24:21.100Z", "session_meta", map[string]any{
	"id": threadID, "timestamp": "2026-05-07T17:24:21.000Z", "cwd": "/home/user/src/app",
	"originator": "codex_cli_rs", "cli_version": "0.112.0", "instructions": nil, "source": "cli",
	"git": map[string]any{"commit_hash": "abc", "branch": "feature/login", "repository_url": "https://github.com/me/app.git"},
})

func userMsg(ts, text string) string {
	return line(ts, "response_item", map[string]any{"type": "message", "role": "user",
		"content": []any{map[string]any{"type": "input_text", "text": text}}})
}

func assistantMsg(ts, text string) string {
	return line(ts, "response_item", map[string]any{"type": "message", "role": "assistant", "phase": "final_answer",
		"content": []any{map[string]any{"type": "output_text", "text": text}}})
}

// A typical rollout: the context Codex injects, a prompt, reasoning, a
// shell call and its output, the answer, and the UI events and bookkeeping
// that say nothing new.
var rollout = strings.Join([]string{
	sessionMetaLine,
	line("2026-05-07T17:24:21.200Z", "turn_context", map[string]any{"cwd": "/home/user/src/app", "model": "gpt-5-codex", "approval_policy": "on-request"}),
	userMsg("2026-05-07T17:24:21.300Z", "<user_instructions>\n\nAlways run the tests.\n\n</user_instructions>"),
	userMsg("2026-05-07T17:24:21.301Z", "<environment_context>\n  <cwd>/home/user/src/app</cwd>\n</environment_context>"),
	userMsg("2026-05-07T17:24:21.400Z", "  Fix the login redirect  "),
	line("2026-05-07T17:24:21.401Z", "event_msg", map[string]any{"type": "user_message", "message": "Fix the login redirect"}),
	line("2026-05-07T17:24:25.000Z", "response_item", map[string]any{"type": "reasoning",
		"summary":           []any{map[string]any{"type": "summary_text", "text": "**Looking at the router**"}},
		"content":           nil,
		"encrypted_content": "gAAAA"}),
	line("2026-05-07T17:24:26.000Z", "response_item", map[string]any{"type": "function_call", "name": "shell",
		"arguments": `{"command":["bash","-lc","grep -rn redirect src/"]}`, "call_id": "call_1"}),
	line("2026-05-07T17:24:27.000Z", "response_item", map[string]any{"type": "function_call_output", "call_id": "call_1",
		"output": "src/login.ts:12: redirect('/')\n"}),
	line("2026-05-07T17:24:28.000Z", "response_item", map[string]any{"type": "custom_tool_call", "name": "apply_patch",
		"input": "*** Begin Patch\n*** End Patch", "call_id": "call_2"}),
	line("2026-05-07T17:24:29.000Z", "response_item", map[string]any{"type": "custom_tool_call_output", "call_id": "call_2",
		"output": []any{map[string]any{"type": "input_text", "text": "Done"}}}),
	assistantMsg("2026-05-07T17:24:30.000Z", "The redirect now goes to /dashboard."),
	line("2026-05-07T17:24:30.100Z", "event_msg", map[string]any{"type": "agent_message", "message": "The redirect now goes to /dashboard."}),
	line("2026-05-07T17:24:30.200Z", "event_msg", map[string]any{"type": "token_count", "info": nil}),
	line("2026-05-07T17:24:30.300Z", "compacted", map[string]any{"message": "summary so far"}),
	userMsg("2026-05-07T17:25:00.000Z", "Thanks, ship it"),
}, "\n") + "\n"

func TestParse(t *testing.T) {
	s := Parse(rollout, "from-file-name", "Login redirect")
	if s == nil {
		t.Fatal("Parse returned nil")
	}
	m := s.Meta
	if m.SessionID != threadID || m.Agent != parser.AgentCodex || m.ProjectPath != "/home/user/src/app" ||
		m.Project != "-home-user-src-app" || m.GitBranch != "feature/login" || m.ClaudeVersion != "0.112.0" ||
		m.Title != "Login redirect" {
		t.Errorf("meta %+v", m)
	}
	if m.StartedAt != "2026-05-07T17:24:21.000Z" || m.EndedAt != "2026-05-07T17:25:00.000Z" {
		t.Errorf("started %s ended %s", m.StartedAt, m.EndedAt)
	}
	if m.FirstPrompt != "Fix the login redirect" {
		t.Errorf("first prompt %q", m.FirstPrompt)
	}
	var got []string
	for _, msg := range s.Messages {
		got = append(got, msg.Role+"/"+msg.BlockType+":"+msg.Content)
	}
	want := []string{
		"user/meta:<user_instructions>\n\nAlways run the tests.\n\n</user_instructions>",
		"user/meta:<environment_context>\n  <cwd>/home/user/src/app</cwd>\n</environment_context>",
		"user/text:Fix the login redirect",
		"assistant/thinking:**Looking at the router**",
		"assistant/tool_use:shell",
		"user/tool_result:src/login.ts:12: redirect('/')\n",
		"assistant/tool_use:apply_patch",
		"user/tool_result:Done",
		"assistant/text:The redirect now goes to /dashboard.",
		"user/text:Thanks, ship it",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("messages:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// Turn indexes count up; UUIDs are the line numbers, unique per block.
	seen := map[string]bool{}
	for i, msg := range s.Messages {
		if msg.TurnIndex != i {
			t.Errorf("message %d has turn %d", i, msg.TurnIndex)
		}
		k := msg.UUID + "/" + string(rune('0'+msg.BlockIndex))
		if seen[k] {
			t.Errorf("duplicate key %s", k)
		}
		seen[k] = true
	}
	shell := s.Messages[4]
	if shell.ToolName == nil || *shell.ToolName != "shell" || shell.ToolInput == nil ||
		*shell.ToolInput != `{"command":["bash","-lc","grep -rn redirect src/"]}` || shell.ToolUseID == nil || *shell.ToolUseID != "call_1" {
		t.Errorf("shell call %+v", shell)
	}
	if r := s.Messages[5]; r.ToolUseID == nil || *r.ToolUseID != "call_1" {
		t.Errorf("shell result %+v", r)
	}
	if patch := s.Messages[6]; patch.ToolInput == nil || *patch.ToolInput != "*** Begin Patch\n*** End Patch" {
		t.Errorf("apply_patch input %+v", patch)
	}
}

func TestParseImagesAndDeveloper(t *testing.T) {
	content := strings.Join([]string{
		sessionMetaLine,
		line("2026-05-07T17:24:21.300Z", "response_item", map[string]any{"type": "message", "role": "developer",
			"content": []any{map[string]any{"type": "input_text", "text": "# Tools\nUse apply_patch."}}}),
		line("2026-05-07T17:24:21.400Z", "response_item", map[string]any{"type": "message", "role": "user",
			"content": []any{
				map[string]any{"type": "input_text", "text": "What is in this screenshot?"},
				map[string]any{"type": "input_image", "image_url": "data:image/png;base64,iVBORw0KGgo="},
			}}),
	}, "\n")
	s := Parse(content, "", "")
	if s == nil {
		t.Fatal("nil")
	}
	if len(s.Messages) != 2 || s.Messages[0].BlockType != "meta" || s.Messages[1].BlockType != "text" {
		t.Errorf("messages %+v", s.Messages)
	}
	if len(s.Images) != 1 || s.Images[0].MediaType != "image/png" || s.Images[0].Data != "iVBORw0KGgo=" ||
		s.Images[0].MessageUUID != s.Messages[1].UUID {
		t.Errorf("images %+v", s.Images)
	}
}

func TestParseLegacyRollout(t *testing.T) {
	// Before session_meta, the first line was the meta and the items
	// followed as they were.
	content := strings.Join([]string{
		`{"id":"old-thread","timestamp":"2025-08-01T10:00:00.000Z","instructions":null,"git":{"branch":"main"}}`,
		`{"record_type":"state","approval_policy":"on-request"}`,
		`{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}`,
		`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}`,
	}, "\n")
	s := Parse(content, "old-thread", "")
	if s == nil {
		t.Fatal("nil")
	}
	if s.Meta.SessionID != "old-thread" || s.Meta.GitBranch != "main" || s.Meta.StartedAt != "2025-08-01T10:00:00.000Z" {
		t.Errorf("meta %+v", s.Meta)
	}
	if len(s.Messages) != 2 || s.Messages[0].Content != "hello" || s.Messages[1].Content != "hi" {
		t.Errorf("messages %+v", s.Messages)
	}
}

func TestParseSkipsSubAgentsAndEmpty(t *testing.T) {
	sub := line("2026-05-07T17:24:21.100Z", "session_meta", map[string]any{
		"id": "sub", "timestamp": "2026-05-07T17:24:21.000Z", "cwd": "/x", "cli_version": "0.112.0",
		"source": map[string]any{"subagent": map[string]any{"thread_spawn": map[string]any{"parent_thread_id": threadID, "depth": 1}}},
	})
	if s := Parse(sub+"\n"+userMsg("2026-05-07T17:24:22.000Z", "review this"), "sub", ""); s != nil {
		t.Errorf("a sub-agent's rollout was parsed: %+v", s.Meta)
	}
	if s := Parse(sessionMetaLine+"\n", "", ""); s != nil {
		t.Error("a rollout with no messages was parsed")
	}
	if s := Parse(userMsg("2026-05-07T17:24:22.000Z", "no meta")+"\n", "", ""); s != nil {
		t.Error("a rollout with no meta was parsed")
	}
}

func TestIsInjected(t *testing.T) {
	cases := map[string]bool{
		"<environment_context>\n<cwd>/x</cwd>\n</environment_context>": true,
		"<user_instructions>\nread me\n</user_instructions>":           true,
		"<permissions instructions>\nnone\n</permissions>":             true,
		"<turn_aborted/>":       true,
		"<a>x</a> and <b>y</b>": false,
		"fix <b>this</b>":       false,
		"<div>hello":            false,
		"":                      false,
	}
	for text, want := range cases {
		if got := isInjected(text); got != want {
			t.Errorf("isInjected(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestTitlesAndReadFile(t *testing.T) {
	home := t.TempDir()
	os.WriteFile(filepath.Join(home, "session_index.jsonl"), []byte(
		`{"id":"a","thread_name":"first","updated_at":"2026-01-01T00:00:00Z"}`+"\n"+
			`{"id":"a","thread_name":"renamed","updated_at":"2026-01-02T00:00:00Z"}`+"\n"+
			`not json`+"\n"+
			`{"id":"b","thread_name":"other","updated_at":"2026-01-02T00:00:00Z"}`+"\n"), 0o644)
	titles := Titles(home)
	if titles["a"] != "renamed" || titles["b"] != "other" || len(titles) != 2 {
		t.Errorf("titles %v", titles)
	}
	if Titles(filepath.Join(home, "none")) == nil {
		t.Error("a missing index should give an empty map")
	}

	plain := filepath.Join(home, "rollout-2026-05-07T17-24-21-"+threadID+".jsonl")
	os.WriteFile(plain, []byte(rollout), 0o644)
	enc, _ := zstd.NewWriter(nil)
	compressed := plain + ".zst"
	os.WriteFile(compressed, enc.EncodeAll([]byte(rollout), nil), 0o644)
	for _, p := range []string{plain, compressed} {
		b, err := ReadFile(p)
		if err != nil || string(b) != rollout {
			t.Errorf("ReadFile(%s): %v, %d bytes", filepath.Base(p), err, len(b))
		}
	}
	if HomeOf(filepath.Join(home, "sessions")) != home {
		t.Error("HomeOf")
	}
}

func TestEncodeProject(t *testing.T) {
	if got := EncodeProject("/Users/me/src/app.v2"); got != "-Users-me-src-app-v2" {
		t.Errorf("got %q", got)
	}
}
