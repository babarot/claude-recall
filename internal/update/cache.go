package update

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CheckEvery is how long a look for the latest release is trusted.
const CheckEvery = 24 * time.Hour

// CheckTimeout bounds a look made for a notice, so a slow network does
// not hold up what the user asked for.
const CheckTimeout = 3 * time.Second

// Cache is what was last learned about the latest release.
type Cache struct {
	CheckedAt time.Time `json:"checked_at"`
	// Latest is the latest release's version, empty if it was never found.
	Latest string `json:"latest,omitempty"`
}

// Stale reports whether it is time to look again.
func (c Cache) Stale(now time.Time) bool { return now.Sub(c.CheckedAt) >= CheckEvery }

// Newer returns the latest release when it is newer than current.
func (c Cache) Newer(current string) (string, bool) {
	if newer, err := Newer(c.Latest, current); err != nil || !newer {
		return "", false
	}
	return c.Latest, true
}

// Checks reports whether this build looks for new releases: a release
// build, with update_check on in the config file and RECALL_NO_UPDATE_CHECK
// unset. A build from source does not; it would be told of the release it
// was built after.
func Checks(source string, configOn bool) bool {
	return source == "release" && configOn && os.Getenv("RECALL_NO_UPDATE_CHECK") == ""
}

// LoadCache reads the cache. A missing or unreadable one is empty, and so
// stale.
func LoadCache(path string) Cache {
	var c Cache
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c
}

// SaveCache writes the cache, replacing it atomically.
func SaveCache(path string, c Cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".update-*.json")
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		os.Remove(f.Name())
	}
	return err
}

// Refresh looks for the latest release and saves what it found in the
// cache at path. A failed look is saved too, keeping the latest release
// known before, so an offline machine does not try again on every start.
func (c *Client) Refresh(ctx context.Context, path string) (Cache, error) {
	cache := LoadCache(path)
	cache.CheckedAt = time.Now()
	latest, err := c.Latest(ctx)
	if err == nil {
		cache.Latest = latest
	}
	if serr := SaveCache(path, cache); err == nil {
		err = serr
	}
	return cache, err
}
