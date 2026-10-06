// Package mcp is the MCP server: the recall_search, recall_list,
// recall_export and recall_stats tools and the recap prompt over stdio,
// on the official Go SDK.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/babarot/claude-recall/internal/api"
	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/jscompat"
	"github.com/babarot/claude-recall/internal/repos"
	"github.com/babarot/claude-recall/internal/version"
)

type prop struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type schema struct {
	Type       string          `json:"type"`
	Properties map[string]prop `json:"properties"`
	Required   []string        `json:"required,omitempty"`
}

var (
	projectProp = prop{"string", "Filter by project name (partial match)"}
	repoProp    = prop{"string", `Only the sessions of the repository this directory is in, its worktrees included ("." is the directory the session runs in)`}

	searchTool = &mcp.Tool{
		Name:        "recall_search",
		Description: "Search past coding agent session conversations by full-text query. Use this when you need to find previous discussions, decisions, or context from past sessions. Each hit carries its session's title, message count, first prompt and repository. Text in Japanese and other scripts written without spaces is matched as a substring, newest first.",
		InputSchema: schema{Type: "object", Required: []string{"query"}, Properties: map[string]prop{
			"query":     {"string", `Full-text search query. Supports FTS5 syntax: "exact phrase", term1 AND term2, term1 OR term2, term1 NOT term2`},
			"project":   projectProp,
			"repo":      repoProp,
			"substring": {"boolean", "Match the query as plain text anywhere in a message, newest first, instead of as FTS5 words (always so for Japanese and other unspaced text)"},
			"limit":     {"number", "Max results (default: 10)"},
			"from":      {"string", "Start date filter (YYYY-MM-DD)"},
			"to":        {"string", "End date filter (YYYY-MM-DD)"},
		}},
	}
	listTool = &mcp.Tool{
		Name:        "recall_list",
		Description: "List archived coding agent sessions. Use this to see what sessions are available before exporting a specific one.",
		InputSchema: schema{Type: "object", Properties: map[string]prop{
			"project": projectProp,
			"repo":    repoProp,
			"limit":   {"number", "Max sessions to return (default: 20)"},
		}},
	}
	exportTool = &mcp.Tool{
		Name:        "recall_export",
		Description: "Export the full conversation of a specific session. Use this to get detailed context from a past session found via recall_search or recall_list.",
		InputSchema: schema{Type: "object", Required: []string{"session_id"}, Properties: map[string]prop{
			"session_id": {"string", "Session ID (full UUID or prefix). Get this from recall_search or recall_list results."},
			"tail":       {"number", "Return only the last N messages, for a long session; omitted in the result counts the earlier ones left out. Default: all"},
		}},
	}
	statsTool = &mcp.Tool{
		Name:        "recall_stats",
		Description: "Show archive statistics: total sessions, messages, breakdown by project and month.",
		InputSchema: schema{Type: "object", Properties: map[string]prop{"project": projectProp}},
	}
)

// args is the loosely typed arguments object. A missing or mistyped value
// falls back to its default instead of failing the call.
type args map[string]any

func (a args) str(key string) string {
	s, _ := a[key].(string)
	return s
}

func (a args) flag(key string) bool {
	switch v := a[key].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	}
	return false
}

// repoPaths is the repo input as the directories to narrow to, or nil when
// it is not given.
func (a args) repoPaths(idx *repos.Index) []string {
	if dir := strings.TrimSpace(a.str("repo")); dir != "" {
		return idx.PathsIn(dir)
	}
	return nil
}

func (a args) num(key string, def int) *int {
	v, ok := a[key]
	if !ok || v == nil {
		return &def
	}
	switch t := v.(type) {
	case float64:
		n := int(t)
		return &n
	case string: // SQLite reads a numeric string in LIMIT as the number
		if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			n := int(f)
			return &n
		}
	}
	return &def
}

// repoOf names the repository and worktree of a session directory.
func repoOf(idx *repos.Index) func(string) (string, string) {
	return func(path string) (string, string) {
		repo := idx.Of(path)
		return repo.Name, repo.Worktree
	}
}

// Handlers holds the tool implementations, separate from the transport so
// they can be tested directly.
type Handlers struct{ DB *db.DB }

// Call runs one tool and returns its result value.
func (h Handlers) Call(name string, a args) (any, error) {
	switch name {
	case "recall_search":
		idx, err := repos.Load(h.DB)
		if err != nil {
			return nil, err
		}
		r, err := h.DB.Search(a.str("query"), db.SearchOptions{Project: a.str("project"), Limit: a.num("limit", 10),
			From: a.str("from"), To: a.str("to"), Substring: a.flag("substring"), ProjectPaths: a.repoPaths(idx)})
		if err != nil {
			return nil, err
		}
		return api.SearchHits(r, true, repoOf(idx)), nil
	case "recall_list":
		idx, err := repos.Load(h.DB)
		if err != nil {
			return nil, err
		}
		s, err := h.DB.ListSessions(db.ListOptions{Project: a.str("project"), Limit: a.num("limit", 20),
			ProjectPaths: a.repoPaths(idx)})
		if err != nil {
			return nil, err
		}
		return api.List(s, repoOf(idx)), nil
	case "recall_export":
		id := a.str("session_id")
		s, msgs, err := h.DB.ExportSession(id)
		if err != nil {
			return nil, err
		}
		return api.ExportResult(id, s, msgs, *a.num("tail", 0)), nil
	case "recall_stats":
		s, err := h.DB.Stats(a.str("project"))
		if err != nil {
			return nil, err
		}
		return api.Stats(s), nil
	}
	return nil, fmt.Errorf("Unknown tool: %s", name)
}

func (h Handlers) handler(name string) mcp.ToolHandler {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a := args{}
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &a); err != nil {
				a = args{}
			}
		}
		result, err := h.Call(name, a)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "Error: " + err.Error()}}}, nil
		}
		text, err := jscompat.Marshal(result, "  ")
		if err != nil {
			return nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(text)}}}, nil
	}
}

// recapPrompt asks the agent to read where a past session ended and say
// where it stood, then stop: how the work goes on is the user's to say,
// with their next message and Claude Code's own settings, not recall's. A
// client shows it as a command (Claude Code: /<server>:recap <session_id>);
// the instruction is English, and the agent answers in the user's language.
var recapPrompt = &mcp.Prompt{
	Name:        "recap",
	Title:       "Recap a past session",
	Description: "Read where a past session ended and sum up what was done, what was decided and what was left, then ask how to go on.",
	Arguments: []*mcp.PromptArgument{{
		Name:        "session_id",
		Description: "Session ID (full UUID or prefix), as recall_search or recall_list shows it.",
		Required:    true,
	}},
}

// recapTail is how many of the last messages the prompt asks for first:
// enough to see where a session stopped, little enough to read whole.
const recapTail = 40

func recapMessage(id string) string {
	return "Recap the past session " + id + ". Read where it ended first, with recall_export and tail " +
		strconv.Itoa(recapTail) + "; read further back (a larger tail, or no tail) only if that is not enough. " +
		"Then sum up in a few lines what was being done, what was decided and what was left open, " +
		"and stop there to ask the user how to go on. If no session has that ID, say so and stop."
}

func handleRecap(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	id := strings.TrimSpace(req.Params.Arguments["session_id"])
	if id == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	return &mcp.GetPromptResult{
		Description: recapPrompt.Description,
		Messages:    []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: recapMessage(id)}}},
	}, nil
}

// NewServer returns the MCP server with the four tools and the prompt.
func NewServer(d *db.DB) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "claude-recall", Version: version.Version}, nil)
	h := Handlers{DB: d}
	for _, t := range []*mcp.Tool{searchTool, listTool, exportTool, statsTool} {
		s.AddTool(t, h.handler(t.Name))
	}
	s.AddPrompt(recapPrompt, handleRecap)
	return s
}

// Run serves MCP over stdin and stdout until the client disconnects.
func Run(ctx context.Context, d *db.DB) error {
	return NewServer(d).Run(ctx, &mcp.StdioTransport{})
}
