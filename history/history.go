// Package history reads the Claude Code sessions recall imported, for
// another program to use: which sessions ran in a time range, the messages
// they hold, and the repository each one belongs to, the same as
// `recall list --format json` shows it.
//
// It only reads. Importing is `recall import`'s job.
package history

import (
	"context"
	"iter"
	"os"
	"path/filepath"
	"time"

	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/repos"
	"github.com/babarot/claude-recall/internal/title"
)

// History is recall's database, opened read-only.
type History struct {
	db    *db.DB
	repos *repos.Index
}

// DefaultPath is where recall keeps its database unless db under [core]
// in its config file moves it. It stays in ~/.claude whatever
// CLAUDE_CONFIG_DIR says, as recall's does.
func DefaultPath() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/tmp"
	}
	return filepath.Join(home, ".claude", "vault.db")
}

// Open opens the database at path read-only, so nothing is ever written to
// it, and resolves the repository of every directory sessions ran in.
func Open(path string) (*History, error) {
	// Two connections, so Sessions can run while Messages is being ranged
	// over.
	d, err := db.Open(path, db.Options{ReadOnly: true, Conns: 2})
	if err != nil {
		return nil, err
	}
	idx, err := repos.Load(d)
	if err != nil {
		d.Close()
		return nil, err
	}
	return &History{db: d, repos: idx}, nil
}

// Close closes the database.
func (h *History) Close() error { return h.db.Close() }

// Session is one session as recall stores it.
type Session struct {
	ID string
	// ProjectPath is the directory the session ran in; empty when unknown.
	ProjectPath string
	// Repository is the repository the session belongs to, as `recall list
	// --format json` shows it: owner/repo for a checkout under
	// ~/src/github.com, the path with ~ for one elsewhere, and the
	// repository's name alone for a herdr worktree since removed. It is not
	// read from a git remote.
	Repository string
	// RepositoryKey is what the sessions of one repository share, its
	// worktrees included: the main checkout's real path, or for a removed
	// herdr worktree a key made of the repository's name.
	RepositoryKey string
	// Worktree is the linked worktree's name, empty outside one.
	Worktree string
	// Remote is the origin of the repository, read from its git config
	// when the session was imported, written as host/path:
	// github.com/owner/repo whether the URL was https or ssh. Empty when
	// the directory has no origin, or was gone before recall first read it.
	Remote string
	// Title is what recall calls the session: the title Claude Code wrote
	// (a /rename or a generated one), or failing that the first prompt.
	Title       string
	FirstPrompt string
	GitBranch   string
	StartedAt   time.Time
	EndedAt     time.Time
	// MessageCount is the number of messages, in the sense of Message,
	// recall stored for the session.
	MessageCount int
}

// Sessions returns the sessions with a message whose timestamp is in
// [from, to), earliest started first. A session that runs past midnight
// is in the range of each day it has messages on.
func (h *History) Sessions(ctx context.Context, from, to time.Time) ([]Session, error) {
	rows, err := h.db.SessionsIn(ctx, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]Session, len(rows))
	for i, s := range rows {
		repo := h.repos.Of(s.ProjectPath)
		out[i] = Session{
			ID:            s.ID,
			ProjectPath:   s.ProjectPath,
			Repository:    repo.Name,
			RepositoryKey: repo.Key,
			Worktree:      repo.Worktree,
			Remote:        s.Remote,
			Title:         title.Display(s.Title, s.FirstPrompt),
			FirstPrompt:   s.FirstPrompt,
			GitBranch:     s.GitBranch,
			StartedAt:     s.StartedAt,
			EndedAt:       s.EndedAt,
			MessageCount:  s.MessageCount,
		}
	}
	return out, nil
}

// Message is one block of a transcript line: a text, a tool_use, a
// tool_result, and so on.
type Message struct {
	SessionID string
	// Turn and Block put the messages of a session in conversation order:
	// the line's place in the transcript, then the block's in the line.
	Turn  int
	Block int
	// UUID is the transcript line the block came from.
	UUID      string
	Timestamp time.Time
	Role      string // user or assistant
	// Kind is the block's type: text, tool_use, tool_result, thinking, ...
	Kind    string
	Content string
	// ToolName and ToolInput (JSON) are set on a tool_use.
	ToolName  string
	ToolInput string
	// ToolUseID ties a tool_result to its tool_use: both carry the same
	// id. Empty when recall stored the session before it kept ids and the
	// transcript is gone since, so it could not import it again; pair such
	// a result with the call by order.
	ToolUseID string
	// IsError is true for a tool_result the transcript marked an error.
	// False when it was not, when the transcript does not say, and for a
	// session stored before recall kept it whose transcript is gone.
	IsError bool
}

// Messages yields the messages whose timestamp is in [from, to), of the
// session sessionID or, when it is empty, of every session: by session,
// then by Turn and Block. They are read from the database as the loop
// asks for them, and the query ends with the loop. An error is yielded
// once and ends the sequence.
func (h *History) Messages(ctx context.Context, sessionID string, from, to time.Time) iter.Seq2[Message, error] {
	return func(yield func(Message, error) bool) {
		for b, err := range h.db.BlocksIn(ctx, sessionID, from, to) {
			if err != nil {
				yield(Message{}, err)
				return
			}
			m := Message{
				SessionID: b.SessionID,
				Turn:      b.Turn,
				Block:     b.Index,
				UUID:      b.UUID,
				Timestamp: b.Timestamp,
				Role:      b.Role,
				Kind:      b.Kind,
				Content:   b.Content,
				ToolName:  b.ToolName,
				ToolInput: b.ToolInput,
				ToolUseID: b.ToolUseID,
				IsError:   b.IsError,
			}
			if !yield(m, nil) {
				return
			}
		}
	}
}
