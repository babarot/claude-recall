package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/babarot/claude-recall/internal/config"
	"github.com/babarot/claude-recall/pkg/db"
	"github.com/charmbracelet/x/ansi"
)

func TestMiddleEllipsis(t *testing.T) {
	const deep = "internal/very/deep/path/to/the/file.go"
	for _, c := range []struct {
		in   string
		n    int
		want string
	}{
		{"internal/tui/view.go", 24, "internal/tui/view.go"},
		{deep, 24, "internal/…/the/file.go"},
		// More room keeps more directories at the end, then at the start.
		{deep, 30, "internal/…/path/to/the/file.go"},
		{deep, 36, "internal/…/deep/path/to/the/file.go"},
		{"~/src/github.com/me/app/internal", 30, "~/…/github.com/me/app/internal"},
		{"~/src/github.com/me/app/internal", 26, "~/src/…/me/app/internal"},
		// Less room drops to the file name, then cuts it.
		{deep, 18, "internal/…/file.go"},
		{deep, 12, "…/file.go"},
		{deep, 5, "…e.go"},
		{"/Users/me/src/app", 12, "/…/src/app"},
	} {
		if got := middleEllipsis(c.in, c.n); got != c.want {
			t.Errorf("middleEllipsis(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

// Tools and commands share a scale: their bars start in the same column,
// and a command's bar is as long against Bash's as its count is.
func TestBarsShareScale(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 140, 40)
	tools := []db.Count{{Name: "Bash", N: 400}, {Name: "AskUserQuestion", N: 13}}
	commands := []db.Count{{Name: "python3", N: 100}}
	sc := newBarScale(append(slices.Clone(tools), commands...), 60)
	lines := append(m.bars(tools, sc), m.bars(commands, sc)...)
	start := func(l string) int { return strings.Index(ansi.Strip(l), "▇") }
	width := func(l string) int { return strings.Count(ansi.Strip(l), "▇") }
	if start(lines[0]) != start(lines[2]) {
		t.Fatalf("bars start apart:\n%s\n%s", ansi.Strip(lines[0]), ansi.Strip(lines[2]))
	}
	if width(lines[0]) != barMaxW || width(lines[2]) != barMaxW/4 {
		t.Fatalf("bar widths %d and %d", width(lines[0]), width(lines[2]))
	}
}
