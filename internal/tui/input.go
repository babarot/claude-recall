package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// newInput makes a field that leaves its cursor to the terminal. An input
// method shows the text being composed where the terminal's cursor is, so a
// field that drew a cursor of its own would have it shown away from the
// field. Every field is made here; a test fails on one made elsewhere.
func newInput() textinput.Model {
	in := textinput.New()
	in.SetVirtualCursor(false)
	return in
}

// cursorMark is drawn where the terminal's cursor goes, and taken out of
// the screen by View, which puts the cursor there. It is an APC sequence,
// which takes no cells, so every step that cuts, pads or lays the screen
// over another measures the line as it will be shown, and the mark moves
// with the field wherever it is drawn: a box over the screen, the
// sidebar, a pane.
const cursorMark = "\x1b_recall-cursor\x1b\\"

// inputView draws in, marking its cursor when in is the field of state,
// the one the keys go to. The cursor is read here, as it is drawn, not
// kept from earlier: a textinput is a value, and a copy taken before a
// resize would put the cursor where the field was.
func (m Model) inputView(state uiState, in textinput.Model) string {
	v := in.View()
	if m.uiState() != state || !in.Focused() {
		return v
	}
	x := cursorX(in, v)
	return ansi.Truncate(v, x, "") + cursorMark + ansi.TruncateLeft(v, x, "")
}

// cursorX is the cell in v, in's view, that in's cursor stands on. It is
// counted in cells, not taken from in.Cursor(), which counts the runes
// before the cursor: after あ it would stand a cell short, in the middle of
// the character.
//
// A text wider than the field is scrolled, and v shows only part of it,
// from a place textinput does not tell. The cursor stands after the
// longest run of text ending at the cursor that v starts with.
func cursorX(in textinput.Model, v string) int {
	val := []rune(in.Value())
	pos := min(in.Position(), len(val))
	pw := ansi.StringWidth(in.Prompt)
	shown := ansi.Strip(ansi.TruncateLeft(v, pw, ""))
	for from := range pos + 1 {
		if before := string(val[from:pos]); strings.HasPrefix(shown, before) {
			return pw + ansi.StringWidth(before)
		}
	}
	return pw
}

// typingInput is the field the keys go to, if one is.
func (m Model) typingInput() (textinput.Model, bool) {
	switch m.uiState() {
	case uiRecall:
		return m.recall.input, true
	case uiAskTyping:
		return m.ask.input, true
	case uiFilter:
		return m.filter, true
	case uiFolderSearch:
		return m.sideSearch, true
	case uiConvSearch:
		return m.conv.input, true
	}
	return textinput.Model{}, false
}

// takeCursor takes cursorMark out of screen and returns where it was, as
// the terminal's cursor in the style of the field being typed in, or nil
// when nothing is typed in or the mark was not drawn.
func (m Model) takeCursor(screen string) (string, *tea.Cursor) {
	if !strings.Contains(screen, cursorMark) {
		return screen, nil
	}
	lines := strings.Split(screen, "\n")
	var cur *tea.Cursor
	for y, l := range lines {
		left, _, ok := strings.Cut(l, cursorMark)
		if !ok {
			continue
		}
		if cur == nil {
			if in, ok := m.typingInput(); ok {
				// in.Cursor() gives the shape and color; the place is
				// where the mark was drawn.
				if c := in.Cursor(); c != nil {
					cur = c
					cur.Position = tea.Position{X: ansi.StringWidth(left), Y: y}
				}
			}
		}
		lines[y] = strings.ReplaceAll(l, cursorMark, "")
	}
	return strings.Join(lines, "\n"), cur
}
