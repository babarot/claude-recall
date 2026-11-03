package db

import (
	"fmt"
	"strings"
	"time"
)

// Scope is one repository to narrow sessions to. A session that recorded a
// remote is in it when that remote is Remote, ignoring case; a session that
// recorded none is in it when it ran in one of Paths. So a path used again
// for another repository does not bring the sessions of the earlier one.
// With neither set, nothing is in it.
type Scope struct {
	Remote string
	Paths  []string
}

// scopeConds is a condition for each of scopes, all of which a session must
// meet, on the columns remote and path (remote is NULL on an archive opened
// read-only before the migration that added it).
func scopeConds(remote, path string, scopes []Scope) ([]string, []any) {
	var conds []string
	var args []any
	for _, sc := range scopes {
		var or []string
		if sc.Remote != "" {
			or = append(or, "COALESCE("+remote+", '') = ? COLLATE NOCASE")
			args = append(args, sc.Remote)
		}
		if len(sc.Paths) > 0 {
			or = append(or, "(COALESCE("+remote+", '') = '' AND "+path+" IN (?"+strings.Repeat(", ?", len(sc.Paths)-1)+"))")
			for _, p := range sc.Paths {
				args = append(args, p)
			}
		}
		if len(or) == 0 {
			or = []string{"0"}
		}
		conds = append(conds, "("+strings.Join(or, " OR ")+")")
	}
	return conds, args
}

// PathRemote is a remote the sessions run in one directory recorded, and
// when the last of them ended.
type PathRemote struct {
	Path   string
	Remote string // "" when they recorded none
	Ended  time.Time
}

// PathRemotes returns, for every directory a session ran in, each remote
// its sessions recorded, the ones that recorded none as "".
func (d *DB) PathRemotes() ([]PathRemote, error) {
	remote, err := d.optionalColumn("remote", "remote")
	if err != nil {
		return nil, fmt.Errorf("path remotes: %w", err)
	}
	rows, err := d.sql.Query(`SELECT project_path, COALESCE(` + remote + `, '') AS r,
             MAX(COALESCE(ended_at, started_at, ''))
      FROM sessions WHERE project_path IS NOT NULL AND project_path != ''
      GROUP BY project_path, r`)
	if err != nil {
		return nil, fmt.Errorf("path remotes: %w", err)
	}
	defer rows.Close()
	var out []PathRemote
	for rows.Next() {
		var p PathRemote
		var ended string
		if err := rows.Scan(&p.Path, &p.Remote, &ended); err != nil {
			return nil, fmt.Errorf("path remotes: %w", err)
		}
		p.Ended = parseTime(ended)
		out = append(out, p)
	}
	return out, rows.Err()
}
