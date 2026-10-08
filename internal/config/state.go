package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// State is what the TUI remembers between runs, as opposed to File, which
// only the user edits.
type State struct {
	// DetailHeight is the detail pane height the user last chose, in lines.
	DetailHeight int `json:"detail_height,omitempty"`
	// Sidebar is whether the folder list was open.
	Sidebar bool `json:"sidebar,omitempty"`
	// ExpandRows is how many list rows stay above a spread Conversation.
	ExpandRows int `json:"expand_rows,omitempty"`
}

// StatePath returns the state file, honoring XDG_STATE_HOME.
func StatePath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(homeDir(), ".local", "state")
	}
	return filepath.Join(dir, "claude-recall", "state.json")
}

// UpdateCachePath returns the file that remembers the latest release and
// when it was looked for, beside the state file. It is a file of its own
// because the TUI rewrites the state file whole when it exits.
func UpdateCachePath() string { return filepath.Join(filepath.Dir(StatePath()), "update.json") }

// LastVersionPath returns the file that holds the version of the last TUI
// run, beside the state file. It is a file of its own because the TUI, and
// an older recall that never heard of it, rewrite the state file whole.
func LastVersionPath() string { return filepath.Join(filepath.Dir(StatePath()), "last_version") }

// LoadLastVersion reads the version of the last TUI run, empty if none.
func LoadLastVersion(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// SaveLastVersion records the version of this TUI run, replacing the file
// atomically.
func SaveLastVersion(path, v string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".last_version-*")
	if err != nil {
		return err
	}
	_, err = f.WriteString(v + "\n")
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		os.Remove(f.Name())
	}
	return err
}

// LoadState reads the state file. A missing or unreadable file is an empty
// state: losing it only loses a remembered pane height or layout.
func LoadState(path string) State {
	var s State
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

// SaveState writes the state file, replacing it atomically.
func SaveState(path string, s State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
