package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// The web UI reads a conversation in ranges, since a long session holds
// tens of megabytes: from the start, from the end, and between, and after
// the last message it has as the session grows.

// Cursor is where a message is in its session: the transcript line
// (turn_index) and the block in it (block_index). Messages are in cursor
// order.
type Cursor struct{ Turn, Block int }

// String writes a cursor as "<turn>.<block>".
func (c Cursor) String() string { return strconv.Itoa(c.Turn) + "." + strconv.Itoa(c.Block) }

// ParseCursor reads a cursor String wrote.
func ParseCursor(s string) (Cursor, error) {
	t, b, ok := strings.Cut(s, ".")
	turn, err1 := strconv.Atoi(t)
	block, err2 := strconv.Atoi(b)
	if !ok || err1 != nil || err2 != nil {
		return Cursor{}, fmt.Errorf("not a cursor: %q", s)
	}
	return Cursor{turn, block}, nil
}

// ConvMessage is a message of a conversation as the web UI reads it.
type ConvMessage struct {
	Cursor    Cursor
	UUID      string
	Role      string
	BlockType string
	Content   string
	ToolName  *string
	ToolInput *string
	Timestamp *string
	// ToolUseID ties a tool_result to its tool_use; IsError says whether
	// a tool_result was an error. nil when the archive does not know.
	ToolUseID *string
	IsError   *bool
}

// convColumns selects a ConvMessage. tool_use_id and is_error read as NULL
// on an archive opened read-only before the migrations that added them.
func (d *DB) convColumns() (string, error) {
	useID, err := d.optionalColumnOf("messages", "tool_use_id", "tool_use_id")
	if err != nil {
		return "", err
	}
	isError, err := d.optionalColumnOf("messages", "is_error", "is_error")
	if err != nil {
		return "", err
	}
	return `SELECT turn_index, block_index, uuid, role, block_type, content, tool_name, tool_input, timestamp, ` +
		useID + `, ` + isError + ` FROM messages`, nil
}

// convMessages reads the messages of query, what follows FROM messages.
func (d *DB) convMessages(query string, args ...any) ([]ConvMessage, error) {
	cols, err := d.convColumns()
	if err != nil {
		return nil, err
	}
	rows, err := d.sql.Query(cols+query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ConvMessage{}
	for rows.Next() {
		var m ConvMessage
		if err := rows.Scan(&m.Cursor.Turn, &m.Cursor.Block, &m.UUID, &m.Role, &m.BlockType, &m.Content,
			&m.ToolName, &m.ToolInput, &m.Timestamp, &m.ToolUseID, &m.IsError); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ConversationAfter returns up to limit messages of a session after c, not
// c itself, oldest first; with c nil, from the start.
func (d *DB) ConversationAfter(sessionID string, c *Cursor, limit int) ([]ConvMessage, error) {
	q, args := ` WHERE session_id = ?`, []any{sessionID}
	if c != nil {
		q, args = q+` AND (turn_index, block_index) > (?, ?)`, append(args, c.Turn, c.Block)
	}
	msgs, err := d.convMessages(q+` ORDER BY turn_index, block_index LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, fmt.Errorf("conversation: %w", err)
	}
	return msgs, nil
}

// ConversationBefore returns up to limit messages of a session before c,
// not c itself, oldest first; with c nil, the last ones.
func (d *DB) ConversationBefore(sessionID string, c *Cursor, limit int) ([]ConvMessage, error) {
	q, args := ` WHERE session_id = ?`, []any{sessionID}
	if c != nil {
		q, args = q+` AND (turn_index, block_index) < (?, ?)`, append(args, c.Turn, c.Block)
	}
	msgs, err := d.convMessages(q+` ORDER BY turn_index DESC, block_index DESC LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, fmt.Errorf("conversation: %w", err)
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// MessageTotal is how many messages a session has.
func (d *DB) MessageTotal(sessionID string) (int, error) {
	var n int
	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM messages WHERE session_id = ?`, sessionID).Scan(&n); err != nil {
		return 0, fmt.Errorf("message total: %w", err)
	}
	return n, nil
}

// Anchor tells whether the messages up to c are still the ones a reader
// has: the message at c has uuid, and there are upTo messages up to it, c
// included. A session imported again after its transcript was rewritten
// can have other messages there.
func (d *DB) Anchor(sessionID string, c Cursor, uuid string, upTo int) (bool, error) {
	var at string
	err := d.sql.QueryRow(`SELECT uuid FROM messages WHERE session_id = ? AND turn_index = ? AND block_index = ?`,
		sessionID, c.Turn, c.Block).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("anchor: %w", err)
	}
	if at != uuid {
		return false, nil
	}
	var n int
	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM messages WHERE session_id = ? AND (turn_index, block_index) <= (?, ?)`,
		sessionID, c.Turn, c.Block).Scan(&n); err != nil {
		return false, fmt.Errorf("anchor: %w", err)
	}
	return n == upTo, nil
}

// MessageAt returns the message at c, or nil.
func (d *DB) MessageAt(sessionID string, c Cursor) (*ConvMessage, error) {
	msgs, err := d.convMessages(` WHERE session_id = ? AND turn_index = ? AND block_index = ?`, sessionID, c.Turn, c.Block)
	if err != nil {
		return nil, fmt.Errorf("message: %w", err)
	}
	if len(msgs) == 0 {
		return nil, nil
	}
	return &msgs[0], nil
}

// Prompts returns what the user wrote in a session, in order: its user text
// blocks but those that are only tags (slash commands, hook output), as
// SessionMessages leaves them out.
func (d *DB) Prompts(sessionID string) ([]ConvMessage, error) {
	msgs, err := d.convMessages(` WHERE session_id = ? AND block_type = 'text' AND role = 'user'
        AND content NOT LIKE '<%' ORDER BY turn_index, block_index`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("prompts: %w", err)
	}
	return msgs, nil
}

// FindInConversation returns the messages of a session's conversation, as
// SessionMessages reads it, whose text contains text, ignoring ASCII case,
// in order.
func (d *DB) FindInConversation(sessionID, text string) ([]ConvMessage, error) {
	msgs, err := d.convMessages(` `+strings.TrimPrefix(textMessages, "FROM messages")+`
        AND content LIKE ? ESCAPE '\' ORDER BY turn_index, block_index`, sessionID, "%"+likeEscaper.Replace(text)+"%")
	if err != nil {
		return nil, fmt.Errorf("find in conversation: %w", err)
	}
	return msgs, nil
}

// Mention is the first message of a session that has a word.
type Mention struct {
	Cursor  Cursor
	Role    string
	Content string
}

// Mentions returns, for every session whose conversation text contains
// text, the first message that does. It finds the sessions
// SessionsWithText finds, which the TUI's filter uses: user blocks that are
// only tags are searched too, unlike in FindInConversation, which is the
// search in one conversation.
func (d *DB) Mentions(text string) (map[string]Mention, error) {
	rows, err := d.sql.Query(`SELECT session_id, turn_index, block_index, role, content, MIN(turn_index * 65536 + block_index)
        FROM messages WHERE block_type = 'text' AND content LIKE ? ESCAPE '\'
        GROUP BY session_id`, "%"+likeEscaper.Replace(text)+"%")
	if err != nil {
		return nil, fmt.Errorf("mentions: %w", err)
	}
	defer rows.Close()
	out := map[string]Mention{}
	for rows.Next() {
		var id string
		var m Mention
		var order int64
		if err := rows.Scan(&id, &m.Cursor.Turn, &m.Cursor.Block, &m.Role, &m.Content, &order); err != nil {
			return nil, fmt.Errorf("mentions: %w", err)
		}
		out[id] = m
	}
	return out, rows.Err()
}

// RecallOnly returns which of the sessions ids were only a look back
// through recall (see recallOnly).
func (d *DB) RecallOnly(ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := d.sql.Query(`SELECT session_id FROM sessions WHERE session_id IN (?`+strings.Repeat(", ?", len(ids)-1)+`)
        AND `+recallOnly("sessions.session_id"), args...)
	if err != nil {
		return nil, fmt.Errorf("recall only: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("recall only: %w", err)
		}
		out[id] = true
	}
	return out, rows.Err()
}

// DataVersion changes when another connection, in this process or another,
// commits to the database; not when this one does.
func (d *DB) DataVersion() (int64, error) {
	var v int64
	if err := d.sql.QueryRow(`PRAGMA data_version`).Scan(&v); err != nil {
		return 0, fmt.Errorf("data version: %w", err)
	}
	return v, nil
}
