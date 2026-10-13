package theme

// The web UI uses the TUI's schemes too. A palette has no background, which
// a terminal gives the TUI, so each scheme also names the backgrounds the
// page is drawn on. The ansi scheme is the terminal's own colors, which a
// page cannot know: it, like Auto, leaves the page to its own catppuccin
// pair.

// web is what a scheme adds for the web UI: whether it is dark, its
// backgrounds (the page, the side columns and modals, raised rows, rules),
// and colors that read better on the page than the TUI's, "" for the same.
type web struct {
	dark                      bool
	bg, mantle, surface, line string
	prompt, worktree          string
}

var webSchemes = map[string]web{
	"catppuccin-mocha": {dark: true, bg: "#1e1e2e", mantle: "#181825", surface: "#2a2a3d", line: "#313147"},
	// Latte's yellow and orange are too light for text on its white.
	"catppuccin-latte": {bg: "#eff1f5", mantle: "#e6e9ef", surface: "#e3e6ed", line: "#d5d8e0", prompt: "#b8720f", worktree: "#d9560a"},
	"tokyo-night":      {dark: true, bg: "#1a1b26", mantle: "#16161e", surface: "#1f2335", line: "#24283b"},
	"dracula":          {dark: true, bg: "#282a36", mantle: "#21222c", surface: "#343746", line: "#3a3c4e"},
	"nord":             {dark: true, bg: "#2e3440", mantle: "#272c36", surface: "#353c4a", line: "#3a4150"},
	"gruvbox-dark":     {dark: true, bg: "#282828", mantle: "#1d2021", surface: "#32302f", line: "#363230"},
}

// WebColors returns the CSS custom properties of a scheme for the web UI,
// named as the page's stylesheet names them, and whether the scheme is
// dark. ok is false for Auto, ansi and unknown names, which the page draws
// with its own colors.
func WebColors(name string) (vars map[string]string, dark, ok bool) {
	w, ok := webSchemes[name]
	if !ok {
		return nil, false, false
	}
	p := palettes[name]
	prompt, worktree := p.Prompt, p.Worktree
	if w.prompt != "" {
		prompt = w.prompt
	}
	if w.worktree != "" {
		worktree = w.worktree
	}
	markText := "#ffffff"
	if w.dark {
		markText = w.bg
	}
	return map[string]string{
		"bg": w.bg, "mantle": w.mantle, "surface": w.surface, "surface-hi": p.Surface,
		"line": w.line, "border": p.Border,
		"text": p.Text, "strong": p.Strong, "subtle": p.Subtle, "muted": p.Muted, "dim": p.Dim,
		"accent": p.Accent, "title": p.Title, "ok": p.OK, "prompt": prompt, "worktree": worktree, "warn": p.Warn,
		"mark-cur": prompt, "mark-cur-text": markText,
	}, w.dark, true
}
