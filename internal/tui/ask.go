package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/babarot/claude-recall/internal/config"
	"github.com/babarot/claude-recall/internal/logging"
)

// a asks Claude to find sessions, in a modal: type a
// question, watch it search, pick one of the sessions it found to jump to,
// or f to narrow the list to all of them. Why Claude picked each stays
// under its row and in Conversation (tui.ask_reasons).

type askStage int

// The stage is read only while Ask shows (m.modal is modalAsk); askClosed
// is the stage before Ask is first opened and after it is closed, not
// what tells whether it shows.
const (
	askClosed askStage = iota
	askTyping
	askRunning
	askAnswered
	askFailed
)

const (
	// askStepRows is how many of a running ask's searches show: with the
	// question and the spinner, as many rows as the box has while the
	// question is typed.
	askStepRows  = 3
	askWidth     = 96
	askTickEvery = 200 * time.Millisecond
	spinFrames   = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
)

type askState struct {
	stage    askStage
	input    textinput.Model
	question string
	steps    []string
	started  time.Time
	took     time.Duration
	hits     []askHit
	cost     float64
	model    string
	sel      int
	err      string
	cancel   context.CancelFunc
	seq      int // the run messages belong to; older ones are dropped
}

type askStepMsg struct {
	seq  int
	step string
	ch   <-chan tea.Msg
}

type askDoneMsg struct {
	seq    int
	answer askAnswer
	err    error
}

type askTickMsg struct{ seq int }

// AskWith makes asking give Claude recall's MCP server as the command line
// recall (this binary, with its database) and run it from dir.
func (m Model) AskWith(recall []string, dir string) Model {
	m.askRun = claudeRunner(recall, config.ModelID(m.cfg.AskModel), dir)
	return m
}

func newAskInput() textinput.Model {
	in := newInput()
	in.Prompt = "› "
	in.Placeholder = "what are you looking for?"
	return in
}

// openAsk opens the modal with the last question to edit.
func (m *Model) openAsk() tea.Cmd {
	m.openModalKind(modalAsk)
	m.ask.stage = askTyping
	m.ask.input.SetValue(m.ask.question)
	m.ask.input.CursorEnd()
	m.ask.input.SetWidth(m.boxWidth(askBox()) - 8)
	m.askHist.recall.Leave()
	return tea.Batch(m.ask.input.Focus(), m.loadAskHistory())
}

// startAsk runs the question in the background.
func (m *Model) startAsk() tea.Cmd {
	q := strings.TrimSpace(m.ask.input.Value())
	if q == "" {
		return nil
	}
	m.ask.input.Blur()
	ctx, cancel := context.WithCancel(context.Background())
	m.ask.seq++
	seq := m.ask.seq
	m.ask.stage, m.ask.question, m.ask.steps, m.ask.cancel = askRunning, q, nil, cancel
	m.ask.started = m.now()
	ch := make(chan tea.Msg, 64)
	run := m.askRun
	go func() {
		answer, err := run(ctx, q, func(step string) { ch <- askStepMsg{seq: seq, step: step} })
		ch <- askDoneMsg{seq, answer, err}
		close(ch)
	}()
	m.askHist.recall.Leave()
	return tea.Batch(waitAsk(ch), m.askTick(seq), m.keepQuestion(q))
}

func (m *Model) askTick(seq int) tea.Cmd {
	return tea.Tick(askTickEvery, func(time.Time) tea.Msg { return askTickMsg{seq} })
}

// closeAsk closes the modal, stopping a run.
func (m *Model) closeAsk() {
	if m.modal == modalAsk {
		m.closeModal()
	}
}

// endAsk is what closing Ask takes: a run stopped, and its search and
// field put away. Its stage is read only while Ask shows.
func (m *Model) endAsk() {
	m.askHist.search = askSearch{}
	if m.ask.cancel != nil {
		m.ask.cancel()
		m.ask.cancel = nil
	}
	m.ask.seq++
	m.ask.stage = askClosed
	m.ask.input.Blur()
}

// askMsg handles the background run's messages.
func (m *Model) askMsg(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case askStepMsg:
		if msg.seq != m.ask.seq {
			return nil
		}
		m.ask.steps = append(m.ask.steps, msg.step)
		return waitAsk(msg.ch)
	case askDoneMsg:
		if msg.seq != m.ask.seq {
			return nil
		}
		m.ask.cancel = nil
		m.ask.took = m.now().Sub(m.ask.started)
		if msg.err != nil {
			m.ask.stage, m.ask.err = askFailed, msg.err.Error()
			m.logger().Warn("ask failed", logging.KeyDurationMS, m.ask.took.Milliseconds(), logging.KeyErr, askErrForLog(msg.err))
			return nil
		}
		m.ask.stage, m.ask.sel = askAnswered, 0
		m.ask.hits = m.resolveHits(msg.answer.hits)
		m.ask.cost, m.ask.model = msg.answer.cost, msg.answer.model
		for _, h := range m.ask.hits {
			m.reasons[h.id] = h.why
		}
		m.logger().Info("asked", logging.KeyCount, len(m.ask.hits), logging.KeyDurationMS, m.ask.took.Milliseconds(),
			logging.KeyModel, m.ask.model, logging.KeyCost, m.ask.cost)
		return m.keepReply(m.ask.hits)
	case askTickMsg:
		if msg.seq == m.ask.seq && m.ask.stage == askRunning {
			return m.askTick(msg.seq)
		}
	}
	return nil
}

// waitAsk delivers the run's next message; a step carries the channel so
// the one after it can be waited for in turn.
func waitAsk(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		if s, isStep := msg.(askStepMsg); isStep {
			s.ch = ch
			return s
		}
		return msg
	}
}

// resolveHits turns Claude's IDs, often shortened, into the sessions' own,
// dropping any it made up or repeated.
func (m Model) resolveHits(hits []askHit) []askHit {
	var out []askHit
	seen := map[string]bool{}
	for _, h := range hits {
		id := strings.ToLower(h.id)
		if len(id) < 4 {
			continue
		}
		for i := range m.rows {
			full := m.rows[i].s.ID
			if strings.HasPrefix(strings.ToLower(full), id) && !seen[full] {
				seen[full] = true
				out = append(out, askHit{full, h.why})
				break
			}
		}
	}
	return out
}

// jumpTo selects session id, showing every folder and dropping the filter
// first when they hide it.
func (m *Model) jumpTo(id string) {
	find := func() int {
		for i, idx := range m.visible {
			if m.rows[idx].s.ID == id {
				return i
			}
		}
		return -1
	}
	if find() < 0 {
		m.asked, m.scope = nil, ""
		m.filter.SetValue("")
		m.refresh()
	}
	if i := find(); i >= 0 {
		m.cursor = i
	}
	m.focus = focusList
	m.clamp()
}

// showAsked narrows the list to the sessions of the last answer, in
// Claude's order.
func (m *Model) showAsked() {
	m.asked = map[string]int{}
	for i, h := range m.ask.hits {
		m.asked[h.id] = i
	}
	m.askedFor = m.ask.question
	m.filter.SetValue("")
	m.refresh()
	m.cursor, m.offset = 0, 0
	m.focus = focusList
	m.clamp()
}

// reasonLine shows why Claude picked the selected session, for the
// Conversation frame.
func (m Model) reasonLine(id string, w int) string {
	why := m.reasons[id]
	if !m.cfg.AskReasons || why == "" {
		return ""
	}
	return ansi.Truncate(m.st.worktree.Render("↳ ")+m.st.subtle.Render(why), w, ellipsis)
}

// askBox is how Ask is drawn in every stage: near the top, as its answer
// grows to the height of the screen.
func askBox() *modalBox {
	return &modalBox{
		title: func(m Model) string {
			if m.ask.stage == askAnswered {
				return fmt.Sprintf("Ask Claude · %d sessions", len(m.ask.hits))
			}
			return "Ask Claude"
		},
		width: askWidth, minWidth: 30,
		frame: frameAccent, place: placeTop,
		lines: func(m Model, inner, rows int) []boxLine {
			body := m.askLines(inner, rows)
			out := make([]boxLine, len(body))
			for i, l := range body {
				out[i] = boxLine{text: l}
			}
			return out
		},
		edgeText: func(m Model) string {
			if m.ask.stage == askAnswered {
				return m.askSummary()
			}
			return ""
		},
	}
}

// askLines are Ask's rows in the stage it is in, inner cells wide, at most
// rows of them.
func (m Model) askLines(inner, rows int) []string {
	var body []string
	q := m.st.filter.Render("› " + m.ask.question)
	switch m.ask.stage {
	case askTyping:
		if m.askHist.search.open {
			return m.askSearchBody(inner)
		}
		body = []string{m.askFieldLine(),
			m.st.muted.Render(`  e.g. "the session where we fixed the terraform state" · "who looked at PR #7427"`), "",
			m.st.dim.Render(fmt.Sprintf("  claude -p · %s · searches with recall only (no files, no shell)", config.ModelID(m.cfg.AskModel))),
			m.st.dim.Render("  uses your Claude plan, as Claude Code does")}
	case askRunning:
		elapsed := m.now().Sub(m.ask.started)
		spin := string([]rune(spinFrames)[int(elapsed/(100*time.Millisecond))%len([]rune(spinFrames))])
		body = []string{ansi.Truncate(q, inner, ellipsis),
			m.st.id.Render(spin) + " " + m.st.subtle.Render("searching…") + m.st.muted.Render(fmt.Sprintf(" %ds", int(elapsed.Seconds())))}
		// The last few searches, in rows kept for them from the start, so
		// the box stays as tall as it was while the question was typed and
		// a new search pushes the oldest out.
		steps := m.ask.steps[max(0, len(m.ask.steps)-askStepRows):]
		for i := range askStepRows {
			line := ""
			if i < len(steps) {
				line = m.st.muted.Render("  · " + ansi.Truncate(steps[i], inner-4, ellipsis))
			}
			body = append(body, line)
		}
	case askAnswered:
		body = []string{ansi.Truncate(q, inner, ellipsis), ""}
		if len(m.ask.hits) == 0 {
			body = append(body, m.st.muted.Render("  No session matched. r asks again."))
		}
		room := max(3, rows-2-len(body))
		var items [][]string
		for i, h := range m.ask.hits {
			items = append(items, m.askItem(h, inner, i == m.ask.sel))
		}
		// Keep the selected item in view.
		from, used := 0, 0
		for i := 0; i <= m.ask.sel && i < len(items); i++ {
			used += len(items[i])
			for used > room && from < i {
				used -= len(items[from])
				from++
			}
		}
		for i := from; i < len(items) && len(body)+len(items[i]) <= room+2; i++ {
			body = append(body, items[i]...)
		}
	case askFailed:
		body = []string{ansi.Truncate(q, inner, ellipsis), ""}
		for _, l := range wrap(m.ask.err, inner) {
			body = append(body, m.st.warn.Render(l))
		}
	}
	return body
}

// askSummary says how an answer came: the model, how long it took and,
// with tui.ask_show_cost, what claude says it cost.
func (m Model) askSummary() string {
	parts := []string{}
	if m.ask.model != "" {
		parts = append(parts, m.ask.model)
	}
	parts = append(parts, fmt.Sprintf("%ds", int(m.ask.took.Seconds())))
	if m.cfg.AskShowCost && m.ask.cost > 0 {
		parts = append(parts, fmt.Sprintf("$%.2f", m.ask.cost))
	}
	return strings.Join(parts, " · ")
}

// askItem is a found session in the modal's list: its title and where it is, then
// Claude's reason in up to two lines.
func (m Model) askItem(h askHit, w int, sel bool) []string {
	var r *row
	for i := range m.rows {
		if m.rows[i].s.ID == h.id {
			r = &m.rows[i]
		}
	}
	if r == nil {
		return nil
	}
	where := r.folder
	if r.worktree != "" {
		where += " " + worktreeM + " " + r.worktree
	}
	meta := fmt.Sprintf("  %s · %s · %d msgs · %s", where, relativeDate(r.s.EndedAt, m.now()), r.s.MessageCount, r.s.ID[:min(8, len(r.s.ID))])
	lines := []string{m.st.title.Render(r.title) + m.st.muted.Render(meta)}
	why := wrap(h.why, w-2)
	if len(why) > 2 {
		why = append(why[:1], ansi.Truncate(why[1]+" "+strings.Join(why[2:], " "), w-2, ellipsis))
	}
	for _, l := range why {
		lines = append(lines, m.st.subtle.Render(l))
	}
	lead := "  "
	if sel {
		lead = m.st.bar.Render("▎") + " "
	}
	for i := range lines {
		lines[i] = lead + ansi.Truncate(lines[i], w-2, ellipsis)
	}
	return lines
}

// askErrForLog is a failure of Ask as the log keeps it. What claude says
// when it fails is left out, since it may hold its answer or the question.
func askErrForLog(err error) string {
	if strings.HasPrefix(err.Error(), "claude: ") {
		return "claude failed (what it said is left out: it may hold the question or its answer)"
	}
	return err.Error()
}

// askUIState is the state of the screen while Ask shows, by its stage.
func (m Model) askUIState() uiState {
	if m.ask.stage == askTyping && m.askHist.search.open {
		return uiAskSearch
	}
	return map[askStage]uiState{askTyping: uiAskTyping, askRunning: uiAskRunning, askAnswered: uiAskAnswered, askFailed: uiAskFailed}[m.ask.stage]
}

// askTypingSpec declares Ask while the question is typed.
func askTypingSpec() modalSpec {
	return modalSpec{
		field: func(m *Model) *textinput.Model { return &m.ask.input },
		history: &modalHistory{
			older: func(m *Model) { m.recallOlder() },
			newer: func(m *Model) { m.recallNewer() },
			leave: func(m *Model) { m.askHist.recall.Leave() },
			any:   func(m Model) bool { return len(m.askHist.list) > 0 },
		},
		own: []modalKey{
			{keys: []string{"enter"}, word: "ask", edge: true, do: func(m *Model) tea.Cmd { return m.startAsk() }},
			{keys: []string{"ctrl+r"}, word: "search", edge: true,
				when: func(m Model) bool { return len(m.askHist.list) > 0 },
				do:   func(m *Model) tea.Cmd { return m.openAskSearch() }},
			{keys: []string{"tab"}, word: "complete",
				when: func(m Model) bool { return m.askGhost() != "" },
				do:   func(m *Model) tea.Cmd { m.completeAsk(); return nil }},
			// → at the end of the text fills it in too; elsewhere it moves.
			{keys: []string{"right"},
				when: func(m Model) bool { return m.askGhost() != "" },
				do:   func(m *Model) tea.Cmd { m.completeAsk(); return nil }},
		},
		esc: closeKey(func(m *Model) { m.closeAsk() }),
		box: askBox(),
	}
}

// askRunningSpec declares Ask while Claude searches: esc stops it.
func askRunningSpec() modalSpec {
	return modalSpec{esc: modalKey{keys: []string{"esc"}, word: "cancel", edge: true, do: func(m *Model) tea.Cmd { m.closeAsk(); return nil }}, box: askBox()}
}

// askAnsweredSpec declares Ask with its answer: the sessions found, to open or narrow the list to.
func askAnsweredSpec() modalSpec {
	return modalSpec{
		list: &modalList{
			len: func(m Model) int { return len(m.ask.hits) },
			at:  func(m Model) int { return m.ask.sel },
			set: func(m *Model, i int) { m.ask.sel = i },
			choose: modalKey{keys: []string{"enter"}, word: "open", do: func(m *Model) tea.Cmd {
				if len(m.ask.hits) > 0 {
					m.jumpTo(m.ask.hits[m.ask.sel].id)
				}
				m.closeAsk()
				return nil
			}},
		},
		own: []modalKey{
			{keys: []string{"f"}, word: "filter the list to these", do: func(m *Model) tea.Cmd {
				if len(m.ask.hits) > 0 {
					m.showAsked()
				}
				m.closeAsk()
				return nil
			}},
			{keys: []string{"r"}, word: "ask again", do: func(m *Model) tea.Cmd { return m.openAsk() }},
		},
		esc: closeKey(func(m *Model) { m.closeAsk() }),
		box: askBox(),
	}
}

// askFailedSpec declares Ask after a failure: r or enter asks again.
func askFailedSpec() modalSpec {
	return modalSpec{
		own: []modalKey{{keys: []string{"r", "enter"}, hint: "r", word: "ask again", edge: true, do: func(m *Model) tea.Cmd { return m.openAsk() }}},
		esc: closeKey(func(m *Model) { m.closeAsk() }),
		box: askBox(),
	}
}
