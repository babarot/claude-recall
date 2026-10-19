package parser

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const codexID = "019a1b2c-3d4e-7f80-9abc-def012345678"

// putRollout writes tree/YYYY/MM/DD/name and returns its path.
func putRollout(t *testing.T, tree, day, name string) string {
	t.Helper()
	dir := filepath.Join(tree, day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCodexSessionID(t *testing.T) {
	cases := map[string]string{
		"rollout-2026-05-07T17-24-21-" + codexID + ".jsonl":               "" + codexID,
		"rollout-2026-05-07T17-24-21-" + codexID + ".jsonl.zst":           codexID,
		"rollout-2026-05-07T17-24-21-" + codexID + "_other-rollout.jsonl": codexID, // thread/revert
		"rollout-2026-05-07T17-24-21-.jsonl":                              "",
		"rollout-2026-05-07T17-24-21" + codexID + ".jsonl":                "", // no hyphen after the time
		codexID + ".jsonl":         "", // a Claude Code transcript
		"rollout-compression.lock": "",
		"notes.txt":                "",
	}
	for name, want := range cases {
		got, ok := CodexSessionID(name)
		if got != want || ok != (want != "") {
			t.Errorf("CodexSessionID(%q) = %q, %v; want %q", name, got, ok, want)
		}
	}
	if !IsCodexPath("/x/sessions/2026/05/07/rollout-2026-05-07T17-24-21-"+codexID+".jsonl") || IsCodexPath("/x/projects/-p/"+codexID+".jsonl") {
		t.Error("IsCodexPath")
	}
}

// A Codex sessions tree is read by day; a Claude Code tree beside it as
// before; a tree holding both lists both.
func TestDiscoverCodexTree(t *testing.T) {
	codex, claude := t.TempDir(), t.TempDir()
	putRollout(t, codex, "2026/05/07", "rollout-2026-05-07T17-24-21-"+codexID+".jsonl")
	putRollout(t, codex, "2026/05/08", "rollout-2026-05-08T09-00-00-aaaa.jsonl.zst")
	putRollout(t, codex, "2026/05/08", "rollout-compression.lock") // not a rollout
	os.WriteFile(filepath.Join(codex, "2026", "05", "08", "stray.jsonl"), nil, 0o644)
	put(t, claude, "-p", "s1")
	put(t, codex, "-q", "s2") // a Claude Code project directory in the same tree

	got := ids(Discover(codex, claude))
	want := []string{claude + "|-p|s1", codex + "|-q|s2", codex + "|2026/05/07|" + codexID, codex + "|2026/05/08|aaaa"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, f := range Discover(codex, claude) {
		if want := f.Project[0] == '2'; f.IsCodex() != want || (f.Agent != AgentCodex && f.Agent != AgentClaude) {
			t.Errorf("%s: agent %q", f.Path, f.Agent)
		}
	}
}

// FindSession finds a rollout by its session ID, wherever its day is.
func TestFindCodexSession(t *testing.T) {
	codex := t.TempDir()
	putRollout(t, codex, "2026/05/07", "rollout-2026-05-07T17-24-21-"+codexID+".jsonl")
	putRollout(t, codex, "2026/06/01", "rollout-2026-06-01T10-00-00-"+codexID+"_revert.jsonl")
	dirs := []string{codex}
	all := Discover(dirs...)
	var want []File
	for _, f := range all {
		if f.SessionID == codexID {
			want = append(want, f)
		}
	}
	got := FindSession(dirs, codexID)
	if len(want) != 2 || !slices.Equal(got, want) {
		t.Errorf("FindSession = %v, want %v", got, want)
	}
	if got := FindSession(dirs, "nope"); len(got) != 0 {
		t.Errorf("FindSession(nope) = %v", got)
	}
}
