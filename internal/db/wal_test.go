package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWALShrinksAfterReset checks that a WAL grown past journalSizeLimit
// by a large write is cut back once a checkpoint lets the next write reset
// it, instead of keeping its largest size for good.
func TestWALShrinksAfterReset(t *testing.T) {
	old := journalSizeLimit
	journalSizeLimit = 64 << 10
	t.Cleanup(func() { journalSizeLimit = old })

	path := filepath.Join(t.TempDir(), "vault.db")
	d, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	walSize := func() int64 {
		t.Helper()
		fi, err := os.Stat(path + "-wal")
		if err != nil {
			t.Fatal(err)
		}
		return fi.Size()
	}

	seedSession(t, d, "s1", "p", "/p")
	if _, err := d.sql.Exec(`PRAGMA wal_autocheckpoint=0`); err != nil {
		t.Fatal(err)
	}
	tx, err := d.sql.Begin()
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Repeat("x", 4096)
	for i := range 200 {
		if _, err := tx.Exec(`INSERT INTO messages (session_id, uuid, role, block_type, block_index, content, timestamp, turn_index)
			VALUES ('s1', ?, 'user', 'text', 0, ?, ?, 0)`, i, body, ts); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if got := walSize(); got <= journalSizeLimit {
		t.Fatalf("WAL is %d bytes after the large write; the test needs it past %d", got, journalSizeLimit)
	}

	if _, err := d.sql.Exec(`PRAGMA wal_checkpoint(PASSIVE)`); err != nil {
		t.Fatal(err)
	}
	seedSession(t, d, "s2", "p", "/p") // resets the WAL
	if got := walSize(); got > journalSizeLimit {
		t.Errorf("WAL is %d bytes after reset; want at most %d", got, journalSizeLimit)
	}
}
