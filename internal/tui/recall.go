package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// c recalls the selected session in a new claude, for a session claude -r
// cannot resume (its folder or transcript is gone, or the transcript is in
// a tree claude -r does not read): a box over the screen
// takes what to recall about it, empty for where it left off, then the TUI
// quits and the caller starts claude in the folder recall was started in,
// asking it to recall the session through recall's MCP server.

// Recall is what the caller should run after the TUI exits instead of a
// resume: a new claude that recalls SessionID, about Topic when it is not
// empty.
type Recall struct {
	SessionID string
	Topic     string
}

// RecallArgs are claude's arguments for recalling a session: recall's MCP
// server, run as the command line self, beside the user's own, its
// read-only tools allowed, and a first prompt asking to recall the session,
// as one would in a session.
func RecallArgs(self []string, r Recall) []string {
	mcp, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{
		"recall": map[string]any{"command": self[0], "args": self[1:]},
	}})
	prompt := "Use the recall tools to recall session " + r.SessionID +
		" (recall_export), then pick up where it left off: say briefly what was being done and how far it got, and wait for my instructions."
	if r.Topic != "" {
		prompt = "Use the recall tools to recall session " + r.SessionID +
			" (recall_export, or recall_search for the parts you need) about: " + r.Topic +
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

// RecallWith sets the command line of recall's MCP server, which a session
// recalled in a new claude gets.
func (m Model) RecallWith(self []string) Model {
	m.recallSelf = self
	return m
}

// recallCommand is the shell command that recalls session id in a new
// claude, as c does, for a session claude -r cannot resume.
func (m Model) recallCommand(id string) string {
	parts := []string{"claude"}
	for _, a := range RecallArgs(m.recallSelf, Recall{SessionID: id}) {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}

type recallState struct {
	open  bool
	id    string
	input textinput.Model
}

func newRecallInput() textinput.Model {
	in := textinput.New()
	in.Prompt = "› "
	in.Placeholder = "what to recall about it (empty: pick up where it left off)"
	return in
}

// openRecall shows the box for session id.
func (m *Model) openRecall(id string) tea.Cmd {
	m.recall.open, m.recall.id = true, id
	m.recall.input.SetValue("")
	m.recall.input.SetWidth(min(askWidth, m.width-4) - 8)
	return m.recall.input.Focus()
}

func (m *Model) closeRecall() {
	m.recall.open = false
	m.recall.input.Blur()
}

// updateRecall handles a key while the box shows.
func (m Model) updateRecall(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.Recall = &Recall{SessionID: m.recall.id, Topic: strings.TrimSpace(m.recall.input.Value())}
		m.closeRecall()
		return m, m.imagesQuit()
	case "esc":
		m.closeRecall()
		return m, nil
	}
	var cmd tea.Cmd
	m.recall.input, cmd = m.recall.input.Update(msg)
	return m, cmd
}

// withRecall lays the box over the screen.
func (m Model) withRecall(screen string) string {
	w := min(askWidth, m.width-4)
	if w < 30 {
		return screen
	}
	var body []string
	for i := range m.rows {
		if r := &m.rows[i]; r.s.ID == m.recall.id {
			where := r.folder
			if r.worktree != "" {
				where += " " + worktreeM + " " + r.worktree
			}
			meta := fmt.Sprintf("  %s · %s", where, r.s.ID[:min(8, len(r.s.ID))])
			body = append(body, m.st.title.Render(r.title)+m.st.muted.Render(meta), "")
		}
	}
	body = append(body, m.recall.input.View(), "")
	where := "  claude with recall's MCP server"
	if m.startDir != "" {
		where += ", in " + tildePath(m.startDir, m.home)
	}
	body = append(body, m.st.dim.Render(where))
	return m.boxOver(screen, w, "Recall in a new claude", "enter start · esc close", body)
}
