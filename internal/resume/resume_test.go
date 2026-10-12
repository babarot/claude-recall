package resume

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/babarot/claude-recall/internal/parser"
)

func TestRecallArgs(t *testing.T) {
	self := []string{"/bin/recall", "mcp", "--db", "/tmp/vault.db"}
	args := RecallArgs(self, "abc-123", "")
	if len(args) != 5 || args[1] != "--mcp-config" || args[3] != "--allowedTools" {
		t.Fatalf("args %q", args)
	}
	// The prompt comes before the flags that take every argument after them.
	if !strings.Contains(args[0], "recall session abc-123") || !strings.Contains(args[0], "where it left off") {
		t.Fatalf("prompt %q", args[0])
	}
	if want := `{"mcpServers":{"recall":{"args":["mcp","--db","/tmp/vault.db"],"command":"/bin/recall"}}}`; args[2] != want {
		t.Fatalf("mcp config %s, want %s", args[2], want)
	}
	if args[4] != "mcp__recall__recall_search,mcp__recall__recall_list,mcp__recall__recall_export" {
		t.Fatalf("allowed tools %q", args[4])
	}
	topic := RecallArgs(self, "abc-123", "the retry policy")[0]
	if !strings.Contains(topic, "recall session abc-123") || !strings.Contains(topic, "about: the retry policy") {
		t.Fatalf("prompt %q", topic)
	}
}

func TestShellQuote(t *testing.T) {
	if got := ShellQuote("/Users/me/src/repo"); got != "/Users/me/src/repo" {
		t.Errorf("plain path quoted: %s", got)
	}
	if got := ShellQuote("/tmp/my dir/it's"); got != `'/tmp/my dir/it'\''s'` {
		t.Errorf("got %s", got)
	}
}

func TestCommand(t *testing.T) {
	if got, want := Command("/tmp/my dir", "abc-123"), `cd '/tmp/my dir' && claude -r abc-123`; got != want {
		t.Errorf("Command = %s, want %s", got, want)
	}
	self := []string{"/bin/recall", "mcp"}
	if got := RecallCommand(self, "abc-123"); !strings.HasPrefix(got, "claude 'Use the recall tools to recall session abc-123") ||
		!strings.Contains(got, ` --mcp-config '{"mcpServers"`) {
		t.Errorf("RecallCommand = %s", got)
	}
}

func TestWhere(t *testing.T) {
	dirs := []string{"/home/.claude/projects", "/box/projects"}
	cases := []struct {
		f       parser.File
		found   bool
		other   string
		missing bool
	}{
		{parser.File{Dir: dirs[0], Project: "-src-app"}, true, "", false},
		{parser.File{Dir: dirs[0], Project: "-src-elsewhere"}, true, "", true}, // claude -r reads the project's own directory
		{parser.File{Dir: dirs[1], Project: "-src-app"}, true, dirs[1], false},
		{parser.File{}, false, "", true},
	}
	for _, c := range cases {
		other, missing := Where(c.f, c.found, dirs, "-src-app")
		if other != c.other || missing != c.missing {
			t.Errorf("Where(%+v, %v) = %q %v, want %q %v", c.f, c.found, other, missing, c.other, c.missing)
		}
	}
}

func TestLocate(t *testing.T) {
	primary, other := t.TempDir(), t.TempDir()
	write := func(tree, project, id string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(tree, project), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tree, project, id+".jsonl"), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(primary, "-src-app", "here")
	write(other, "-src-app", "there")
	dirs := []string{primary, other}
	if o, missing := Locate(dirs, "here", "-src-app"); o != "" || missing {
		t.Errorf("here: %q %v", o, missing)
	}
	if o, missing := Locate(dirs, "there", "-src-app"); o != other || missing {
		t.Errorf("there: %q %v", o, missing)
	}
	if _, missing := Locate(dirs, "gone", "-src-app"); !missing {
		t.Error("gone: a session with no transcript is missing")
	}
}
