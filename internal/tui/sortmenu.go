package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// s opens a menu of the sort orders over the screen:
// the arrows, j k or the mouse pick one and Enter applies it, as does its
// number; Esc closes the menu as it was.

// sortNotes say what each order puts first.
var sortNotes = map[string]string{
	"Ended":   "latest activity first",
	"Started": "newest session first",
	"Msgs":    "most messages first",
	"Size":    "largest transcript first",
}

const sortMenuWidth = 44

// openSortMenu shows the menu with the current order highlighted.
func (m *Model) openSortMenu() {
	m.openModalKind(modalSort)
	m.sortSel = m.sortIdx
}

// applySort switches to order i and closes the menu.
func (m *Model) applySort(i int) {
	m.closeModal()
	if i == m.sortIdx {
		return
	}
	m.sortIdx = i
	m.refresh()
	m.cursor, m.offset = 0, 0
}

// sortLines are the sort menu's rows: an order a row, by number, the one
// in use marked, the one highlighted on the selection's background.
func (m Model) sortLines(inner int) []boxLine {
	out := make([]boxLine, len(sorts))
	for i, s := range sorts {
		mark := "  "
		if i == m.sortIdx {
			mark = "● "
		}
		name := fmt.Sprintf("%d %s%-8s", i+1, mark, s.name)
		line := " " + m.st.text.Render(name) + " " + m.st.muted.Render(sortNotes[s.name])
		if i == m.sortSel {
			line = m.st.bar.Render("▎") + m.st.on(m.st.key, true).Render(name) + m.st.selected.Render(" ") + m.st.on(m.st.muted, true).Render(sortNotes[s.name])
		}
		out[i] = boxLine{text: ansi.Truncate(line, inner, ellipsis), selected: i == m.sortSel}
	}
	return out
}

// sortSpec declares the sort menu: a list of the orders, chosen by number too.
func sortSpec() modalSpec {
	return modalSpec{
		list: &modalList{
			len:  func(Model) int { return len(sorts) },
			at:   func(m Model) int { return m.sortSel },
			set:  func(m *Model, i int) { m.sortSel = i },
			wrap: true,
			choose: modalKey{keys: []string{"enter", "space"}, hint: "enter", word: "apply", edge: true,
				do: func(m *Model) tea.Cmd { m.applySort(m.sortSel); return nil }},
		},
		own: sortNumberKeys(),
		esc: closeKey(func(m *Model) { m.closeModal() }),
		box: &modalBox{
			title: func(Model) string { return "Sort by" },
			width: sortMenuWidth, margin: 2, minWidth: 24,
			place: placeCenter, click: clickList,
			lines: func(m Model, inner, _ int) []boxLine { return m.sortLines(inner) },
		},
	}
}

// sortNumberKeys choose an order by its number, 1 for the first.
func sortNumberKeys() []modalKey {
	out := make([]modalKey, len(sorts))
	for i := range sorts {
		out[i] = modalKey{keys: []string{string(rune('1' + i))}, do: func(m *Model) tea.Cmd { m.applySort(i); return nil }}
	}
	// One hint for all of them.
	out[0].hint, out[0].word = "1-"+string(rune('0'+len(sorts))), "apply that one"
	return out
}
