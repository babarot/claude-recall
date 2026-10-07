//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package flock

import (
	"path/filepath"
	"testing"
)

func TestTryLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	a, err := TryLock(path)
	if err != nil || a == nil {
		t.Fatalf("first TryLock = %v, %v; want the lock", a, err)
	}
	// flock(2) locks belong to the open file, so a second open in the same
	// process contends as another process would.
	if b, err := TryLock(path); err != nil || b != nil {
		t.Fatalf("second TryLock = %v, %v; want nil, nil while held", b, err)
	}
	if err := a.Unlock(); err != nil {
		t.Fatal(err)
	}
	c, err := TryLock(path)
	if err != nil || c == nil {
		t.Fatalf("TryLock after Unlock = %v, %v; want the lock", c, err)
	}
	c.Unlock()
}
