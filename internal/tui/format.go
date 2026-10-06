package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/title"
	"github.com/babarot/claude-recall/internal/worktree"
)

// row is a session with everything the list shows, formatted once at load.
type row struct {
	s db.Session

	title    string
	folder   string // repo name, or the main checkout's name for a worktree
	worktree string // worktree name, empty outside a linked worktree
	gone     bool   // the session directory no longer exists
	mainRoot string // main checkout of a linked worktree
	// noTranscript is set when Claude Code has deleted the session's JSONL
	// transcript, which claude -r reads.
	noTranscript bool
	// transcriptDir is the extra tree (extra_projects_dirs) the transcript
	// is in, which claude -r does not read; empty for the primary tree.
	transcriptDir string
	// group is the folder the list narrows by: the repository, worktrees
	// included, or the directory outside git; groupName is how it shows.
	group, groupName string

	search string // lower-cased text the filter matches against
}

// resumable reports whether claude -r can resume the session: its folder
// and its transcript are both still there, the transcript where claude -r
// reads it.
func (r *row) resumable() bool { return !r.gone && !r.noTranscript && r.transcriptDir == "" }

// removed reports whether the session's folder is shown as removed. A
// folder missing for a session whose transcript is in another tree, as a
// container's, was most likely never on this host.
func (r *row) removed() bool { return r.gone && r.transcriptDir == "" }

func newRow(s db.Session, home string, wt *worktree.Resolver) row {
	r := row{s: s, title: displayTitle(s)}
	repo := wt.Repo(s.ProjectPath, home)
	r.gone = !repo.Exists
	r.mainRoot, r.folder, r.worktree = repo.MainRoot, repo.Folder, repo.Worktree
	r.group, r.groupName = repo.Key, repo.Name
	r.search = strings.ToLower(strings.Join([]string{r.title, r.folder, r.worktree, s.GitBranch, s.ID}, " "))
	return r
}

// tildePath shortens $HOME to ~ for display.
func tildePath(path, home string) string {
	if home != "" {
		if path == home {
			return "~"
		}
		if rest, ok := strings.CutPrefix(path, home+"/"); ok {
			return "~/" + rest
		}
	}
	return path
}

// displayTitle picks what the list shows for a session.
func displayTitle(s db.Session) string { return title.Display(s.Title, s.FirstPrompt) }

// formatEnded shows a time of day for today, a date and time this year, and
// a full date otherwise.
func formatEnded(t, now time.Time) string {
	if t.IsZero() {
		return "-"
	}
	t, now = t.Local(), now.Local()
	switch {
	case t.Year() == now.Year() && t.YearDay() == now.YearDay():
		return t.Format("15:04")
	case t.Year() == now.Year():
		return t.Format("01/02 15:04")
	default:
		return t.Format("2006/01/02")
	}
}

// relativeDate is how the list shows when a session ended, as cc360 does:
// "Today 15:04", "Yesterday", "3d ago", "Sep 28", "2025-12-31".
func relativeDate(t, now time.Time) string {
	if t.IsZero() {
		return "-"
	}
	t, now = t.Local(), now.Local()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	days := int(today.Sub(day).Hours() / 24)
	switch {
	case days <= 0:
		return "Today " + t.Format("15:04")
	case days == 1:
		return "Yesterday"
	case days < 7:
		return fmt.Sprintf("%dd ago", days)
	case t.Year() == now.Year():
		return t.Format("Jan _2")
	default:
		return t.Format("2006-01-02")
	}
}

func formatSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%dK", n>>10)
	default:
		return fmt.Sprintf("%dB", n)
	}
}
