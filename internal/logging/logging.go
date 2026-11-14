// Package logging makes the logger recall writes its log file with: JSON
// lines through log/slog, to one file under the state directory that every
// recall process appends to, never to the terminal the TUI draws on.
//
// The logger is made once, in cmd/recall, and passed down; packages add
// what they know with With and never keep a logger of their own in a
// package variable. What is said in a session is never logged: not the
// conversation, the questions asked in Ask, the words searched for or a
// session's title, which all come from it. A session is named by its ID.
// See ADR-011.
package logging

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/babarot/claude-recall/internal/flock"
)

// Attribute keys every package uses, so a log line can be searched by them.
const (
	KeyProc       = "proc" // tui, mcp, ui, import or cli
	KeyRun        = "run"  // an ID for each process
	KeyVersion    = "version"
	KeyOp         = "op"
	KeySession    = "session"
	KeyPath       = "path"
	KeyCount      = "count"
	KeyFailed     = "failed"
	KeyDurationMS = "duration_ms"
	KeyErr        = "err"
	KeyPort       = "port"
	KeyTool       = "tool"
	KeyModel      = "model"
	KeyCost       = "cost"
	KeyFrom       = "from"
	KeyTo         = "to"
	KeyLatest     = "latest"
)

// The log file is moved aside when it is over maxSize as a process starts,
// and keep old ones are kept. They are constants, not settings: they only
// bound the space the log takes.
const (
	maxSize = 10 << 20
	keep    = 3
)

// DefaultPath is the log file: $XDG_STATE_HOME/claude-recall/logs/recall.log,
// or ~/.local/state/claude-recall/logs/recall.log.
func DefaultPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "claude-recall", "logs", "recall.log")
}

// Levels are the levels a level may be written as.
var Levels = []string{"debug", "info", "warn", "error"}

// ParseLevel reads a level as the config file or RECALL_LOG_LEVEL write
// it; empty is info.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return slog.LevelInfo, fmt.Errorf("%q is not a log level: %s", s, strings.Join(Levels, ", "))
}

// Open moves the log file at path aside if it has grown past maxSize, and
// returns a logger appending JSON lines to it at level. Only the process
// holding the file's lock moves it, so two starting at once do not both.
func Open(path string, level slog.Leveler) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, err
	}
	if l, err := flock.TryLock(path + ".lock"); err == nil && l != nil {
		rerr := rotate(path, maxSize, keep)
		_ = l.Unlock() // closing the file releases the lock all the same
		if rerr != nil {
			return nil, nil, rerr
		}
	}
	w := &file{path: path}
	if err := w.open(); err != nil {
		return nil, nil, err
	}
	return New(w, level), w, nil
}

// New returns a logger writing JSON lines to w at level.
func New(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// Discard is a logger that writes nothing, for a process with no log file
// and for tests.
func Discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// NewID is an ID for a process, short enough to read in a line.
func NewID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// file appends to the log file, and opens it again when another process
// has moved it aside, before every line: a process that runs for long,
// as recall mcp and recall ui do, would otherwise write to a file moved
// aside, and on past the old ones kept, to one that is gone.
type file struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

func (w *file) open() error {
	f, err := os.OpenFile(w.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if w.f != nil {
		_ = w.f.Close()
	}
	w.f = f
	return nil
}

// Write writes one line, as slog's handler hands each line in one call.
func (w *file) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.current() {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	return w.f.Write(p)
}

// current reports whether the file open is the one at the path.
func (w *file) current() bool {
	at, err := os.Stat(w.path)
	if err != nil {
		return false
	}
	open, err := w.f.Stat()
	return err == nil && os.SameFile(at, open)
}

func (w *file) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}

// rotate moves path to path.1, path.1 to path.2 and so on when path is
// over max, dropping what would go past keep.
func rotate(path string, max int64, keep int) error {
	fi, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) || err == nil && fi.Size() <= max {
		return nil
	}
	if err != nil {
		return err
	}
	for i := keep - 1; i >= 1; i-- {
		from, to := fmt.Sprintf("%s.%d", path, i), fmt.Sprintf("%s.%d", path, i+1)
		if err := os.Rename(from, to); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return os.Rename(path, path+".1")
}
