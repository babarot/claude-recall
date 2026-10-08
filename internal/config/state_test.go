package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatePath(t *testing.T) {
	t.Setenv("HOME", "/home/me")
	t.Setenv("XDG_STATE_HOME", "")
	if got := StatePath(); got != "/home/me/.local/state/claude-recall/state.json" {
		t.Errorf("got %q", got)
	}
	t.Setenv("XDG_STATE_HOME", "/state")
	if got := StatePath(); got != "/state/claude-recall/state.json" {
		t.Errorf("got %q", got)
	}
}

func TestState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claude-recall", "state.json")

	// A missing or broken file is an empty state.
	if got := LoadState(path); got != (State{}) {
		t.Errorf("missing: got %+v", got)
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("{nope"), 0o644)
	if got := LoadState(path); got != (State{}) {
		t.Errorf("broken: got %+v", got)
	}

	want := State{DetailHeight: 20, Sidebar: true, ExpandRows: 4}
	if err := SaveState(path, want); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(path); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	// It is written through a temporary file, which does not stay behind.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("the temporary file stayed: %v", err)
	}
}

func TestLastVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "last_version")
	if v := LoadLastVersion(path); v != "" {
		t.Errorf("missing: %q", v)
	}
	if err := SaveLastVersion(path, "1.8.0"); err != nil {
		t.Fatal(err)
	}
	if v := LoadLastVersion(path); v != "1.8.0" {
		t.Errorf("got %q", v)
	}
}
