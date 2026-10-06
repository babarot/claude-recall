package parser

import (
	"os"
	"path/filepath"
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

// isDir reports whether the entry e at path is a directory, following a
// symlink.
func isDir(path string, e os.DirEntry) bool {
	if e.Type()&os.ModeSymlink == 0 {
		return e.IsDir()
	}
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
