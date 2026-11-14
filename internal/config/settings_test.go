package config

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestRecordSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	// No record: kept, and nothing told.
	if added, err := RecordSettings(path, []string{"tui.a", "tui.b"}, "1.0.0"); err != nil || added != nil {
		t.Fatalf("first: %v, %v", added, err)
	}
	// An update that adds one tells of it, and keeps it as the new one.
	added, err := RecordSettings(path, []string{"tui.a", "tui.b", "ui.c"}, "1.1.0")
	if err != nil || !slices.Equal(added, []string{"ui.c"}) {
		t.Fatalf("added: %v, %v", added, err)
	}
	r, _ := LoadSettingsRecord(path)
	if !slices.Equal(r.New, []string{"ui.c"}) || r.Since != "1.1.0" {
		t.Errorf("record %+v", r)
	}
	// A run that adds none tells of nothing and keeps the new ones marked;
	// one going back a version drops no name.
	if added, err := RecordSettings(path, []string{"tui.a"}, "1.0.0"); err != nil || added != nil {
		t.Fatalf("back: %v, %v", added, err)
	}
	if added, _ := RecordSettings(path, []string{"tui.a", "tui.b", "ui.c"}, "1.1.0"); added != nil {
		t.Errorf("forward again told of %v", added)
	}
	if r, _ := LoadSettingsRecord(path); !slices.Equal(r.New, []string{"ui.c"}) || len(r.Known) != 3 {
		t.Errorf("record %+v", r)
	}
}

// A record that cannot be read is left as it is.
func TestRecordSettingsUnreadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordSettings(path, []string{"tui.a"}, "1.0.0"); err == nil {
		t.Error("expected an error")
	}
	if b, _ := os.ReadFile(path); string(b) != "{not json" {
		t.Errorf("the record was replaced: %q", b)
	}
}

// The mark goes on a line of its own above the settings the last update
// added, in their table only, and the reference uncommented still reads.
func TestMarkNew(t *testing.T) {
	r := SettingsRecord{New: []string{"tui.images", "ui.port"}, Since: "1.16.0"}
	marked := MarkNew(Template, r)
	for _, want := range []string{"# (new in 1.16.0)\n# images = ", "# (new in 1.16.0)\n# port = "} {
		if !strings.Contains(marked, want) {
			t.Errorf("no mark above %q", want)
		}
	}
	if n := strings.Count(marked, "(new in"); n != 2 {
		t.Errorf("%d marks, want 2", n)
	}
	if ui, _ := Reference("ui"); !strings.HasPrefix(MarkNew(ui, r), "[ui]\n") {
		t.Error("a table's part lost its header")
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(uncommented(marked)), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got.Core.DB, got.Keys = "", nil
	if !reflect.DeepEqual(got, Default()) {
		t.Errorf("the marked reference uncommented does not read as the defaults: %+v", got)
	}
	if MarkNew(Template, SettingsRecord{}) != Template {
		t.Error("marked with nothing new")
	}
}

func TestKnownSettingsLeaveOutKeys(t *testing.T) {
	for _, k := range KnownSettings() {
		if strings.HasPrefix(k, "keys.") {
			t.Errorf("%s is a key", k)
		}
	}
}
