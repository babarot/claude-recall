package tui

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/babarot/claude-recall/internal/askhistory"
	"github.com/babarot/claude-recall/internal/config"
)

// fakeStore keeps the questions in memory and records its writes in order.
type fakeStore struct {
	mu      sync.Mutex
	list    []askhistory.Entry
	writes  []string
	loadErr error
	// addGate, when set, holds every Add until it is closed.
	addGate chan struct{}
}

func (f *fakeStore) Load() ([]askhistory.Entry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]askhistory.Entry(nil), f.list...), f.loadErr
}

func (f *fakeStore) Add(e askhistory.Entry) error {
	if f.addGate != nil {
		<-f.addGate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.list = askhistory.Prepend(f.list, e)
	f.writes = append(f.writes, "add "+e.Question)
	return nil
}

func (f *fakeStore) Reply(id, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.list {
		if f.list[i].ID == id {
			f.list[i].Reply = text
		}
	}
	f.writes = append(f.writes, "reply "+text)
	return nil
}

func (f *fakeStore) written() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.writes...)
}

// drive runs what cmd starts, each command on its own as bubbletea runs
// them, and gives the model the messages of Ask and its history until
// nothing is left running. The spinner's ticks are let go.
func drive(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	msgs := make(chan tea.Msg)
	var running sync.WaitGroup
	start := func(c tea.Cmd) {
		if c == nil {
			return
		}
		running.Add(1)
		go func() { msgs <- c() }()
	}
	start(cmd)
	done := make(chan struct{})
	go func() { running.Wait(); close(done) }()
	for {
		select {
		case msg := <-msgs:
			switch msg := msg.(type) {
			case tea.BatchMsg:
				for _, sub := range msg {
					start(sub)
				}
			case askStepMsg, askDoneMsg, askHistoryLoaded, askHistoryWritten:
				next, more := m.Update(msg)
				m = asModel(t, next)
				start(more)
			}
			running.Done()
		case <-done:
			return m
		}
	}
}

func historyModel(t *testing.T, f *fakeAsk, s *fakeStore) Model {
	t.Helper()
	m := askModel(t, f, config.Default().TUI)
	m.askStore = s
	return m
}

// askDriven opens Ask, types q and sends it, running what that starts.
func askDriven(t *testing.T, m Model, q string) Model {
	t.Helper()
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = drive(t, asModel(t, next), cmd)
	m = typeText(t, m, q)
	next, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return drive(t, asModel(t, next), cmd)
}

func TestAskKeepsAndRecallsQuestions(t *testing.T) {
	s := &fakeStore{list: []askhistory.Entry{{ID: "old", Question: "an older question"}}}
	f := &fakeAsk{hits: []askHit{{"cccccccc", "talked about the parser"}}}
	m := historyModel(t, f, s)
	m = askDriven(t, m, "parser work")
	if m.ask.stage != askAnswered {
		t.Fatalf("not answered: %v", m.ask.stage)
	}
	if got := strings.Join(s.written(), "|"); got != "add parser work|reply 1 session · talked about the parser" {
		t.Fatalf("writes %q", got)
	}

	// Opened again, the box holds the last question; up goes past it to
	// the one before, and down back to it.
	m = press(t, m, "esc")
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = drive(t, asModel(t, next), cmd)
	if v := m.ask.input.Value(); v != "parser work" {
		t.Fatalf("opened on %q", v)
	}
	m = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if v := m.ask.input.Value(); v != "an older question" {
		t.Errorf("up recalled %q", v)
	}
	m = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if v := m.ask.input.Value(); v != "parser work" {
		t.Errorf("down came back to %q", v)
	}
	// k and j are typed, not recalled.
	m = typeText(t, m, "kj")
	if v := m.ask.input.Value(); v != "parser workkj" {
		t.Errorf("k and j typed as %q", v)
	}
	if !strings.Contains(ansi.Strip(m.renderHelp()), "↑↓ earlier questions") {
		t.Errorf("footer %q", ansi.Strip(m.renderHelp()))
	}
	if !strings.Contains(screen(m), "enter ask · ctrl+r search · ↑↓ earlier · esc close") {
		t.Errorf("no ↑↓ earlier on the box's edge:\n%s", screen(m))
	}
	// With no question asked before, neither says it.
	m = historyModel(t, &fakeAsk{}, &fakeStore{})
	next, cmd = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = drive(t, asModel(t, next), cmd)
	if strings.Contains(screen(m), "earlier") || strings.Contains(ansi.Strip(m.renderHelp()), "earlier") {
		t.Errorf("earlier questions told with none:\n%s\n%s", screen(m), ansi.Strip(m.renderHelp()))
	}
}

// A question recalled and changed is the user's: down no longer goes
// through the questions from it.
func TestEditingARecalledQuestion(t *testing.T) {
	s := &fakeStore{list: []askhistory.Entry{{ID: "b", Question: "b"}, {ID: "a", Question: "a"}}}
	m := historyModel(t, &fakeAsk{}, s)
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = drive(t, asModel(t, next), cmd)
	m = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	m = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	m = typeText(t, m, "!")
	if m.askHist.recall.Active() {
		t.Fatal("still recalling after an edit")
	}
	if m = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown}); m.ask.input.Value() != "a!" {
		t.Errorf("down changed the edited question: %q", m.ask.input.Value())
	}
}

// An answer that comes before its question is written is kept after it:
// a reply first would be read as one with no question.
func TestReplyWaitsForItsQuestion(t *testing.T) {
	gate := make(chan struct{})
	s := &fakeStore{addGate: gate}
	m := historyModel(t, &fakeAsk{hits: []askHit{{"cccccccc", "why"}}}, s)
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = drive(t, asModel(t, next), cmd)
	m = typeText(t, m, "q")
	next, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	done := make(chan struct{})
	go func() { drive(t, asModel(t, next), cmd); close(done) }()
	// The answer comes at once, and its reply waits on the question still
	// held: nothing is written meanwhile.
	time.Sleep(100 * time.Millisecond)
	if w := s.written(); len(w) > 0 {
		t.Fatalf("written before the question: %q", w)
	}
	select {
	case <-done:
		t.Fatal("everything ended while the question was held")
	default:
	}
	close(gate)
	<-done
	if got := strings.Join(s.written(), "|"); got != "add q|reply 1 session · why" {
		t.Errorf("writes %q", got)
	}
}

// A history that cannot be read is told once, and Ask goes on.
func TestAskWithoutItsHistory(t *testing.T) {
	s := &fakeStore{loadErr: errors.New("disk full")}
	m := historyModel(t, &fakeAsk{hits: []askHit{{"cccccccc", "why"}}}, s)
	m = askDriven(t, m, "parser work")
	if m.ask.stage != askAnswered {
		t.Fatalf("Ask did not go on: %v", m.ask.stage)
	}
	if !strings.Contains(m.toast, "disk full") {
		t.Errorf("toast %q", m.toast)
	}
}

func ctrlR(t *testing.T, m Model) Model {
	t.Helper()
	return update(t, m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
}

func pressCode(t *testing.T, m Model, code rune) Model {
	t.Helper()
	return update(t, m, tea.KeyPressMsg{Code: code})
}

func TestAskSearch(t *testing.T) {
	f := &fakeAsk{}
	m := askModel(t, f, config.Default().TUI)
	m.askHist.list = []askhistory.Entry{
		{ID: "3", Question: "ログインのバグを直したセッション", Reply: "1 session · fixed the login"},
		{ID: "2", Question: "parser work"},
		{ID: "1", Question: "terraform state"},
	}
	m = typeText(t, press(t, m, "a"), "draft")

	// A word in the middle of a question in Japanese finds it.
	m = typeText(t, ctrlR(t, m), "バグ")
	if m.uiState() != uiAskSearch {
		t.Fatalf("ctrl+r opened %q", m.uiState())
	}
	if hits := m.askSearchHits(); len(hits) != 1 || hits[0].ID != "3" {
		t.Fatalf("hits %+v", hits)
	}
	if s := screen(m); !strings.Contains(s, "→ 1 session · fixed the login") || !strings.Contains(s, "enter use · esc back") {
		t.Errorf("the answer of the one chosen is not shown:\n%s", s)
	}
	// enter puts it in the field and sends nothing.
	m = pressCode(t, m, tea.KeyEnter)
	if m.uiState() != uiAskTyping || m.ask.input.Value() != "ログインのバグを直したセッション" || len(f.asked) != 0 {
		t.Fatalf("after enter: %q in %q, asked %q", m.ask.input.Value(), m.uiState(), f.asked)
	}

	// esc goes back to the field as it was.
	m.ask.input.SetValue("draft")
	m = pressCode(t, typeText(t, ctrlR(t, m), "parser"), tea.KeyEscape)
	if m.uiState() != uiAskTyping || m.ask.input.Value() != "draft" {
		t.Fatalf("after esc: %q in %q", m.ask.input.Value(), m.uiState())
	}

	// j and k are typed; the arrows move.
	m = ctrlR(t, m)
	if m = pressCode(t, m, tea.KeyDown); m.askHist.search.at != 1 {
		t.Errorf("down did not move: %d", m.askHist.search.at)
	}
	if m = typeText(t, m, "jk"); m.askHist.search.input.Value() != "jk" || m.askHist.search.at != 0 {
		t.Errorf("j k: %q at %d", m.askHist.search.input.Value(), m.askHist.search.at)
	}
	if s := screen(m); !strings.Contains(s, "no question matches") {
		t.Errorf("nothing found is not said:\n%s", s)
	}
}

// The search's field edits as any field does: the list beside it leaves
// it backspace, the arrows across and home and end.
func TestAskSearchEdits(t *testing.T) {
	m := askSearchOpen(t)
	m = typeText(t, m, "parser")
	m = pressCode(t, m, tea.KeyBackspace)
	m = pressCode(t, m, tea.KeyLeft)
	m = typeText(t, m, "X")
	m = pressCode(t, m, tea.KeyHome)
	m = typeText(t, m, "<")
	m = pressCode(t, m, tea.KeyEnd)
	m = typeText(t, m, ">")
	if got := m.askHist.search.input.Value(); got != "<parsXe>" {
		t.Errorf("edited to %q", got)
	}
}

// With no question asked before, ctrl+r does nothing and is not told.
func TestAskSearchWithNothingToFind(t *testing.T) {
	m := press(t, askModel(t, &fakeAsk{}, config.Default().TUI), "a")
	if strings.Contains(screen(m), "ctrl+r") || strings.Contains(ansi.Strip(m.renderHelp()), "ctrl+r") {
		t.Errorf("ctrl+r told with nothing to find:\n%s", screen(m))
	}
	if m = ctrlR(t, m); m.uiState() != uiAskTyping {
		t.Errorf("ctrl+r opened %q", m.uiState())
	}
}
