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

// A failed request is logged by its path; what it carried beside its path,
// which its error may echo, is left out.
func TestFailedRequestLogged(t *testing.T) {
	var log bytes.Buffer
	s := newServer(t)
	s.Log = logging.New(&log, slog.LevelDebug)
	// Through the server: a value that cannot be read is echoed in the
	// error the API returns.
	if rec := get(t, s, "/api/sessions?limit=secretword"); rec.Code < 400 {
		t.Fatalf("the request did not fail: %d", rec.Code)
	}
	s.logFailed(httptest.NewRequest("GET", "/api/v2/sessions/abc", nil), errors.New("database is locked"))
	got := log.String()
	if strings.Contains(got, "secretword") {
		t.Errorf("what the request carried is in the log:\n%s", got)
	}
	for _, want := range []string{`"path":"/api/sessions"`, `"path":"/api/v2/sessions/abc"`, `"err":"database is locked"`} {
		if !strings.Contains(got, want) {
			t.Errorf("no %s in:\n%s", want, got)
		}
	}
}
