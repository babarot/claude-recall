package tui

import (
	"fmt"
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
	// The groups are all there, from its top and scrolled to its bottom.
	s := screen(m) + screen(press(t, m, "G"))
	for _, want := range []string{"Keys", "Sessions · works here", "Folder list", "Detail frames", "Filter · works here", "Reading (space)", "resume the session", "folder:bdot"} {
		if !strings.Contains(s, want) {
			t.Errorf("key list lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "SESSIONS") || strings.Contains(s, "  here ") {
		t.Errorf("the old upper-case headers or the here badge are still drawn:\n%s", s)
	}
	// Other keys do nothing while it shows, the key that opened it and q
	// among them; esc alone closes it.
	cursor := m.cursor
	if m = press(t, m, "j", "s", "?", "q"); m.cursor != cursor || m.modal != modalHelp || m.Result != nil {
		t.Fatal("keys should not reach the list behind the key list, nor close it")
	}
	if m = press(t, m, "esc"); m.modal == modalHelp {
		t.Error("esc should close the key list")
	}
	// In the filter, ? is a character to type.
	m = press(t, m, "/", "?")
	if (m.modal == modalHelp) || m.filter.Value() != "?" {
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
	for _, want := range []string{"Reading (space) · works here", "search the conversation", "put the pane back", "Sessions · bright rows work here"} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in the key list:\n%s", want, got)
		}
	}
}

// TestNarrowModalsSayHowOut checks that on a terminal too narrow to draw
// the key list or the sort menu, the footer still says how to close it.
func TestNarrowModalsSayHowOut(t *testing.T) {
	for k, want := range map[string]string{"?": "esc close", "s": "esc close"} {
		m, _ := newTestModel(t, config.Default().TUI, 25, 20)
		m = press(t, m, k)
		if got := ansi.Strip(m.render()); !strings.Contains(got, want) {
			t.Errorf("%s: no %q on the screen:\n%s", k, want, got)
		}
	}
}

// The key list tells the version and the homepage at its top, and a newer
// release with how to update to it.
func TestKeyListTellsTheVersion(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 140, 40)
	m.version = "1.16.0"
	got := screen(press(t, m, "?"))
	if !strings.Contains(got, "recall 1.16.0 · github.com/babarot/claude-recall") || strings.Contains(got, "is out") {
		t.Errorf("no version, or a release that is not known:\n%s", got)
	}
	m = m.TellOfRelease(Release{Version: "1.17.0", How: "recall update", Command: true})
	if got := screen(press(t, m, "?")); !strings.Contains(got, "1.17.0 is out · recall update") {
		t.Errorf("no newer release:\n%s", got)
	}
}

// On a terminal too short for it, the key list scrolls, telling where it
// is on its bottom edge, and opens at its top again.
func TestKeyListScrolls(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 140, 24)
	m = press(t, m, "?")
	n := len(m.helpBody(m.helpInner()))
	rows := m.boxRows()
	if n <= rows {
		t.Fatalf("the key list fits in %d rows (%d lines); the test needs it not to", rows, n)
	}
	if got := screen(m); !strings.Contains(got, fmt.Sprintf(" 1-%d/%d ", rows, n)) || strings.Contains(got, "taller terminal") {
		t.Fatalf("no position, or the old cut line:\n%s", got)
	}
	if m = press(t, m, "j", "j"); m.helpOffset != 2 {
		t.Errorf("j j: offset %d", m.helpOffset)
	}
	if m = press(t, m, "G"); m.helpOffset != n-rows || !strings.Contains(screen(m), "put the pane back") {
		t.Errorf("G: offset %d, want %d, or the last group not shown:\n%s", m.helpOffset, n-rows, screen(m))
	}
	m = update(t, m, tea.MouseWheelMsg{X: 1, Y: 1, Button: tea.MouseWheelUp})
	if m.helpOffset != n-rows-3 {
		t.Errorf("the wheel: offset %d, want %d", m.helpOffset, n-rows-3)
	}
	// Grown so the list fits, the first k moves from where it shows.
	tall := update(t, m, tea.WindowSizeMsg{Width: 140, Height: 80})
	if tall = press(t, tall, "G", "k"); tall.helpOffset != 0 {
		t.Errorf("k on a terminal tall enough: offset %d", tall.helpOffset)
	}
	grown := update(t, press(t, m, "G"), tea.WindowSizeMsg{Width: 140, Height: 34})
	most := len(grown.helpBody(grown.helpInner())) - grown.boxRows()
	if grown = press(t, grown, "k"); grown.helpOffset != most-1 {
		t.Errorf("k after growing: offset %d, want %d", grown.helpOffset, most-1)
	}
	if m = press(t, m, "esc", "?"); m.helpOffset != 0 {
		t.Errorf("opened again at offset %d", m.helpOffset)
	}
}
