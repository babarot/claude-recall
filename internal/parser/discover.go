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
	Dir       string // the projects directory (tree) it was found in
	Project   string // the encoded project directory name
	SessionID string
	Path      string
	Size      int64
	ModTime   time.Time
}

// Discover lists every <dir>/<project>/<session>.jsonl of the trees dirs,
// in their order. A symlinked project directory or transcript is followed.
// A file reached twice, through a symlink or a second mount of a tree, is
// listed once, under the path it was first reached by.
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
				})
			}
		}
	}
	return out
}

// FindSession lists the transcripts of the session id in the trees dirs,
// as Discover would list them, with one stat per project directory instead
// of reading each.
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
			out = append(out, File{Dir: tree, Project: p.Name(), SessionID: id, Path: path, Size: info.Size(), ModTime: info.ModTime()})
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
