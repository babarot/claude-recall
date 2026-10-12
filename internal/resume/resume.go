// Package resume builds the commands that go back to a session, for the
// TUI and the web UI: claude -r in the session's folder, or, for a session
// claude -r cannot resume, a new claude that recalls it through recall's
// MCP server.
package resume

import (
	"encoding/json"
	"strings"

	"github.com/babarot/claude-recall/internal/parser"
)

// Command is the shell command that resumes session id in dir.
func Command(dir, id string) string {
	return "cd " + ShellQuote(dir) + " && claude -r " + id
}

// RecallArgs are claude's arguments for recalling a session: recall's MCP
// server, run as the command line self, beside the user's own, its
// read-only tools allowed, and a first prompt asking to recall the session,
// as one would in a session. A topic that is not empty asks about that
// rather than where the session left off.
func RecallArgs(self []string, sessionID, topic string) []string {
	mcp, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{
		"recall": map[string]any{"command": self[0], "args": self[1:]},
	}})
	prompt := "Use the recall tools to recall session " + sessionID +
		" (recall_export), then pick up where it left off: say briefly what was being done and how far it got, and wait for my instructions."
	if topic != "" {
		prompt = "Use the recall tools to recall session " + sessionID +
			" (recall_export, or recall_search for the parts you need) about: " + topic +
			"\nSay briefly what it says about that, and wait for my instructions."
	}
	// The prompt goes first: --mcp-config and --allowedTools take every
	// argument after them.
	return []string{
		prompt,
		"--mcp-config", string(mcp),
		"--allowedTools", "mcp__recall__recall_search,mcp__recall__recall_list,mcp__recall__recall_export",
	}
}

// RecallCommand is the shell command that recalls session id in a new
// claude, for a session claude -r cannot resume.
func RecallCommand(self []string, id string) string {
	parts := []string{"claude"}
	for _, a := range RecallArgs(self, id, "") {
		parts = append(parts, ShellQuote(a))
	}
	return strings.Join(parts, " ")
}

// ShellQuote quotes s for a POSIX shell, leaving a plain path as it is.
func ShellQuote(s string) string {
	if s != "" && strings.IndexFunc(s, func(c rune) bool {
		return !(c == '/' || c == '.' || c == '-' || c == '_' || c == '~' ||
			c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9')
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Where tells whether claude -r can read a session's transcript, given f,
// the copy the importer takes, and whether there is one at all (found), in
// the transcript trees dirs, the primary first. otherTree is the tree the
// transcript is in when it is not the primary one, which claude -r does not
// read; missing is set when there is no transcript claude -r reads: none at
// all, or one under a project directory other than the session's own,
// project, which claude -r reads it from.
func Where(f parser.File, found bool, dirs []string, project string) (otherTree string, missing bool) {
	switch {
	case found && f.Dir != dirs[0]:
		otherTree = f.Dir
	case found:
		found = f.Project == project
	}
	return otherTree, !found
}

// Locate is Where for one session, looked up in the trees dirs.
func Locate(dirs []string, id, project string) (otherTree string, missing bool) {
	if len(dirs) == 0 {
		return "", true
	}
	copies := parser.FindSession(dirs, id)
	if len(copies) == 0 {
		return Where(parser.File{}, false, dirs, project)
	}
	return Where(parser.Choose(copies)[0], true, dirs, project)
}
