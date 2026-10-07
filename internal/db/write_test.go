package db

import (
	"errors"
	"testing"
)

// A failed check leaves the stored session as it was.
func TestReplaceSessionCheckRollsBack(t *testing.T) {
	d := newTestDB(t)
	row := func(n int) SessionRow { return SessionRow{SessionID: "s1", Project: "p", MessageCount: n} }
	msg := func(uuid string) MessageRow {
		return MessageRow{UUID: uuid, Role: "user", BlockType: "text", Content: uuid}
	}
	if err := d.ReplaceSession(row(1), []MessageRow{msg("u1")}, nil, nil); err != nil {
		t.Fatal(err)
	}

	stale := errors.New("stale")
	err := d.ReplaceSession(row(2), []MessageRow{msg("u1"), msg("u2")}, nil, func() error { return stale })
	if !errors.Is(err, stale) {
		t.Fatalf("ReplaceSession = %v, want the check's error", err)
	}
	fi, err := d.GetFileInfo("s1")
	if err != nil || fi == nil || fi.MessageCount != 1 {
		t.Fatalf("after a failed check: %+v, %v; want the stored session untouched", fi, err)
	}
	var n int
	d.sql.QueryRow(`SELECT count(*) FROM messages WHERE session_id = 's1'`).Scan(&n)
	if n != 1 {
		t.Errorf("%d messages stored after a failed check; want 1", n)
	}
}
