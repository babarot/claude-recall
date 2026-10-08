package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		source, exe string
		want        Method
	}{
		{"release", "/home/u/.local/bin/recall", Binary},
		{"release", "/usr/local/bin/recall", Binary},
		{"", "/home/u/.local/bin/recall", Source},
		{"", "/nix/store/abc-claude-recall-1.7.2/bin/recall", Source},
		{"release", "/nix/store/abc-claude-recall-1.7.2/bin/recall", Nix},
		{"release", "/opt/homebrew/Cellar/claude-recall/1.7.2/bin/recall", Homebrew},
		{"release", "/usr/local/Cellar/claude-recall/1.7.2/bin/recall", Homebrew},
		{"release", "/home/linuxbrew/.linuxbrew/bin/recall", Homebrew},
	}
	for _, c := range cases {
		if got := Detect(c.source, c.exe); got != c.want {
			t.Errorf("Detect(%q, %q) = %v, want %v", c.source, c.exe, got, c.want)
		}
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.8.0", "1.7.2", true},
		{"1.7.10", "1.7.9", true},
		{"2.0.0", "1.99.99", true},
		{"1.7.2", "1.7.2", false},
		{"1.7.1", "1.7.2", false},
		{"v1.8.0", "1.7.2", true},
	}
	for _, c := range cases {
		got, err := Newer(c.a, c.b)
		if err != nil || got != c.want {
			t.Errorf("Newer(%q, %q) = %v, %v, want %v", c.a, c.b, got, err, c.want)
		}
	}
	for _, v := range []string{"", "1.8", "1.8.0-rc1", "latest"} {
		if _, err := Newer(v, "1.7.2"); err == nil {
			t.Errorf("Newer(%q) took a malformed version", v)
		}
	}
}

func TestAsset(t *testing.T) {
	if a, _ := Asset("darwin", "arm64"); a != "claude-recall-darwin-arm64" {
		t.Errorf("darwin/arm64: %q", a)
	}
	if a, _ := Asset("linux", "amd64"); a != "claude-recall-linux-x86_64" {
		t.Errorf("linux/amd64: %q", a)
	}
	if _, err := Asset("windows", "amd64"); err == nil {
		t.Error("windows has no release binary")
	}
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/latest" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/releases/tag/1.8.0", http.StatusFound)
	}))
	defer srv.Close()
	tag, err := (&Client{BaseURL: srv.URL}).Latest(context.Background())
	if err != nil || tag != "1.8.0" {
		t.Fatalf("got %q, %v", tag, err)
	}

	none := httptest.NewServer(http.NotFoundHandler())
	defer none.Close()
	if _, err := (&Client{BaseURL: none.URL}).Latest(context.Background()); err == nil {
		t.Error("a repository without releases has no latest tag")
	}
}

// release serves one asset and checksums.txt for tag 1.8.0. A nil body
// leaves that file out.
func release(t *testing.T, asset, sums []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte
		switch r.URL.Path {
		case "/releases/download/1.8.0/claude-recall-linux-x86_64":
			body = asset
		case "/releases/download/1.8.0/checksums.txt":
			body = sums
		}
		if body == nil {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// installed writes the old binary and returns its path.
func installed(t *testing.T) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "recall")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	return target
}

// untouched checks that target is still the old binary and nothing else
// was left beside it.
func untouched(t *testing.T, target string) {
	t.Helper()
	if b, _ := os.ReadFile(target); string(b) != "old" {
		t.Errorf("target changed to %q", b)
	}
	entries, _ := os.ReadDir(filepath.Dir(target))
	if len(entries) != 1 {
		t.Errorf("left behind: %v", entries)
	}
}

const asset = "claude-recall-linux-x86_64"

func TestApply(t *testing.T) {
	bin := []byte("new")
	srv := release(t, bin, []byte(sum([]byte("other"))+"  claude-recall-darwin-arm64\n"+sum(bin)+"  "+asset+"\n"))
	target := installed(t)
	var verified string
	err := (&Client{BaseURL: srv.URL}).Apply(context.Background(), "1.8.0", asset, target, func(p string) error {
		verified = p
		if b, _ := os.ReadFile(p); string(b) != "new" {
			t.Errorf("verified %q", b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(target); string(b) != "new" {
		t.Errorf("target is %q", b)
	}
	if info, _ := os.Stat(target); info.Mode().Perm() != 0o755 {
		t.Errorf("mode %v", info.Mode())
	}
	if filepath.Dir(verified) != filepath.Dir(target) {
		t.Errorf("verified %s, not beside the target", verified)
	}
	if entries, _ := os.ReadDir(filepath.Dir(target)); len(entries) != 1 {
		t.Errorf("left behind: %v", entries)
	}
}

func TestApplyFailures(t *testing.T) {
	bin := []byte("new")
	ok := func(string) error { return nil }
	cases := []struct {
		name      string
		asset     []byte
		sums      []byte
		verify    func(string) error
		wantErr   string
		published bool
	}{
		{"checksum mismatch", bin, []byte(sum([]byte("evil")) + "  " + asset + "\n"), ok, "checksum mismatch", true},
		{"no checksum line", bin, []byte(sum(bin) + "  claude-recall-darwin-arm64\n"), ok, "no checksum", true},
		{"asset missing", nil, []byte(sum(bin) + "  " + asset + "\n"), ok, "", false},
		{"checksums missing", bin, nil, ok, "", false},
		{"verify fails", bin, []byte(sum(bin) + "  " + asset + "\n"), func(string) error { return errors.New("broken binary") }, "broken binary", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := release(t, c.asset, c.sums)
			target := installed(t)
			err := (&Client{BaseURL: srv.URL}).Apply(context.Background(), "1.8.0", asset, target, c.verify)
			if err == nil {
				t.Fatal("no error")
			}
			if !c.published && !errors.Is(err, ErrNotPublished) {
				t.Errorf("err %v, want ErrNotPublished", err)
			}
			if c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("err %v, want %q", err, c.wantErr)
			}
			untouched(t, target)
		})
	}
}

func TestApplyUnwritableDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root writes anywhere")
	}
	bin := []byte("new")
	srv := release(t, bin, []byte(sum(bin)+"  "+asset+"\n"))
	target := installed(t)
	dir := filepath.Dir(target)
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	err := (&Client{BaseURL: srv.URL}).Apply(context.Background(), "1.8.0", asset, target, func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "cannot write to "+dir) {
		t.Fatalf("err %v", err)
	}
	untouched(t, target)
}
