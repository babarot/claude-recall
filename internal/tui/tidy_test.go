package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/babarot/claude-recall/internal/config"
	"github.com/babarot/claude-recall/internal/db"
	"github.com/charmbracelet/x/ansi"
)

func TestGroupFiles(t *testing.T) {
	home := "/Users/me"
	files := []db.Count{
		{Name: "/Users/me/src/github.com/me/app/main.go", N: 2},
		{Name: "/Users/me/src/github.com/me/lib/a/b/c.go", N: 1},
		{Name: "/Users/me/src/github.com/me/lib/d.go", N: 1},
		{Name: "/private/tmp/x/scratch.py", N: 1},
		{Name: "/Users/me/.herdr/worktrees/blog/worktree-calm-sea/posts/a.md", N: 3},
		{Name: "/Users/me/.config/foo/bar.toml", N: 1},
	}
	groups, temps := groupFiles(files, "/Users/me/src/github.com/me/app", home)
	if temps != 1 {
		t.Errorf("temps %d", temps)
	}
	got := map[string][]db.Count{}
	for _, g := range groups {
		got[g.name] = g.files
	}
	if groups[0].name != "" || got[""][0].Name != "main.go" {
		t.Errorf("own folder first, relative: %+v", groups)
	}
	if len(got["lib"]) != 2 || got["lib"][0].Name != "a/b/c.go" {
		t.Errorf("other repo: %+v", got["lib"])
	}
	if got["blog ⌥calm-sea"][0].Name != "posts/a.md" {
		t.Errorf("herdr worktree: %+v", got)
	}
	if got["~/.config/foo"][0].Name != "bar.toml" {
		t.Errorf("elsewhere: %+v", got)
	}
}

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

func TestProgramOf(t *testing.T) {
	cases := []struct{ in, want string }{
		{"cd /Users/me/src/app && go test ./... 2>&1 | tail -5", "go test"},
		{"S=/tmp/x && cd $S && python3 run.py", "python3"},
		{"git status\nsecond line", "git status"},
		// Assignments whose values hold spaces and flags.
		{`c=$(readlink -f "$(command -v codex)"); echo $c`, "echo"},
		{`UA='Mozilla/5.0 (Macintosh; Intel)' curl -A "$UA" x`, "curl"},
		{"FROM=$(($(date -v-30d +%s) * 1000)); for i in a b; do pup api $i; done", "pup"},
		// Loops, comments, continuations, subshells, functions, wrappers.
		{"for f in a.tf b.tf; do\n  terraform fmt $f\ndone", "terraform fmt"},
		{"until [ -f /tmp/done ]; do sleep 5; done", "sleep"},
		{"# check the alerts first\ngh api repos/x/y", "gh api"},
		{"FILENAME=a.tf \\\n  conftest test a.tf", "conftest"},
		{"(npx wrangler dev > /tmp/log 2>&1 &)", "npx"},
		{`q() { gh api -X GET search/issues; }`, "gh api"},
		{"builtin cd ~/x\ngit fetch", "git fetch"},
		{"timeout 60 go test ./...", "go test"},
		{"q(){ pup metrics query; }; q a", "pup"},
		{"for n in 1 2; do\n  case $n in\n    1) team=a;;\n    2) team=b;;\n  esac\n  gh issue view $n\ndone", "gh issue"},
		{`"$SKILL_DIR/get-summary.ts" --json`, "get-summary.ts"},
		{`K="kubectl --context dev"; $K get pods`, ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := programOf(c.in); got != c.want {
			t.Errorf("programOf(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCommandCounts(t *testing.T) {
	got := commandCounts([]string{
		"cd /x && go test ./...", "go test ./internal/...", "go vet ./...",
		"git -C /repo status", "git commit -m x", "/usr/bin/python3 a.py", "python3 b.py", "",
	})
	want := []db.Count{{Name: "go test", N: 2}, {Name: "python3", N: 2}, {Name: "git commit", N: 1}, {Name: "git status", N: 1}, {Name: "go vet", N: 1}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: got %+v want %+v", i, got[i], want[i])
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
