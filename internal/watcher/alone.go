package watcher

import (
	"context"
	"crypto/sha256"
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
)

// aloneSlots bounds how many stopped holders are watched around.
const aloneSlots = 4

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

// takeSlot takes the first free lock unless a holder is watching. It
// returns no lock and no error when one is, or when every slot is held by
// a stopped process.
func takeSlot(base string) (*flock.Lock, string, error) {
	for i := range aloneSlots {
		if fresh(slotPath(base, i)) {
			return nil, "", nil
		}
	}
	for i := range aloneSlots {
		path := slotPath(base, i)
		l, err := flock.TryLock(path)
		if err != nil {
			return nil, "", err
		}
		if l != nil {
			return l, path, nil
		}
		if fresh(path) {
			return nil, "", nil // taken a moment ago
		}
	}
	return nil, "", nil
}

// hold runs run while touching the lock file at path, then marks the file
// untouched for long, so the next process takes over at its next look
// rather than after aloneStale, and releases the lock.
func hold(ctx context.Context, l *flock.Lock, path string, run func(context.Context)) {
	touch := func(t time.Time) { os.Chtimes(path, t, t) }
	touch(time.Now())

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
				touch(time.Now())
			}
		}
	})
	run(ctx)
	stop()
	wg.Wait()
	touch(time.Unix(0, 0))
	l.Unlock()
}
