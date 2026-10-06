//go:build unix

package parser

import (
	"os"
	"syscall"
)

// fileID is a file's device and inode: two paths to one file have the same.
type fileID struct{ dev, ino uint64 }

// idOf is the identity of the file info describes. Some FUSE mounts give
// every file inode 0, which tells nothing.
func idOf(info os.FileInfo) (fileID, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Ino == 0 {
		return fileID{}, false
	}
	return fileID{dev: uint64(st.Dev), ino: uint64(st.Ino)}, true
}
