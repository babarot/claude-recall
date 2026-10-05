package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/babarot/claude-recall/internal/config"
)

// TestUIStatesCovered checks that every state of the screen has its footer
// pinned in the golden file, and the key list every pane it opens from. A
// state or pane added without them fails here, naming it.
func TestUIStatesCovered(t *testing.T) {
	cases := footerCases(t)
	seen := map[uiState]bool{}
	for _, c := range cases {
		seen[c.m.uiState()] = true
	}
	for s := range numUIStates {
		if !seen[s] {
			t.Errorf("no footer case is in state %q: add one to footerCases and run go test ./internal/tui -run TestKeyHints -update", s)
		}
	}

	m, _ := newTestModel(t, config.Default().TUI, 140, 40)
	fm := folderModel(t, config.Default().TUI, newFolderFixture(t), 140, 40)
	ctxs := map[keyContext]bool{}
	for _, c := range helpCases(t, m, fm) {
		ctxs[c.m.keyContext()] = true
	}
	for c := range numKeyContexts {
		if !ctxs[c] {
			t.Errorf("no key list case opens from keyContext %d: add one to helpCases", c)
		}
	}
}

// TestFootersSayWhereTheyAre checks that each state's footer says
// something, and that no state but the list falls back to the list's
// keys, which do not work over a box or in a field.
func TestFootersSayWhereTheyAre(t *testing.T) {
	cases := footerCases(t)
	list := ansi.Strip(cases[0].m.renderHelp())
	for _, c := range cases {
		got := ansi.Strip(c.m.renderHelp())
		switch st := c.m.uiState(); {
		case strings.TrimSpace(got) == "":
			t.Errorf("%s (%s): no footer; give the state a case in renderHelp", c.name, st)
		case st != uiList && got == list:
			t.Errorf("%s (%s): the footer is the session list's", c.name, st)
		}
	}
}

// TestHelpOpensWhereItShould checks that ? opens the key list in the
// states that say so and nowhere else.
func TestHelpOpensWhereItShould(t *testing.T) {
	for _, c := range footerCases(t) {
		st := c.m.uiState()
		if st == uiHelp {
			continue // ? closes it
		}
		if got := press(t, c.m, "?").uiState() == uiHelp; got != st.opensHelp() {
			t.Errorf("%s (%s): ? opens the key list: %v, want %v", c.name, st, got, st.opensHelp())
		}
	}
}

// TestHelpCovers checks that the key list has a group for every pane and
// names every operation, and that every operation is reachable from some
// pane, so the conflict check sees it.
func TestHelpCovers(t *testing.T) {
	for c := range numKeyContexts {
		if !slices.ContainsFunc(helpGroups, func(g helpGroup) bool { return slices.Contains(g.in, c) }) {
			t.Errorf("no key list group works in keyContext %d: give one its in", c)
		}
	}
	named := map[string]bool{
		"help": true, // on the key list's own frame and in the footers
	}
	for _, g := range helpGroups {
		for _, r := range g.rows {
			for _, tmpl := range []string{r.keys, r.what} {
				for _, ref := range keyRef.FindAllStringSubmatch(tmpl, -1) {
					named[ref[1]] = true
				}
			}
		}
	}
	reachable := map[string]bool{}
	for c := range numKeyContexts {
		for _, op := range opsIn(c) {
			reachable[op] = true
		}
	}
	for name := range defaultKeyMap().byName() {
		if !named[name] {
			t.Errorf("the key list does not name %s: add a row to helpGroups", name)
		}
		if !reachable[name] {
			t.Errorf("%s is in no pane's opsIn, so no key reaches it and the conflict check misses it", name)
		}
	}
}
