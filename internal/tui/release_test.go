package tui

import (
	"strings"
	"testing"

	"github.com/babarot/claude-recall/internal/config"
)

// statusLine is the line above the footer.
func statusLine(m Model) string {
	lines := strings.Split(screen(m), "\n")
	return lines[len(lines)-2]
}

func TestReleaseNotice(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 110, 24)
	if got := statusLine(m); strings.TrimSpace(got) != "" {
		t.Errorf("no release: %q", got)
	}

	m = m.TellOfRelease(Release{Version: "1.8.0", How: "recall update", Command: true})
	if got, want := statusLine(m), " recall 1.8.0 is available · recall update"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// A toast takes the line while it shows, and the notice comes back.
	m.showToast(toastOK, "Copied session ID aaaaaaaa-1111")
	if got := statusLine(m); !strings.Contains(got, "Copied session ID") {
		t.Errorf("toast: %q", got)
	}
	m = update(t, m, toastExpired{m.toastID})
	if got := statusLine(m); !strings.Contains(got, "1.8.0 is available") {
		t.Errorf("after the toast: %q", got)
	}
}

func TestReleaseCheck(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 110, 24)
	m = m.TellOfRelease(Release{How: "update it with Nix", Check: func() string { return "1.8.0" }})
	msg := m.checkRelease()()
	m = update(t, m, msg)
	if got, want := statusLine(m), " recall 1.8.0 is available · update it with Nix"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// A look that finds nothing leaves the known release.
	m = update(t, m, releaseFound{})
	if got := statusLine(m); !strings.Contains(got, "1.8.0") {
		t.Errorf("after an empty look: %q", got)
	}
	if (Model{}).checkRelease() != nil {
		t.Error("no Check, no look")
	}
}
