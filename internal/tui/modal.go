package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// A modal is a box over the screen that takes every key while it shows:
// the key list, the sort menu, Ask, the box before recalling a session and
// what's new. Each declares the parts it holds, and one layer reads their
// keys, the same in every modal, so the keys, the hints on the box's edge,
// the footer and the conflict check all come from the declaration. See
// docs/design/keybindings.md and ADR-006.

// modalSpec declares a modal in the state it is in. It holds no state of
// its own: what it reads and changes is the Model's, through its funcs, so
// one is built again for every key and every draw.
type modalSpec struct {
	list *modalList
	body *modalBody
	// field is the field typed in, while the modal has one.
	field func(m *Model) *textinput.Model
	// own are the modal's own keys, read after typed characters and before
	// its parts.
	own []modalKey
	// recall goes through what was typed in the field before, on nav's up
	// and down, which it claims from the field.
	recall *modalRecall
	// edited is called when the field's text changes.
	edited func(m *Model)
	// esc is what esc does, the only key that closes a modal.
	esc modalKey
}

// modalList is a list to choose from, moved by nav's keys.
type modalList struct {
	len  func(m Model) int
	at   func(m Model) int
	set  func(m *Model, i int)
	wrap bool // moving past an end goes round to the other
	// choose is what the choose keys do with the item at the cursor.
	choose modalKey
}

// modalRecall goes through the texts typed in a field before.
type modalRecall struct {
	older, newer func(m *Model)
	// leave is called when the field is edited: a text recalled and
	// changed is the user's.
	leave func(m *Model)
	// any reports whether there is anything to recall, which the hints say
	// only then.
	any func(m Model) bool
}

// modalBody is text to read, scrolled by nav's keys.
type modalBody struct {
	scroll func(m *Model, step int)
	page   func(m Model) int
}

// modalKey is a key a modal reads as it is written: esc, a list's choose
// keys and a modal's own keys, which the config file does not change.
type modalKey struct {
	keys []string
	// hint is how the keys show in a hint, when not as written; word is
	// what they do there.
	hint, word string
	// frame puts the key on the box's top edge as well as the footer.
	frame bool
	// when, if set, says whether the key does anything now; its hint
	// shows only then.
	when func(m Model) bool
	do   func(m *Model) tea.Cmd
}

func closeKey(do func(m *Model)) modalKey {
	return modalKey{keys: []string{"esc"}, word: "close", frame: true, do: func(m *Model) tea.Cmd { do(m); return nil }}
}

// openModal declares the modal that shows, if one does.
func (m Model) openModal() (modalSpec, bool) {
	s, ok := modalSpecs[m.uiState()]
	return s, ok
}

// modalSpecs are the modals by the state of the screen they show in. A
// state over which the panes do not take keys and that is not a field of
// the screen is one of them; a test checks it.
var modalSpecs = map[uiState]modalSpec{
	uiHelp:        {esc: closeKey(func(m *Model) { m.helpOpen = false })},
	uiSort:        sortSpec,
	uiAskTyping:   askTypingSpec,
	uiAskSearch:   askSearchSpec,
	uiAskRunning:  {esc: modalKey{keys: []string{"esc"}, word: "cancel", frame: true, do: func(m *Model) tea.Cmd { m.closeAsk(); return nil }}},
	uiAskAnswered: askAnsweredSpec,
	uiAskFailed: {
		own: []modalKey{{keys: []string{"r", "enter"}, hint: "r", word: "ask again", frame: true, do: func(m *Model) tea.Cmd { return m.openAsk() }}},
		esc: closeKey(func(m *Model) { m.closeAsk() }),
	},
	uiRecall:   recallSpec,
	uiWhatsNew: whatsNewSpec,
}

// modalKeyFor gives msg to the modal s, in the order of its parts: esc,
// a character typed in its field, its own keys, its list or body on nav's
// keys, then what is left to the field. What none of them takes does
// nothing; the keys of the panes, quit among them, are not read.
func (m Model) modalKeyFor(s modalSpec, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "esc" {
		return m, s.esc.do(&m)
	}
	if s.field != nil && msg.Text != "" {
		return m.typeInModal(s, msg)
	}
	for _, o := range s.own {
		if slices.Contains(o.keys, k) {
			return m, o.do(&m)
		}
	}
	nav := m.km.Nav
	if r := s.recall; r != nil {
		switch {
		case key.Matches(msg, nav.Up):
			r.older(&m)
			return m, nil
		case key.Matches(msg, nav.Down):
			r.newer(&m)
			return m, nil
		}
	}
	if l := s.list; l != nil {
		n := l.len(m)
		step, taken := 0, true
		// Beside a field, top and bottom are left to it, whose home and
		// end move to the start and the end of the text.
		switch {
		case slices.Contains(l.choose.keys, k):
			return m, l.choose.do(&m)
		case key.Matches(msg, nav.Down):
			step = 1
		case key.Matches(msg, nav.Up):
			step = -1
		case key.Matches(msg, nav.PageDown):
			step = n
		case key.Matches(msg, nav.PageUp):
			step = -n
		case s.field == nil && key.Matches(msg, nav.Top):
			l.set(&m, 0)
		case s.field == nil && key.Matches(msg, nav.Bottom):
			l.set(&m, max(0, n-1))
		default:
			taken = false
		}
		if step != 0 && n > 0 {
			i := l.at(m) + step
			if l.wrap && (step == 1 || step == -1) {
				i = (i + n) % n
			}
			l.set(&m, max(0, min(i, n-1)))
		}
		if taken || s.field == nil {
			return m, nil
		}
	}
	if b := s.body; b != nil {
		page := b.page(m)
		switch {
		case key.Matches(msg, nav.Down):
			b.scroll(&m, 1)
		case key.Matches(msg, nav.Up):
			b.scroll(&m, -1)
		case key.Matches(msg, nav.PageDown):
			b.scroll(&m, page)
		case key.Matches(msg, nav.PageUp):
			b.scroll(&m, -page)
		case key.Matches(msg, nav.Top):
			b.scroll(&m, -1<<30)
		case key.Matches(msg, nav.Bottom):
			b.scroll(&m, 1<<30)
		}
		return m, nil
	}
	if s.field != nil {
		return m.typeInModal(s, msg)
	}
	return m, nil
}

func (m Model) typeInModal(s modalSpec, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	in := s.field(&m)
	before := in.Value()
	var cmd tea.Cmd
	*in, cmd = in.Update(msg)
	if in.Value() != before {
		if r := s.recall; r != nil {
			r.leave(&m)
		}
		if s.edited != nil {
			s.edited(&m)
		}
	}
	return m, cmd
}

// hints are the modal's keys as footer pairs (key hint templates and what
// they do, see hintKeys), frame alone for those on the box's top edge.
func (s modalSpec) hints(m Model, frame bool) [][2]string {
	var out [][2]string
	// The way out comes first in the footer, so a terminal too narrow for
	// the box, or for every hint, still says it; on the box's edge it comes
	// last, after what the box is for.
	if !frame {
		out = append(out, [2]string{"esc", s.esc.word})
	}
	add := func(k modalKey) {
		if k.word == "" || frame && !k.frame || k.when != nil && !k.when(m) {
			return
		}
		h := k.hint
		if h == "" {
			h = strings.Join(k.keys, " ")
		}
		out = append(out, [2]string{h, k.word})
	}
	if l := s.list; l != nil {
		if !frame {
			out = append(out, [2]string{"{up.0}{down.0}", "pick"})
		}
		add(l.choose)
	}
	if s.body != nil && !frame {
		out = append(out, [2]string{"{up.0}{down.0}", "scroll"}, [2]string{"{page_down.0} {page_up.0}", "page"})
	}
	for _, o := range s.own {
		add(o)
	}
	// The questions asked before show on the box's edge too, where the eye
	// goes, before the way out.
	if r := s.recall; r != nil && r.any(m) {
		word := "earlier questions"
		if frame {
			word = "earlier"
		}
		out = append(out, [2]string{"{up.0}{down.0}", word})
	}
	if frame {
		add(s.esc)
	}
	return out
}

// frameHint is the hint on the top edge of the modal's box.
func (m Model) frameHint(s modalSpec) string {
	var parts []string
	for _, p := range s.hints(m, true) {
		if k := m.hintKeys(p[0]); k != "" {
			parts = append(parts, k+" "+p[1])
		}
	}
	return strings.Join(parts, " · ")
}

// fixedKeys are the keys s reads as written, which no operation from the
// config file may share with it.
func (s modalSpec) fixedKeys() []string {
	var out []string
	if s.list != nil {
		out = append(out, s.list.choose.keys...)
	}
	for _, o := range s.own {
		out = append(out, o.keys...)
	}
	return append(out, s.esc.keys...)
}

// navOps are the operations of nav s reads: every move for a list or a
// body, up and down to recall in a field.
func (s modalSpec) navOps() []string {
	switch {
	case s.list != nil && s.field != nil:
		return []string{"up", "down", "page_up", "page_down"}
	case s.list != nil || s.body != nil:
		return modalNavOps
	case s.recall != nil:
		return []string{"up", "down"}
	}
	return nil
}

var modalNavOps = []string{"up", "down", "page_up", "page_down", "top", "bottom"}

var sortSpec = modalSpec{
	list: &modalList{
		len:  func(Model) int { return len(sorts) },
		at:   func(m Model) int { return m.sortSel },
		set:  func(m *Model, i int) { m.sortSel = i },
		wrap: true,
		choose: modalKey{keys: []string{"enter", "space"}, hint: "enter", word: "apply", frame: true,
			do: func(m *Model) tea.Cmd { m.applySort(m.sortSel); return nil }},
	},
	own: sortNumberKeys(),
	esc: closeKey(func(m *Model) { m.sortMenu = false }),
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

var askTypingSpec = modalSpec{
	field: func(m *Model) *textinput.Model { return &m.ask.input },
	recall: &modalRecall{
		older: func(m *Model) { m.recallOlder() },
		newer: func(m *Model) { m.recallNewer() },
		leave: func(m *Model) { m.askHist.recall.Leave() },
		any:   func(m Model) bool { return len(m.askHist.list) > 0 },
	},
	own: []modalKey{
		{keys: []string{"enter"}, word: "ask", frame: true, do: func(m *Model) tea.Cmd { return m.startAsk() }},
		{keys: []string{"ctrl+r"}, word: "search", frame: true,
			when: func(m Model) bool { return len(m.askHist.list) > 0 },
			do:   func(m *Model) tea.Cmd { return m.openAskSearch() }},
	},
	esc: closeKey(func(m *Model) { m.closeAsk() }),
}

var askSearchSpec = modalSpec{
	field: func(m *Model) *textinput.Model { return &m.askHist.search.input },
	list: &modalList{
		len: func(m Model) int { return len(m.askSearchHits()) },
		at:  func(m Model) int { return m.askHist.search.at },
		set: func(m *Model, i int) { m.askHist.search.at = i },
		choose: modalKey{keys: []string{"enter"}, word: "use", frame: true,
			do: func(m *Model) tea.Cmd { return m.useAskSearch() }},
	},
	edited: func(m *Model) { m.askHist.search.at = 0 },
	esc: modalKey{keys: []string{"esc"}, word: "back", frame: true,
		do: func(m *Model) tea.Cmd { return m.closeAskSearch() }},
}

var askAnsweredSpec = modalSpec{
	list: &modalList{
		len: func(m Model) int { return len(m.ask.hits) },
		at:  func(m Model) int { return m.ask.sel },
		set: func(m *Model, i int) { m.ask.sel = i },
		choose: modalKey{keys: []string{"enter"}, word: "open", do: func(m *Model) tea.Cmd {
			if len(m.ask.hits) > 0 {
				m.jumpTo(m.ask.hits[m.ask.sel].id)
			}
			m.closeAsk()
			return nil
		}},
	},
	own: []modalKey{
		{keys: []string{"f"}, word: "filter the list to these", do: func(m *Model) tea.Cmd {
			if len(m.ask.hits) > 0 {
				m.showAsked()
			}
			m.closeAsk()
			return nil
		}},
		{keys: []string{"r"}, word: "ask again", do: func(m *Model) tea.Cmd { return m.openAsk() }},
	},
	esc: closeKey(func(m *Model) { m.closeAsk() }),
}

var recallSpec = modalSpec{
	field: func(m *Model) *textinput.Model { return &m.recall.input },
	own: []modalKey{{keys: []string{"enter"}, word: "start claude", frame: true, do: func(m *Model) tea.Cmd {
		m.Recall = &Recall{SessionID: m.recall.id, Topic: strings.TrimSpace(m.recall.input.Value())}
		m.closeRecall()
		return m.imagesQuit()
	}}},
	esc: closeKey(func(m *Model) { m.closeRecall() }),
}

var whatsNewSpec = modalSpec{
	body: &modalBody{
		scroll: func(m *Model, step int) { m.scrollWhatsNew(step) },
		page:   func(m Model) int { return max(1, m.whatsNewRows()-1) },
	},
	esc: closeKey(func(m *Model) { m.whatsNew.open = false }),
}
