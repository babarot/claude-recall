//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package watcher

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/babarot/claude-recall/internal/flock"
)

func shortAlone(t *testing.T, beat time.Duration) {
	t.Helper()
	r, b, s := aloneRetry, aloneBeat, aloneStale
	aloneRetry, aloneBeat, aloneStale = 10*time.Millisecond, beat, 300*time.Millisecond
	t.Cleanup(func() { aloneRetry, aloneBeat, aloneStale = r, b, s })
}

// runner starts RunAlone in the background and reports when its run is
// called. flock(2) locks belong to the open file, so two runners in one
// test contend as two processes would.
type runner struct {
	cancel  context.CancelFunc
	started chan struct{}
	wg      sync.WaitGroup
}

func startRunner(t *testing.T, lock string) *runner {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r := &runner{cancel: cancel, started: make(chan struct{})}
	r.wg.Go(func() {
		RunAlone(ctx, lock, func(ctx context.Context) {
			close(r.started)
			<-ctx.Done()
		}, io.Discard)
	})
	t.Cleanup(func() { r.stop() })
	return r
}

func (r *runner) stop() {
	r.cancel()
	r.wg.Wait()
}

func (r *runner) runs(within time.Duration) bool {
	select {
	case <-r.started:
		return true
	case <-time.After(within):
		return false
	}
}

func TestRunAloneOneAtATime(t *testing.T) {
	shortAlone(t, 20*time.Millisecond)
	lock := filepath.Join(t.TempDir(), "vault.db.watch.lock")

	a := startRunner(t, lock)
	if !a.runs(time.Second) {
		t.Fatal("the first runner never ran")
	}
	b := startRunner(t, lock)
	// Past aloneStale: a holder that keeps beating is never watched around.
	if b.runs(2 * aloneStale) {
		t.Fatal("a second runner ran while the first held the lock")
	}
	a.stop()
	// The lock is tried before the file is looked at, so the takeover
	// comes at the next look, well before aloneStale.
	if !b.runs(aloneStale / 2) {
		t.Fatal("the second runner did not take over when the first exited")
	}
}

func TestRunAloneAroundStoppedHolder(t *testing.T) {
	// A beat too slow to come during the test stands for a stopped holder.
	shortAlone(t, time.Hour)
	lock := filepath.Join(t.TempDir(), "vault.db.watch.lock")

	a := startRunner(t, lock)
	if !a.runs(time.Second) {
		t.Fatal("the first runner never ran")
	}
	b := startRunner(t, lock)
	c := startRunner(t, lock)
	if b.runs(aloneStale / 2) {
		t.Fatal("a second runner ran while the holder was fresh")
	}
	old := time.Now().Add(-2 * aloneStale)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	// One of the waiting runners watches around the stopped holder, and
	// only one.
	select {
	case <-b.started:
		if c.runs(4 * aloneRetry) {
			t.Error("both waiting runners ran around the stopped holder")
		}
	case <-c.started:
		if b.runs(4 * aloneRetry) {
			t.Error("both waiting runners ran around the stopped holder")
		}
	case <-time.After(time.Second):
		t.Fatal("no runner ran around the stopped holder")
	}
}

func TestLockPath(t *testing.T) {
	one := LockPath("/h/.claude/vault.db", []string{"/h/.claude/projects"})
	if one != LockPath("/h/.claude/vault.db", []string{"/h/.claude/projects"}) {
		t.Error("the same trees gave different lock files")
	}
	if one == LockPath("/h/.claude/vault.db", []string{"/h/other/projects"}) {
		t.Error("other trees gave the same lock file")
	}
	if one == LockPath("/h/.claude/vault.db", []string{"/h/.claude/projects", "/h/c/projects"}) {
		t.Error("an extra tree gave the same lock file")
	}
	if filepath.Dir(one) != "/h/.claude" {
		t.Errorf("lock file %s is not next to the database", one)
	}
}

// holdSlot takes the lock at path as another process would, touched at
// mtime.
func holdSlot(t *testing.T, path string, mtime time.Time) *flock.Lock {
	t.Helper()
	l, err := flock.TryLock(path)
	if err != nil || l == nil {
		t.Fatalf("TryLock(%s) = %v, %v", path, l, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Unlock() })
	return l
}

func TestRunAloneAfterCrashedHolder(t *testing.T) {
	shortAlone(t, 20*time.Millisecond)
	aloneStale = time.Hour
	lock := filepath.Join(t.TempDir(), "vault.db.watch.lock")

	// A crash releases the lock but leaves the file touched a moment ago.
	holdSlot(t, lock, time.Now()).Unlock()
	if !startRunner(t, lock).runs(time.Second) {
		t.Fatal("a runner waited on the freshly touched file of a crashed holder")
	}
}

func TestRunAloneLeavesFreedSlotBelowWatcher(t *testing.T) {
	shortAlone(t, 20*time.Millisecond)
	lock := filepath.Join(t.TempDir(), "vault.db.watch.lock")

	// Slot 0's holder exited after slot 1's watched around it.
	holdSlot(t, lock, time.Now()).Unlock()
	holdSlot(t, slotPath(lock, 1), time.Now().Add(time.Hour))
	if startRunner(t, lock).runs(aloneStale / 2) {
		t.Fatal("a runner took slot 0 while slot 1's holder was watching")
	}
}

func TestRunAloneAllSlotsStopped(t *testing.T) {
	shortAlone(t, 20*time.Millisecond)
	old := aloneSlots
	aloneSlots = 2
	t.Cleanup(func() { aloneSlots = old })
	lock := filepath.Join(t.TempDir(), "vault.db.watch.lock")

	stale := time.Now().Add(-2 * aloneStale)
	holdSlot(t, slotPath(lock, 0), stale)
	holdSlot(t, slotPath(lock, 1), stale)
	if !startRunner(t, lock).runs(time.Second) {
		t.Fatal("no runner ran with every slot held by a stopped process")
	}
}
