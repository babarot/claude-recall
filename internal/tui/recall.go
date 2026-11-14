package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/babarot/claude-recall/internal/resume"
	"github.com/babarot/claude-recall/internal/worktree"
)

// c recalls the selected session in a new claude, for a session claude -r
// cannot resume (its folder or transcript is gone, or the transcript is in
// a tree claude -r does not read): a modal takes what to recall about it, empty for where it left off, then the TUI
// quits and the caller starts claude in the folder recall was started in,
// asking it to recall the session through recall's MCP server.

// Recall is what the caller should run after the TUI exits instead of a
// resume: a new claude that recalls SessionID, about Topic when it is not
// empty.
type Recall struct {
	SessionID string
	Topic     string
}

// RecallWith sets the command line of recall's MCP server, which a session
// recalled in a new claude gets.
func (m Model) RecallWith(self []string) Model {
	m.recallSelf = self
	return m
}

// recallCommand is the shell command that recalls session id in a new
// claude, as c does, for a session claude -r cannot resume.
func (m Model) recallCommand(id string) string { return resume.RecallCommand(m.recallSelf, id) }

type recallState struct {
	id    string
	input textinput.Model
}

func newRecallInput() textinput.Model {
	in := newInput()
	in.Prompt = "› "
	in.Placeholder = "what to recall about it (empty: pick up where it left off)"
	return in
}

// openRecall opens the modal for session id.
func (m *Model) openRecall(id string) tea.Cmd {
	m.openModalKind(modalRecall)
	m.recall.id = id
	m.recall.input.SetValue("")
	m.recall.input.SetWidth(m.boxWidth(recallSpec().box) - 8)
	return m.recall.input.Focus()
}

func (m *Model) closeRecall() {
	if m.modal == modalRecall {
		m.closeModal()
	}
}

// recallLines are the rows of the modal before recalling: the session,
// the field for what to recall about it, and where claude will run.
func (m Model) recallLines() []boxLine {
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
	body = append(body, m.inputView(uiRecall, m.recall.input), "")
	where := "  claude with recall's MCP server"
	if m.startDir != "" {
		where += ", in " + worktree.TildePath(m.startDir, m.home)
	}
	body = append(body, m.st.dim.Render(where))
	out := make([]boxLine, len(body))
	for i, l := range body {
		out[i] = boxLine{text: l}
	}
	return out
}

// recallSpec declares the modal before recalling a session: what to recall about it.
func recallSpec() modalSpec {
	return modalSpec{
		field: func(m *Model) *textinput.Model { return &m.recall.input },
		own: []modalKey{{keys: []string{"enter"}, word: "start claude", edge: true, do: func(m *Model) tea.Cmd {
			m.Recall = &Recall{SessionID: m.recall.id, Topic: strings.TrimSpace(m.recall.input.Value())}
			m.closeRecall()
			return m.imagesQuit()
		}}},
		esc: closeKey(func(m *Model) { m.closeRecall() }),
		box: &modalBox{
			title: func(Model) string { return "Recall in a new claude" },
			width: askWidth, minWidth: 30,
			frame: frameAccent,
			lines: func(m Model, _, _ int) []boxLine { return m.recallLines() },
		},
	}
}
