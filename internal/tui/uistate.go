package tui

// uiState is what the screen is in, read from the fields that hold it, in
// the order keys are given out: a box over the screen, then a field being
// typed in, then the focused pane. Keys, the screen, the footer and paste
// all go by it, and the tests run through every value, so a state added
// here is one the footer and the key list cannot miss.
type uiState int

const (
	uiList         uiState = iota // the session list
	uiFrame                       // a detail frame
	uiReading                     // the spread conversation, focused
	uiReadingList                 // the conversation spread, the list focused
	uiFolders                     // the folder list
	uiFolderSearch                // typing the folder search
	uiConvSearch                  // typing the conversation search
	uiFilter                      // typing the filter
	uiHelp                        // the key list
	uiSort                        // the sort menu
	uiAskTyping
	uiAskRunning
	uiAskAnswered
	uiAskFailed
	uiRecall // the box before recalling a session
	numUIStates
)

var uiStateNames = [numUIStates]string{
	"list", "frame", "reading", "reading, list focused", "folder list", "folder search", "conversation search", "filter",
	"key list", "sort menu", "ask typing", "ask running", "ask answered", "ask failed", "recall",
}

func (s uiState) String() string { return uiStateNames[s] }

func (m Model) uiState() uiState {
	switch {
	case m.recall.open:
		return uiRecall
	case m.ask.stage != askClosed:
		return map[askStage]uiState{askTyping: uiAskTyping, askRunning: uiAskRunning, askAnswered: uiAskAnswered, askFailed: uiAskFailed}[m.ask.stage]
	case m.sortMenu:
		return uiSort
	case m.helpOpen:
		return uiHelp
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
// box or in a field, where ? is a character or nothing.
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
	case uiRecall, uiAskTyping, uiFilter, uiFolderSearch, uiConvSearch:
		return true
	}
	return false
}
