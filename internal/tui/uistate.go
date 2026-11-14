package tui

// uiState is what the screen is in, read from the fields that hold it, in
// the order keys are given out: a modal, then a field being
// typed in, then the focused pane. Keys, the screen, the footer and paste
// all go by it, and the tests run through every value, so a state added
// here is one the footer and the key list cannot miss.
type uiState int

const (
	uiList         uiState = iota // the session list
	uiFrame                       // a detail frame
	uiReading                     // the reading view, focused
	uiReadingList                 // the reading view open, the list focused
	uiFolders                     // the folder list
	uiFolderSearch                // typing the folder search
	uiConvSearch                  // typing the conversation search
	uiFilter                      // typing the filter
	uiHelp                        // the key list
	uiSort                        // the sort menu
	uiAskTyping
	uiAskSearch // searching the questions asked before, in place of Ask's field
	uiAskRunning
	uiAskAnswered
	uiAskFailed
	uiRecall   // the modal before recalling a session
	uiWhatsNew // the release notes
	numUIStates
)

var uiStateNames = [numUIStates]string{
	"list", "frame", "reading", "reading, list focused", "folder list", "folder search", "conversation search", "filter",
	"key list", "sort menu", "ask typing", "ask search", "ask running", "ask answered", "ask failed", "recall", "what's new",
}

func (s uiState) String() string { return uiStateNames[s] }

func (m Model) uiState() uiState {
	switch m.modal {
	case modalRecall:
		return uiRecall
	case modalAsk:
		return m.askUIState()
	case modalSort:
		return uiSort
	case modalWhatsNew:
		return uiWhatsNew
	case modalHelp:
		return uiHelp
	}
	switch {
	case m.mode == modeFilter:
		return uiFilter
	case m.focus == focusFolders && m.sideTyping:
		return uiFolderSearch
	case m.conv.typing:
		return uiConvSearch
	}
	switch m.keyContext() {
	case ctxFolders:
		return uiFolders
	case ctxReading:
		return uiReading
	case ctxFrame:
		return uiFrame
	}
	if m.expanded {
		return uiReadingList
	}
	return uiList
}

// opensHelp reports whether ? opens the key list: in a pane, not over a
// modal or in a field, where ? is a character or nothing.
func (s uiState) opensHelp() bool {
	switch s {
	case uiList, uiFrame, uiReading, uiReadingList, uiFolders:
		return true
	}
	return false
}

// typing reports whether a field is being typed in, the one a key goes to.
func (s uiState) typing() bool {
	switch s {
	case uiRecall, uiAskTyping, uiAskSearch, uiFilter, uiFolderSearch, uiConvSearch:
		return true
	}
	return false
}

// modalKind is the modal that shows, if any. One field holds it, so two
// are never open at once.
type modalKind int

const (
	modalNone modalKind = iota
	modalHelp
	modalSort
	modalAsk
	modalRecall
	modalWhatsNew
)

// openModalKind opens modal k, closing first the one that shows, with what
// closing it takes (a running ask stopped), rather than writing over it.
func (m *Model) openModalKind(k modalKind) {
	m.closeModal()
	m.modal = k
}

// closeModal closes the modal that shows, if any.
func (m *Model) closeModal() {
	switch m.modal {
	case modalAsk:
		m.endAsk()
	case modalRecall:
		m.recall.input.Blur()
	}
	m.modal = modalNone
}
