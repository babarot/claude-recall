package tui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/babarot/claude-recall/internal/askhistory"
)

// askStore keeps the questions asked between launches (askhistory.History,
// or a fake in the tests).
type askStore interface {
	Load() ([]askhistory.Entry, error)
	Add(e askhistory.Entry) error
	Reply(id, text string) error
}

// AskHistoryIn makes Ask keep its questions in dir, the state directory,
// and recall them from its field.
func (m Model) AskHistoryIn(dir string) Model {
	m.askStore = askhistory.Open(dir)
	return m
}

// askHistory is the questions asked before and the place among them.
type askHistory struct {
	list   []askhistory.Entry
	recall askhistory.Recall
	// id is the question sent last, and added receives once whether its
	// line was written, which its answer's line waits for.
	id    string
	added chan error
	// warned is set once a failure to read or keep the questions was told.
	warned bool
	search askSearch
}

// askSearch looks through the questions asked before, in place of Ask's
// field, which keeps what was typed in it.
type askSearch struct {
	open  bool
	input textinput.Model
	at    int
}

type askHistoryLoaded struct {
	list []askhistory.Entry
	err  error
}

type askHistoryWritten struct{ err error }

// loadAskHistory reads the questions in the background; the field can be
// typed in meanwhile.
func (m *Model) loadAskHistory() tea.Cmd {
	store := m.askStore
	if store == nil {
		return nil
	}
	return func() tea.Msg {
		list, err := store.Load()
		return askHistoryLoaded{list, err}
	}
}

// keepQuestion keeps q as it is sent, and returns the write.
func (m *Model) keepQuestion(q string) tea.Cmd {
	store := m.askStore
	if store == nil {
		return nil
	}
	e := askhistory.Entry{ID: newAskID(), At: m.now(), Question: q}
	added := make(chan error, 1)
	m.askHist.list = askhistory.Prepend(m.askHist.list, e)
	m.askHist.id, m.askHist.added = e.ID, added
	return func() tea.Msg {
		err := store.Add(e)
		added <- err
		return askHistoryWritten{err}
	}
}

// keepReply keeps what the answer to the question sent last found, once
// the question's own line is written: a reply written first would be read
// as one with no question, and dropped.
func (m *Model) keepReply(hits []askHit) tea.Cmd {
	store, id, added := m.askStore, m.askHist.id, m.askHist.added
	if store == nil || added == nil {
		return nil
	}
	text := askReplyText(hits)
	for i := range m.askHist.list {
		if m.askHist.list[i].ID == id {
			m.askHist.list[i].Reply = text
		}
	}
	m.askHist.added = nil
	return func() tea.Msg {
		if err := <-added; err != nil {
			return nil // told by the question's write
		}
		return askHistoryWritten{store.Reply(id, text)}
	}
}

// askReplyText is what an answer found, as the history keeps it: how many
// sessions and the first one's reason, cut short.
func askReplyText(hits []askHit) string {
	if len(hits) == 0 {
		return "no session"
	}
	s := fmt.Sprintf("%d sessions", len(hits))
	if len(hits) == 1 {
		s = "1 session"
	}
	if why := strings.TrimSpace(strings.SplitN(hits[0].why, "\n", 2)[0]); why != "" {
		if r := []rune(why); len(r) > askReplyRunes {
			why = string(r[:askReplyRunes]) + "…"
		}
		s += " · " + why
	}
	return s
}

// askReplyRunes is how much of a reason the history keeps.
const askReplyRunes = 80

func (m *Model) askHistoryMsg(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case askHistoryLoaded:
		if msg.err != nil {
			return m.warnAskHistory(msg.err)
		}
		// A question sent before the read came back stays first.
		list := msg.list
		for _, e := range m.askHist.list {
			if e.ID == m.askHist.id {
				list = askhistory.Prepend(list, e)
			}
		}
		m.askHist.list = list
	case askHistoryWritten:
		if msg.err != nil {
			return m.warnAskHistory(msg.err)
		}
	}
	return nil
}

func (m *Model) warnAskHistory(err error) tea.Cmd {
	if m.askHist.warned {
		return nil
	}
	m.askHist.warned = true
	return m.showToast(toastWarn, "Ask's questions are not kept: "+err.Error())
}

// recallOlder and recallNewer put the question before or after the one in
// the field there, from the newest.
func (m *Model) recallOlder() {
	if q, ok := m.askHist.recall.Older(m.askHist.list, m.ask.input.Value()); ok {
		m.ask.input.SetValue(q)
		m.ask.input.CursorEnd()
	}
}

func (m *Model) recallNewer() {
	if !m.askHist.recall.Active() {
		return
	}
	m.ask.input.SetValue(m.askHist.recall.Newer(m.askHist.list))
	m.ask.input.CursorEnd()
}

func newAskID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// openAskSearch opens the search of the questions in place of Ask's field,
// when there is a question to find.
func (m *Model) openAskSearch() tea.Cmd {
	if len(m.askHist.list) == 0 {
		return nil
	}
	in := newInput()
	in.Prompt = "ctrl+r › "
	in.Placeholder = "a word of a question asked before"
	in.SetWidth(min(askWidth, m.width-4) - 8 - len(in.Prompt))
	m.askHist.search = askSearch{open: true, input: in}
	m.ask.input.Blur()
	return m.askHist.search.input.Focus()
}

// closeAskSearch goes back to Ask's field as it was.
func (m *Model) closeAskSearch() tea.Cmd {
	m.askHist.search = askSearch{}
	return m.ask.input.Focus()
}

// useAskSearch puts the question chosen in Ask's field, and sends nothing:
// it is asked again about the list as it is now, when enter says so.
func (m *Model) useAskSearch() tea.Cmd {
	hits := m.askSearchHits()
	if len(hits) == 0 {
		return nil
	}
	q := hits[min(m.askHist.search.at, len(hits)-1)].Question
	m.askHist.recall.Leave()
	m.ask.input.SetValue(q)
	m.ask.input.CursorEnd()
	return m.closeAskSearch()
}

// askSearchHits are the questions, the newest first, holding what is typed
// in the search anywhere, ignoring case: a question in Japanese seldom
// starts with the word one remembers.
func (m Model) askSearchHits() []askhistory.Entry {
	want := strings.ToLower(strings.TrimSpace(m.askHist.search.input.Value()))
	if want == "" {
		return m.askHist.list
	}
	var out []askhistory.Entry
	for _, e := range m.askHist.list {
		if strings.Contains(strings.ToLower(e.Question), want) {
			out = append(out, e)
		}
	}
	return out
}

// askSearchRows is how many questions found show at once: with the search
// and the answer of the one chosen, as many rows as Ask's box has.
const askSearchRows = 3

// askSearchBody is the search drawn in place of Ask's box's rows.
func (m Model) askSearchBody(inner int) []string {
	body := []string{m.inputView(uiAskSearch, m.askHist.search.input)}
	hits := m.askSearchHits()
	at := min(m.askHist.search.at, max(0, len(hits)-1))
	from := max(0, at-askSearchRows+1)
	for i := range askSearchRows {
		j := from + i
		switch {
		case len(hits) == 0 && i == 0:
			body = append(body, m.st.muted.Render("  no question matches"))
		case j >= len(hits):
			body = append(body, "")
		default:
			e := hits[j]
			line := "  " + m.askWhen(e.At) + "  " + e.Question
			line = ansi.Truncate(line, inner, ellipsis)
			if j == at {
				line = m.st.selected.Render(line + strings.Repeat(" ", max(0, inner-ansi.StringWidth(line))))
			}
			body = append(body, line)
		}
	}
	reply := ""
	if len(hits) > 0 && hits[at].Reply != "" {
		reply = m.st.muted.Render(ansi.Truncate("  → "+hits[at].Reply, inner, ellipsis))
	}
	return append(body, reply)
}

// askWhen is when a question was asked: the time today, the date before.
func (m Model) askWhen(t time.Time) string {
	if t.IsZero() {
		return "     "
	}
	t, now := t.Local(), m.now().Local()
	if y, mo, d := t.Date(); y == now.Year() && mo == now.Month() && d == now.Day() {
		return t.Format("15:04")
	}
	return t.Format("01-02")
}
