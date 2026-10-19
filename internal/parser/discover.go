package parser

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// File is a transcript found on disk.
type File struct {
	Dir string // the projects directory (tree) it was found in
	// Project is the encoded project directory name of a Claude Code
	// transcript, or the YYYY/MM/DD directory of a Codex rollout, under Dir.
	Project   string
	SessionID string
	Path      string
	Size      int64
	ModTime   time.Time
	// Agent is who wrote the transcript: AgentClaude for
	// <dir>/<project>/<session>.jsonl, AgentCodex for a rollout under
	// <dir>/YYYY/MM/DD.
	Agent string
}

// IsCodex reports whether the file is a Codex rollout.
func (f File) IsCodex() bool { return f.Agent == AgentCodex }

// Discover lists every transcript of the trees dirs, in their order: a
// Claude Code projects directory holds <project>/<session>.jsonl, a Codex
// sessions directory (~/.codex/sessions or archived_sessions) holds
// YYYY/MM/DD/rollout-<time>-<session>.jsonl, possibly compressed to
// .jsonl.zst; a tree may hold both. A symlinked project directory or
// transcript is followed. A file reached twice, through a symlink or a
// second mount of a tree, is listed once, under the path it was first
// reached by.
func Discover(dirs ...string) []File {
	var out []File
	seen := map[fileID]bool{}
	for _, tree := range dirs {
		projects, err := os.ReadDir(tree)
		if err != nil {
			continue
		}
		for _, p := range projects {
			dir := filepath.Join(tree, p.Name())
			if !isDir(dir, p) {
				continue
			}
			if isYear(p.Name()) {
				out = append(out, discoverCodexYear(tree, p.Name(), seen)...)
				continue
			}
			files, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, f := range files {
				if !strings.HasSuffix(f.Name(), ".jsonl") {
					continue
				}
				path := filepath.Join(dir, f.Name())
				info, err := os.Stat(path)
				if err != nil || !info.Mode().IsRegular() {
					continue
				}
				if id, ok := idOf(info); ok {
					if seen[id] {
						continue
					}
					seen[id] = true
				}
				out = append(out, File{
					Dir:       tree,
					Project:   p.Name(),
					SessionID: strings.TrimSuffix(f.Name(), ".jsonl"),
					Path:      path,
					Size:      info.Size(),
					ModTime:   info.ModTime(),
					Agent:     AgentClaude,
				})
			}
		}
	}
	return out
}

// discoverCodexYear lists the rollouts under <tree>/<year>/MM/DD.
func discoverCodexYear(tree, year string, seen map[fileID]bool) []File {
	var out []File
	months, err := os.ReadDir(filepath.Join(tree, year))
	if err != nil {
		return nil
	}
	for _, m := range months {
		if !isDir(filepath.Join(tree, year, m.Name()), m) {
			continue
		}
		days, err := os.ReadDir(filepath.Join(tree, year, m.Name()))
		if err != nil {
			continue
		}
		for _, d := range days {
			project := filepath.Join(year, m.Name(), d.Name())
			dir := filepath.Join(tree, project)
			if !isDir(dir, d) {
				continue
			}
			files, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, f := range files {
				id, ok := CodexSessionID(f.Name())
				if !ok {
					continue
				}
				path := filepath.Join(dir, f.Name())
				info, err := os.Stat(path)
				if err != nil || !info.Mode().IsRegular() {
					continue
				}
				if fid, ok := idOf(info); ok {
					if seen[fid] {
						continue
					}
					seen[fid] = true
				}
				out = append(out, File{Dir: tree, Project: project, SessionID: id, Path: path,
					Size: info.Size(), ModTime: info.ModTime(), Agent: AgentCodex})
			}
		}
	}
	return out
}

// isYear reports whether a directory name is a year, as Codex names the
// directories of its sessions tree.
func isYear(name string) bool {
	if len(name) != 4 {
		return false
	}
	for _, c := range name {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// CodexSessionID is the session (thread) ID in the name of a Codex
// rollout file, rollout-<YYYY-MM-DDThh-mm-ss>-<id>.jsonl, or with _<rollout
// id> after the session ID for a rollout `thread/revert` wrote, or with
// .zst after .jsonl once Codex compressed it. It reports false for any
// other name.
func CodexSessionID(name string) (string, bool) {
	core, ok := strings.CutPrefix(name, "rollout-")
	if !ok {
		return "", false
	}
	core = strings.TrimSuffix(core, ".zst")
	core, ok = strings.CutSuffix(core, ".jsonl")
	if !ok || len(core) < 21 || core[19] != '-' {
		return "", false
	}
	ids := core[20:]
	id, _, _ := strings.Cut(ids, "_")
	if id == "" {
		return "", false
	}
	return id, true
}

// IsCodexPath reports whether path names a Codex rollout, by its name.
func IsCodexPath(path string) bool {
	_, ok := CodexSessionID(filepath.Base(path))
	return ok
}

// FindSession lists the transcripts of the session id in the trees dirs,
// as Discover would list them, with one stat per Claude Code project
// directory instead of reading each, and the day directories of a Codex
// tree read for a rollout of the session.
func FindSession(dirs []string, id string) []File {
	var out []File
	seen := map[fileID]bool{}
	name := id + ".jsonl"
	for _, tree := range dirs {
		projects, err := os.ReadDir(tree)
		if err != nil {
			continue
		}
		for _, p := range projects {
			if isYear(p.Name()) {
				for _, f := range discoverCodexYear(tree, p.Name(), seen) {
					if f.SessionID == id {
						out = append(out, f)
					}
				}
				continue
			}
			path := filepath.Join(tree, p.Name(), name)
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			if fid, ok := idOf(info); ok {
				if seen[fid] {
					continue
				}
				seen[fid] = true
			}
			out = append(out, File{Dir: tree, Project: p.Name(), SessionID: id, Path: path, Size: info.Size(), ModTime: info.ModTime(), Agent: AgentClaude})
		}
	}
	return out
}

// isDir reports whether the entry e at path is a directory, following a
// symlink.
func isDir(path string, e os.DirEntry) bool {
	if e.Type()&os.ModeSymlink == 0 {
		return e.IsDir()
	}
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// Choose keeps one file per session, for a session whose transcript is in
// more than one tree: the one written last, as the archive mirrors the
// current transcript; on a tie the larger, then the one in the earlier
// tree. The files keep their order.
func Choose(files []File) []File {
	best := map[string]int{}
	for i, f := range files {
		j, ok := best[f.SessionID]
		if !ok || better(f, files[j]) {
			best[f.SessionID] = i
		}
	}
	out := make([]File, 0, len(best))
	for i, f := range files {
		if best[f.SessionID] == i {
			out = append(out, f)
		}
	}
	return out
}

// better reports whether a, found after b, wins over it.
func better(a, b File) bool {
	if c := a.ModTime.Compare(b.ModTime); c != 0 {
		return c > 0
	}
	return a.Size > b.Size
}

// Trees are the files Discover found, by tree, in the order of dirs.
func Trees(dirs []string, files []File) [][]File {
	out := make([][]File, len(dirs))
	for _, f := range files {
		if i := slices.Index(dirs, f.Dir); i >= 0 {
			out[i] = append(out[i], f)
		}
	}
	return out
}
