package recall_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The packages and what each may import, from the bottom layer up. A
// package imports only from lower layers and only the outside modules
// listed for it; the parts of the standard library that reach outside the
// process belong to the packages listed in reach. docs/architecture.md
// shows this table, and TestArchitectureDoc keeps the two the same. Test
// files are not checked: a test may import what helps it. See ADR-010.
//
// A package not in the table fails the test, so a new one is placed here
// on purpose.
var layers = []layer{
	{"internal/jscompat", 0, nil, nil, "JavaScript's trim, UTF-16 lengths and JSON.stringify, so stored text stays as the TypeScript implementation wrote it"},
	{"internal/pricing", 0, nil, nil, "What the tokens of a session cost at the Claude API's list prices"},
	{"internal/theme", 0, nil, nil, "The TUI's color schemes, which the web UI is drawn in too"},
	{"internal/title", 0, nil, nil, "A session's name, as every client shows it"},
	{"internal/version", 0, nil, nil, "The release version, which tagpr updates"},
	{"internal/filter", 0, nil, nil, "The filter the TUI and the web UI narrow sessions with: words and key:value terms"},
	{"internal/worktree", 0, nil, nil, "The repository a directory belongs to, and the main checkout of a linked worktree"},
	{"internal/webui", 0, nil, nil, "The web UI's built files, embedded"},
	{"internal/flock", 0, nil, nil, "A lock on a file, taken without waiting"},
	{"internal/update", 0, nil, nil, "Whether a newer release is out, its notes, and replacing an install.sh install with it"},
	{"demo/fakeapi", 0, nil, nil, "A made-up GitHub API for the demo; never built into recall"},
	{"internal/db", 1, []string{"internal/jscompat"}, []string{"modernc.org/sqlite"}, "The archive: SQLite with FTS5, its schema and migrations"},
	{"internal/config", 1, []string{"internal/pricing", "internal/theme"}, []string{"github.com/pelletier/go-toml/v2"}, "Where recall reads and writes, the config file and its mistakes, and the reference of every setting"},
	{"internal/parser", 1, []string{"internal/jscompat"}, nil, "A Claude Code JSONL transcript as the rows the archive stores"},
	{"internal/askhistory", 1, []string{"internal/flock"}, nil, "The questions asked in Ask, kept between runs"},
	{"internal/logging", 1, []string{"internal/flock"}, nil, "The log file every recall process appends to, as JSON lines, and its rotation"},
	{"internal/cli", 2, []string{"internal/db", "internal/jscompat"}, nil, "What the CLI prints for search, list, export and stats"},
	{"internal/repos", 2, []string{"internal/db", "internal/worktree"}, nil, "The repository each session directory belongs to"},
	{"internal/resume", 2, []string{"internal/parser"}, nil, "The commands that go back to a session: claude -r, or a new claude that recalls it"},
	{"internal/tidy", 2, []string{"internal/db", "internal/worktree"}, nil, "What a session's tools did, to read at a glance: files by where they are, commands by program"},
	{"internal/importer", 2, []string{"internal/config", "internal/db", "internal/logging", "internal/parser", "internal/worktree"}, nil, "Importing transcripts into the archive, one transaction a session"},
	{"internal/api", 3, []string{"internal/cli", "internal/db", "internal/jscompat", "internal/title"}, nil, "The JSON the MCP tools and the web API return"},
	{"history", 3, []string{"internal/db", "internal/repos", "internal/title"}, nil, "The read-only Go API other modules import, under v1 semver"},
	{"internal/watcher", 3, []string{"internal/db", "internal/flock", "internal/importer", "internal/logging", "internal/parser"}, nil, "Watching the transcript trees and importing what changes, one watcher at a time"},
	{"internal/fixture", 3, []string{"internal/db", "internal/importer"}, nil, "Transcripts made up for the tests"},
	{"demo/gen", 3, []string{"internal/db", "internal/importer"}, nil, "Makes the demo's archive; never built into recall"},
	{"internal/mcp", 4, []string{"internal/api", "internal/db", "internal/jscompat", "internal/repos", "internal/version", "internal/worktree"}, []string{"github.com/modelcontextprotocol/go-sdk"}, "The MCP server: the recall tools and the recap prompt over stdio"},
	{"internal/tui", 4, []string{"internal/askhistory", "internal/config", "internal/db", "internal/filter", "internal/parser", "internal/resume", "internal/theme", "internal/tidy", "internal/title", "internal/update", "internal/version", "internal/worktree"},
		[]string{"charm.land/bubbles/v2", "charm.land/bubbletea/v2", "charm.land/lipgloss/v2", "github.com/atotto/clipboard", "github.com/charmbracelet/ultraviolet", "github.com/charmbracelet/x/ansi"}, "The terminal UI"},
	{"internal/web", 4, []string{"internal/api", "internal/db", "internal/filter", "internal/importer", "internal/jscompat", "internal/pricing", "internal/repos", "internal/resume", "internal/theme", "internal/tidy", "internal/title", "internal/version", "internal/watcher", "internal/webui", "internal/worktree"}, nil, "The web UI's server and its API, on this machine"},
	{"cmd/recall", 5, []string{"internal/cli", "internal/config", "internal/db", "internal/importer", "internal/logging", "internal/mcp", "internal/pricing", "internal/repos", "internal/resume", "internal/title", "internal/tui", "internal/update", "internal/version", "internal/watcher", "internal/web", "internal/webui"},
		[]string{"charm.land/bubbletea/v2", "github.com/spf13/cobra"}, "Putting it together: the commands and their flags"},
}

// layer is a package's place: its layer, the packages of this module
// (named by their path in it) and outside modules it may import, and what
// it does.
type layer struct {
	pkg      string
	layer    int
	local    []string
	external []string
	does     string
}

// reach are the parts of the standard library that reach outside the
// process, and the packages allowed to use each.
var reach = map[string][]string{
	"net":      {"cmd/recall", "internal/web"},
	"net/http": {"cmd/recall", "demo/fakeapi", "internal/update", "internal/web"},
	"os/exec":  {"cmd/recall", "demo/gen", "internal/tui"},
	"syscall":  {"cmd/recall", "internal/flock", "internal/parser"},
}

const module = "github.com/babarot/claude-recall"

type goPackage struct {
	ImportPath string
	GoFiles    []string
	Imports    []string
	Standard   bool
}

func goList(t *testing.T, args ...string) []goPackage {
	t.Helper()
	out, err := exec.Command("go", append([]string{"list", "-json"}, args...)...).Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var pkgs []goPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p goPackage
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}

// readSources reads every Go file of the module. The imports come from go
// list, a process of its own, whose reads go test does not see, so it
// would keep a passing result cached after a file changed; files the test
// reads itself are part of what the cache is kept by.
func readSources(t *testing.T) {
	t.Helper()
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") && path != "." {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			_, err = os.ReadFile(path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// listPackages are the packages of this module, but for one of test files
// alone, as this one is.
func listPackages(t *testing.T) []goPackage {
	t.Helper()
	readSources(t)
	var out []goPackage
	for _, p := range goList(t, "./...") {
		if len(p.GoFiles) > 0 {
			out = append(out, p)
		}
	}
	return out
}

func rel(path string) string { return strings.TrimPrefix(strings.TrimPrefix(path, module), "/") }

func outside(imp string) bool { return strings.Contains(strings.SplitN(imp, "/", 2)[0], ".") }

func inModule(imp string, m string) bool { return imp == m || strings.HasPrefix(imp, m+"/") }

func TestArchitecture(t *testing.T) {
	for _, p := range listPackages(t) {
		r := rel(p.ImportPath)
		i := slices.IndexFunc(layers, func(l layer) bool { return l.pkg == r })
		if i < 0 {
			t.Errorf("%s is not in the layer table in arch_test.go and docs/architecture.md", p.ImportPath)
			continue
		}
		l := layers[i]
		for _, imp := range p.Imports {
			switch {
			case strings.HasPrefix(imp, module+"/"):
				if !slices.Contains(l.local, rel(imp)) {
					t.Errorf("%s imports %s, which is not in its allowed imports", p.ImportPath, imp)
				}
			case outside(imp):
				if !slices.ContainsFunc(l.external, func(m string) bool { return inModule(imp, m) }) {
					t.Errorf("%s imports %s, an outside module not allowed for it", p.ImportPath, imp)
				}
			default:
				if who, ok := reach[imp]; ok && !slices.Contains(who, r) {
					t.Errorf("%s imports %s, which only %s may use", p.ImportPath, imp, strings.Join(who, ", "))
				}
			}
		}
	}
}

// TestLayersGoDownward checks the table itself: every allowed import is a
// package of a lower layer.
func TestLayersGoDownward(t *testing.T) {
	layerOf := map[string]int{}
	for _, l := range layers {
		layerOf[l.pkg] = l.layer
	}
	for _, l := range layers {
		for _, dep := range l.local {
			dl, ok := layerOf[dep]
			if !ok || dl >= l.layer {
				t.Errorf("%s (layer %d) may import %s (layer %d, known %v)", l.pkg, l.layer, dep, dl, ok)
			}
		}
	}
}

// TestNoUnusedImportAllowed checks that every package of the table exists
// and every import it allows is made, so the table tells what each
// package depends on. reach is not checked: it limits who may use a
// package, and a package no one uses is still limited.
func TestNoUnusedImportAllowed(t *testing.T) {
	imports := map[string][]string{}
	for _, p := range listPackages(t) {
		imports[rel(p.ImportPath)] = p.Imports
	}
	for _, l := range layers {
		imps, ok := imports[l.pkg]
		if !ok {
			t.Errorf("%s is in the table but is not a package", l.pkg)
			continue
		}
		for _, dep := range l.local {
			if !slices.Contains(imps, module+"/"+dep) {
				t.Errorf("%s may import %s but does not; remove it from the table", l.pkg, dep)
			}
		}
		for _, m := range l.external {
			if !slices.ContainsFunc(imps, func(imp string) bool { return inModule(imp, m) }) {
				t.Errorf("%s may import %s but does not; remove it from the table", l.pkg, m)
			}
		}
	}
}

// TestHistoryDependsOnLittle checks what other modules get with history:
// every package of this module it reaches imports only the standard
// library, modernc.org/sqlite and packages of this module, as CLAUDE.md
// promises. The table checks one step; this follows every one.
func TestHistoryDependsOnLittle(t *testing.T) {
	readSources(t)
	for _, p := range goList(t, "-deps", "./history") {
		if !inModule(p.ImportPath, module) {
			continue
		}
		for _, imp := range p.Imports {
			if outside(imp) && !inModule(imp, module) && !inModule(imp, "modernc.org/sqlite") {
				t.Errorf("history reaches %s, which imports %s", p.ImportPath, imp)
			}
		}
	}
}

var update = flag.Bool("update", false, "rewrite the layer table in docs/architecture.md")

const (
	docPath  = "docs/architecture.md"
	docStart = "<!-- layers:start (go test -run TestArchitectureDoc -update) -->"
	docEnd   = "<!-- layers:end -->"
)

// docTable is the layer table as docs/architecture.md shows it.
func docTable() string {
	var b strings.Builder
	b.WriteString("| Layer | Package | Does | May import | Outside modules |\n|---|---|---|---|---|\n")
	join := func(ss []string) string {
		if len(ss) == 0 {
			return "-"
		}
		q := make([]string, len(ss))
		for i, s := range ss {
			q[i] = "`" + s + "`"
		}
		return strings.Join(q, ", ")
	}
	for _, l := range layers {
		fmt.Fprintf(&b, "| %d | `%s` | %s | %s | %s |\n", l.layer, l.pkg, l.does, join(l.local), join(l.external))
	}
	b.WriteString("\nStandard library packages that reach outside the process:\n\n| Package | Only in |\n|---|---|\n")
	names := make([]string, 0, len(reach))
	for n := range reach {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		fmt.Fprintf(&b, "| `%s` | %s |\n", n, join(reach[n]))
	}
	return b.String()
}

func TestArchitectureDoc(t *testing.T) {
	src, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(src)
	start, end := strings.Index(doc, docStart), strings.Index(doc, docEnd)
	if start < 0 || end < start {
		t.Fatalf("%s has no %s ... %s block", docPath, docStart, docEnd)
	}
	want := docStart + "\n" + docTable() + docEnd
	got := doc[start : end+len(docEnd)]
	if got == want {
		return
	}
	if *update {
		if err := os.WriteFile(docPath, []byte(doc[:start]+want+doc[end+len(docEnd):]), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Errorf("the layer table in %s differs from arch_test.go; run go test -run TestArchitectureDoc -update", docPath)
}
