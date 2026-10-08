package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheStale(t *testing.T) {
	now := time.Now()
	if !(Cache{}).Stale(now) {
		t.Error("an empty cache is stale")
	}
	if (Cache{CheckedAt: now.Add(-time.Hour)}).Stale(now) {
		t.Error("an hour old is fresh")
	}
	if !(Cache{CheckedAt: now.Add(-25 * time.Hour)}).Stale(now) {
		t.Error("a day old is stale")
	}
}

func TestCacheNewer(t *testing.T) {
	if v, ok := (Cache{Latest: "1.8.0"}).Newer("1.7.2"); !ok || v != "1.8.0" {
		t.Errorf("got %q, %v", v, ok)
	}
	for _, latest := range []string{"", "1.7.2", "1.7.1", "garbage"} {
		if _, ok := (Cache{Latest: latest}).Newer("1.7.2"); ok {
			t.Errorf("%q is not newer than 1.7.2", latest)
		}
	}
}

func TestChecks(t *testing.T) {
	t.Setenv("RECALL_NO_UPDATE_CHECK", "")
	if !Checks("release", true) {
		t.Error("a release build checks by default")
	}
	if Checks("", true) {
		t.Error("a build from source does not check")
	}
	if Checks("release", false) {
		t.Error("update_check = false turns it off")
	}
	t.Setenv("RECALL_NO_UPDATE_CHECK", "1")
	if Checks("release", true) {
		t.Error("RECALL_NO_UPDATE_CHECK turns it off")
	}
}

func TestLoadCache(t *testing.T) {
	dir := t.TempDir()
	if c := LoadCache(filepath.Join(dir, "missing.json")); !c.CheckedAt.IsZero() || c.Latest != "" {
		t.Errorf("missing: %+v", c)
	}
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte("{"), 0o644)
	if c := LoadCache(bad); !c.CheckedAt.IsZero() {
		t.Errorf("unreadable: %+v", c)
	}
	path := filepath.Join(dir, "sub", "update.json")
	want := Cache{CheckedAt: time.Now().Truncate(time.Second).UTC(), Latest: "1.8.0"}
	if err := SaveCache(path, want); err != nil {
		t.Fatal(err)
	}
	if got := LoadCache(path); !got.CheckedAt.Equal(want.CheckedAt) || got.Latest != want.Latest {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestRefresh(t *testing.T) {
	up := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !up {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		http.Redirect(w, r, "/releases/tag/1.8.0", http.StatusFound)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL}
	path := filepath.Join(t.TempDir(), "update.json")

	got, err := c.Refresh(context.Background(), path)
	if err != nil || got.Latest != "1.8.0" || got.Stale(time.Now()) {
		t.Fatalf("got %+v, %v", got, err)
	}

	// A failed look keeps what was known and is not tried again for a day.
	SaveCache(path, Cache{CheckedAt: time.Now().Add(-48 * time.Hour), Latest: "1.8.0"})
	up = false
	got, err = c.Refresh(context.Background(), path)
	if err == nil {
		t.Error("no error from a failed look")
	}
	if saved := LoadCache(path); saved.Latest != "1.8.0" || saved.Stale(time.Now()) || saved.Latest != got.Latest {
		t.Errorf("saved %+v, returned %+v", saved, got)
	}
}
