package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/babarot/claude-recall/internal/flock"
)

func lines(t *testing.T, path string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for l := range bytes.Lines(b) {
		var m map[string]any
		if err := json.Unmarshal(l, &m); err != nil {
			t.Fatalf("not a JSON line: %q", l)
		}
		out = append(out, m)
	}
	return out
}

func TestOpenWritesJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "recall.log")
	log, c, err := Open(path, slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	log.Debug("not written")
	log.Info("imported", KeyCount, 3)
	got := lines(t, path)
	if len(got) != 1 || got[0]["msg"] != "imported" || got[0][KeyCount] != float64(3) {
		t.Errorf("lines %v", got)
	}
}

func TestRotate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recall.log")
	for i := range keep + 2 {
		if err := os.WriteFile(path, []byte(fmt.Sprint("generation ", i)), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := rotate(path, 5, keep); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= keep; i++ {
		b, err := os.ReadFile(fmt.Sprintf("%s.%d", path, i))
		if want := fmt.Sprint("generation ", keep+2-i); err != nil || string(b) != want {
			t.Errorf("%s.%d holds %q, %v; want %q", path, i, b, err, want)
		}
	}
	if _, err := os.Stat(fmt.Sprintf("%s.%d", path, keep+1)); err == nil {
		t.Errorf("more than %d old files kept", keep)
	}
	// A file under the size stays.
	if err := os.WriteFile(path, []byte("ab"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rotate(path, 5, keep); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "ab" {
		t.Errorf("a small file was moved: %q", b)
	}
}

// A process that cannot take the lock does not move the file.
func TestNoRotateWithoutTheLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recall.log")
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), maxSize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := flock.TryLock(path + ".lock")
	if err != nil || l == nil {
		t.Skip("no lock to hold on this platform")
	}
	defer func() { _ = l.Unlock() }()
	_, c, err := Open(path, slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := os.Stat(path + ".1"); err == nil {
		t.Error("moved aside while another held the lock")
	}
}

// A writer that stays open while others move the file aside, past the old
// ones kept, writes its next line into the file at the path.
func TestLongWriterFollowsRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recall.log")
	log, c, err := Open(path, slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	log.Info("before")
	for range keep + 1 {
		if err := rotate(path, 0, keep); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	log.Info("after")
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), `"msg":"after"`) {
		t.Errorf("the line after the rotations is not in %s:\n%s", path, b)
	}
}

func TestParseLevel(t *testing.T) {
	for in, want := range map[string]slog.Level{"": slog.LevelInfo, "DEBUG": slog.LevelDebug, " warn ": slog.LevelWarn, "error": slog.LevelError} {
		if got, err := ParseLevel(in); err != nil || got != want {
			t.Errorf("%q: %v, %v", in, got, err)
		}
	}
	if _, err := ParseLevel("verbose"); err == nil || !strings.Contains(err.Error(), "debug, info, warn, error") {
		t.Errorf("verbose: %v", err)
	}
}
