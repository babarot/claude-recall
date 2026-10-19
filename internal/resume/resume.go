// Package resume builds the commands that go back to a session, for the
// TUI and the web UI: the agent's own resume in the session's folder
// (claude -r, or codex resume for a Codex session), or, for a session that
// cannot be resumed, a new agent that recalls it through recall's MCP
// server.
package resume

import (
	"encoding/json"
	"strings"

	"github.com/babarot/claude-recall/internal/parser"
)

// Program is the agent's command: claude, or codex for a Codex session.
func Program(agent string) string {
	if agent == parser.AgentCodex {
		return "codex"
	}
	return "claude"
}

// ResumeArgs are the agent's arguments that resume session id: claude -r
// id, or codex resume id.
func ResumeArgs(agent, id string) []string {
	if agent == parser.AgentCodex {
		return []string{"resume", id}
	}
	return []string{"-r", id}
}

// Verb is how the agent resumes a session, as a command without the ID:
// "claude -r" or "codex resume".
func Verb(agent string) string {
	args := ResumeArgs(agent, "")
	return Program(agent) + " " + strings.TrimSpace(strings.Join(args, " "))
}

// Command is the shell command that resumes session id of agent in dir.
func Command(agent, dir, id string) string {
	return "cd " + ShellQuote(dir) + " && " + Program(agent) + " " + strings.Join(ResumeArgs(agent, id), " ")
}

// recallPrompt asks a new agent to recall a session through recall's MCP
// server: where it left off, or a topic when one is given.
func recallPrompt(sessionID, topic string) string {
	if topic != "" {
		return "Use the recall tools to recall session " + sessionID +
			" (recall_export, or recall_search for the parts you need) about: " + topic +
			"\nSay briefly what it says about that, and wait for my instructions."
	}
	return "Use the recall tools to recall session " + sessionID +
		" (recall_export), then pick up where it left off: say briefly what was being done and how far it got, and wait for my instructions."
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
	// The prompt goes first: --mcp-config and --allowedTools take every
	// argument after them.
	return []string{
		recallPrompt(sessionID, topic),
		"--mcp-config", string(mcp),
		"--allowedTools", "mcp__recall__recall_search,mcp__recall__recall_list,mcp__recall__recall_export",
	}
}

// CodexRecallArgs are codex's arguments for recalling a session, as
// RecallArgs are claude's: recall's MCP server given as config overrides
// (-c), and the prompt.
func CodexRecallArgs(self []string, sessionID, topic string) []string {
	command, _ := json.Marshal(self[0])
	args, _ := json.Marshal(self[1:])
	// A JSON string or array of strings is a TOML one.
	return []string{
		"-c", "mcp_servers.recall.command=" + string(command),
		"-c", "mcp_servers.recall.args=" + string(args),
		recallPrompt(sessionID, topic),
	}
}

// AgentRecallArgs are the agent's arguments for recalling a session:
// RecallArgs for claude, CodexRecallArgs for codex.
func AgentRecallArgs(agent string, self []string, sessionID, topic string) []string {
	if agent == parser.AgentCodex {
		return CodexRecallArgs(self, sessionID, topic)
	}
	return RecallArgs(self, sessionID, topic)
}

// RecallCommand is the shell command that recalls session id of agent in
// a new agent, for a session it cannot resume.
func RecallCommand(agent string, self []string, id string) string {
	parts := []string{Program(agent)}
	for _, a := range AgentRecallArgs(agent, self, id, "") {
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

// Where tells whether the agent can read a session's transcript to resume
// it, given f, the copy the importer takes, and whether there is one at
// all (found), in the transcript trees dirs, the primary first. otherTree
// is the tree the transcript is in when it is one the agent does not read:
// for claude -r, any but the primary; for codex resume, which reads every
// tree under its own home, none. missing is set when there is no transcript
// the agent reads: none at all, or for claude -r one under a project
// directory other than the session's own, project, which it reads it from.
func Where(f parser.File, found bool, dirs []string, project string) (otherTree string, missing bool) {
	switch {
	case found && f.IsCodex():
		// codex resume finds a rollout wherever it is under its home.
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
