package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Release tells the TUI of a release of recall newer than the running one,
// which the status line shows until recall is updated.
type Release struct {
	// Version is the newer release known when the TUI starts, or empty.
	Version string
	// How updates this install: a command to run when Command is true
	// (recall update, brew upgrade claude-recall), else words (update it
	// with Nix).
	How     string
	Command bool
	// Check, when not nil, looks for the latest release again and returns
	// it if it is newer than the running one, or empty. It runs in the
	// background, so the TUI does not wait for the network.
	Check func() string
}

type releaseFound struct{ version string }

// TellOfRelease shows r in the status line, and looks again with r.Check.
func (m Model) TellOfRelease(r Release) Model {
	m.release = r
	return m
}

func (m Model) checkRelease() tea.Cmd {
	check := m.release.Check
	if check == nil {
		return nil
	}
	return func() tea.Msg { return releaseFound{check()} }
}

// foundRelease keeps what a look found. A look that found nothing, offline
// say, leaves a release already known in place.
func (m *Model) foundRelease(msg releaseFound) {
	if msg.version != "" {
		m.release.Version = msg.version
	}
}

// renderRelease is the status line while a newer release is known and no
// toast shows.
func (m Model) renderRelease() string {
	if m.release.Version == "" {
		return ""
	}
	how := m.st.subtle.Render(m.release.How)
	if m.release.Command {
		how = m.st.key.Render(m.release.How)
	}
	s := m.st.subtle.Render("recall ") + m.st.ok.Render(m.release.Version) + m.st.subtle.Render(" is available · ") + how
	return " " + ansi.Truncate(s, m.width-2, ellipsis)
}
