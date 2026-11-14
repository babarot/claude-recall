package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/babarot/claude-recall/internal/config"
)

// typingCases open every field that is typed in, one for each state whose
// typing() is true.
func typingCases(t *testing.T) map[uiState]Model {
	t.Helper()
	base := func() Model { m, _ := newTestModel(t, config.Default().TUI, 140, 40); return m }
	fm := folderModel(t, config.Default().TUI, newFolderFixture(t), 140, 40)
	return map[uiState]Model{
		uiFilter:       press(t, base(), "/"),
		uiConvSearch:   press(t, base(), "space", "/"),
		uiAskTyping:    press(t, askModel(t, &fakeAsk{}, config.Default().TUI), "a"),
		uiRecall:       press(t, base(), "c"),
		uiFolderSearch: press(t, fm, "left", "left", "/"),
	}
}

// The terminal's cursor stands right after the text typed in each field,
// where an input method shows the text being composed. あ checks that a
// wide character moves it two cells.
func TestCursorAfterTheTypedText(t *testing.T) {
	cases := typingCases(t)
	for s := range numUIStates {
		if _, ok := cases[s]; s.typing() && !ok {
			t.Errorf("no case opens the field of state %q: add one to typingCases", s)
		}
	}
	const typed = "aあb"
	for st, m := range cases {
		t.Run(st.String(), func(t *testing.T) {
			if got := m.uiState(); got != st {
				t.Fatalf("the case is in state %q, not %q", got, st)
			}
			m = typeText(t, m, typed)
			s, cur := m.renderWithCursor()
			if cur == nil {
				t.Fatalf("no cursor:\n%s", ansi.Strip(s))
			}
			if strings.Contains(s, cursorMark) {
				t.Error("the cursor's mark is left on the screen")
			}
			lines := strings.Split(ansi.Strip(s), "\n")
			if cur.Y < 0 || cur.Y >= len(lines) {
				t.Fatalf("cursor on row %d of %d", cur.Y, len(lines))
			}
			if left := ansi.Truncate(lines[cur.Y], cur.X, ""); !strings.HasSuffix(left, typed) {
				t.Errorf("left of the cursor: %q", left)
			}
		})
	}
}

// No cursor is shown where nothing is typed in, and no field draws one of
// its own.
func TestNoCursorOutsideAField(t *testing.T) {
	for _, c := range footerCases(t) {
		if c.m.uiState().typing() {
			continue
		}
		s, cur := c.m.renderWithCursor()
		if cur != nil {
			t.Errorf("%s: cursor at %+v", c.name, cur.Position)
		}
		if strings.Contains(s, cursorMark) {
			t.Errorf("%s: the cursor's mark is left on the screen", c.name)
		}
	}
}

// TestFieldsFromNewInput fails on a field made, or its cursor changed,
// outside newInput, which leaves the cursor to the terminal. A field made
// there still has to be drawn through inputView; TestCursorAfterTheTypedText
// checks that for each.
func TestFieldsFromNewInput(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	inputPackages := map[string]bool{
		"charm.land/bubbles/v2/textinput": true,
		"charm.land/bubbles/v2/textarea":  true,
	}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]bool{}
		for _, im := range f.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			if !inputPackages[p] {
				continue
			}
			name := p[strings.LastIndex(p, "/")+1:]
			if im.Name != nil {
				name = im.Name.Name
			}
			names[name] = true
		}
		for _, decl := range f.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "newInput" {
				continue
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if x, ok := sel.X.(*ast.Ident); ok && names[x.Name] && sel.Sel.Name == "New" {
					t.Errorf("%s: make a field with newInput", fset.Position(n.Pos()))
				}
				if sel.Sel.Name == "SetVirtualCursor" {
					t.Errorf("%s: only newInput sets where a field's cursor is drawn", fset.Position(n.Pos()))
				}
				return true
			})
		}
	}
}

// A field narrower than what is typed in it scrolls, and the cursor stays
// right after the last character typed.
func TestCursorAtTheEndOfAScrolledField(t *testing.T) {
	m := typingCases(t)[uiFolderSearch]
	typed := strings.Repeat("あいうえおabc", 6)
	m = typeText(t, m, typed)
	s, cur := m.renderWithCursor()
	if cur == nil {
		t.Fatal("no cursor")
	}
	line := ansi.Strip(strings.Split(s, "\n")[cur.Y])
	if left := ansi.Truncate(line, cur.X, ""); !strings.HasSuffix(left, "おabc") {
		t.Errorf("left of the cursor: %q", left)
	}
}

// Moved back over a wide character, the cursor stands before it.
func TestCursorMovedBack(t *testing.T) {
	m := typeText(t, typingCases(t)[uiFilter], "aあb")
	m = update(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	m = update(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	s, cur := m.renderWithCursor()
	if cur == nil {
		t.Fatal("no cursor")
	}
	line := ansi.Strip(strings.Split(s, "\n")[cur.Y])
	if left := ansi.Truncate(line, cur.X, ""); !strings.HasSuffix(left, "/ a") {
		t.Errorf("left of the cursor: %q", left)
	}
}
