package db

import (
	"context"
	"fmt"
	"iter"
	"time"
)

// timestampLayout is how Claude Code writes a message's timestamp, which
// the archive stores as text: compared as strings, a bound in this layout
// orders the same as the time it stands for.
const timestampLayout = "2006-01-02T15:04:05.000Z"

func timestampBound(t time.Time) string { return t.UTC().Format(timestampLayout) }

// SessionsIn returns the sessions with a message whose timestamp is in
// [from, to), earliest started first.
func (d *DB) SessionsIn(ctx context.Context, from, to time.Time) ([]Session, error) {
	titleExpr := "''"
	if ok, err := d.hasColumn("sessions", "title"); err != nil {
		return nil, fmt.Errorf("sessions in: %w", err)
	} else if ok {
		titleExpr = "COALESCE(title, '')"
	}
	remote, err := d.optionalColumn("remote", "remote")
	if err != nil {
		return nil, fmt.Errorf("sessions in: %w", err)
	}

	rows, err := d.sql.QueryContext(ctx, `
      SELECT session_id, project, COALESCE(project_path, ''), COALESCE(git_branch, ''),
             COALESCE(first_prompt, ''), `+titleExpr+`, COALESCE(message_count, 0),
             COALESCE(file_size, 0), COALESCE(started_at, ''), COALESCE(ended_at, ''),
             COALESCE(claude_version, ''), COALESCE(`+remote+`, '')
      FROM sessions s
      WHERE EXISTS (SELECT 1 FROM messages m
                    WHERE m.session_id = s.session_id AND m.timestamp >= ? AND m.timestamp < ?)
      ORDER BY started_at, session_id`, timestampBound(from), timestampBound(to))
	if err != nil {
		return nil, fmt.Errorf("sessions in: %w", err)
	}
	defer rows.Close()

	var out []Session
	for rows.Next() {
		var s Session
		var started, ended string
		if err := rows.Scan(&s.ID, &s.Project, &s.ProjectPath, &s.GitBranch, &s.FirstPrompt,
			&s.Title, &s.MessageCount, &s.FileSize, &started, &ended, &s.ClaudeVersion, &s.Remote); err != nil {
			return nil, fmt.Errorf("sessions in: %w", err)
		}
		s.StartedAt, s.EndedAt = parseTime(started), parseTime(ended)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sessions in: %w", err)
	}
	return out, nil
}

// Block is one stored block of a message, of any kind.
type Block struct {
	SessionID string
	Turn      int
	Index     int
	UUID      string
	Timestamp time.Time
	Role      string
	Kind      string // block_type
	Content   string
	ToolName  string
	ToolInput string
}

// BlocksIn yields the blocks whose timestamp is in [from, to), of the
// session sessionID or, when it is empty, of every session: by session,
// then in conversation order. The rows are read as they are yielded, and
// let go of when the loop ends, early or not. An error ends the sequence.
func (d *DB) BlocksIn(ctx context.Context, sessionID string, from, to time.Time) iter.Seq2[Block, error] {
	return func(yield func(Block, error) bool) {
		q := `SELECT session_id, COALESCE(turn_index, 0), block_index, uuid, timestamp, role, block_type,
                     content, COALESCE(tool_name, ''), COALESCE(tool_input, '')
              FROM messages
              WHERE timestamp >= ? AND timestamp < ?`
		args := []any{timestampBound(from), timestampBound(to)}
		if sessionID != "" {
			q += ` AND session_id = ?`
			args = append(args, sessionID)
		}
		q += ` ORDER BY session_id, turn_index, block_index`

		rows, err := d.sql.QueryContext(ctx, q, args...)
		if err != nil {
			yield(Block{}, fmt.Errorf("blocks in: %w", err))
			return
		}
		defer rows.Close()
		for rows.Next() {
			var b Block
			var ts string
			if err := rows.Scan(&b.SessionID, &b.Turn, &b.Index, &b.UUID, &ts, &b.Role, &b.Kind,
				&b.Content, &b.ToolName, &b.ToolInput); err != nil {
				yield(Block{}, fmt.Errorf("blocks in: %w", err))
				return
			}
			b.Timestamp = parseTime(ts)
			if !yield(b, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(Block{}, fmt.Errorf("blocks in: %w", err))
		}
	}
}
