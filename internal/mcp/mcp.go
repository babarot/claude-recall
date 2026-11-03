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
	"github.com/babarot/claude-recall/internal/worktree"
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
	repoProp    = prop{"string", `Only the sessions of one repository, given as a directory in it ("." is the directory the session runs in) or as its git remote (https://github.com/owner/repo, git@github.com:owner/repo.git or github.com/owner/repo). Every checkout and worktree whose origin is that remote is included`}

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

// repoScopes is the repo input as the repository to narrow to, or nil
// when it is not given.
func (a args) repoScopes(idx *repos.Index) []db.Scope {
	if repo := strings.TrimSpace(a.str("repo")); repo != "" {
		return []db.Scope{idx.Scope(repo)}
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

// repoOf names the repository of a session the way repoOf does, but on a
// server held to a remote names every one by that remote: the index
// guesses at a removed worktree's repository from every directory of the
// archive, and its guess could name a directory outside the pin.
func (h Handlers) repoOf(idx *repos.Index) func(string) (string, string) {
	names := repoOf(idx)
	if h.Pin == nil || h.Pin.Remote == "" {
		return names
	}
	name := worktree.RemoteName(h.Pin.Remote)
	return func(path string) (string, string) {
		_, wt := names(path)
		return name, wt
	}
}

// Handlers holds the tool implementations, separate from the transport so
// they can be tested directly.
type Handlers struct {
	DB *db.DB
	// Pin, when not nil, holds every tool to one repository: what a
	// client asks for narrows within it and never widens it.
	Pin *repos.Pin
}

// scopes is the repositories a call narrows to: the pinned one, if any,
// and the one its repo input names.
func (h Handlers) scopes(idx *repos.Index, a args) []db.Scope {
	var out []db.Scope
	if h.Pin != nil {
		out = append(out, idx.PinScope(*h.Pin))
	}
	return append(out, a.repoScopes(idx)...)
}

// index resolves the archive's repositories, for a call that narrows to
// one, or that shows them.
func (h Handlers) index() (*repos.Index, error) { return repos.Load(h.DB) }

// Call runs one tool and returns its result value.
func (h Handlers) Call(name string, a args) (any, error) {
	switch name {
	case "recall_search":
		idx, err := h.index()
		if err != nil {
			return nil, err
		}
		r, err := h.DB.Search(a.str("query"), db.SearchOptions{Project: a.str("project"), Limit: a.num("limit", 10),
			From: a.str("from"), To: a.str("to"), Substring: a.flag("substring"), Scopes: h.scopes(idx, a)})
		if err != nil {
			return nil, err
		}
		return api.SearchHits(r, true, h.repoOf(idx)), nil
	case "recall_list":
		idx, err := h.index()
		if err != nil {
			return nil, err
		}
		s, err := h.DB.ListSessions(db.ListOptions{Project: a.str("project"), Limit: a.num("limit", 20),
			Scopes: h.scopes(idx, a)})
		if err != nil {
			return nil, err
		}
		return api.List(s, h.repoOf(idx)), nil
	case "recall_export":
		var scopes []db.Scope
		if h.Pin != nil {
			idx, err := h.index()
			if err != nil {
				return nil, err
			}
			scopes = []db.Scope{idx.PinScope(*h.Pin)}
		}
		id := a.str("session_id")
		s, msgs, err := h.DB.ExportSessionIn(id, scopes)
		if err != nil {
			return nil, err
		}
		return api.ExportResult(id, s, msgs, *a.num("tail", 0)), nil
	case "recall_stats":
		var scopes []db.Scope
		if h.Pin != nil {
			idx, err := h.index()
			if err != nil {
				return nil, err
			}
			scopes = []db.Scope{idx.PinScope(*h.Pin)}
		}
		s, err := h.DB.StatsIn(a.str("project"), scopes)
		if err != nil {
			return nil, err
		}
		return api.Stats(s), nil
	}
	return nil, fmt.Errorf("unknown tool: %s", name)
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

// NewServer returns the MCP server with the four tools and the prompt,
// held to one repository when pin is not nil.
func NewServer(d *db.DB, pin *repos.Pin) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "claude-recall", Version: version.Version}, nil)
	h := Handlers{DB: d, Pin: pin}
	for _, t := range []*mcp.Tool{searchTool, listTool, exportTool, statsTool} {
		if pin != nil {
			pinned := *t
			pinned.Description += " This server holds only the sessions of the repository " + pin.String() + "."
			t = &pinned
		}
		s.AddTool(t, h.handler(t.Name))
	}
	s.AddPrompt(recapPrompt, handleRecap)
	return s
}

// Run serves MCP over stdin and stdout until the client disconnects, held
// to one repository when pin is not nil.
func Run(ctx context.Context, d *db.DB, pin *repos.Pin) error {
	return NewServer(d, pin).Run(ctx, &mcp.StdioTransport{})
}
