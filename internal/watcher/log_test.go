package watcher

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/babarot/claude-recall/internal/logging"
)

// A watcher with no transcripts to watch says so in the log, where the
// recall mcp that runs it is read, not only on its standard error.
func TestStoppedWatcherLogged(t *testing.T) {
	var log bytes.Buffer
	missing := filepath.Join(t.TempDir(), "projects")
	w := &Watcher{ProjectsDirs: []string{missing}, Log: io.Discard, Logger: logging.New(&log, slog.LevelInfo)}
	w.Run(context.Background())
	if !strings.Contains(log.String(), "not watching") || !strings.Contains(log.String(), missing) {
		t.Errorf("log:\n%s", log.String())
	}
}
