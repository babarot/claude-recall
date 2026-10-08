package tui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/babarot/claude-recall/internal/config"
)

var updateHints = flag.Bool("update", false, "rewrite testdata/keyhints.golden")

// helpCases are the key list opened from each pane.
func helpCases(t *testing.T, m, folders Model) []footerCase {
	return []footerCase{
		{"list", m},
		{"conversation frame", press(t, m, "tab")},
		{"reading", press(t, m, "space")},
		{"reading, list focused", press(t, m, "space", "tab")},
		{"folder list", press(t, folders, "left", "left")},
	}
}

// footerCase is a state of the screen whose footer is pinned.
type footerCase struct {
	name string
	m    Model
}

// footerCases are the footer in each state of the screen, and in the
// variants of a state that change it. Every uiState must be among them:
// TestUIStatesCovered fails on one that is not.
func footerCases(t *testing.T) []footerCase {
	var out []footerCase
	add := func(name string, m Model) { out = append(out, footerCase{name, m}) }
	base := func() Model { m, _ := newTestModel(t, config.Default().TUI, 140, 40); return m }
	m := base()
	add("list", m)
	add("conversation frame", press(t, m, "tab"))
	add("details frame", press(t, m, "tab", "tab"))
	add("what was done frame", press(t, m, "tab", "tab", "tab"))
	add("reading", press(t, m, "space"))
	add("reading, list focused", press(t, m, "space", "tab"))
	add("conversation search typing", press(t, m, "space", "/"))
	add("conversation search kept", press(t, typeText(t, press(t, m, "space", "/"), "older"), "enter"))
	add("filter", press(t, m, "/"))
	add("filter, key hint", typeText(t, press(t, m, "/"), "bra"))
	add("filter, key term", typeText(t, press(t, m, "/"), "title:"))
	for _, st := range []askStage{askTyping, askRunning, askAnswered, askFailed} {
		a := base()
		a.ask.stage = st
		add("ask "+[]string{"closed", "typing", "running", "answered", "failed"}[st], a)
	}
	add("key list", press(t, m, "?"))
	add("sort menu", press(t, m, "s"))
	add("recall", press(t, m, "c"))
	add("what's new", press(t, m, "w"))

	f := newFolderFixture(t)
	fm := folderModel(t, config.Default().TUI, f, 140, 40)
	add("list, started in a folder", fm.StartIn(f.repo))
	add("list, folder list open", press(t, fm, "left"))
	add("folder list", press(t, fm, "left", "left"))
	add("folder search typing", press(t, fm, "left", "left", "/"))
	add("folder search kept", press(t, typeText(t, press(t, fm, "left", "left", "/"), "app"), "enter"))
	add("filter, suggestions", typeText(t, press(t, fm, "/"), "folder:"))
	return out
}

// TestKeyHints pins every place that names keys, as rendered: the footer
// in each context, the ? key list, and the hints on the modals' top
// edges. A change to a key or its wording shows up here.
func TestKeyHints(t *testing.T) {
	var b strings.Builder
	add := func(name, s string) {
		b.WriteString("== " + name + "\n" + s + "\n")
	}
	topLine := func(name string, m Model, has string) {
		for _, l := range strings.Split(m.render(), "\n") {
			if strings.Contains(ansi.Strip(l), has) {
				add(name, l)
				return
			}
		}
		t.Errorf("%s: no line with %q:\n%s", name, has, screen(m))
	}

	for _, c := range footerCases(t) {
		add("footer: "+c.name, c.m.renderHelp())
	}

	base := func() Model { m, _ := newTestModel(t, config.Default().TUI, 140, 40); return m }
	m := base()
	fm := folderModel(t, config.Default().TUI, newFolderFixture(t), 140, 40)
	// The key list is pinned opened from every pane: TestUIStatesCovered
	// checks these reach every keyContext.
	for _, c := range helpCases(t, m, fm) {
		add("? key list: "+c.name, strings.Join(c.m.helpBox(100, 80), "\n"))
	}
	topLine("sort menu top edge", press(t, m, "s"), "Sort by")
	topLine("what's new top edge", press(t, m, "w"), "What's new")
	for _, st := range []askStage{askTyping, askRunning, askFailed} {
		a := base()
		a.ask.stage = st
		topLine("ask box top edge "+[]string{"closed", "typing", "running", "answered", "failed"}[st], a, "Ask Claude")
	}

	got := b.String()
	path := filepath.Join("testdata", "keyhints.golden")
	if *updateHints {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/tui -run TestKeyHints -update)", err)
	}
	if got != string(want) {
		gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := range max(len(gl), len(wl)) {
			var g, w string
			if i < len(gl) {
				g = gl[i]
			}
			if i < len(wl) {
				w = wl[i]
			}
			if g != w {
				t.Fatalf("line %d differs:\n got  %q\n want %q", i+1, ansi.Strip(g), ansi.Strip(w))
			}
		}
	}
}
