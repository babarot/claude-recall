package watcher

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/babarot/claude-recall/internal/flock"
)

// Variables so a test can shorten them.
var (
	// aloneRetry is how often a process that is not watching looks again.
	aloneRetry = 5 * time.Second
	// aloneBeat is how often the watching process touches its lock file.
	aloneBeat = 10 * time.Second
	// aloneStale is how long a lock file can go untouched before its holder
	// is taken to be stopped, as a `claude` suspended with Ctrl-Z stops its
	// `recall mcp` with it, and another process starts watching.
	aloneStale = time.Minute
	// aloneSlots bounds how many stopped holders are watched around; past
	// it, every process watches without a lock.
	aloneSlots = 64
)

// LockPath is the lock file of the watcher that imports the trees dirs
// into the database at dbPath. Each set of trees has its own, so processes
// configured with different ones (another CLAUDE_CONFIG_DIR or
// extra_projects_dirs) each have one of theirs watching.
func LockPath(dbPath string, dirs []string) string {
	sum := sha256.Sum256([]byte(strings.Join(dirs, "\n")))
	return fmt.Sprintf("%s.watch-%x.lock", dbPath, sum[:4])
}

// RunAlone calls run, until ctx is canceled, in one process at a time of
// those passing the same lock path; the others wait and take over when it
// exits. The holder touches the lock file while it runs, and a holder
// stopped for longer than aloneStale is watched around through the next
// lock file of the same name with a number appended. When no lock can be
// taken at all, run is called anyway, as watching in every process is
// better than in none.
func RunAlone(ctx context.Context, lockPath string, run func(context.Context), log io.Writer) {
	tick := time.NewTicker(aloneRetry)
	defer tick.Stop()
	for {
		l, path, err := takeSlot(lockPath)
		if err != nil {
			fmt.Fprintf(log, "[watcher] %v; watching without the lock.\n", err)
			run(ctx)
			return
		}
		if l != nil {
			hold(ctx, l, path, run)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func slotPath(base string, i int) string {
	if i == 0 {
		return base
	}
	return fmt.Sprintf("%s.%d", base, i)
}

// fresh reports whether the lock file at path was touched within
// aloneStale, as its holder does while it watches.
func fresh(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && time.Since(fi.ModTime()) < aloneStale
}

// errNoSlot is returned by takeSlot when every lock is held by a stopped
// process.
var errNoSlot = errors.New("every watch lock is held by a stopped process")

// takeSlot takes the first free lock unless a holder is watching, and
// returns no lock and no error when one is. A lock is tried before its
// file is looked at: a holder that crashed left its file touched a moment
// ago, but the lock went with it.
func takeSlot(base string) (*flock.Lock, string, error) {
	for i := range aloneSlots {
		path := slotPath(base, i)
		l, err := flock.TryLock(path)
		if err != nil {
			return nil, "", err
		}
		if l == nil {
			if fresh(path) {
				return nil, "", nil
			}
			continue // stopped
		}
		if watchingAfter(base, i) {
			l.Unlock()
			return nil, "", nil
		}
		return l, path, nil
	}
	return nil, "", errNoSlot
}

// watchingAfter reports whether a holder of a slot after i is watching, as
// one is when the holder of i exited after the other watched around it.
func watchingAfter(base string, i int) bool {
	for j := i + 1; j < aloneSlots; j++ {
		path := slotPath(base, j)
		if _, err := os.Stat(path); err != nil {
			return false // slots are taken in order
		}
		l, err := flock.TryLock(path)
		if err != nil {
			return false
		}
		if l != nil {
			l.Unlock()
			continue
		}
		if fresh(path) {
			return true
		}
	}
	return false
}

// hold runs run while touching the lock file at path, then releases the
// lock.
func hold(ctx context.Context, l *flock.Lock, path string, run func(context.Context)) {
	touch := func() {
		now := time.Now()
		os.Chtimes(path, now, now)
	}
	touch()

	beatCtx, stop := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() {
		t := time.NewTicker(aloneBeat)
		defer t.Stop()
		for {
			select {
			case <-beatCtx.Done():
				return
			case <-t.C:
				touch()
			}
		}
	})
	run(ctx)
	stop()
	wg.Wait()
	l.Unlock()
}
