// Package askhistory keeps the questions asked of Claude in the TUI's Ask
// box between launches, and the place among them while the user goes back
// through them.
//
// The history is a file of JSON lines in the state directory: a line as
// each question is sent, and a line with what its answer found, joined by
// ID when they are read. It is not the archive: a question asked is not a
// Claude Code session, and the archive is kept for what only it holds.
package askhistory

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/babarot/claude-recall/internal/flock"
)

// version is the format of a line; a line of another format is not read,
// and leaves the file as it is.
const version = 1

// limit is how many questions the history keeps. It is a constant, not a
// setting: it only keeps the file from growing without end.
const limit = 500

// lockWait is how long a read or a write waits for another TUI's.
const lockWait = time.Second

// Entry is a question asked, and what its answer found when one came.
type Entry struct {
	ID       string
	At       time.Time
	Question string
	Reply    string
}

// History is the history kept in a directory.
//
// Every read and write holds a lock on a file of its own, which is never
// replaced, and opens the history only once it holds it: the history
// itself is written back when it is read, so a lock on it, or a file kept
// open, would miss another TUI's write.
type History struct {
	dir string
}

// Open returns the history kept in dir, the state directory.
func Open(dir string) *History { return &History{dir: dir} }

func (h *History) path() string     { return filepath.Join(h.dir, "ask-history.jsonl") }
func (h *History) lockPath() string { return filepath.Join(h.dir, "ask-history.lock") }

type line struct {
	V     int       `json:"v"`
	ID    string    `json:"id"`
	At    time.Time `json:"at,omitzero"`
	Q     string    `json:"q,omitempty"`
	Reply string    `json:"reply,omitempty"`
}

// ErrBusy is returned when another TUI held the history for longer than a
// read or a write waits.
var ErrBusy = errors.New("the history of questions is held by another recall")

// locked runs f holding the history's lock.
func (h *History) locked(f func() error) error {
	if err := os.MkdirAll(h.dir, 0o700); err != nil {
		return err
	}
	deadline := time.Now().Add(lockWait)
	for {
		l, err := flock.TryLock(h.lockPath())
		if err != nil {
			return err
		}
		if l != nil {
			defer func() { _ = l.Unlock() }() // closing the file releases the lock all the same
			return f()
		}
		if time.Now().After(deadline) {
			return ErrBusy
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (h *History) append(l line) error {
	l.V = version
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	return h.locked(func() error {
		f, err := os.OpenFile(h.path(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, werr := f.Write(append(b, '\n'))
		return errors.Join(werr, f.Close())
	})
}

// Add keeps a question as it is sent, so that it stays when the TUI quits
// before the answer.
func (h *History) Add(e Entry) error {
	return h.append(line{ID: e.ID, At: e.At, Q: e.Question})
}

// Reply keeps what the answer to the question id found. It must follow the
// Add of that question in the file: a read drops a reply with no question
// before it.
func (h *History) Reply(id, text string) error {
	return h.append(line{ID: id, Reply: text})
}

// Load reads the questions, the newest first, the newest of each question
// and at most limit, and writes the file back as that. A line it cannot
// read is left out; when one is of another format, the file is left as it
// is, so an older recall does not lose what a newer one wrote.
func (h *History) Load() ([]Entry, error) {
	var out []Entry
	err := h.locked(func() error {
		b, err := os.ReadFile(h.path())
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		var n int
		var other bool
		out, n, other = parse(b)
		if other || n == written(out) {
			return nil
		}
		return h.rewrite(out)
	})
	return out, err
}

// parse reads the lines of b, and tells how many there were and whether
// one was of another format.
func parse(b []byte) (out []Entry, n int, other bool) {
	var asked []line
	replies := map[string]string{}
	for l := range bytes.Lines(b) {
		n++
		// The version first: a line of another one may hold its fields in
		// another form, which must not make it read as one that cannot be
		// read, and be dropped when the file is written back.
		var v struct {
			V int `json:"v"`
		}
		if json.Unmarshal(l, &v) != nil {
			continue
		}
		if v.V != version {
			other = true
			continue
		}
		var x line
		if json.Unmarshal(l, &x) != nil {
			continue
		}
		switch {
		case x.Q != "":
			asked = append(asked, x)
		case x.Reply != "":
			replies[x.ID] = x.Reply
		}
	}
	slices.SortStableFunc(asked, func(a, b line) int { return b.At.Compare(a.At) })
	seen := map[string]bool{}
	for _, x := range asked {
		if seen[x.Q] || len(out) == limit {
			continue
		}
		seen[x.Q] = true
		out = append(out, Entry{ID: x.ID, At: x.At, Question: x.Q, Reply: replies[x.ID]})
	}
	return out, n, other
}

// written is how many lines es take in the file.
func written(es []Entry) int {
	n := len(es)
	for _, e := range es {
		if e.Reply != "" {
			n++
		}
	}
	return n
}

// rewrite replaces the file with es, oldest first, through a file beside
// it and a rename, so the history is whole at every moment.
func (h *History) rewrite(es []Entry) error {
	var b bytes.Buffer
	for _, e := range slices.Backward(es) {
		for _, l := range []line{{V: version, ID: e.ID, At: e.At, Q: e.Question}, {V: version, ID: e.ID, Reply: e.Reply}} {
			if l.Q == "" && l.Reply == "" {
				continue
			}
			j, err := json.Marshal(l)
			if err != nil {
				return err
			}
			b.Write(append(j, '\n'))
		}
	}
	tmp, err := os.CreateTemp(h.dir, "ask-history-*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(b.Bytes())
	if err := errors.Join(werr, tmp.Close()); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), h.path())
}

// Prepend puts e first in list, the newest first, in place of the same
// question asked earlier.
func Prepend(list []Entry, e Entry) []Entry {
	out := []Entry{e}
	for _, o := range list {
		if o.Question != e.Question {
			out = append(out, o)
		}
	}
	return out
}

// Recall is the place among the questions, the newest first, while the
// user goes back through them, and the text that was typed before. The
// zero value is out of them.
type Recall struct {
	in    bool
	at    int
	typed string
}

// Active reports whether the field holds a question recalled.
func (r *Recall) Active() bool { return r.in }

// Older moves to the question before the one recalled, keeping typed when
// it is the first move; ok is false when there is none. A first move skips
// a newest question that is what is typed, as the box opens on the last
// question asked.
func (r *Recall) Older(list []Entry, typed string) (string, bool) {
	switch {
	case !r.in:
		at := 0
		if len(list) > 0 && list[0].Question == typed {
			at = 1
		}
		if at >= len(list) {
			return "", false
		}
		r.in, r.at, r.typed = true, at, typed
	case r.at+1 < len(list):
		r.at++
	default:
		return "", false
	}
	return list[r.at].Question, true
}

// Newer moves to the question after the one recalled, or, past the
// newest, out of them, back to the text that was typed. The list may have
// become shorter meanwhile.
func (r *Recall) Newer(list []Entry) string {
	r.at = min(r.at, len(list))
	if r.at == 0 || r.at == 1 && len(list) > 0 && list[0].Question == r.typed {
		typed := r.typed
		r.Leave()
		return typed
	}
	r.at--
	return list[r.at].Question
}

// Leave goes out of the questions; the field keeps what it holds.
func (r *Recall) Leave() { *r = Recall{} }
