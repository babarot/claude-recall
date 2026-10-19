// Package codex turns a Codex CLI rollout (~/.codex/sessions/YYYY/MM/DD/
// rollout-<time>-<id>.jsonl) into the parser.Session the archive stores,
// the same rows a Claude Code transcript gives, so every way into the
// archive reads both alike.
//
// A rollout is one JSON object per line: {"timestamp", "type", "payload"},
// the type naming what the payload is. The session's identity is the
// session_meta line; the conversation is the response_item lines, the
// items Codex sends the model: messages (user, assistant, developer),
// reasoning, function calls and their outputs. The event_msg lines are the
// UI events of older rollouts, which say the same things again, and
// turn_context, compacted and the rest are bookkeeping; they are skipped.
package codex

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"

	"github.com/babarot/claude-recall/internal/jscompat"
	"github.com/babarot/claude-recall/internal/parser"
)

// HomeOf is the Codex home directory of a rollout tree (sessions or
// archived_sessions): its parent, where session_index.jsonl is. The trees
// themselves come from config.CodexSessionsDirs.
func HomeOf(tree string) string { return filepath.Dir(tree) }

// ReadFile reads a rollout, decompressing a .jsonl.zst one, which Codex
// makes of a rollout a week after it was last written.
func ReadFile(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(path, ".zst") {
		return b, nil
	}
	dec, err := zstd.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer dec.Close()
	return io.ReadAll(dec)
}

// Titles reads the names given to threads (/rename, or `codex resume`'s
// picker), from <home>/session_index.jsonl: one entry per name change, the
// last one for a thread winning. A missing or broken file yields none.
func Titles(home string) map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(filepath.Join(home, "session_index.jsonl"))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		var e struct {
			ID   string `json:"id"`
			Name string `json:"thread_name"`
		}
		if json.Unmarshal([]byte(line), &e) != nil || e.ID == "" {
			continue
		}
		out[e.ID] = e.Name
	}
	return out
}

// EncodeProject names a directory the way Claude Code names its project
// directory: every character but a letter or a digit becomes a hyphen, so
// /Users/me/src/app is -Users-me-src-app. A Codex session of a directory
// then has the project name a Claude Code session of it has.
func EncodeProject(cwd string) string {
	var b strings.Builder
	for _, r := range cwd {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// rolloutLine is a line of a rollout, in the current form or the one from
// before session_meta (mid 2025), where the first line was the meta itself
// and the rest were the items.
type rolloutLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	// The legacy first line: id, timestamp, instructions, git.
	ID  string          `json:"id"`
	Git json.RawMessage `json:"git"`
	// A legacy item line carries the item itself.
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type sessionMeta struct {
	ID             string          `json:"id"`
	Timestamp      string          `json:"timestamp"`
	Cwd            string          `json:"cwd"`
	CLIVersion     string          `json:"cli_version"`
	Source         json.RawMessage `json:"source"`
	ParentThreadID string          `json:"parent_thread_id"`
	Git            *gitInfo        `json:"git"`
}

type gitInfo struct {
	Branch string `json:"branch"`
}

// item is a response_item payload: whichever of the fields its type uses.
type item struct {
	Type    string            `json:"type"`
	Role    string            `json:"role"`
	Content json.RawMessage   `json:"content"`
	Summary []json.RawMessage `json:"summary"`
	// function_call and custom_tool_call
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Input     json.RawMessage `json:"input"`
	CallID    string          `json:"call_id"`
	// local_shell_call and web_search_call
	Action json.RawMessage `json:"action"`
	// function_call_output and custom_tool_call_output
	Output json.RawMessage `json:"output"`
}

type contentItem struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageURL string `json:"image_url"`
}

// contentItems reads a content array; a string is one text item.
func contentItems(raw json.RawMessage) []contentItem {
	if s, ok := jsString(raw); ok {
		return []contentItem{{Type: "input_text", Text: s}}
	}
	var items []contentItem
	if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &items) != nil {
		return nil
	}
	return items
}

func jsString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || raw[0] != '"' {
		return "", false
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

// isInjected reports whether a user message is context Codex put in the
// conversation rather than something the user typed: an XML-like element
// that is the whole message, as <environment_context>, <user_instructions>
// (AGENTS.md), <permissions instructions>, <turn_aborted> and the others
// are written.
func isInjected(text string) bool {
	if !strings.HasPrefix(text, "<") {
		return false
	}
	end := strings.IndexAny(text, " >/\n")
	if end < 2 {
		return false
	}
	name := text[1:end]
	for _, r := range name {
		if !(r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return strings.HasSuffix(text, "</"+name+">") || strings.HasSuffix(text, "/>")
}

// isSubAgent reports whether a session's source is a sub-agent spawned by
// another thread (a review, a compaction, a spawned agent): the side
// conversations of a session, not one of the user's.
func isSubAgent(meta sessionMeta) bool {
	if meta.ParentThreadID != "" {
		return true
	}
	var src map[string]json.RawMessage
	if json.Unmarshal(meta.Source, &src) != nil {
		return false
	}
	_, ok := src["subagent"]
	return ok
}

const toolResultLimit = 10000

// Parse parses a rollout. sessionID is the ID from the file's name, used
// when the rollout has no session_meta line; title is the thread's name
// from Titles, or "". It returns nil for a rollout with no messages, and
// for a sub-agent's.
func Parse(content, sessionID, title string) *parser.Session {
	var (
		meta      *sessionMeta
		messages  []parser.Message
		images    []parser.Image
		firstUser string
		lastTS    string
		turn      int
	)
	for n, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var l rolloutLine
		if json.Unmarshal([]byte(line), &l) != nil {
			continue
		}
		var it item
		switch l.Type {
		case "session_meta":
			var m sessionMeta
			if json.Unmarshal(l.Payload, &m) == nil && meta == nil {
				if m.Timestamp == "" {
					m.Timestamp = l.Timestamp
				}
				meta = &m
			}
			continue
		case "response_item":
			if json.Unmarshal(l.Payload, &it) != nil {
				continue
			}
		case "":
			switch {
			case l.ID != "" && meta == nil: // the legacy first line
				m := sessionMeta{ID: l.ID, Timestamp: l.Timestamp}
				if l.Git != nil {
					json.Unmarshal(l.Git, &m.Git)
				}
				meta = &m
				continue
			default:
				continue
			}
		case "message", "reasoning", "function_call", "function_call_output", "custom_tool_call",
			"custom_tool_call_output", "local_shell_call", "web_search_call":
			// A legacy item line.
			if json.Unmarshal([]byte(line), &it) != nil {
				continue
			}
		default:
			continue
		}
		uuid := strconv.Itoa(n + 1)
		ts := l.Timestamp
		msg := parser.Message{UUID: uuid, Timestamp: ts, TurnIndex: turn}
		switch it.Type {
		case "message":
			role := it.Role
			if role != "user" && role != "assistant" {
				// developer and system: instructions put in by Codex.
				role = "user"
				if text := jscompat.Trim(joinText(contentItems(it.Content))); text != "" {
					msg.Role, msg.BlockType, msg.Content = role, "meta", text
					messages = append(messages, msg)
					turn++
				}
				continue
			}
			imgIdx := 0
			for i, c := range contentItems(it.Content) {
				switch c.Type {
				case "input_text", "output_text", "text":
					text := jscompat.Trim(c.Text)
					if text == "" {
						continue
					}
					m := msg
					m.Role, m.BlockIndex, m.TurnIndex = role, i, turn
					if role == "user" && isInjected(text) {
						m.BlockType = "meta"
					} else {
						m.BlockType = "text"
						lastTS = ts
						if role == "user" && firstUser == "" {
							firstUser = text
						}
					}
					m.Content = text
					messages = append(messages, m)
					turn++
				case "input_image":
					if mediaType, data, ok := dataURL(c.ImageURL); ok {
						images = append(images, parser.Image{MessageUUID: uuid, ImageIndex: imgIdx, MediaType: mediaType, Data: data})
						imgIdx++
					}
				}
			}
			continue
		case "reasoning":
			i := 0
			for _, raw := range it.Summary {
				var c contentItem
				if json.Unmarshal(raw, &c) != nil {
					continue
				}
				if text := jscompat.Trim(c.Text); text != "" {
					m := msg
					m.Role, m.BlockType, m.BlockIndex, m.Content, m.TurnIndex = "assistant", "thinking", i, text, turn
					messages = append(messages, m)
					turn++
					i++
				}
			}
			for _, c := range contentItems(it.Content) {
				if text := jscompat.Trim(c.Text); text != "" {
					m := msg
					m.Role, m.BlockType, m.BlockIndex, m.Content, m.TurnIndex = "assistant", "thinking", i, text, turn
					messages = append(messages, m)
					turn++
					i++
				}
			}
			continue
		case "function_call", "custom_tool_call", "local_shell_call", "web_search_call":
			name := it.Name
			input := it.Arguments
			switch it.Type {
			case "custom_tool_call":
				input = it.Input
			case "local_shell_call":
				name, input = "local_shell", it.Action
			case "web_search_call":
				name, input = "web_search", it.Action
			}
			if name == "" {
				name = "unknown"
			}
			in := ""
			if s, ok := jsString(input); ok {
				in = s
			} else if len(input) > 0 && string(input) != "null" {
				in = string(input)
			}
			msg.Role, msg.BlockType, msg.Content = "assistant", "tool_use", name
			msg.ToolName, msg.ToolInput = ptr(name), ptr(in)
			msg.ToolUseID = nonEmpty(it.CallID)
		case "function_call_output", "custom_tool_call_output":
			text := joinText(contentItems(it.Output))
			if jscompat.Len(text) > toolResultLimit {
				text = jscompat.Slice(text, toolResultLimit) + "\n... (truncated)"
			}
			msg.Role, msg.BlockType, msg.Content = "user", "tool_result", text
			msg.ToolUseID = nonEmpty(it.CallID)
			lastTS = ts
		default:
			continue
		}
		messages = append(messages, msg)
		turn++
	}
	if meta == nil || len(messages) == 0 || isSubAgent(*meta) {
		return nil
	}
	id := meta.ID
	if id == "" {
		id = sessionID
	}
	m := parser.Meta{
		SessionID:     id,
		Agent:         parser.AgentCodex,
		Project:       EncodeProject(meta.Cwd),
		ProjectPath:   meta.Cwd,
		FirstPrompt:   jscompat.Slice(firstUser, 500),
		StartedAt:     meta.Timestamp,
		EndedAt:       lastTS,
		ClaudeVersion: meta.CLIVersion,
		Title:         jscompat.Trim(title),
	}
	if meta.Git != nil {
		m.GitBranch = meta.Git.Branch
	}
	return &parser.Session{Meta: m, Messages: messages, Images: images}
}

// joinText joins the text items of a content array, as they are: a tool's
// output keeps its trailing newline, as a Claude Code tool_result does.
func joinText(items []contentItem) string {
	var parts []string
	for _, c := range items {
		switch c.Type {
		case "input_text", "output_text", "text", "reasoning_text":
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// dataURL splits a data:<media type>;base64,<data> URL.
func dataURL(u string) (mediaType, data string, ok bool) {
	rest, ok := strings.CutPrefix(u, "data:")
	if !ok {
		return "", "", false
	}
	head, data, ok := strings.Cut(rest, ",")
	if !ok || !strings.HasSuffix(head, ";base64") {
		return "", "", false
	}
	return strings.TrimSuffix(head, ";base64"), data, true
}

func ptr(s string) *string { return &s }

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
