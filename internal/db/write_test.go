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
	if err := d.ReplaceSession(row(1), []MessageRow{msg("u1")}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	stale := errors.New("stale")
	err := d.ReplaceSession(row(2), []MessageRow{msg("u1"), msg("u2")}, nil, nil, func() error { return stale })
	if !errors.Is(err, stale) {
		t.Fatalf("ReplaceSession = %v, want the check's error", err)
	}
	fi, err := d.GetFileInfo("s1")
	if err != nil || fi == nil || fi.MessageCount != 1 {
		t.Fatalf("after a failed check: %+v, %v; want the stored session untouched", fi, err)
	}
	var n int
	if err := d.sql.QueryRow(`SELECT count(*) FROM messages WHERE session_id = 's1'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("%d messages stored after a failed check; want 1", n)
	}
}

// Usage is added to, not replaced: a response stored again is updated in
// place, a lower output count does not lower it, and a later import without
// the response keeps it.
func TestReplaceSessionKeepsUsage(t *testing.T) {
	d := newTestDB(t)
	row := SessionRow{SessionID: "s1", Project: "p"}
	u := func(id string, out int64) UsageRow {
		return UsageRow{MessageID: id, Model: "claude-opus-5-5", Timestamp: "2026-01-01T00:00:00Z", Input: 1, Output: out, CacheRead: 3}
	}
	steps := []struct {
		usage []UsageRow
		want  map[string]int64
	}{
		{[]UsageRow{u("m1", 10), u("m2", 5)}, map[string]int64{"m1": 10, "m2": 5}},
		{[]UsageRow{u("m1", 10), u("m2", 800)}, map[string]int64{"m1": 10, "m2": 800}},
		{[]UsageRow{u("m2", 17)}, map[string]int64{"m1": 10, "m2": 800}},
		{nil, map[string]int64{"m1": 10, "m2": 800}},
	}
	for i, s := range steps {
		if err := d.ReplaceSession(row, nil, nil, s.usage, nil); err != nil {
			t.Fatal(err)
		}
		got, err := d.Usage("s1")
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]int64{}
		for _, g := range got {
			out[g.MessageID] = g.Output
		}
		if len(out) != len(s.want) || out["m1"] != s.want["m1"] || out["m2"] != s.want["m2"] {
			t.Errorf("step %d: output by response %v, want %v", i, out, s.want)
		}
	}
}

// Usage is summed by model and by kind of response: fast mode, and prompts
// over the length given.
func TestUsageByModel(t *testing.T) {
	d := newTestDB(t)
	u := func(id, model, speed string, input int64) UsageRow {
		return UsageRow{MessageID: id, Model: model, Speed: speed, Input: input, Output: 1}
	}
	if err := d.ReplaceSession(SessionRow{SessionID: "s1", Project: "p"}, nil, nil, []UsageRow{
		u("m1", "a", "standard", 10), u("m2", "a", "standard", 20), u("m3", "a", "fast", 30), u("m4", "a", "standard", 500), u("m5", "b", "standard", 1),
	}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := d.UsageByModel([]string{"s1", "none"}, 100)
	if err != nil {
		t.Fatal(err)
	}
	want := []ModelUsage{
		{Model: "a", Input: 30, Output: 2},
		{Model: "a", Long: true, Input: 500, Output: 1},
		{Model: "a", Fast: true, Input: 30, Output: 1},
		{Model: "b", Input: 1, Output: 1},
	}
	if len(got) != 1 || len(got["s1"]) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, w := range want {
		if got["s1"][i] != w {
			t.Errorf("[%d] = %+v, want %+v", i, got["s1"][i], w)
		}
	}
}
