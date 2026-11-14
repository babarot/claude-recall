package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"image/color"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/babarot/claude-recall/internal/config"
)

// Over every modal, q and the keys that open the modals neither close it
// nor quit: esc alone steps out.
func TestModalsCloseOnEscAlone(t *testing.T) {
	for _, c := range footerCases(t) {
		st := c.m.uiState()
		if _, ok := c.m.openModal(); !ok {
			continue
		}
		for _, k := range []string{"q", "?", "s", "w"} {
			m := press(t, c.m, k)
			if m.uiState() != st || m.Result != nil {
				t.Errorf("%s: %s left it for %q (result %v)", c.name, k, m.uiState(), m.Result)
			}
		}
		if m := press(t, c.m, "esc"); m.uiState() == st {
			t.Errorf("%s: esc did not step out", c.name)
		}
	}
}

// A modal's list moves on nav's keys as the config file sets them, and
// not on tab.
func TestModalListsMoveOnNavKeys(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 140, 40)
	m.km.Nav.Down = keys("x")
	m = press(t, m, "s")
	at := m.sortSel
	if m = press(t, m, "x"); m.sortSel != (at+1)%len(sorts) {
		t.Errorf("x did not move the sort menu: %d", m.sortSel)
	}
	at = m.sortSel
	if m = press(t, m, "j", "tab"); m.sortSel != at {
		t.Errorf("j or tab moved the sort menu after down was set to x: %d", m.sortSel)
	}

	f := &fakeAsk{hits: []askHit{{"cccccccc", "talked about the parser"}, {"aaaaaaaa", "the login bug"}}}
	m = askModel(t, f, config.Default().TUI)
	m.km.Nav.Down = keys("x")
	m = ask(t, m, "parser work")
	if m.ask.stage != askAnswered {
		t.Fatalf("not answered: %v", m.ask.stage)
	}
	if m = press(t, m, "x"); m.ask.sel != 1 {
		t.Errorf("x did not move Ask's answer: %d", m.ask.sel)
	}
	if m = press(t, m, "x"); m.ask.sel != 1 {
		t.Errorf("x moved past the last session: %d", m.ask.sel)
	}
}

// In a modal's field, the letters nav moves by are typed, as is any key
// set for an operation.
func TestModalFieldsTypeNavLetters(t *testing.T) {
	m := askModel(t, &fakeAsk{}, config.Default().TUI)
	m.km.Nav.Down = keys("x")
	m = press(t, m, "a", "j", "k", "x", "q")
	if got := m.ask.input.Value(); got != "jkxq" || m.uiState() != uiAskTyping {
		t.Errorf("typed %q in state %q", got, m.uiState())
	}
	m = update(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	m = typeText(t, m, "!")
	if got := m.ask.input.Value(); got != "jkx!q" {
		t.Errorf("left did not reach the field: %q", got)
	}
}

// One field holds the modal that shows, and opening one closes the one
// that shows first, with what closing it takes: a running ask is stopped,
// not left running under another modal.
func TestOpeningAModalClosesTheOther(t *testing.T) {
	f := &fakeAsk{block: make(chan struct{})}
	m := askModel(t, f, config.Default().TUI)
	m = press(t, m, "a", "x")
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = asModel(t, next)
	if m.uiState() != uiAskRunning || m.ask.cancel == nil {
		t.Fatalf("not running: %v", m.uiState())
	}
	seq := m.ask.seq
	m.openSortMenu()
	if m.modal != modalSort || m.uiState() != uiSort {
		t.Fatalf("the sort menu did not take the screen: %v", m.uiState())
	}
	if m.ask.cancel != nil || m.ask.seq == seq {
		t.Error("the ask was left running under the sort menu")
	}
}

// modal.go is the mechanism every modal shares, and knows none of them:
// it reaches a Model only for the keymap, the key hints, the screen's size
// and styles, closing the modal that shows and its own methods, never a
// modal's fields. The test reads names, not types: it
// follows the parameters and receivers declared as Model or *Model, in
// functions, func literals and package variables alike, and would miss a
// Model reached through another name, which review is left to catch. A modal declares itself in its own
// file, and uistate.go says which state is which modal.
func TestModalGoKnowsNoModal(t *testing.T) {
	allowed := map[string]bool{
		// The keymap and the key hints, for the keys a modal reads.
		"km": true, "hintKeys": true,
		// The screen's size and the styles, to draw and place a box.
		"width": true, "height": true, "st": true,
		// Closing whatever modal shows, on a click; which modal shows, and
		// the row every modal is drawn from, to place its box.
		"closeModal": true, "openModal": true, "modalTop": true,
		// A toast on the status line, which the dimmed screen leaves in color.
		"toast": true,
		// modal.go's own methods.
		"modalKeyFor": true, "typeInModal": true, "edgeHint": true,
		"modalRow": true, "placeModal": true, "boxLines": true, "dim": true, "boxRows": true, "boxWidth": true, "boxRect": true, "drawBox": true, "drawModal": true, "clickModal": true, "wheelModal": true,
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "modal.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	isModel := func(e ast.Expr) bool {
		if s, ok := e.(*ast.StarExpr); ok {
			e = s.X
		}
		id, ok := e.(*ast.Ident)
		return ok && id.Name == "Model"
	}
	names := func(fields ...*ast.FieldList) map[string]bool {
		out := map[string]bool{}
		for _, fl := range fields {
			if fl == nil {
				continue
			}
			for _, fd := range fl.List {
				if isModel(fd.Type) {
					for _, n := range fd.Names {
						out[n.Name] = true
					}
				}
			}
		}
		return out
	}
	var check func(models map[string]bool, body ast.Node)
	check = func(models map[string]bool, body ast.Node) {
		ast.Inspect(body, func(n ast.Node) bool {
			// A func literal's own Model params count inside it alone.
			if lit, ok := n.(*ast.FuncLit); ok && n != body {
				inner := names(lit.Type.Params)
				for k := range models {
					inner[k] = true
				}
				check(inner, lit)
				return false
			}
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && models[x.Name] && !allowed[sel.Sel.Name] {
				t.Errorf("%s: modal.go reaches %s.%s, which a modal's own file should hold", fset.Position(sel.Pos()), x.Name, sel.Sel.Name)
			}
			return true
		})
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Body != nil {
				check(names(d.Recv, d.Type.Params), d.Body)
			}
		default:
			// A package variable's funcs too, as the declarations were
			// before they moved out.
			check(map[string]bool{}, d)
		}
	}
}

// While a modal shows, the wheel reaches it alone: its list moves without
// going round an end, its body scrolls, and a modal with neither leaves
// the list under it where it was.
func TestWheelReachesOnlyTheModal(t *testing.T) {
	down := tea.MouseWheelMsg{X: 10, Y: 10, Button: tea.MouseWheelDown}
	up := tea.MouseWheelMsg{X: 10, Y: 10, Button: tea.MouseWheelUp}

	m, _ := newTestModel(t, config.Default().TUI, 140, 40)
	m = press(t, m, "s")
	at, cursor := m.sortSel, m.cursor
	if m = update(t, m, down); m.sortSel != at+1 || m.cursor != cursor {
		t.Errorf("the wheel did not move the sort menu alone: sel %d, cursor %d", m.sortSel, m.cursor)
	}
	m.sortSel = 0
	if m = update(t, m, up); m.sortSel != 0 {
		t.Errorf("the wheel went round the sort menu's top: %d", m.sortSel)
	}

	m, _ = newTestModel(t, config.Default().TUI, 140, 40)
	m = press(t, m, "?")
	cursor = m.cursor
	if m = update(t, m, down); m.cursor != cursor || m.uiState() != uiHelp {
		t.Errorf("the wheel over the key list reached the list: cursor %d, state %v", m.cursor, m.uiState())
	}

	f := &fakeAsk{hits: []askHit{{"cccccccc", "talked about the parser"}, {"aaaaaaaa", "the login bug"}}}
	m = ask(t, askModel(t, f, config.Default().TUI), "parser work")
	if m = update(t, m, down); m.ask.sel != 1 {
		t.Errorf("the wheel did not move Ask's answer: %d", m.ask.sel)
	}
}

// A modal opens a quarter down the screen, or centered when it is too
// tall for that, and never above the third row (placeModal).
func TestModalsOpenWhereTheEyeIs(t *testing.T) {
	for _, h := range []int{12, 16, 24, 34, 46, 60} {
		for _, k := range []string{"?", "s", "c", "a", "w"} {
			m, _ := newTestModel(t, config.Default().TUI, 140, h)
			m = press(t, m, k)
			s, ok := m.openModal()
			if !ok {
				t.Fatalf("%d rows, %s: no modal", h, k)
			}
			tall := len(m.drawBox(s, m.boxWidth(s.box), h-modalTopMost-4))
			want := max(modalTopMost, min(h/4, (h-tall)/2))
			if m.modalTop != want {
				t.Errorf("%d rows, %s (%d tall): row %d, want %d", h, k, tall, m.modalTop, want)
			}
			if r, ok := m.boxRect(s); ok && r.y != want {
				t.Errorf("%d rows, %s: drawn from row %d, want %d", h, k, r.y, want)
			}
			if m = press(t, m, "esc"); m.modalTop != 0 {
				t.Errorf("%d rows, %s: row %d kept after closing", h, k, m.modalTop)
			}
		}
	}
	// The small sort menu sits a quarter down, not in the middle.
	m, _ := newTestModel(t, config.Default().TUI, 140, 40)
	if m = press(t, m, "s"); m.modalTop != 10 {
		t.Errorf("sort menu on 40 rows: row %d, want 10", m.modalTop)
	}
}

// The row holds while the modal shows, its box growing down from it, and
// is decided again on a resize.
func TestModalRowHolds(t *testing.T) {
	f := &fakeAsk{hits: []askHit{{"aaaaaaaa", "the login bug"}, {"bbbbbbbb", "the docs"}, {"cccccccc", "the parser"}}}
	m := askModel(t, f, config.Default().TUI)
	m = press(t, m, "a")
	top := m.modalTop
	s, _ := m.openModal()
	typed, _ := m.boxRect(s)
	m = ask(t, m, "parser")
	s, _ = m.openModal()
	answered, _ := m.boxRect(s)
	if m.uiState() != uiAskAnswered || m.modalTop != top || answered.y != top {
		t.Errorf("answered: state %v, row %d (box from %d), want %d", m.uiState(), m.modalTop, answered.y, top)
	}
	if answered.h <= typed.h {
		t.Errorf("the answer did not grow the box: %d, then %d rows", typed.h, answered.h)
	}
	if bottom := answered.y + answered.h; bottom > m.height-2 {
		t.Errorf("the box reaches row %d, over the status line at %d", bottom-1, m.height-2)
	}
	m = update(t, m, tea.WindowSizeMsg{Width: 140, Height: 24})
	if m.modalTop != 24/4 {
		t.Errorf("resized to 24 rows: row %d, want %d", m.modalTop, 24/4)
	}
}

// A box whose rows run past what it may hold is cut there: Ask's failure,
// however long, leaves the status line and the footer in sight.
func TestBoxStopsAboveTheStatusLine(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 140, 24)
	m = press(t, m, "a")
	m.ask.stage, m.ask.question = askFailed, "parser"
	m.ask.err = strings.Repeat("claude said something went wrong. ", 120)
	s, _ := m.openModal()
	r, _ := m.boxRect(s)
	if bottom := r.y + r.h; bottom > m.height-2 {
		t.Errorf("the box ends on row %d, over the status line on %d", bottom-1, m.height-2)
	}
	lines := strings.Split(m.render(), "\n")
	if got := ansi.Strip(lines[m.height-1]); !strings.Contains(got, "esc") {
		t.Errorf("the footer is covered: %q", got)
	}
	if len(m.drawBox(s, r.w, m.boxRows())) != r.h {
		t.Errorf("drawn %d rows, placed as %d", len(m.drawBox(s, r.w, m.boxRows())), r.h)
	}
}

// On a terminal too short for the sort menu, a click on its ellipsis or
// its bottom edge closes it rather than choosing an order not drawn.
func TestClickChoosesOnlyRowsDrawn(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 140, 8)
	m = press(t, m, "s")
	s, _ := m.openModal()
	r, _ := m.boxRect(s)
	if len(s.box.lines(m, r.w-4, m.boxRows())) <= m.boxRows() {
		t.Fatalf("the sort menu is not cut on 8 rows (holds %d)", m.boxRows())
	}
	sel := m.sortSel
	m = update(t, m, tea.MouseClickMsg{X: r.x + 4, Y: r.y + r.h - 2, Button: tea.MouseLeft})
	if m.modal != modalNone || m.sortSel != sel {
		t.Errorf("a click on the ellipsis: modal %v, sort %d (was %d)", m.modal, m.sortSel, sel)
	}
}

// Under a modal the screen is drawn in one faint color, but for the
// footer, which says the modal's keys, and the box, framed in the accent
// color.
func TestScreenDimsUnderAModal(t *testing.T) {
	for _, bg := range []color.Color{color.Black, color.White} {
		m, _ := newTestModel(t, config.Default().TUI, 140, 40)
		m = update(t, m, tea.BackgroundColorMsg{Color: bg})
		screenDimsUnder(t, press(t, m, "s"))
	}
}

func screenDimsUnder(t *testing.T, m Model) {
	t.Helper()
	s, _ := m.openModal()
	r, _ := m.boxRect(s)
	lines := strings.Split(m.render(), "\n")
	faint := m.st.dim.Render("x")
	faint = faint[:strings.Index(faint, "x")]
	for i, l := range lines[:len(lines)-1] {
		if i >= r.y && i < r.y+r.h {
			// Beside the box, left and right of it.
			l = ansi.Truncate(l, r.x, "") + ansi.TruncateLeft(l, r.x+r.w, "")
		}
		for _, sgr := range sgrs(l) {
			if sgr != faint && sgr != "\x1b[m" {
				t.Fatalf("line %d keeps %q:\n%q", i, sgr, l)
			}
		}
	}
	if foot := lines[len(lines)-1]; !strings.Contains(foot, m.st.key.Render("esc")) {
		t.Errorf("the footer lost its colors: %q", foot)
	}
	accent := m.st.id.Render("╭─")
	if top := lines[r.y]; !strings.Contains(top, accent) {
		t.Errorf("the sort menu is not framed in the accent color: %q", top)
	}
}

// The rows of an image keep their color under a modal: it names the
// image Kitty draws in those cells.
func TestDimKeepsImageRows(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 140, 40)
	row := placeholders(7, 3, 2)[1]
	line := m.st.bar.Render("▎") + " " + row + m.st.title.Render(" after")
	got := m.dim(line)
	if !strings.Contains(got, row) {
		t.Errorf("the image row changed:\n got %q\nwant it to hold %q", got, row)
	}
	if want := m.st.dim.Render("▎ "); !strings.HasPrefix(got, want) {
		t.Errorf("the text before the image is not dimmed: %q", got)
	}
	if ansi.Strip(got) != ansi.Strip(line) {
		t.Errorf("the text changed: %q, was %q", ansi.Strip(got), ansi.Strip(line))
	}
}

// sgrs are the SGR sequences in s that color some text, in order: one
// followed at once by another, as ansi.Truncate leaves past its cut,
// colors nothing.
func sgrs(s string) []string {
	var out []string
	for _, m := range sgrRe.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

var sgrRe = regexp.MustCompile(`(\x1b\[[0-9;]*m)[^\x1b]`)

// A toast shown while a modal is open keeps its color: it is there to be
// read.
func TestToastKeepsItsColorUnderAModal(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 140, 40)
	m = press(t, m, "s")
	m.toast, m.toastKind = "Could not keep it", toastWarn
	lines := strings.Split(m.render(), "\n")
	if status := lines[len(lines)-2]; !strings.Contains(status, m.st.warn.Render("Could not keep it")) {
		t.Errorf("the toast lost its color: %q", status)
	}
}

// A row of an image cut short, its color's end gone, is still kept.
func TestDimKeepsACutImageRow(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 140, 40)
	row := ansi.Truncate(placeholders(7, 10, 1)[0], 5, ellipsis)
	if !strings.Contains(row, ellipsis+"\x1b[39m") {
		t.Fatalf("the ellipsis is not between the cells and the color's end: %q", row)
	}
	if got := m.dim(row); !strings.Contains(got, "\x1b[38;5;7m") {
		t.Errorf("the cut row lost its color: %q", got)
	}
}
