//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

package flock

import "os"

// tryLock always succeeds where flock(2) is not available, so every
// process acts as the holder, as before there was a lock.
func tryLock(*os.File) (bool, error) { return true, nil }
