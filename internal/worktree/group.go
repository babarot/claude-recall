package worktree

import (
	"strings"
	"time"
)

// remoteKey prefixes the key of a folder that stands for a git remote, so
// it is never a directory's key.
const remoteKey = "remote:"

// Member is a session as RemoteGroups sees it: the key of the folder it
// ran in (Repo.Key, after SettleKeys), the remote recorded for it, and
// when it ended.
type Member struct {
	Key    string
	Remote string
	Ended  time.Time
}

// Group is a folder the list narrows by, as Key and Name of a Repo are.
type Group struct{ Key, Name string }

// RemoteGroups maps the key of each folder whose sessions recorded a git
// remote to the group of that remote, so the checkouts of one repository,
// wherever they are, are one folder. A folder's remote is the one of its
// latest session that has one, since a remote can change, as when a
// repository is renamed. A folder no session of which recorded a remote
// is left out and stays as it is.
func RemoteGroups(members []Member) map[string]Group {
	type pick struct {
		remote string
		ended  time.Time
	}
	latest := map[string]pick{}
	for _, m := range members {
		if m.Remote == "" {
			continue
		}
		if p, ok := latest[m.Key]; !ok || m.Ended.After(p.ended) {
			latest[m.Key] = pick{m.Remote, m.Ended}
		}
	}
	out := make(map[string]Group, len(latest))
	for key, p := range latest {
		out[key] = Group{Key: remoteKey + p.remote, Name: RemoteName(p.remote)}
	}
	return out
}

// RemoteName is how a remote shows as a folder: owner/repo on GitHub, and
// host/path elsewhere.
func RemoteName(remote string) string {
	return strings.TrimPrefix(remote, "github.com/")
}
