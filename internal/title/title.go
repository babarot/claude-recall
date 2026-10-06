// Package title names a session the way every client shows it: the TUI,
// `recall list` and `recall search`, and the MCP server.
package title

import (
	"regexp"
	"strings"
)

var (
	commandName = regexp.MustCompile(`<command-name>\s*([^<]*?)\s*</command-name>`)
	commandArgs = regexp.MustCompile(`<command-args>\s*([^<]*?)\s*</command-args>`)
	bashInput   = regexp.MustCompile(`<bash-input>\s*([^<]*?)\s*</bash-input>`)
	anyTag      = regexp.MustCompile(`</?[a-zA-Z][\w-]*(\s[^>]*)?>`)
	spaces      = regexp.MustCompile(`\s+`)
)

// Display is what a session is called: its stored title, or failing that
// its first prompt with Claude Code's markup turned into text.
func Display(stored, firstPrompt string) string {
	if t := strings.TrimSpace(stored); t != "" {
		return t
	}
	return Prompt(firstPrompt)
}

// Prompt turns a prompt as Claude Code stored it into one line of text: a
// slash command's record reads as `/name args`, a `!` shell command as
// `! command`, and other markup is dropped.
func Prompt(p string) string {
	if m := commandName.FindStringSubmatch(p); m != nil {
		name := m[1]
		if !strings.HasPrefix(name, "/") {
			name = "/" + name
		}
		if a := commandArgs.FindStringSubmatch(p); a != nil && a[1] != "" {
			name += " " + a[1]
		}
		return Collapse(name)
	}
	if m := bashInput.FindStringSubmatch(p); m != nil {
		return Collapse("! " + m[1])
	}
	if t := Collapse(anyTag.ReplaceAllString(p, " ")); t != "" {
		return t
	}
	return "(no prompt)"
}

// Collapse joins whitespace runs into single spaces and trims the ends.
func Collapse(s string) string {
	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
}
