package tui

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// ? shows every key, grouped by where it works, over the screen; ?, Esc or
// q closes it. The group of the pane it was opened from comes first, marked
// as the selected row is, and keys that do not work there are muted.

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
// conversation spread, the list's j k read the next session, which the
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

// helpLine is a line of the key list's body; here lines carry the selected
// row's bar and background.
type helpLine struct {
	text string
	here bool
}

// helpBody is the key list's lines for the focused pane.
func (m Model) helpBody(inner int) []helpLine {
	c := m.keyContext()
	var body []helpLine
	for i, g := range helpOrder(c) {
		if i > 0 {
			body = append(body, helpLine{})
		}
		here := i == 0
		on := func(s lipgloss.Style) lipgloss.Style { return m.st.on(s, here) }
		works := m.works(g, c)
		title := strings.ToUpper(g.title)
		switch {
		case here:
			title = on(m.st.subtle.Bold(true)).Render(title) + on(m.st.text).Render("  ") + m.st.helpHere.Render(" here ")
		case !works && g.title == helpGroups[0].title:
			title = m.section(g.title) + m.st.muted.Render("  bright rows work here too")
		case works:
			title = m.section(g.title)
		default:
			title = m.st.muted.Render(title)
		}
		body = append(body, helpLine{title, here})
		for _, r := range g.rows {
			keys := m.hintKeys(r.keys)
			if keys == "" {
				continue // remapped away
			}
			ks, ws := on(m.st.key), on(m.st.subtle)
			if !here && !r.worksIn(g, c) && !works {
				ks, ws = m.st.muted, m.st.muted
			}
			key := ks.Render(keys) + on(m.st.text).Render(strings.Repeat(" ", max(1, helpKeyWidth-ansi.StringWidth(keys))))
			body = append(body, helpLine{ansi.Truncate(key+ws.Render(m.hintKeys(r.what)), inner, ellipsis), here})
		}
	}
	return body
}

// helpBox draws the key list in a rounded box at most w cells wide and h
// lines tall.
func (m Model) helpBox(w, h int) []string {
	inner := w - 4
	b := m.st.rule
	body := m.helpBody(inner)
	if len(body) > h-2 {
		body = append(body[:max(0, h-3)], helpLine{text: m.st.muted.Render("… a taller terminal shows the rest")})
	}
	title := " Keys "
	hint := " " + m.hintKeys("{help.0} esc q close") + " "
	fill := max(0, w-4-len(title)-ansi.StringWidth(hint))
	out := []string{b.Render("╭─") + m.st.key.Render(title) + b.Render(strings.Repeat("─", fill)) + m.st.muted.Render(hint) + b.Render("─╮")}
	for _, l := range body {
		pad := strings.Repeat(" ", max(0, inner-ansi.StringWidth(l.text)))
		if l.here {
			out = append(out, b.Render("│")+m.st.bar.Render("▎")+l.text+m.st.selected.Render(pad)+b.Render(" │"))
			continue
		}
		out = append(out, b.Render("│ ")+l.text+pad+b.Render(" │"))
	}
	return append(out, b.Render("╰"+strings.Repeat("─", w-2)+"╯"))
}

// withHelp lays the key list over the screen, centered.
func (m Model) withHelp(screen string) string {
	lines := strings.Split(screen, "\n")
	w := min(64, m.width-2)
	if w < 30 {
		return screen
	}
	box := m.helpBox(w, m.height-2)
	top := max(0, (len(lines)-len(box))/2)
	x := (m.width - w) / 2
	for i, l := range box {
		if j := top + i; j < len(lines) {
			lines[j] = overlay(lines[j], l, x)
		}
	}
	return strings.Join(lines, "\n")
}
