package tui

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// A modal opens over the screen, drawn as a box, and takes every key while
// it shows: the key list, the sort menu, Ask, the one before recalling a session and
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
	// history goes through what was typed in the field before, on nav's up
	// and down, which it claims from the field.
	history *modalHistory
	// edited is called when the field's text changes.
	edited func(m *Model)
	// esc is what esc does, the only key that closes a modal.
	esc modalKey
	// box is how the modal is drawn and clicked.
	box *modalBox
}

// modalBox is how a modal is drawn as a box and what a click does. The
// boxes differ in a few ways the user sees (a selected row, what a click
// does); those are values here, drawn by one drawBox. Every box is placed
// by one rule (placeModal), and framed in the accent color.
type modalBox struct {
	title func(m Model) string
	// width is the widest the box gets, and minWidth the narrowest it is
	// drawn at; it leaves boxMargin of the terminal's width.
	width, minWidth int
	click           boxClick
	// lines are the box's rows, inner cells wide, at most rows of them.
	lines func(m Model, inner, rows int) []boxLine
	// edgeText, if set and not empty, is on the top edge in place of the
	// hint.
	edgeText func(m Model) string
	// bottom, if set, is on the bottom edge, at its right.
	bottom func(m Model, inner, rows int) string
}

// boxClick is what a click does while the box shows.
type boxClick int

const (
	clickIgnored boxClick = iota
	clickCloses           // anywhere
	clickList             // chooses the row of the list clicked, closes outside
)

// boxLine is a row of a box: its text, and whether the rest of the row
// has the selection's background.
type boxLine struct {
	text     string
	selected bool
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

// move moves the cursor step items, going round past an end by one item
// when wrap is set, else stopping there.
func (l *modalList) move(m *Model, step int, wrap bool) {
	n := l.len(*m)
	if n == 0 {
		return
	}
	i := l.at(*m) + step
	if wrap && (step == 1 || step == -1) {
		i = (i + n) % n
	}
	l.set(m, max(0, min(i, n-1)))
}

// modalHistory goes through the texts typed in a field before.
type modalHistory struct {
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
	// edge puts the key on the box's top edge as well as the footer.
	edge bool
	// when, if set, says whether the key does anything now: only then is
	// it taken, and its hint shown.
	when func(m Model) bool
	do   func(m *Model) tea.Cmd
}

func closeKey(do func(m *Model)) modalKey {
	return modalKey{keys: []string{"esc"}, word: "close", edge: true, do: func(m *Model) tea.Cmd { do(m); return nil }}
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
		// A key that does nothing now is left to the parts after it.
		if slices.Contains(o.keys, k) && (o.when == nil || o.when(m)) {
			return m, o.do(&m)
		}
	}
	nav := m.km.Nav
	if r := s.history; r != nil {
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
		if step != 0 {
			l.move(&m, step, l.wrap)
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
		if r := s.history; r != nil {
			r.leave(&m)
		}
		if s.edited != nil {
			s.edited(&m)
		}
	}
	return m, cmd
}

// hints are the modal's keys as footer pairs (key hint templates and what
// they do, see hintKeys), edge alone for those on the box's top edge.
func (s modalSpec) hints(m Model, edge bool) [][2]string {
	var out [][2]string
	// The way out comes first in the footer, so a terminal too narrow for
	// the box, or for every hint, still says it; on the box's edge it comes
	// last, after what the modal is for.
	if !edge {
		out = append(out, [2]string{"esc", s.esc.word})
	}
	add := func(k modalKey) {
		if k.word == "" || edge && !k.edge || k.when != nil && !k.when(m) {
			return
		}
		h := k.hint
		if h == "" {
			h = strings.Join(k.keys, " ")
		}
		out = append(out, [2]string{h, k.word})
	}
	if l := s.list; l != nil {
		if !edge {
			out = append(out, [2]string{"{up.0}{down.0}", "pick"})
		}
		add(l.choose)
	}
	if s.body != nil && !edge {
		out = append(out, [2]string{"{up.0}{down.0}", "scroll"}, [2]string{"{page_down.0} {page_up.0}", "page"})
	}
	for _, o := range s.own {
		add(o)
	}
	// The questions asked before show on the box's edge too, where the eye
	// goes, before the way out.
	if r := s.history; r != nil && r.any(m) {
		word := "earlier questions"
		if edge {
			word = "earlier"
		}
		out = append(out, [2]string{"{up.0}{down.0}", word})
	}
	if edge {
		add(s.esc)
	}
	return out
}

// edgeHint is the hint on the top edge of the modal's box.
func (m Model) edgeHint(s modalSpec) string {
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
	case s.history != nil:
		return []string{"up", "down"}
	}
	return nil
}

var modalNavOps = []string{"up", "down", "page_up", "page_down", "top", "bottom"}

// modalTopMost is the highest row a box is drawn from: under the header
// and its rule.
const modalTopMost = 2

// modalRow is the row the modal's box is drawn from: the one placeModal
// decided, or the highest before it has.
func (m Model) modalRow() int { return cmp.Or(m.modalTop, modalTopMost) }

// boxRows is how many rows a box may hold: from its row down to above the
// status line and the footer, which stay in sight under it, inside its
// frame.
func (m Model) boxRows() int { return max(1, m.height-m.modalRow()-4) }

// placeModal decides the row modals are drawn from when one opens with
// none showing: a quarter down the screen, where the eye rests, or
// centered for a box that would leave less room under it than above it
// there, and never above modalTopMost. The row holds while modals show,
// one taking the place of another or its box growing or shrinking, so
// nothing in it moves; it is decided again after all have closed and on a
// resize (which sets it to 0). It runs after every message, once the
// modal opened has the state it is drawn from.
func (m *Model) placeModal() {
	s, ok := m.openModal()
	if !ok || s.box == nil {
		m.modalTop = 0
		return
	}
	if m.modalTop != 0 {
		return
	}
	h := len(m.boxLines(s.box, m.boxWidth(s.box)-4, m.boxRows())) + 2
	m.modalTop = max(modalTopMost, min(m.height/4, (m.height-h)/2))
}

// boxLines are the box's rows, inner cells wide, at most rows of them: a
// modal whose rows run past them (a long failure of Ask) is cut, its last
// row saying there is more, so the box never covers the status line and
// the footer.
func (m Model) boxLines(b *modalBox, inner, rows int) []boxLine {
	rows = max(1, rows)
	ls := b.lines(m, inner, rows)
	if len(ls) <= rows {
		return ls
	}
	ls = slices.Clone(ls[:rows])
	ls[rows-1] = boxLine{text: m.st.muted.Render(ellipsis)}
	return ls
}

// boxWidth is how wide the box is drawn on this terminal.
func (m Model) boxWidth(b *modalBox) int { return min(b.width, m.width-boxMargin) }

// boxMargin is how much of the terminal's width a box leaves.
const boxMargin = 2

// boxRect is where the modal's box is on the screen, and false when the
// terminal is too narrow for it.
func (m Model) boxRect(s modalSpec) (rect, bool) {
	b := s.box
	if b == nil || m.boxWidth(b) < b.minWidth {
		return rect{}, false
	}
	w := m.boxWidth(b)
	h := len(m.boxLines(b, w-4, m.boxRows())) + 2
	return rect{(m.width - w) / 2, m.modalRow(), w, h}, true
}

// drawBox draws the modal's box w cells wide, holding at most rows rows.
func (m Model) drawBox(s modalSpec, w, rows int) []string {
	b := s.box
	inner := w - 4
	hint := m.edgeHint(s)
	if b.edgeText != nil {
		if t := b.edgeText(m); t != "" {
			hint = t
		}
	}
	// Every frame is in the accent color: over the screen dimmed under it,
	// the border color would be fainter than the text around the box.
	border := m.st.id
	title, hint := " "+b.title(m)+" ", " "+hint+" "
	fill := max(0, w-4-ansi.StringWidth(title)-ansi.StringWidth(hint))
	out := []string{border.Render("╭─") + m.st.key.Render(title) + border.Render(strings.Repeat("─", fill)) + m.st.muted.Render(hint) + border.Render("─╮")}
	for _, l := range m.boxLines(b, inner, rows) {
		text := ansi.Truncate(l.text, inner, ellipsis)
		pad := strings.Repeat(" ", max(0, inner-ansi.StringWidth(text)))
		if l.selected {
			pad = m.st.selected.Render(pad)
		}
		out = append(out, border.Render("│ ")+text+pad+border.Render(" │"))
	}
	if b.bottom != nil {
		label := b.bottom(m, inner, rows)
		return append(out, border.Render("╰"+strings.Repeat("─", max(0, w-3-len(label))))+m.st.muted.Render(label)+border.Render("─╯"))
	}
	return append(out, border.Render("╰"+strings.Repeat("─", w-2)+"╯"))
}

// drawModal lays the modal's box over the screen, dimmed under it, where
// boxRect places it; a terminal too narrow for it leaves it undrawn, and
// the footer says how out.
func (m Model) drawModal(s modalSpec, screen string) string {
	r, ok := m.boxRect(s)
	if !ok {
		return screen
	}
	lines := strings.Split(screen, "\n")
	// The footer, the last line, says the modal's keys and keeps its
	// colors, and so does a toast on the status line above it, there to
	// be read for a moment.
	for i := range len(lines) - 1 {
		if i == len(lines)-2 && m.toast != "" {
			continue
		}
		lines[i] = m.dim(lines[i])
	}
	for i, l := range m.drawBox(s, r.w, m.boxRows()) {
		if j := r.y + i; j < len(lines) {
			lines[j] = overlay(lines[j], l, r.x)
		}
	}
	return strings.Join(lines, "\n")
}

// imageRun is a row of an image as the reading view draws it: Kitty's
// placeholder cells, each with the diacritics that name its row and
// column, in a foreground color that is the image's ID (see placeholders).
// The color's end is left optional, so a row cut short still matches.
var imageRun = regexp.MustCompile(`\x1b\[38;5;\d+m(?:\x{10EEEE}\p{Mn}*)+(?:\x1b\[39m)?`)

// dim draws a line of the screen under a modal in one faint color, so
// only the box keeps its colors. The rows of an image keep theirs: their
// color names the image, and drawn in another the cells would show
// another image or none.
func (m Model) dim(line string) string {
	runs := imageRun.FindAllStringIndex(line, -1)
	if runs == nil {
		return m.st.dim.Render(ansi.Strip(line))
	}
	var b strings.Builder
	at := 0
	for _, r := range runs {
		if t := ansi.Strip(line[at:r[0]]); t != "" {
			b.WriteString(m.st.dim.Render(t))
		}
		b.WriteString(line[r[0]:r[1]])
		at = r[1]
	}
	if t := ansi.Strip(line[at:]); t != "" {
		b.WriteString(m.st.dim.Render(t))
	}
	return b.String()
}

// clickModal is a click at x, y while the modal shows.
func (m Model) clickModal(s modalSpec, x, y int) Model {
	if s.box == nil {
		return m
	}
	switch s.box.click {
	case clickCloses:
		m.closeModal()
	case clickList:
		r, ok := m.boxRect(s)
		i := y - r.y - 1
		// Only the rows drawn are chosen; a box cut short ends with an
		// ellipsis, which is not one of them.
		shown := r.h - 2
		if len(s.box.lines(m, r.w-4, m.boxRows())) > m.boxRows() {
			shown--
		}
		if !ok || !r.contains(x, y) || i < 0 || i >= min(s.list.len(m), shown) {
			m.closeModal()
			return m
		}
		s.list.set(&m, i)
		_ = s.list.choose.do(&m)
	}
	return m
}

// wheelModal is the mouse wheel turned step rows while the modal shows: it
// reaches the modal alone, moving its list (without going round an end, as
// a turned wheel would) or scrolling its body three rows a step, and does
// nothing to a modal with neither.
func (m Model) wheelModal(s modalSpec, step int) Model {
	switch {
	case s.list != nil:
		s.list.move(&m, step, false)
	case s.body != nil:
		s.body.scroll(&m, 3*step)
	}
	return m
}
