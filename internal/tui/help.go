package tui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// ? shows every key, grouped by where it works, over the screen; Esc
// closes it. It tells the version at its top, and a newer release with how
// to update to it. The group of the pane it was opened from comes first;
// each group's header says whether its keys work there, and keys that do
// not are faint. It scrolls when the terminal is too short for it.

// helpRow is a key hint template (see hintKeys) and what it does, which may
// name keys the same way, with the panes it works in (nil: its group's).
type helpRow struct {
	keys, what string
	in         []keyContext
}

type helpGroup struct {
	title string
	in    []keyContext // where its keys work
	rows  []helpRow
}

var (
	everywhere   = []keyContext{ctxList, ctxFrame, ctxReading, ctxFolders}
	sessionPanes = []keyContext{ctxList, ctxFrame, ctxReading}
	// Keys the reading group tells its own way are left out of reading.
	notReading = []keyContext{ctxList, ctxFrame, ctxFolders}
)

// move is the key hint of the four arrows' keys, as ↑ ↓  j k.
const move = "{up.0} {down.0}  {down.1} {up.1}"

// helpGroups are the key list's rows. Rows that are not keys (the filter's
// syntax) and fixed keys (esc, a field's own keys) are written as they are.
var helpGroups = []helpGroup{
	{"Sessions", []keyContext{ctxList}, []helpRow{
		{move, "move; {top.1} {bottom.1} top and bottom, {page_up.0} {page_down.0} by page", nil},
		{"{resume.0}", "resume the session", sessionPanes},
		{"{recall.0}", "recall it in a new claude, through recall's MCP server", sessionPanes},
		{"{read.0}", "read the conversation over the pane", []keyContext{ctxList, ctxFrame}},
		{"{copy_id.0}  {copy_command.0}", "copy the session ID, the resume command (or the recall one)", sessionPanes},
		{"{search.0}", "filter (see below)", []keyContext{ctxList, ctxFrame}},
		{"{ask.0}", "ask Claude to find sessions (claude -p)", everywhere},
		{"{sort.0}", "choose the sort order", everywhere},
		{"{scope.0}", "this folder or all folders", everywhere},
		{"{whats_new.0}", "what's new in recall: the release notes", everywhere},
		{"{list.folders_open.0} {list.folders_open.1}", "open the folder list, then move into it", nil},
		{"{list.folders_close.0} {list.folders_close.1}", "close the folder list", nil},
		{"{focus_next.0}  {focus_prev.0}", "next or previous frame ({focus_prev.1} and {focus_next.1} too)", notReading},
		{"{grow.0} {shrink.0}", "resize the detail pane (or drag its edge)", []keyContext{ctxList, ctxFrame}},
		{"{quit.0}", "quit", notReading},
	}},
	{"Folder list", []keyContext{ctxFolders}, []helpRow{
		{move, "pick a folder; the sessions follow", nil},
		{"{search.0}", "search folders by fuzzy match", nil},
		{"esc", "clear the search, then back to the sessions", nil},
		{"{folders.back.0} {folders.back.1} {folders.back.2}", "back to the sessions", nil},
	}},
	{"Detail frames", []keyContext{ctxFrame}, []helpRow{
		{move, "scroll; {top.1} {bottom.1}, {page_up.0} {page_down.0} too", nil},
		{"esc", "back to the sessions", nil},
	}},
	// The filter opens on / from the list and from a frame.
	{"Filter", []keyContext{ctxList, ctxFrame}, []helpRow{
		{"words", "title, folder, branch, ID or what was said", nil},
		{"folder:  in:", "folder, fuzzy (folder:bdot)", nil},
		{"text:", "only what was said", nil},
		{"title: branch:", "only that field", nil},
		{"worktree: id:", "worktree name, start of the ID", nil},
		{"tab  →", "complete a key or a suggested value", nil},
		{"↑ ↓  enter", "pick a suggestion", nil},
		{"esc", "close suggestions, then clear the filter", nil},
	}},
	{"Reading (space)", []keyContext{ctxReading}, []helpRow{
		{move, "scroll; {top.1} {bottom.1}, {page_up.0} {page_down.0} too", nil},
		{"{focus_next.0}", "to the sessions: {down.1} {up.1} read the next one", nil},
		{"{grow.0} {shrink.0}", "more or fewer session rows (or drag)", nil},
		{"{search.0}", "search the conversation", nil},
		{"{next_match.0} {prev_match.0}", "next, previous match; esc clears", nil},
		{"{read.0} {quit.0} esc", "put the pane back", nil},
	}},
}

func (r helpRow) worksIn(g helpGroup, c keyContext) bool {
	if r.in != nil {
		return slices.Contains(r.in, c)
	}
	return slices.Contains(g.in, c)
}

// works reports whether group g's keys work in context c; with the
// reading view open, the list's j k read the next session, which the
// reading group tells.
func (m Model) works(g helpGroup, c keyContext) bool {
	return slices.Contains(g.in, c) || (c == ctxList && m.expanded && slices.Contains(g.in, ctxReading))
}

// helpOrder is helpGroups for context c: the group of c first, then
// Sessions (whose rows work from most panes), then the rest as they are.
func helpOrder(c keyContext) []helpGroup {
	here := 0
	for i, g := range helpGroups {
		if slices.Contains(g.in, c) {
			here = i
			break
		}
	}
	out := []helpGroup{helpGroups[here]}
	if here != 0 {
		out = append(out, helpGroups[0])
	}
	for i, g := range helpGroups {
		if i != here && i != 0 {
			out = append(out, g)
		}
	}
	return out
}

const helpKeyWidth = 15

// homepage is where recall lives, told at the top of the key list.
const homepage = "github.com/babarot/claude-recall"

// helpWidth is the widest the key list gets.
const helpWidth = 64

// helpTop are the key list's first lines, right-aligned: the version and
// the homepage, and a newer release with how to update to it.
func (m Model) helpTop(inner int) []string {
	right := func(s string) string {
		s = ansi.Truncate(s, inner, ellipsis)
		return strings.Repeat(" ", max(0, inner-ansi.StringWidth(s))) + s
	}
	out := []string{right(m.st.dim.Render("recall " + cmp.Or(m.version, "dev") + " · " + homepage))}
	if v := m.release.Version; v != "" {
		how := m.st.subtle.Render(m.release.How)
		if m.release.Command {
			how = m.st.key.Render(m.release.How)
		}
		out = append(out, right(m.st.ok.Render(v+" is out")+m.st.dim.Render(" · ")+how))
	}
	return out
}

// helpBody is every line of the key list, inner cells wide, for the
// focused pane: its top lines, then the groups, each under a header that
// rules off the rest of its line.
func (m Model) helpBody(inner int) []string {
	c := m.keyContext()
	body := append(m.helpTop(inner), "")
	for i, g := range helpOrder(c) {
		if i > 0 {
			body = append(body, "")
		}
		works := m.works(g, c)
		some := slices.ContainsFunc(g.rows, func(r helpRow) bool { return r.worksIn(g, c) })
		var head string
		switch {
		case works:
			head = m.st.id.Bold(true).Render(g.title) + m.st.dim.Render(" · works here")
		case some:
			head = m.st.muted.Bold(true).Render(g.title) + m.st.dim.Render(" · bright rows work here")
		default:
			head = m.st.muted.Bold(true).Render(g.title)
		}
		head += " " + m.st.rule.Render(strings.Repeat("─", max(0, inner-ansi.StringWidth(head)-1)))
		body = append(body, head)
		for _, r := range g.rows {
			keys := m.hintKeys(r.keys)
			if keys == "" {
				continue // remapped away
			}
			ks, ws := m.st.key, m.st.subtle
			if !works && !r.worksIn(g, c) {
				ks, ws = m.st.muted, m.st.dim
			}
			key := "  " + ks.Render(keys) + strings.Repeat(" ", max(1, helpKeyWidth-ansi.StringWidth(keys)))
			body = append(body, ansi.Truncate(key+ws.Render(m.hintKeys(r.what)), inner, ellipsis))
		}
	}
	return body
}

// helpInner is how wide the key list's lines are on this terminal.
func (m Model) helpInner() int { return m.boxWidth(helpSpec().box) - 4 }

// helpShown is the part of the key list the box shows, rows at a time
// from where it is scrolled to, where that part starts, and how many lines
// there are.
func (m Model) helpShown(inner, rows int) (shown []string, off, total int) {
	body := m.helpBody(inner)
	off = max(0, min(m.helpOffset, len(body)-rows))
	return body[off:min(len(body), off+rows)], off, len(body)
}

func (m *Model) scrollHelp(step int) {
	n := len(m.helpBody(m.helpInner()))
	m.helpOffset = max(0, min(m.helpOffset+step, n-max(1, m.boxRows())))
}

// helpSpec declares the key list: a body to scroll, and esc.
func helpSpec() modalSpec {
	return modalSpec{
		body: &modalBody{
			scroll: func(m *Model, step int) { m.scrollHelp(step) },
			page:   func(m Model) int { return max(1, m.boxRows()-1) },
		},
		esc: closeKey(func(m *Model) { m.closeModal() }),
		box: &modalBox{
			title: func(Model) string { return "Keys" },
			width: helpWidth, minWidth: 30,
			click: clickCloses,
			lines: func(m Model, inner, rows int) []boxLine {
				shown, _, _ := m.helpShown(inner, rows)
				out := make([]boxLine, len(shown))
				for i, l := range shown {
					out[i] = boxLine{text: l}
				}
				return out
			},
			// Where the part shown is in the list, when it is longer.
			bottom: func(m Model, inner, rows int) string {
				shown, off, total := m.helpShown(inner, rows)
				if total <= len(shown) {
					return ""
				}
				return fmt.Sprintf(" %d-%d/%d ", off+1, off+len(shown), total)
			},
		},
	}
}
