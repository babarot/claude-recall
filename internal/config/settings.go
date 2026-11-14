package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// KnownSettings lists every setting of the config file as table.key, the
// operations under [keys] left out: they come with their keys, which the
// TUI's key list and What's new show.
func KnownSettings() []string { return knownKeys() }

// SettingsPath returns the file that keeps the settings the TUI has known,
// beside the state file, a file of its own as last_version is.
func SettingsPath() string { return filepath.Join(filepath.Dir(StatePath()), "settings.json") }

// SettingsRecord is what the TUI keeps of the settings: every one it has
// known, and the last ones an update added, with the version that found
// them, which the reference marks until an update adds others.
type SettingsRecord struct {
	Known []string `json:"known"`
	New   []string `json:"new,omitempty"`
	Since string   `json:"since,omitempty"`
}

// LoadSettingsRecord reads the record; ok is false when there is none or
// it cannot be read.
func LoadSettingsRecord(path string) (SettingsRecord, bool) {
	var r SettingsRecord
	b, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &r) != nil {
		return SettingsRecord{}, false
	}
	return r, true
}

// RecordSettings compares known, the settings of the running version, with
// the record at path, and returns those it adds. With no record it keeps
// known and adds nothing: a new install, or the update to the first
// version that keeps the record, has nothing it can tell apart as new.
// Names are never dropped, so going back a version and forward again tells
// of nothing twice.
func RecordSettings(path string, known []string, version string) ([]string, error) {
	r, ok := LoadSettingsRecord(path)
	if !ok {
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			// A record that cannot be read is left for a later version to
			// read, rather than replaced by one that tells of nothing again.
			return nil, errors.New("the record of settings cannot be read: " + path)
		}
		return nil, saveJSON(path, SettingsRecord{Known: known})
	}
	var added []string
	for _, k := range known {
		if !slices.Contains(r.Known, k) {
			added = append(added, k)
		}
	}
	if len(added) == 0 {
		return nil, nil
	}
	r.Known = append(r.Known, added...)
	r.New, r.Since = added, version
	return added, saveJSON(path, r)
}

func saveJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
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

// MarkNew puts a line saying which version added it above each of r.New
// in ref, the reference or a table's part of it. The mark is a comment
// line of its own, so a setting's line uncommented from the reference
// still reads.
func MarkNew(ref string, r SettingsRecord) string {
	if len(r.New) == 0 {
		return ref
	}
	var b strings.Builder
	table := ""
	for _, l := range strings.SplitAfter(ref, "\n") {
		if topTable.MatchString(l) {
			table = strings.Trim(strings.TrimSpace(l), "[]")
		}
		if m := settingLine.FindStringSubmatch(l); m != nil && slices.Contains(r.New, table+"."+m[1]) {
			b.WriteString("# (new in " + r.Since + ")\n")
		}
		b.WriteString(l)
	}
	return b.String()
}

// settingLine is a setting as Template writes it, commented out.
var settingLine = regexp.MustCompile(`^# ([a-z_]+) = `)
