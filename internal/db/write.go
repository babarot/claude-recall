package db

import (
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

// ImageRow is a row of the images table.
type ImageRow struct {
	MessageUUID string
	ImageIndex  int
	MediaType   string
	Data        []byte
}

// ReplaceSession deletes everything stored for a session and inserts the
// given rows, in one transaction, so a reader or a concurrent import never
// sees the session half written. check, when not nil, is called once the
// transaction holds the write lock, so no other import can commit before
// this one; an error from it rolls the transaction back and is returned as
// it is.
func (d *DB) ReplaceSession(s SessionRow, msgs []MessageRow, imgs []ImageRow, check func() error) (err error) {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("replace session: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
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

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("replace session: %w", err)
	}
	return nil
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
