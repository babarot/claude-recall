// Package update replaces the installed recall with the latest release from
// GitHub, for installs that the release binary was copied into by
// install.sh. Installs another tool manages (Nix, Homebrew) and builds from
// source are told how to update instead.
package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultBaseURL is the repository the releases are published to.
const DefaultBaseURL = "https://github.com/babarot/claude-recall"

// Method is how recall was installed, which decides whether it may replace
// itself.
type Method int

const (
	// Binary is a release binary install.sh copied into place: the only
	// method recall updates itself.
	Binary Method = iota
	// Source is a build from source (make install, go install).
	Source
	// Nix is the release binary in the Nix store.
	Nix
	// Homebrew is an install under Homebrew's prefix.
	Homebrew
)

// Detect works out the method from the build's version.Source and the path
// of the running binary with its symlinks resolved.
func Detect(source, exe string) Method {
	switch {
	case source != "release":
		return Source
	case strings.HasPrefix(exe, "/nix/store/"):
		return Nix
	case strings.Contains(exe, "/Cellar/"),
		strings.HasPrefix(exe, "/opt/homebrew/"),
		strings.HasPrefix(exe, "/home/linuxbrew/"):
		return Homebrew
	}
	return Binary
}

// Hint says how to update an install of this method.
func (m Method) Hint() string {
	switch m {
	case Source:
		return "recall was built from source; update the checkout and build it again (git pull && make install)."
	case Nix:
		return "recall was installed with Nix; update it with Nix (nix profile upgrade, or your flake)."
	case Homebrew:
		return "recall was installed with Homebrew; update it with: brew upgrade claude-recall"
	}
	return "Run: recall update"
}

// ErrNotPublished is a release whose binaries are not uploaded yet: the
// release workflow creates the release a few minutes before its assets.
var ErrNotPublished = errors.New("the release is still being published")

// Client talks to the releases of a GitHub repository.
type Client struct {
	// BaseURL is the repository, DefaultBaseURL unless a test says
	// otherwise.
	BaseURL string
	// HTTP downloads assets. Nil means http.DefaultClient.
	HTTP *http.Client
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

// Latest returns the tag of the latest release. It reads the redirect of
// /releases/latest rather than asking api.github.com, whose limit of 60
// requests an hour without a token a shared address can use up.
func (c *Client) Latest(ctx context.Context) (string, error) {
	hc := *c.http()
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.BaseURL+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	dir, tag := path.Split(loc)
	if resp.StatusCode/100 != 3 || !strings.HasSuffix(dir, "/releases/tag/") || tag == "" {
		return "", fmt.Errorf("no latest release at %s (%s)", c.BaseURL, resp.Status)
	}
	return tag, nil
}

// Newer reports whether version a is newer than b. Both are three numbers,
// as in 1.7.2.
func Newer(a, b string) (bool, error) {
	pa, err := parse(a)
	if err != nil {
		return false, err
	}
	pb, err := parse(b)
	if err != nil {
		return false, err
	}
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] > pb[i], nil
		}
	}
	return false, nil
}

func parse(v string) ([3]int, error) {
	var p [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return p, fmt.Errorf("malformed version %q", v)
	}
	for i, s := range parts {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return p, fmt.Errorf("malformed version %q", v)
		}
		p[i] = n
	}
	return p, nil
}

// Asset is the name of the release binary for an OS and architecture.
func Asset(goos, goarch string) (string, error) {
	arch := map[string]string{"arm64": "arm64", "amd64": "x86_64"}[goarch]
	if (goos != "darwin" && goos != "linux") || arch == "" {
		return "", fmt.Errorf("no release binary for %s/%s", goos, goarch)
	}
	return "claude-recall-" + goos + "-" + arch, nil
}

// Apply replaces the file at target with the asset of release tag. The
// asset is written to a temporary file beside target, checked against the
// release's checksums.txt, given target's mode, passed to verify (which
// runs it) and renamed over target. Until the rename target is untouched,
// and a process running the old binary keeps it.
func (c *Client) Apply(ctx context.Context, tag, asset, target string, verify func(path string) error) error {
	want, err := c.checksum(ctx, tag, asset)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(target), ".recall-update-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s: %w", filepath.Dir(target), err)
	}
	tmp := f.Name()
	done := false
	defer func() {
		if !done {
			os.Remove(tmp)
		}
	}()

	h := sha256.New()
	err = c.download(ctx, tag, asset, io.MultiWriter(f, h))
	// Closed before it runs: Linux refuses to execute a file open for
	// writing (ETXTBSY).
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", asset, want, got)
	}
	if err := os.Chmod(tmp, info.Mode().Perm()); err != nil {
		return err
	}
	if err := verify(tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		return err
	}
	done = true
	return nil
}

// checksum returns the SHA256 checksums.txt gives for asset.
func (c *Client) checksum(ctx context.Context, tag, asset string) (string, error) {
	var b strings.Builder
	if err := c.download(ctx, tag, "checksums.txt", &b); err != nil {
		return "", err
	}
	sc := bufio.NewScanner(strings.NewReader(b.String()))
	for sc.Scan() {
		if f := strings.Fields(sc.Text()); len(f) == 2 && f[1] == asset {
			return f[0], nil
		}
	}
	return "", fmt.Errorf("no checksum for %s in checksums.txt of %s", asset, tag)
}

func (c *Client) download(ctx context.Context, tag, name string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/releases/download/"+tag+"/"+name, nil)
	if err != nil {
		return err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotPublished
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("download %s: %s", name, resp.Status)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}
