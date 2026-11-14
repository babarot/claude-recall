package tui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

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
