package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/babarot/claude-recall/internal/config"
)

func TestQuestionMarkShowsTheKeys(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 120, 50)
	if !strings.Contains(screen(m), "? keys") {
		t.Fatal("the help line should mention ?")
	}
	m = press(t, m, "?")
	s := screen(m)
	for _, want := range []string{"Keys", "SESSIONS", "FOLDER LIST", "DETAIL FRAMES", "FILTER", "READING (SPACE)", "resume the session", "folder:bdot"} {
		if !strings.Contains(s, want) {
			t.Errorf("key list lacks %q:\n%s", want, s)
		}
	}
	// Other keys do nothing while it shows; ?, esc and q close it.
	cursor := m.cursor
	if m = press(t, m, "j", "s"); m.cursor != cursor || !m.helpOpen {
		t.Fatal("keys should not reach the list behind the key list")
	}
	for _, k := range []string{"?", "esc", "q"} {
		if !m.helpOpen {
			m = press(t, m, "?")
		}
		if m = press(t, m, k); m.helpOpen {
			t.Errorf("%s should close the key list", k)
		}
	}
	if m.Result != nil {
		t.Fatal("q closed the list, it should not quit")
	}
	// In the filter, ? is a character to type.
	m = press(t, m, "/", "?")
	if m.helpOpen || m.filter.Value() != "?" {
		t.Fatalf("in the filter ? is typed: %q", m.filter.Value())
	}
}

func TestKeyListFitsSmallTerminals(t *testing.T) {
	for _, size := range [][2]int{{60, 20}, {80, 24}, {200, 60}} {
		m, _ := newTestModel(t, config.Default().TUI, size[0], size[1])
		m = update(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})
		lines := strings.Split(m.render(), "\n")
		if len(lines) > size[1] {
			t.Errorf("%v: %d lines", size, len(lines))
		}
		for i, l := range lines {
			if w := ansi.StringWidth(l); w > size[0] {
				t.Errorf("%v: line %d is %d wide", size, i, w)
			}
		}
	}
}

// TestHelpRowsWork checks that a key the key list shows as working in a
// pane is one the pane takes: each operation a row names is reachable
// there.
func TestHelpRowsWork(t *testing.T) {
	for _, g := range helpGroups {
		for _, r := range g.rows {
			for _, c := range everywhere {
				if !r.worksIn(g, c) {
					continue
				}
				ops := opsIn(c)
				for _, ref := range keyRef.FindAllStringSubmatch(r.keys, -1) {
					if !slices.Contains(ops, ref[1]) {
						t.Errorf("%s: %q is shown as working in context %d, where %s does not reach", g.title, r.keys, c, ref[1])
					}
				}
			}
		}
	}
}

// TestHelpReadingFirst checks that the key list opened while reading, on a
// terminal too short for every key, still shows the reading keys.
func TestHelpReadingFirst(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 140, 40)
	m = press(t, m, "space", "?")
	got := ansi.Strip(m.render())
	for _, want := range []string{"READING (SPACE)", "search the conversation", "put the pane back", "bright rows work here too"} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in the key list:\n%s", want, got)
		}
	}
}

// TestNarrowModalsSayHowOut checks that on a terminal too narrow to draw
// the key list or the sort menu, the footer still says how to close it.
func TestNarrowModalsSayHowOut(t *testing.T) {
	for k, want := range map[string]string{"?": "esc q close", "s": "esc close"} {
		m, _ := newTestModel(t, config.Default().TUI, 25, 20)
		m = press(t, m, k)
		if got := ansi.Strip(m.render()); !strings.Contains(got, want) {
			t.Errorf("%s: no %q on the screen:\n%s", k, want, got)
		}
	}
}
