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
// with the field wherever it is drawn: a modal, the
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
	x := cursorX(in)
	return ansi.Truncate(v, x, "") + cursorMark + ansi.TruncateLeft(v, x, "")
}

// cursorX is the cell in in's view that its cursor stands on. It is not
// taken from in.Cursor(), which counts the runes before the cursor, a cell
// short after each wide character such as あ, and leaves out how far a text
// wider than the field is scrolled, which textinput does not tell.
//
// A copy of in is drawn with a character of no width at the cursor, and
// the cursor stands where that character is drawn. textinput scrolls by
// the width of the text, which the character does not change, so the copy
// shows the same part of the text as in. When the cursor stands just past
// the last character shown, as it does while typing at the end of a
// scrolled field, the character is past what the copy shows; the copy's
// cursor is then put after it, which textinput scrolls to from the text
// before the cursor alone, as it did for in.
func cursorX(in textinput.Model) int {
	val := []rune(in.Value())
	pos := min(in.Position(), len(val))
	c := in
	c.SetValue(string(val[:pos]) + noWidth + string(val[pos:]))
	if before, _, ok := strings.Cut(c.View(), noWidth); ok {
		return ansi.StringWidth(before)
	}
	c.SetCursor(pos + 1)
	if before, _, ok := strings.Cut(c.View(), noWidth); ok {
		return ansi.StringWidth(before)
	}
	return ansi.StringWidth(in.Prompt) + ansi.StringWidth(string(val[:pos]))
}

// noWidth is the character cursorX draws at the cursor: a zero width
// space, which takes no cell and is not typed in a search.
const noWidth = "\u200b"

// typingInput is the field the keys go to, if one is.
func (m Model) typingInput() (textinput.Model, bool) {
	switch m.uiState() {
	case uiRecall:
		return m.recall.input, true
	case uiAskTyping:
		return m.ask.input, true
	case uiAskSearch:
		return m.askHist.search.input, true
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
