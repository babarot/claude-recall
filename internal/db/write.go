package db

import (
	"cmp"
	"database/sql"
	"errors"
	"fmt"
)

// FileInfo is what the importer stored about a transcript file, for change
// detection.
type FileInfo struct {
	MessageCount int
	FileMtime    *float64 // milliseconds since the epoch
	FileSize     *int64
	// HasTitle is false for a session imported before titles were stored.
	HasTitle bool
	// ImportVersion is the importer's version that stored the session, 0
	// before versions were stored.
	ImportVersion int
}

// GetFileInfo returns the stored file metadata of a session, or nil when the
// session is not archived.
func (d *DB) GetFileInfo(sessionID string) (*FileInfo, error) {
	var fi FileInfo
	err := d.sql.QueryRow(`SELECT message_count, file_mtime, file_size, title IS NOT NULL, COALESCE(import_version, 0)
      FROM sessions WHERE session_id = ?`, sessionID).
		Scan(&fi.MessageCount, &fi.FileMtime, &fi.FileSize, &fi.HasTitle, &fi.ImportVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("file info: %w", err)
	}
	return &fi, nil
}

// SessionRow is a row of the sessions table.
type SessionRow struct {
	SessionID     string
	Project       string
	ProjectPath   string
	GitBranch     string
	FirstPrompt   string
	Summary       *string
	MessageCount  int
	StartedAt     string
	EndedAt       string
	ClaudeVersion string
	FileMtime     *float64
	FileSize      *int64
	Title         string // "" when the session has no title
	// Remote is the origin of the session's repository, "" when it has
	// none, or nil when it cannot be known now; ReplaceSession then keeps
	// what was stored.
	Remote *string
	// ImportVersion is the importer's version storing the session.
	ImportVersion int
}

// MessageRow is a row of the messages table.
type MessageRow struct {
	UUID       string
	Role       string
	BlockType  string
	BlockIndex int
	Content    string
	ToolName   *string
	ToolInput  *string
	ToolUseID  *string
	IsError    *bool
	Timestamp  string
	TurnIndex  int
}

// UsageRow is a row of the usage table: the tokens of one API response.
type UsageRow struct {
	MessageID    string
	Agent        string // the subagent's transcript, "" for the session's own
	Model        string
	Timestamp    string
	Input        int64
	Output       int64
	CacheRead    int64
	CacheWrite5m int64
	CacheWrite1h int64
	Speed        string // "fast" or "standard"
}

// ImageRow is a row of the images table.
type ImageRow struct {
	MessageUUID string
	ImageIndex  int
	MediaType   string
	Data        []byte
}

// ReplaceSession deletes everything stored for a session and inserts the
// given rows, in one transaction, except usage, which it adds to what is
// stored (see addUsage), so a reader or a concurrent import never
// sees the session half written. check, when not nil, is called once the
// transaction holds the write lock, so no other import can commit before
// this one; an error from it rolls the transaction back and is returned as
// it is.
func (d *DB) ReplaceSession(s SessionRow, msgs []MessageRow, imgs []ImageRow, usage []UsageRow, check func() error) (err error) {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("replace session: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback() // err is the one to report
		}
	}()
	if check != nil {
		if err = check(); err != nil {
			return err
		}
	}

	remote := s.Remote
	if remote == nil {
		// The directory may be gone since the session was first imported;
		// what was read from it then still holds.
		if err = tx.QueryRow(`SELECT remote FROM sessions WHERE session_id = ?`, s.SessionID).Scan(&remote); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("replace session: %w", err)
		}
		err = nil
	}

	for _, q := range []string{
		`DELETE FROM images WHERE session_id = ?`,
		`DELETE FROM messages WHERE session_id = ?`,
		`DELETE FROM sessions WHERE session_id = ?`,
	} {
		if _, err = tx.Exec(q, s.SessionID); err != nil {
			return fmt.Errorf("replace session: %w", err)
		}
	}

	if _, err = tx.Exec(`INSERT INTO sessions (session_id, project, project_path, git_branch, first_prompt, summary, message_count, started_at, ended_at, claude_version, file_mtime, file_size, title, remote, import_version)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.SessionID, s.Project, s.ProjectPath, s.GitBranch, s.FirstPrompt, s.Summary, s.MessageCount,
		s.StartedAt, s.EndedAt, s.ClaudeVersion, s.FileMtime, s.FileSize, s.Title, remote, s.ImportVersion); err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	msgStmt, err := tx.Prepare(`INSERT OR IGNORE INTO messages (session_id, uuid, role, block_type, block_index, content, tool_name, tool_input, tool_use_id, is_error, timestamp, turn_index)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("insert messages: %w", err)
	}
	defer msgStmt.Close()
	for _, m := range msgs {
		if _, err = msgStmt.Exec(s.SessionID, m.UUID, m.Role, m.BlockType, m.BlockIndex, m.Content,
			m.ToolName, m.ToolInput, m.ToolUseID, m.IsError, m.Timestamp, m.TurnIndex); err != nil {
			return fmt.Errorf("insert message: %w", err)
		}
	}

	imgStmt, err := tx.Prepare(`INSERT INTO images (session_id, message_uuid, image_index, media_type, data)
       VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("insert images: %w", err)
	}
	defer imgStmt.Close()
	for _, img := range imgs {
		if _, err = imgStmt.Exec(s.SessionID, img.MessageUUID, img.ImageIndex, img.MediaType, img.Data); err != nil {
			return fmt.Errorf("insert image: %w", err)
		}
	}

	if err = addUsage(tx, s.SessionID, usage); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("replace session: %w", err)
	}
	return nil
}

// addUsage stores the usage of a session's responses. A response already
// stored is updated rather than added again: a subagent's transcript read
// while the response was still being written has a lower output count, put
// right by the next import, so the output kept is the highest seen. Nothing
// is deleted, so the usage of a transcript that is gone stays.
func addUsage(tx *sql.Tx, sessionID string, usage []UsageRow) error {
	if len(usage) == 0 {
		return nil
	}
	stmt, err := tx.Prepare(`INSERT INTO usage (session_id, message_id, agent, model, timestamp, input, output, cache_read, cache_write_5m, cache_write_1h, speed)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
       ON CONFLICT (session_id, message_id) DO UPDATE SET
         model = excluded.model, timestamp = COALESCE(usage.timestamp, excluded.timestamp),
         input = excluded.input, output = max(usage.output, excluded.output), cache_read = excluded.cache_read,
         cache_write_5m = excluded.cache_write_5m, cache_write_1h = excluded.cache_write_1h, speed = excluded.speed`)
	if err != nil {
		return fmt.Errorf("insert usage: %w", err)
	}
	defer stmt.Close()
	for _, u := range usage {
		if _, err := stmt.Exec(sessionID, u.MessageID, u.Agent, u.Model, nullIfEmpty(u.Timestamp), u.Input, u.Output,
			u.CacheRead, u.CacheWrite5m, u.CacheWrite1h, cmp.Or(u.Speed, "standard")); err != nil {
			return fmt.Errorf("insert usage: %w", err)
		}
	}
	return nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ProjectPathsWithoutRemote returns the directories of the sessions whose
// remote is not known yet.
func (d *DB) ProjectPathsWithoutRemote() ([]string, error) {
	rows, err := d.sql.Query(`SELECT DISTINCT project_path FROM sessions
      WHERE remote IS NULL AND project_path IS NOT NULL AND project_path != ''`)
	if err != nil {
		return nil, fmt.Errorf("paths without remote: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("paths without remote: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetRemote stores remote for the sessions run in projectPath whose remote
// is not known yet, leaving the known ones as they are.
func (d *DB) SetRemote(projectPath, remote string) error {
	if _, err := d.sql.Exec(`UPDATE sessions SET remote = ? WHERE project_path = ? AND remote IS NULL`, remote, projectPath); err != nil {
		return fmt.Errorf("set remote: %w", err)
	}
	return nil
}

// Exec runs a statement. It exists for tests and maintenance.
func (d *DB) Exec(query string, args ...any) (sql.Result, error) { return d.sql.Exec(query, args...) }

// Usage returns the usage stored for a session, ordered by message id.
func (d *DB) Usage(sessionID string) ([]UsageRow, error) {
	rows, err := d.sql.Query(`SELECT message_id, agent, model, COALESCE(timestamp, ''), input, output, cache_read, cache_write_5m, cache_write_1h, speed
      FROM usage WHERE session_id = ? ORDER BY message_id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("usage: %w", err)
	}
	defer rows.Close()
	var out []UsageRow
	for rows.Next() {
		var u UsageRow
		if err := rows.Scan(&u.MessageID, &u.Agent, &u.Model, &u.Timestamp, &u.Input, &u.Output, &u.CacheRead, &u.CacheWrite5m, &u.CacheWrite1h, &u.Speed); err != nil {
			return nil, fmt.Errorf("usage: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
