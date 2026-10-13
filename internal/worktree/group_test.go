package worktree

import (
	"testing"
	"time"
)

func TestRemoteGroups(t *testing.T) {
	at := func(h int) time.Time { return time.Date(2026, 10, 1, h, 0, 0, 0, time.UTC) }
	got := RemoteGroups([]Member{
		{Key: "/work/api", Remote: "github.com/acme/api", Ended: at(1)},
		{Key: "/work/api", Remote: "", Ended: at(5)}, // stored before remotes were
		{Key: "/tmp/api-hotfix", Remote: "github.com/acme/api", Ended: at(2)},
		// Renamed: the latest remote wins.
		{Key: "/work/old-name", Remote: "github.com/acme/old-name", Ended: at(1)},
		{Key: "/work/old-name", Remote: "github.com/acme/billing", Ended: at(3)},
		{Key: "/work/self-hosted", Remote: "git.example.com/team/tool", Ended: at(1)},
		{Key: "/home/me", Remote: "", Ended: at(4)},
	})
	want := map[string]Group{
		"/work/api":         {"remote:github.com/acme/api", "acme/api"},
		"/tmp/api-hotfix":   {"remote:github.com/acme/api", "acme/api"},
		"/work/old-name":    {"remote:github.com/acme/billing", "acme/billing"},
		"/work/self-hosted": {"remote:git.example.com/team/tool", "git.example.com/team/tool"},
	}
	if len(got) != len(want) {
		t.Fatalf("RemoteGroups = %v, want %v", got, want)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("RemoteGroups[%q] = %v, want %v", k, got[k], w)
		}
	}
}
