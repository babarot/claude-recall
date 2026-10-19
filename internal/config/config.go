// Package config holds the filesystem locations claude-recall reads and
// writes, and the user's config file.
package config

import (
	"os"
	"path/filepath"
)

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "/tmp"
}

// ClaudeDir is Claude Code's config directory: CLAUDE_CONFIG_DIR, as Claude
// Code itself reads it, or ~/.claude.
func ClaudeDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	return filepath.Join(homeDir(), ".claude")
}

// ProjectsDir holds Claude Code's JSONL transcripts, one directory per project.
func ProjectsDir() string { return filepath.Join(ClaudeDir(), "projects") }

// CodexDir is Codex's home directory: CODEX_HOME, as Codex itself reads
// it, or ~/.codex.
func CodexDir() string {
	if d := os.Getenv("CODEX_HOME"); d != "" {
		return d
	}
	return filepath.Join(homeDir(), ".codex")
}

// CodexSessionsDirs are the rollout trees under a Codex home directory
// that are on disk: sessions, and archived_sessions, where `codex archive`
// moves a thread.
func CodexSessionsDirs(home string) []string {
	var out []string
	for _, name := range []string{"sessions", "archived_sessions"} {
		d := filepath.Join(home, name)
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			out = append(out, d)
		}
	}
	return out
}

// DefaultDBPath is the archive database. It stays in ~/.claude whatever
// CLAUDE_CONFIG_DIR says: following it would leave an archive that already
// exists behind and start an empty one, and db under [core] moves it.
func DefaultDBPath() string { return filepath.Join(homeDir(), ".claude", "vault.db") }
