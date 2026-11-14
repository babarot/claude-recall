package web

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/babarot/claude-recall/internal/logging"
)

// A failed request is logged by its path; the query string and the error
// of a search, which may hold the words searched for, are left out.
func TestFailedRequestLogged(t *testing.T) {
	var log bytes.Buffer
	s := &Server{Log: logging.New(&log, slog.LevelDebug)}
	s.logFailed(httptest.NewRequest("GET", "/api/v2/search?q=secretword", nil), errors.New(`fts5: syntax error near "secretword"`))
	s.logFailed(httptest.NewRequest("GET", "/api/v2/sessions/abc", nil), errors.New("database is locked"))
	got := log.String()
	if strings.Contains(got, "secretword") {
		t.Errorf("what was searched for is in the log:\n%s", got)
	}
	for _, want := range []string{`"path":"/api/v2/search"`, `"path":"/api/v2/sessions/abc"`, `"err":"database is locked"`} {
		if !strings.Contains(got, want) {
			t.Errorf("no %s in:\n%s", want, got)
		}
	}
}
