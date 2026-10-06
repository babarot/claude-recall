//go:build !unix

package parser

import "os"

type fileID struct{}

// idOf tells nothing where files have no device and inode: each path is
// its own file.
func idOf(os.FileInfo) (fileID, bool) { return fileID{}, false }
