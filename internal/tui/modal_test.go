package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/claude-recall/internal/config"
)

// Over every modal, q and the keys that open the modals neither close it
// nor quit: esc alone steps out.
func TestModalsCloseOnEscAlone(t *testing.T) {
	for _, c := range footerCases(t) {
		st := c.m.uiState()
		if _, ok := c.m.openModal(); !ok {
			continue
		}
		for _, k := range []string{"q", "?", "s", "w"} {
			m := press(t, c.m, k)
			if m.uiState() != st || m.Result != nil {
				t.Errorf("%s: %s left it for %q (result %v)", c.name, k, m.uiState(), m.Result)
			}
		}
		if m := press(t, c.m, "esc"); m.uiState() == st {
			t.Errorf("%s: esc did not step out", c.name)
		}
	}
}

// A modal's list moves on nav's keys as the config file sets them, and
// not on tab.
func TestModalListsMoveOnNavKeys(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 140, 40)
	m.km.Nav.Down = keys("x")
	m = press(t, m, "s")
	at := m.sortSel
	if m = press(t, m, "x"); m.sortSel != (at+1)%len(sorts) {
		t.Errorf("x did not move the sort menu: %d", m.sortSel)
	}
	at = m.sortSel
	if m = press(t, m, "j", "tab"); m.sortSel != at {
		t.Errorf("j or tab moved the sort menu after down was set to x: %d", m.sortSel)
	}

	f := &fakeAsk{hits: []askHit{{"cccccccc", "talked about the parser"}, {"aaaaaaaa", "the login bug"}}}
	m = askModel(t, f, config.Default().TUI)
	m.km.Nav.Down = keys("x")
	m = ask(t, m, "parser work")
	if m.ask.stage != askAnswered {
		t.Fatalf("not answered: %v", m.ask.stage)
	}
	if m = press(t, m, "x"); m.ask.sel != 1 {
		t.Errorf("x did not move Ask's answer: %d", m.ask.sel)
	}
	if m = press(t, m, "x"); m.ask.sel != 1 {
		t.Errorf("x moved past the last session: %d", m.ask.sel)
	}
}

// In a modal's field, the letters nav moves by are typed, as is any key
// set for an operation.
func TestModalFieldsTypeNavLetters(t *testing.T) {
	m := askModel(t, &fakeAsk{}, config.Default().TUI)
	m.km.Nav.Down = keys("x")
	m = press(t, m, "a", "j", "k", "x", "q")
	if got := m.ask.input.Value(); got != "jkxq" || m.uiState() != uiAskTyping {
		t.Errorf("typed %q in state %q", got, m.uiState())
	}
	m = update(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	m = typeText(t, m, "!")
	if got := m.ask.input.Value(); got != "jkx!q" {
		t.Errorf("left did not reach the field: %q", got)
	}
}
