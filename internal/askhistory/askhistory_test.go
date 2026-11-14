package askhistory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/babarot/claude-recall/internal/flock"
)

var t0 = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func questions(es []Entry) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.Question)
	}
	return out
}

func TestAddReplyLoad(t *testing.T) {
	h := Open(t.TempDir())
	if es, err := h.Load(); err != nil || len(es) != 0 {
		t.Fatalf("an empty history: %v, %v", es, err)
	}
	for i, q := range []string{"parser work", "ログインのバグ", "parser work"} {
		id := fmt.Sprint(i)
		if err := h.Add(Entry{ID: id, At: t0.Add(time.Duration(i) * time.Minute), Question: q}); err != nil {
			t.Fatal(err)
		}
		if err := h.Reply(id, fmt.Sprintf("%d sessions", i)); err != nil {
			t.Fatal(err)
		}
	}
	es, err := h.Load()
	if err != nil {
		t.Fatal(err)
	}
	// The newest first, and the newest of a question asked twice.
	if got := strings.Join(questions(es), "|"); got != "parser work|ログインのバグ" {
		t.Fatalf("questions %q", got)
	}
	if es[0].ID != "2" || es[0].Reply != "2 sessions" || es[1].Reply != "1 sessions" {
		t.Errorf("entries %+v", es)
	}
	// Written back as what was read: a second read reads the same.
	again, err := h.Load()
	if err != nil || fmt.Sprint(again) != fmt.Sprint(es) {
		t.Errorf("read again: %v, %v", again, err)
	}
	b, _ := os.ReadFile(h.path())
	if n := strings.Count(string(b), "\n"); n != 4 {
		t.Errorf("%d lines written back, want 4:\n%s", n, b)
	}
}

func TestReplyWithoutQuestionDropped(t *testing.T) {
	h := Open(t.TempDir())
	if err := h.Reply("lost", "2 sessions"); err != nil {
		t.Fatal(err)
	}
	if err := h.Add(Entry{ID: "a", At: t0, Question: "q"}); err != nil {
		t.Fatal(err)
	}
	es, err := h.Load()
	if err != nil || len(es) != 1 || es[0].Reply != "" {
		t.Fatalf("got %+v, %v", es, err)
	}
}

func TestLimit(t *testing.T) {
	h := Open(t.TempDir())
	for i := range limit + 5 {
		if err := h.Add(Entry{ID: fmt.Sprint(i), At: t0.Add(time.Duration(i) * time.Second), Question: fmt.Sprint("q", i)}); err != nil {
			t.Fatal(err)
		}
	}
	es, err := h.Load()
	if err != nil || len(es) != limit || es[0].Question != fmt.Sprint("q", limit+4) {
		t.Fatalf("%d kept, newest %q, %v", len(es), es[0].Question, err)
	}
}

// A line of another format leaves the file as it is, so an older recall
// does not lose what a newer one wrote.
func TestOtherFormatLeftAlone(t *testing.T) {
	h := Open(t.TempDir())
	for _, q := range []string{"a", "a"} {
		if err := h.Add(Entry{ID: q, At: t0, Question: q}); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.OpenFile(h.path(), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Its at is a number, which this version cannot read as a time.
	if _, err := f.WriteString(`{"v":2,"id":"x","at":1730000000,"q":"from a newer recall"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(h.path())
	es, err := h.Load()
	if err != nil || len(es) != 1 {
		t.Fatalf("got %+v, %v", es, err)
	}
	if after, _ := os.ReadFile(h.path()); string(after) != string(before) {
		t.Errorf("the file was written back:\n%s", after)
	}
}

func TestBusy(t *testing.T) {
	dir := t.TempDir()
	h := Open(dir)
	l, err := flock.TryLock(filepath.Join(dir, "ask-history.lock"))
	if err != nil || l == nil {
		t.Skip("no lock to hold on this platform")
	}
	defer func() { _ = l.Unlock() }()
	if err := h.Add(Entry{ID: "a", At: t0, Question: "q"}); !errors.Is(err, ErrBusy) {
		t.Errorf("Add while held: %v, want ErrBusy", err)
	}
}

func TestRecall(t *testing.T) {
	list := []Entry{{Question: "c"}, {Question: "b"}, {Question: "a"}}
	var r Recall
	if q, ok := r.Older(list, "typed"); !ok || q != "c" {
		t.Fatalf("first older: %q %v", q, ok)
	}
	r.Older(list, "")
	if q, _ := r.Older(list, ""); q != "a" {
		t.Fatalf("third older: %q", q)
	}
	if _, ok := r.Older(list, ""); ok {
		t.Fatal("older past the oldest")
	}
	r.Newer(list)
	r.Newer(list)
	if q := r.Newer(list); q != "typed" || r.Active() {
		t.Fatalf("past the newest: %q, active %v", q, r.Active())
	}

	// The box opens on the last question asked: the first move skips it,
	// and coming back gives it.
	r = Recall{}
	if q, _ := r.Older(list, "c"); q != "b" {
		t.Fatalf("first older over the last question: %q", q)
	}
	if q := r.Newer(list); q != "c" || r.Active() {
		t.Fatalf("back: %q, active %v", q, r.Active())
	}
	if _, ok := (&Recall{}).Older(list[:1], "c"); ok {
		t.Fatal("nothing older than the only question, which is typed")
	}

	// The list grown shorter meanwhile.
	r = Recall{}
	r.Older(list, "")
	r.Older(list, "")
	r.Older(list, "")
	if q := r.Newer(list[:1]); q != "c" || !r.Active() {
		t.Fatalf("newer on a shorter list: %q, active %v", q, r.Active())
	}
	if q := r.Newer(list[:1]); q != "" || r.Active() {
		t.Fatalf("past the newest of a shorter list: %q, active %v", q, r.Active())
	}
}
