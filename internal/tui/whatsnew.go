package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/babarot/claude-recall/internal/config"
	"github.com/babarot/claude-recall/internal/logging"
	selfupdate "github.com/babarot/claude-recall/internal/update"
)

// releasesPage is where the release notes are when they cannot be loaded.
const releasesPage = "https://github.com/babarot/claude-recall/releases"

// whatsNewWidth is the widest the What's new box gets.
const whatsNewWidth = 76

// updatedToastFor is how long the toast after an update shows: longer than
// other toasts, since it is read once and its key is the point.
const updatedToastFor = 6 * time.Second

// whatsNewState is the What's new modal: the release notes from CHANGELOG.md,
// fetched the first time it opens and kept for the run.
type whatsNewState struct {
	open    bool
	loading bool
	// tag is the release the notes were fetched at, and notes what came,
	// or err why nothing did.
	tag    string
	notes  []selfupdate.Release
	err    error
	offset int
}

type whatsNewLoaded struct {
	tag   string
	notes []selfupdate.Release
	err   error
}

// NotesFrom has the What's new modal fetch CHANGELOG.md as of a tag with
// fetch, which runs in the background.
func (m Model) NotesFrom(fetch func(tag string) ([]selfupdate.Release, error)) Model {
	m.fetchNotes = fetch
	return m
}

// RememberVersionIn records this run's version in the file at path, and
// when the last run's was older, says once that recall was updated.
func (m Model) RememberVersionIn(path string) Model {
	m.versionPath = path
	last := config.LoadLastVersion(path)
	if last == "" {
		return m // a first run, or one from before the file
	}
	if newer, err := selfupdate.Newer(m.version, last); err != nil || !newer {
		return m
	}
	// Rendered when drawn, so it takes the colors the terminal's
	// background picks.
	v := m.version
	m.updated = true
	m.toast, m.toastKind = "Updated to "+v, toastOK
	m.toastRender = func(m Model) string {
		s := m.st.ok.Render("Updated to " + v)
		if n := len(m.newSettings); n > 0 {
			s += m.st.subtle.Render(" · ") + m.st.ok.Render(plural(n, "new setting"))
		}
		if k := m.hintKeys("{whats_new.0}"); k != "" {
			s += m.st.subtle.Render(" · ") + m.st.key.Render(k) + m.st.subtle.Render(" what's new")
		}
		return s
	}
	m.toastID++
	m.startToast = updatedToastFor
	return m
}

// startCmds are what Init starts besides the terminal queries: the look
// for a newer release, the version record and the toast after an update.
func (m Model) startCmds() tea.Cmd {
	cmds := []tea.Cmd{m.checkRelease(), m.recordVersion()}
	if m.startToast > 0 {
		id := m.toastID
		cmds = append(cmds, tea.Tick(m.startToast, func(time.Time) tea.Msg { return toastExpired{id} }))
	}
	return tea.Batch(cmds...)
}

// recordVersion writes this run's version for the next run to compare.
func (m Model) recordVersion() tea.Cmd {
	path, v, log := m.versionPath, m.version, m.logger()
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		if err := config.SaveLastVersion(path, v); err != nil {
			log.Warn("the version could not be kept", logging.KeyPath, path, logging.KeyErr, err.Error())
		}
		return nil
	}
}

// notesTag is the release whose CHANGELOG.md has every release worth
// showing: a newer one when it is known, else the running one.
func (m Model) notesTag() string {
	if m.release.Version != "" {
		return m.release.Version
	}
	return m.version
}

func (m *Model) openWhatsNew() tea.Cmd {
	w := &m.whatsNew
	w.open, w.offset = true, 0
	tag := m.notesTag()
	if w.loading || (w.notes != nil && w.tag == tag) || m.fetchNotes == nil {
		return nil
	}
	w.loading, w.err = true, nil
	fetch := m.fetchNotes
	return func() tea.Msg {
		notes, err := fetch(tag)
		return whatsNewLoaded{tag, notes, err}
	}
}

func (m *Model) whatsNewLoaded(msg whatsNewLoaded) {
	w := &m.whatsNew
	w.loading = false
	if msg.err != nil {
		w.err = msg.err
		m.logger().Warn("the release notes could not be read", logging.KeyErr, msg.err.Error())
		return
	}
	w.tag, w.notes, w.err = msg.tag, msg.notes, nil
}

func (m *Model) scrollWhatsNew(step int) {
	n := len(m.whatsNewBody(m.whatsNewInner()))
	m.whatsNew.offset = max(0, min(m.whatsNew.offset+step, n-m.whatsNewRows()))
}

func (m Model) whatsNewWidth() int { return min(whatsNewWidth, m.width-2) }

func (m Model) whatsNewInner() int { return m.whatsNewWidth() - 4 }

// whatsNewRows is how many lines of notes the box shows at once.
func (m Model) whatsNewRows() int { return max(1, m.height-4) }

// whatsNewBody is every line of the box's content, at inner columns.
func (m Model) whatsNewBody(inner int) []string {
	w := m.whatsNew
	// The settings the update added are known here, so they show whether
	// the release notes have come, are coming or could not.
	out := m.newSettingsLines()
	switch {
	case w.loading:
		return append(out, m.st.muted.Render("Loading the release notes…"))
	case w.err != nil && w.notes == nil:
		return append(out,
			m.st.warn.Render("Could not load the release notes: "+w.err.Error()),
			"",
			m.st.muted.Render("They are also at ")+m.st.id.Render(releasesPage),
		)
	}
	for i, r := range w.notes {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, m.st.title.Render(r.Version)+"  "+m.st.dim.Render(r.Date)+m.releaseMark(r.Version))
		if len(r.Categories) == 0 {
			out = append(out, "  "+m.st.muted.Render("Nothing listed"))
		}
		for _, c := range r.Categories {
			out = append(out, "  "+m.st.strong.Render(c.Name))
			for _, e := range c.Entries {
				out = append(out, m.noteEntry(e, inner)...)
			}
		}
	}
	return out
}

// releaseMark marks a release newer than the running one as not
// installed, with how to update, and the running one as installed.
func (m Model) releaseMark(v string) string {
	if v == m.version {
		return "  " + m.st.subtle.Render("installed")
	}
	if newer, err := selfupdate.Newer(v, m.version); err != nil || !newer {
		return ""
	}
	mark := "  " + m.st.ok.Render("not installed")
	if how := m.release.How; how != "" {
		style := m.st.subtle
		if m.release.Command {
			style = m.st.key
		}
		mark += m.st.subtle.Render(" · ") + style.Render(how)
	}
	return mark
}

// noteEntry is an entry, "· title (#75)", wrapped under itself so nothing
// is cut, with the pull request's number faint.
func (m Model) noteEntry(e string, inner int) []string {
	num := ""
	if i := strings.LastIndex(e, " (#"); i >= 0 {
		num = e[i:]
	}
	lines := strings.Split(ansi.Wordwrap(e, max(10, inner-4), ""), "\n")
	out := make([]string, len(lines))
	for i, l := range lines {
		lead := "  " + m.st.dim.Render("·") + " "
		if i > 0 {
			lead = "    "
		}
		if i == len(lines)-1 && num != "" && strings.HasSuffix(l, num) {
			out[i] = lead + m.st.text.Render(strings.TrimSuffix(l, num)) + m.st.dim.Render(num)
		} else {
			out[i] = lead + m.st.text.Render(l)
		}
	}
	return out
}

// withWhatsNew lays the box over the screen, centered. A terminal too
// narrow for it leaves it undrawn; the footer says how out.
func (m Model) withWhatsNew(screen string) string {
	w := m.whatsNewWidth()
	if w < 30 {
		return screen
	}
	inner := w - 4
	body := m.whatsNewBody(inner)
	rows := min(len(body), m.whatsNewRows())
	off := max(0, min(m.whatsNew.offset, len(body)-rows))
	shown := body[off : off+rows]

	b := m.st.rule
	title := " What's new "
	hint := " " + m.frameHint(whatsNewSpec) + " "
	fill := max(0, w-4-len(title)-ansi.StringWidth(hint))
	box := []string{b.Render("╭─") + m.st.key.Render(title) + b.Render(strings.Repeat("─", fill)) + m.st.muted.Render(hint) + b.Render("─╮")}
	for _, l := range shown {
		l = ansi.Truncate(l, inner, ellipsis)
		box = append(box, b.Render("│ ")+l+strings.Repeat(" ", max(0, inner-ansi.StringWidth(l)))+b.Render(" │"))
	}
	pos := ""
	if len(body) > rows {
		pos = fmt.Sprintf(" %d-%d/%d ", off+1, off+rows, len(body))
	}
	box = append(box, b.Render("╰"+strings.Repeat("─", max(0, w-3-len(pos))))+m.st.muted.Render(pos)+b.Render("─╯"))

	lines := strings.Split(screen, "\n")
	top := max(0, (len(lines)-len(box))/2)
	x := (m.width - w) / 2
	for i, l := range box {
		if j := top + i; j < len(lines) {
			lines[j] = overlay(lines[j], l, x)
		}
	}
	return strings.Join(lines, "\n")
}

// TellOfNewSettings has an update tell of the settings it added, in the
// toast after it and at the top of What's new, for this run. A run that
// is not an update tells of nothing: a build from source adds settings
// without a release to tell of.
func (m Model) TellOfNewSettings(names []string) Model {
	if m.updated {
		m.newSettings = names
	}
	return m
}

// newSettingsLines head What's new with the settings the update added, a
// line a table, with the command that describes them.
func (m Model) newSettingsLines() []string {
	if len(m.newSettings) == 0 {
		return nil
	}
	out := []string{m.st.title.Render("New settings in " + m.version)}
	byTable := map[string][]string{}
	var tables []string
	for _, n := range m.newSettings {
		table, key, _ := strings.Cut(n, ".")
		if _, ok := byTable[table]; !ok {
			tables = append(tables, table)
		}
		byTable[table] = append(byTable[table], key)
	}
	for _, t := range tables {
		out = append(out, "  "+m.st.strong.Render("["+t+"] "+strings.Join(byTable[t], ", "))+
			m.st.muted.Render(" · recall config reference "+t))
	}
	return append(out, "")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
