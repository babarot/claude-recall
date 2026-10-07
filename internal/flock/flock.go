// Package flock takes an exclusive lock on a file without waiting for it.
// The operating system releases the lock when the holder exits, however it
// exits, so a lock is never left behind by a crashed process.
package flock

import "os"

// Lock is a held lock.
type Lock struct {
	f *os.File
}

// TryLock takes the lock on path, creating the file if needed. It returns
// nil and no error when another process holds it.
func TryLock(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	ok, err := tryLock(f)
	if err != nil || !ok {
		f.Close()
		return nil, err
	}
	return &Lock{f: f}, nil
}

// Unlock releases the lock.
func (l *Lock) Unlock() error {
	return l.f.Close()
}
