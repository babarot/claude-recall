package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

// pasteKey is the key that pastes from the clipboard. The terminal's own
// paste (cmd+v) arrives as a tea.PasteMsg instead.
const pasteKey = "ctrl+v"

// paste puts pasted text in the field being typed in, if any.
func (m Model) paste(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	switch m.uiState() {
	case uiRecall:
		var cmd tea.Cmd
		m.recall.input, cmd = m.recall.input.Update(msg)
		return m, cmd
	case uiAskTyping:
		var cmd tea.Cmd
		m.ask.input, cmd = m.ask.input.Update(msg)
		return m, cmd
	case uiFilter:
		return m.typeFilter(msg)
	case uiFolderSearch:
		return m.typeSideSearch(msg)
	case uiConvSearch:
		return m.typeConvSearch(msg)
	}
	return m, nil
}

// readClipboard reads the clipboard into a paste.
func readClipboard() tea.Msg {
	s, err := clipboard.ReadAll()
	if err != nil || s == "" {
		return nil
	}
	return tea.PasteMsg{Content: s}
}
