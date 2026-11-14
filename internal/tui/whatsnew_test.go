package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/babarot/claude-recall/internal/config"
	selfupdate "github.com/babarot/claude-recall/internal/update"
)

func testNotes() []selfupdate.Release {
	return []selfupdate.Release{
		{Version: "1.8.0", Date: "2026-10-12", Categories: []selfupdate.Category{{Name: "New Features", Entries: []string{"Add recall update to replace recall with the latest release (#75)"}}}},
		{Version: "1.7.2", Date: "2026-10-07", Categories: []selfupdate.Category{{Name: "Bug fixes", Entries: []string{"Do not let an import replace a newer one with stale content (#71)"}}}},
		{Version: "0.2.0", Date: "2026-09-29"},
	}
}

// notesModel is a model running 1.7.2 whose What's new fetches with fetch,
// counting the fetches.
func notesModel(t *testing.T, h int, fetch func(string) ([]selfupdate.Release, error)) (Model, *[]string) {
	t.Helper()
	m, _ := newTestModel(t, config.Default().TUI, 110, h)
	m.version = "1.7.2"
	var tags []string
	return m.NotesFrom(func(tag string) ([]selfupdate.Release, error) {
		tags = append(tags, tag)
		return fetch(tag)
	}), &tags
}

// openNotes presses w and delivers what the fetch returns.
func openNotes(t *testing.T, m Model) Model {
	t.Helper()
	cmd := m.openWhatsNew()
	if cmd == nil {
		return m
	}
	if !strings.Contains(screen(m), "Loading the release notes") {
		t.Errorf("no loading line:\n%s", screen(m))
	}
	return update(t, m, cmd())
}

func TestWhatsNew(t *testing.T) {
	m, tags := notesModel(t, 40, func(string) ([]selfupdate.Release, error) { return testNotes(), nil })
	m = m.TellOfRelease(Release{Version: "1.8.0", How: "recall update", Command: true})
	m = openNotes(t, m)
	if m.uiState() != uiWhatsNew {
		t.Fatalf("state %s", m.uiState())
	}
	s := screen(m)
	for _, want := range []string{
		"What's new",
		"1.8.0  2026-10-12  not installed · recall update",
		"New Features",
		"· Add recall update to replace recall with the latest release (#75)",
		"1.7.2  2026-10-07  installed",
		"0.2.0  2026-09-29",
		"Nothing listed",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("no %q:\n%s", want, s)
		}
	}
	if len(*tags) != 1 || (*tags)[0] != "1.8.0" {
		t.Errorf("fetched %v, want the newer release's", *tags)
	}

	// Closed by esc alone, not by w or q; open again without fetching
	// again.
	for _, k := range []string{"w", "q"} {
		if got := press(t, m, k).uiState(); got != uiWhatsNew {
			t.Errorf("%s closed it", k)
		}
	}
	m = press(t, m, "esc")
	if m.openWhatsNew() != nil {
		t.Error("fetched again")
	}
	if m.uiState() != uiWhatsNew || m.whatsNew.loading || len(*tags) != 1 {
		t.Errorf("reopened: state %s, loading %v, fetched %v", m.uiState(), m.whatsNew.loading, *tags)
	}
	// ? and the list's keys do nothing over it.
	if got := press(t, m, "?", "s").uiState(); got != uiWhatsNew {
		t.Errorf("? s: %s", got)
	}
}

func TestWhatsNewWithoutANewerRelease(t *testing.T) {
	m, tags := notesModel(t, 40, func(string) ([]selfupdate.Release, error) { return testNotes()[1:], nil })
	m = openNotes(t, m)
	if (*tags)[0] != "1.7.2" {
		t.Errorf("fetched %v, want the running release's", *tags)
	}
	if s := screen(m); strings.Contains(s, "not installed") || !strings.Contains(s, "1.7.2  2026-10-07  installed") {
		t.Errorf("marks:\n%s", s)
	}
}

func TestWhatsNewFailed(t *testing.T) {
	fail := true
	m, tags := notesModel(t, 40, func(string) ([]selfupdate.Release, error) {
		if fail {
			return nil, errors.New("no network")
		}
		return testNotes(), nil
	})
	m = openNotes(t, m)
	s := screen(m)
	if !strings.Contains(s, "Could not load the release notes: no network") || !strings.Contains(s, releasesPage) {
		t.Errorf("failure:\n%s", s)
	}
	// Opened again, it tries again.
	fail = false
	m = openNotes(t, press(t, m, "esc"))
	if len(*tags) != 2 || !strings.Contains(screen(m), "1.7.2  2026-10-07  installed") {
		t.Errorf("retry: fetched %v\n%s", *tags, screen(m))
	}
}

func TestWhatsNewScrolls(t *testing.T) {
	var many []selfupdate.Release
	for i := range 30 {
		many = append(many, selfupdate.Release{Version: "1.0." + string(rune('a'+i)), Date: "2026-01-01",
			Categories: []selfupdate.Category{{Name: "Bug fixes", Entries: []string{"Fix a thing (#1)"}}}})
	}
	m, _ := notesModel(t, 24, func(string) ([]selfupdate.Release, error) { return many, nil })
	m = openNotes(t, m)
	n := len(m.whatsNewBody(m.whatsNewInner()))
	// From row 2 down to above the status line and the footer, inside
	// the frame.
	rows := m.whatsNewRows()
	if rows != 24-2-4 {
		t.Errorf("rows %d, want %d", rows, 24-2-4)
	}
	if !strings.Contains(screen(m), fmt.Sprintf("1-%d/%d", rows, n)) {
		t.Fatalf("no position:\n%s", screen(m))
	}
	m = press(t, m, "j", "j")
	if m.whatsNew.offset != 2 {
		t.Errorf("j j: offset %d", m.whatsNew.offset)
	}
	m = press(t, m, "G")
	if m.whatsNew.offset != n-rows {
		t.Errorf("G: offset %d, want %d", m.whatsNew.offset, n-rows)
	}
	m = press(t, m, "j", "g")
	if m.whatsNew.offset != 0 {
		t.Errorf("g: offset %d", m.whatsNew.offset)
	}
	m = update(t, m, tea.MouseWheelMsg{X: 1, Y: 1, Button: tea.MouseWheelDown})
	if m.whatsNew.offset != 3 {
		t.Errorf("the wheel, anywhere: offset %d, want 3", m.whatsNew.offset)
	}
}

// A long entry wraps under itself rather than being cut.
func TestWhatsNewWraps(t *testing.T) {
	long := []selfupdate.Release{{Version: "1.7.2", Date: "2026-10-07", Categories: []selfupdate.Category{{Name: "Improvements",
		Entries: []string{"Give search results their repository, title and size; find words in Japanese and in every other language (#62)"}}}}}
	m, _ := notesModel(t, 40, func(string) ([]selfupdate.Release, error) { return long, nil })
	m = openNotes(t, m)
	body := ansi.Strip(strings.Join(m.whatsNewBody(m.whatsNewInner()), "\n"))
	if strings.Contains(body, "…") || !strings.Contains(body, "(#62)") || strings.Count(body, "\n") < 3 {
		t.Errorf("not wrapped:\n%s", body)
	}
}

// The first start after an update says so once; a first run, the same
// version and a downgrade say nothing. Each records the version.
func TestUpdatedToast(t *testing.T) {
	for _, c := range []struct {
		last  string
		toast bool
	}{
		{"", false},
		{"1.7.2", true},
		{"1.8.0", false},
		{"1.9.0", false},
	} {
		path := filepath.Join(t.TempDir(), "last_version")
		if c.last != "" {
			if err := config.SaveLastVersion(path, c.last); err != nil {
				t.Fatal(err)
			}
		}
		m, _ := newTestModel(t, config.Default().TUI, 110, 24)
		m.version = "1.8.0"
		m = m.RememberVersionIn(path)
		got := statusLine(m)
		if want := " Updated to 1.8.0 · ? w what's new"; (got == want) != c.toast {
			t.Errorf("last %q: status %q, toast %v", c.last, got, c.toast)
		}
		if c.toast && m.startToast != updatedToastFor {
			t.Errorf("last %q: shows for %v", c.last, m.startToast)
		}
		m.recordVersion()()
		if v := config.LoadLastVersion(path); v != "1.8.0" {
			t.Errorf("last %q: recorded %q", c.last, v)
		}
	}
}

// An update that added settings tells of them in its toast and at the top
// of What's new; a run that is no update tells of none.
func TestNewSettingsAfterAnUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "last_version")
	if err := config.SaveLastVersion(path, "1.7.2"); err != nil {
		t.Fatal(err)
	}
	m, _ := newTestModel(t, config.Default().TUI, 110, 24)
	m.version = "1.8.0"
	m = m.RememberVersionIn(path).TellOfNewSettings([]string{"tui.images", "tui.theme", "ui.port"})
	if got := statusLine(m); got != " Updated to 1.8.0 · 3 new settings · ? w what's new" {
		t.Errorf("toast %q", got)
	}
	m.whatsNew.notes = []selfupdate.Release{{Version: "1.8.0"}}
	body := ansi.Strip(strings.Join(m.whatsNewBody(70), "\n"))
	for _, want := range []string{"New settings in 1.8.0", "[tui] images, theme · recall config reference tui", "[ui] port · recall config reference ui"} {
		if !strings.Contains(body, want) {
			t.Errorf("What's new lacks %q:\n%s", want, body)
		}
	}

	// While the notes come, and when they could not, the settings show.
	m.whatsNew.notes, m.whatsNew.loading = nil, true
	if body := ansi.Strip(strings.Join(m.whatsNewBody(70), "\n")); !strings.Contains(body, "New settings in 1.8.0") {
		t.Errorf("not while loading:\n%s", body)
	}
	m.whatsNew.loading, m.whatsNew.err = false, errors.New("offline")
	if body := ansi.Strip(strings.Join(m.whatsNewBody(70), "\n")); !strings.Contains(body, "New settings in 1.8.0") || !strings.Contains(body, "offline") {
		t.Errorf("not when the notes could not be read:\n%s", body)
	}

	m, _ = newTestModel(t, config.Default().TUI, 110, 24)
	m = m.TellOfNewSettings([]string{"tui.images"})
	if len(m.newSettings) != 0 {
		t.Errorf("told of %v without an update", m.newSettings)
	}
}

// What's new opens from the key list only, on the same row, and esc goes
// back to the key list where it was scrolled to.
func TestWhatsNewOpensFromTheKeyList(t *testing.T) {
	m, _ := notesModel(t, 24, func(string) ([]selfupdate.Release, error) { return testNotes(), nil })
	if got := press(t, m, "w"); got.modal != modalNone {
		t.Fatalf("w on the list opened %v", got.uiState())
	}
	m = press(t, m, "?")
	if !strings.Contains(screen(m), "w what's new") {
		t.Errorf("the key list does not tell its key:\n%s", screen(m))
	}
	m = press(t, m, "j", "j")
	top, offset := m.modalTop, m.helpOffset
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	m = asModel(t, next)
	if cmd != nil {
		m = update(t, m, cmd())
	}
	if m.uiState() != uiWhatsNew || m.modalTop != top {
		t.Fatalf("w in the key list: %v on row %d, want what's new on row %d", m.uiState(), m.modalTop, top)
	}
	if m = press(t, m, "esc"); m.uiState() != uiHelp || m.modalTop != top || m.helpOffset != offset {
		t.Errorf("esc: %v on row %d at offset %d, want the key list on row %d at offset %d", m.uiState(), m.modalTop, m.helpOffset, top, offset)
	}
	if m = press(t, m, "esc"); m.modal != modalNone {
		t.Errorf("esc in the key list: %v", m.uiState())
	}
}

// whats_new is set under [keys] as before; turned off, nothing tells of
// it, and a key it shares with the key list's own is a conflict there.
func TestWhatsNewKeyIsSet(t *testing.T) {
	m, _ := newTestModel(t, config.Default().TUI, 140, 40)
	moved, err := m.WithKeys(config.Keys{"whats_new": {Keys: []string{"n"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := press(t, moved, "?", "n"); got.uiState() != uiWhatsNew {
		t.Errorf("? n: %v", got.uiState())
	}
	off, err := m.WithKeys(config.Keys{"whats_new": {Keys: []string{}}})
	if err != nil {
		t.Fatal(err)
	}
	if s := screen(press(t, off.TellOfRelease(Release{Version: "9.9.9", How: "recall update", Command: true}), "?")); strings.Contains(s, "what's new") {
		t.Errorf("turned off, still told:\n%s", s)
	}
	// Given down's j, it takes it in the key list, as any key given takes
	// it from the operations that have it by default.
	took, err := m.WithKeys(config.Keys{"whats_new": {Keys: []string{"j"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := press(t, took, "?", "j"); got.uiState() != uiWhatsNew {
		t.Errorf("? j with whats_new on j: %v", got.uiState())
	}
	// Set to the same key as down, both in the file, it is a conflict in
	// the key list, where both are read.
	if _, err := m.WithKeys(config.Keys{"whats_new": {Keys: []string{"x"}}, "down": {Keys: []string{"x"}}}); err == nil || !strings.Contains(err.Error(), "key list") {
		t.Errorf("whats_new and down on x: %v", err)
	}
	// It is not one in the panes, where whats_new is not read.
	if _, err := m.WithKeys(config.Keys{"whats_new": {Keys: []string{"x"}}, "sort": {Keys: []string{"x"}}}); err != nil {
		t.Errorf("whats_new and sort on x: %v", err)
	}
}
