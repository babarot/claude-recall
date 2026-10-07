// Package importer copies Claude Code transcripts into the archive. Each
// import mirrors the file's current content: a changed file replaces every
// stored row of its session, so appends, /compact rewrites and interrupted
// earlier imports all resolve the same way.
package importer

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/babarot/claude-recall/internal/config"
	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/parser"
)

// Status says what an import did.
type Status string

const (
	New       Status = "new"
	Resynced  Status = "resynced"
	Unchanged Status = "unchanged"
)

// Result is the outcome of importing one file.
type Result struct {
	Status        Status
	SessionID     string
	Project       string
	TotalMessages int
}

// afterRead, when set by a test, is called once ImportFile has read the
// file, to change it or import it again before the read content is stored.
var afterRead func()

// importAttempts bounds how often ImportFile reads a file again that
// changed while it was being imported.
const importAttempts = 3

var (
	// errChanged is the file changing between its stat and the write.
	errChanged = errors.New("transcript changed while it was imported")
	// errSuperseded is another copy of the session, in another tree, being
	// the one to import by then, or the file gone.
	errSuperseded = errors.New("transcript is no longer the copy to import")
)

// ImportFile imports <projects>/<project>/<session>.jsonl, a transcript of
// the trees dirs. It returns nil, nil when the file cannot be read or holds
// no session, so callers can try again later.
//
// Several imports can run at once, from the watcher, the catch-up import
// and the SessionEnd hook, and one that read the file before it changed
// must not replace what one that read it after stored. So the rows are
// written only if, with the write lock held, the file is still the copy of
// the session parser.Choose picks among the trees, as it was read. A file
// that changed is read again, up to importAttempts times; a copy elsewhere
// that became the one to import is left to the watcher, which sees it
// change. With no dirs, the file's own tree is used.
func ImportFile(d *db.DB, path string, index *parser.IndexEntry, dirs []string) (*Result, error) {
	for range importAttempts {
		r, err := importOnce(d, path, index, dirs)
		switch {
		case errors.Is(err, errChanged):
			continue
		case errors.Is(err, errSuperseded):
			return nil, nil
		}
		return r, err
	}
	return nil, nil
}

func importOnce(d *db.DB, path string, index *parser.IndexEntry, dirs []string) (*Result, error) {
	sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	project := filepath.Base(filepath.Dir(path))
	if sessionID == "" || project == "" {
		return nil, nil
	}

	fi, err := os.Stat(path)
	if err != nil {
		return nil, nil
	}
	// JavaScript Dates hold whole milliseconds.
	mtime := float64(fi.ModTime().UnixMilli())
	size := fi.Size()

	existing, err := d.GetFileInfo(sessionID)
	if err != nil {
		return nil, err
	}
	// A session without a stored title predates titles: import it again
	// once to fill the title in.
	if existing != nil && existing.HasTitle && existing.FileMtime != nil && *existing.FileMtime == mtime &&
		existing.FileSize != nil && *existing.FileSize == size {
		return &Result{Status: Unchanged, SessionID: sessionID, Project: project, TotalMessages: existing.MessageCount}, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	if afterRead != nil {
		afterRead()
	}
	parsed := parser.Parse(decodeText(raw), project, index)
	if parsed == nil {
		return nil, nil
	}

	row := db.SessionRow{
		SessionID:     sessionID,
		Project:       parsed.Meta.Project,
		ProjectPath:   parsed.Meta.ProjectPath,
		GitBranch:     parsed.Meta.GitBranch,
		FirstPrompt:   parsed.Meta.FirstPrompt,
		Summary:       parsed.Meta.Summary,
		MessageCount:  len(parsed.Messages),
		StartedAt:     parsed.Meta.StartedAt,
		EndedAt:       parsed.Meta.EndedAt,
		ClaudeVersion: parsed.Meta.ClaudeVersion,
		FileMtime:     &mtime,
		FileSize:      &size,
		Title:         parsed.Meta.Title,
	}
	msgs := make([]db.MessageRow, len(parsed.Messages))
	for i, m := range parsed.Messages {
		msgs[i] = db.MessageRow{UUID: m.UUID, Role: m.Role, BlockType: m.BlockType, BlockIndex: m.BlockIndex,
			Content: m.Content, ToolName: m.ToolName, ToolInput: m.ToolInput, Timestamp: m.Timestamp, TurnIndex: m.TurnIndex}
	}
	var imgs []db.ImageRow
	for _, img := range parsed.Images {
		data, err := atob(img.Data)
		if err != nil {
			continue // an unreadable image does not fail the session
		}
		imgs = append(imgs, db.ImageRow{MessageUUID: img.MessageUUID, ImageIndex: img.ImageIndex, MediaType: img.MediaType, Data: data})
	}

	if len(dirs) == 0 {
		dirs = []string{filepath.Dir(filepath.Dir(path))}
	}
	check := func() error {
		copies := parser.FindSession(dirs, sessionID)
		if len(copies) == 0 {
			return errSuperseded
		}
		pick := parser.Choose(copies)[0]
		if filepath.Clean(pick.Path) != filepath.Clean(path) {
			return errSuperseded
		}
		if !pick.ModTime.Equal(fi.ModTime()) || pick.Size != fi.Size() {
			return errChanged
		}
		return nil
	}
	if err := d.ReplaceSession(row, msgs, imgs, check); err != nil {
		return nil, err
	}
	status := New
	if existing != nil {
		status = Resynced
	}
	return &Result{Status: status, SessionID: sessionID, Project: project, TotalMessages: len(msgs)}, nil
}

// decodeText reads bytes the way Deno.readTextFileSync does: a leading BOM
// is dropped and invalid UTF-8 becomes U+FFFD.
func decodeText(b []byte) string {
	b = bytes.TrimPrefix(b, []byte("\xEF\xBB\xBF"))
	if utf8.Valid(b) {
		return string(b)
	}
	return strings.ToValidUTF8(string(b), "�")
}

// atob decodes base64 like the browser's atob: ASCII whitespace is ignored
// and padding is optional.
func atob(s string) ([]byte, error) {
	s = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\f', '\r':
			return -1
		}
		return r
	}, s)
	if len(s)%4 == 0 {
		s = strings.TrimSuffix(strings.TrimSuffix(s, "="), "=")
	}
	if len(s)%4 == 1 || strings.Contains(s, "=") {
		return nil, fmt.Errorf("invalid base64")
	}
	out, err := base64.RawStdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []byte{}
	}
	return out, nil
}

// Options selects what Run imports.
type Options struct {
	// ProjectsDirs are the transcript trees, the primary one first.
	ProjectsDirs []string
	Session      string // a session ID or its prefix
	Project      string // case-insensitive substring of the project dir name
	DryRun       bool
}

// Run imports every transcript that matches opts and prints a summary. d
// may be nil for a dry run. A session that fails to import does not stop the
// others; Run reports how many failed and returns the first error.
func Run(vault *db.DB, opts Options, w io.Writer) error {
	found := parser.Discover(opts.ProjectsDirs...)
	if len(opts.ProjectsDirs) > 1 {
		for i, files := range parser.Trees(opts.ProjectsDirs, found) {
			dir := opts.ProjectsDirs[i]
			if _, err := os.Stat(dir); err != nil {
				fmt.Fprintf(w, "%s: not found\n", config.TildePath(dir))
				continue
			}
			fmt.Fprintf(w, "%s: %d session files\n", config.TildePath(dir), len(files))
		}
	}
	// One file per session before narrowing, so --session never picks a
	// copy the full import would not.
	all := parser.Choose(found)
	targets := all
	switch {
	case opts.Session != "":
		targets = nil
		for _, f := range all {
			if f.SessionID == opts.Session || strings.HasPrefix(f.SessionID, opts.Session) {
				targets = append(targets, f)
			}
		}
	case opts.Project != "":
		targets = nil
		p := strings.ToLower(opts.Project)
		for _, f := range all {
			if strings.Contains(strings.ToLower(f.Project), p) {
				targets = append(targets, f)
			}
		}
	}

	if len(targets) == 0 {
		fmt.Fprintln(w, "No sessions found to import.")
		return nil
	}
	if opts.DryRun {
		fmt.Fprintf(w, "Would import %d session files:\n", len(targets))
		for _, t := range targets[:min(20, len(targets))] {
			fmt.Fprintf(w, "  %s (%s)%s\n", t.SessionID, t.Project, whereFrom(t, found, opts.ProjectsDirs))
		}
		if len(targets) > 20 {
			fmt.Fprintf(w, "  ... and %d more\n", len(targets)-20)
		}
		return nil
	}

	fmt.Fprintf(w, "Syncing %d sessions...\n", len(targets))

	// sessions-index.json is per project directory, which two trees can
	// both have.
	type projectDir struct{ tree, project string }
	var order []projectDir
	byProject := map[projectDir][]parser.File{}
	for _, t := range targets {
		k := projectDir{t.Dir, t.Project}
		if _, ok := byProject[k]; !ok {
			order = append(order, k)
		}
		byProject[k] = append(byProject[k], t)
	}

	var imported, messages, unchanged, skipped, failed int
	var firstErr error
	for _, project := range order {
		index := parser.LoadIndex(filepath.Join(project.tree, project.project))
		for _, f := range byProject[project] {
			r, err := ImportFile(vault, f.Path, index[f.SessionID], opts.ProjectsDirs)
			if err != nil {
				failed++
				if firstErr == nil {
					firstErr = fmt.Errorf("import %s: %w", f.SessionID, err)
				}
				continue
			}
			switch {
			case r == nil:
				skipped++
			case r.Status == Unchanged:
				unchanged++
			default:
				imported++
				messages += r.TotalMessages
			}
		}
	}

	parts := []string{fmt.Sprintf("Imported %d sessions (%d messages).", imported, messages)}
	if unchanged > 0 {
		parts = append(parts, fmt.Sprintf("%d unchanged.", unchanged))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("Skipped %d unreadable files.", skipped))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("Failed to import %d sessions.", failed))
	}
	fmt.Fprintln(w, strings.Join(parts, " "))
	return firstErr
}

// whereFrom says, for a dry run, which tree a file outside the primary is
// in, and which copies of its session in other trees it was chosen over.
func whereFrom(f parser.File, found []parser.File, dirs []string) string {
	var out string
	if len(dirs) > 0 && f.Dir != dirs[0] {
		out = "  in " + config.TildePath(f.Dir)
	}
	var over []string
	for _, o := range found {
		if o.SessionID == f.SessionID && o.Path != f.Path {
			over = append(over, config.TildePath(o.Dir))
		}
	}
	if len(over) > 0 {
		sep := "  "
		if out != "" {
			sep = ", "
		}
		out += sep + "chosen over the copy in " + strings.Join(over, " and ")
	}
	return out
}
