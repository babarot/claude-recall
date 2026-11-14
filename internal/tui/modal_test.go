package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	tea "charm.land/bubbletea/v2"

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
		// Closing whatever modal shows, on a click.
		"closeModal": true,
		// modal.go's own methods.
		"modalKeyFor": true, "typeInModal": true, "edgeHint": true,
		"boxRows": true, "boxWidth": true, "boxRect": true, "drawBox": true, "drawModal": true, "clickModal": true,
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
