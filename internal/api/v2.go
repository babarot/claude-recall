package api

import (
	"strings"
	"unicode"

	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/jscompat"
)

// The /api/v2 responses, which the web UI reads. As with the others, add
// fields rather than change them.

// V2Session is an entry of /api/v2/sessions: a session as the TUI lists
// it.
type V2Session struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Folder is where the session ran, shortened (the main checkout for a
	// worktree); GroupKey and GroupName the folder the list narrows by: the
	// repository with its worktrees, every checkout of one git remote
	// together, or the directory outside git.
	Folder    string `json:"folder"`
	GroupKey  string `json:"groupKey"`
	GroupName string `json:"groupName"`
	Worktree  string `json:"worktree"`
	// Repository is whether the folder is a repository, not a directory
	// outside git: one in a git checkout, every checkout of a remote, or a
	// removed worktree. For a directory gone since, which can no longer be
	// told, it is whether its name reads as owner/repo rather than a path.
	Repository bool `json:"repository"`
	// Gone is set when the session's folder is no longer on disk.
	Gone       bool    `json:"gone"`
	Branch     string  `json:"branch"`
	Path       string  `json:"path"`
	Messages   int     `json:"messages"`
	Size       int64   `json:"size"`
	StartedAt  *string `json:"startedAt"`
	EndedAt    *string `json:"endedAt"`
	RecallOnly bool    `json:"recallOnly"`
	// Activity is the number of messages in each of 12 equal slices of the
	// session.
	Activity []int `json:"activity"`
	// Agent is who wrote the session: "claude" (Claude Code) or "codex".
	Agent string `json:"agent"`
}

// V2Resume is how to go back to a session: claude -r in its folder when
// Resumable, else a new claude that recalls it. Reason says why it is not
// resumable: "gone" (its folder), "other-tree" (its transcript is in a tree
// claude -r does not read, OtherTree) or "no-transcript".
type V2Resume struct {
	Resumable bool   `json:"resumable"`
	Command   string `json:"command"`
	Reason    string `json:"reason,omitempty"`
	OtherTree string `json:"otherTree,omitempty"`
}

// V2SessionDetail is /api/v2/sessions/:id: the list's entry and the rest of
// the header.
type V2SessionDetail struct {
	V2Session
	FullPath string `json:"fullPath"`
	// ClaudeVersion is the release of the agent that wrote the session,
	// under the name it had when every session was Claude Code's;
	// AgentVersion is the same value.
	ClaudeVersion string   `json:"claudeVersion"`
	AgentVersion  string   `json:"agentVersion"`
	Resume        V2Resume `json:"resume"`
}

// V2Error is the body of a v2 error response.
type V2Error struct {
	Error string `json:"error"`
}

// Clip is how much of a message's content and tool input a range of the
// conversation holds, in UTF-16 units as the browser counts them; the rest
// is read with /api/v2/sessions/:id/messages/:cursor.
const Clip = 2000

// V2Message is a message of the conversation. ContentLength and
// ToolInputLength are the whole lengths of a clipped content or tool input,
// and are left out of one that is whole.
type V2Message struct {
	Cursor          string  `json:"cursor"`
	UUID            string  `json:"uuid"`
	Role            string  `json:"role"`
	Type            string  `json:"type"`
	Content         string  `json:"content"`
	ContentLength   int     `json:"contentLength,omitempty"`
	ToolName        *string `json:"toolName"`
	ToolInput       *string `json:"toolInput"`
	ToolInputLength int     `json:"toolInputLength,omitempty"`
	Timestamp       *string `json:"timestamp"`
	// ToolUseID ties a tool_result to its tool_use, and IsError says
	// whether a tool_result was an error; absent when the archive does not
	// know, for a session stored before it kept them.
	ToolUseID *string `json:"toolUseId,omitempty"`
	IsError   *bool   `json:"isError,omitempty"`
}

// V2Messages converts messages, clipping them at clip units when clip is
// above zero.
func V2Messages(msgs []db.ConvMessage, clip int) []V2Message {
	out := make([]V2Message, len(msgs))
	for i, m := range msgs {
		v := V2Message{Cursor: m.Cursor.String(), UUID: m.UUID, Role: m.Role, Type: m.BlockType, Content: m.Content,
			ToolName: m.ToolName, ToolInput: m.ToolInput, Timestamp: m.Timestamp, ToolUseID: m.ToolUseID, IsError: m.IsError}
		if clip > 0 {
			if n := jscompat.Len(v.Content); n > clip {
				v.Content, v.ContentLength = jscompat.Slice(v.Content, clip), n
			}
			if v.ToolInput != nil {
				if n := jscompat.Len(*v.ToolInput); n > clip {
					s := jscompat.Slice(*v.ToolInput, clip)
					v.ToolInput, v.ToolInputLength = &s, n
				}
			}
		}
		out[i] = v
	}
	return out
}

// V2Conversation is a range of a session's conversation. First and Last
// are the cursors of its first and last messages, absent when it is empty.
// Stale is set when the messages a reader asked to continue after are not
// there as they were; Messages is then empty and the reader reads the
// conversation again.
type V2Conversation struct {
	Total    int         `json:"total"`
	Messages []V2Message `json:"messages"`
	First    *string     `json:"first,omitempty"`
	Last     *string     `json:"last,omitempty"`
	Stale    bool        `json:"stale,omitempty"`
}

// V2ConversationResult builds a range.
func V2ConversationResult(total int, msgs []db.ConvMessage) V2Conversation {
	out := V2Conversation{Total: total, Messages: V2Messages(msgs, Clip)}
	if len(msgs) > 0 {
		first, last := msgs[0].Cursor.String(), msgs[len(msgs)-1].Cursor.String()
		out.First, out.Last = &first, &last
	}
	return out
}

// V2Prompt is an entry of /api/v2/sessions/:id/outline: something the user
// wrote, its start.
type V2Prompt struct {
	Cursor    string  `json:"cursor"`
	Timestamp *string `json:"timestamp"`
	Text      string  `json:"text"`
}

// V2Outline converts the prompts of a session.
func V2Outline(msgs []db.ConvMessage) []V2Prompt {
	out := make([]V2Prompt, len(msgs))
	for i, m := range msgs {
		out[i] = V2Prompt{Cursor: m.Cursor.String(), Timestamp: m.Timestamp, Text: jscompat.Slice(oneLine(m.Content), 200)}
	}
	return out
}

// V2Hit is a message of a session that has what was searched for.
type V2Hit struct {
	Cursor string `json:"cursor"`
	UUID   string `json:"uuid"`
}

// V2Hits converts the messages found.
func V2Hits(msgs []db.ConvMessage) []V2Hit {
	out := make([]V2Hit, len(msgs))
	for i, m := range msgs {
		out[i] = V2Hit{Cursor: m.Cursor.String(), UUID: m.UUID}
	}
	return out
}

// V2Count is a name and how often it occurred.
type V2Count struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

// V2Counts converts counts.
func V2Counts(cs []db.Count) []V2Count {
	out := make([]V2Count, len(cs))
	for i, c := range cs {
		out[i] = V2Count{c.Name, c.N}
	}
	return out
}

// V2FileGroup is edited files under one place: Place is "" for the
// session's own folder, Worktree a herdr worktree of the repository Place.
type V2FileGroup struct {
	Place    string    `json:"place"`
	Worktree string    `json:"worktree,omitempty"`
	Files    []V2Count `json:"files"`
}

// V2Detail is /api/v2/sessions/:id/detail: what the TUI's What was done and
// Details frames show.
type V2Detail struct {
	You      int       `json:"you"`
	Claude   int       `json:"claude"`
	Tools    int       `json:"tools"`
	Thinking int       `json:"thinking"`
	Images   int       `json:"images"`
	TopTools []V2Count `json:"topTools"`
	// Commands is how many Bash commands ran, Programs what they ran, most
	// run first.
	Commands int       `json:"commands"`
	Programs []V2Count `json:"programs"`
	// FileCount is how many files were edited; Groups holds the most edited
	// of them, by place, and Temps counts the temporary ones apart.
	FileCount int           `json:"fileCount"`
	Groups    []V2FileGroup `json:"groups"`
	Temps     int           `json:"temps"`
	// Activity is the number of messages in each of 24 equal slices of the
	// session.
	Activity []int `json:"activity"`
}

// V2Snippet is where a session has a word that was searched for: its
// first message that does, around the word.
type V2Snippet struct {
	Cursor string `json:"cursor"`
	Role   string `json:"role"`
	Text   string `json:"text"`
}

// V2Find is /api/v2/find: the sessions that pass a filter, as the TUI's
// list would show them. Text is set when the conversations were searched
// too; without it a word matches the fields only. In is set when the
// filter names folders (folder:, in:), which override the folder the list
// is narrowed to. Snippets are keyed by session ID.
type V2Find struct {
	IDs      []string             `json:"ids"`
	Text     bool                 `json:"text"`
	In       bool                 `json:"in"`
	Snippets map[string]V2Snippet `json:"snippets,omitempty"`
}

// snippetBefore and snippetAfter are how many runes of the message a
// snippet keeps on each side of the word.
const snippetBefore, snippetAfter = 40, 80

// Snippet cuts the text around the first word of a message, as one line,
// marking what it leaves out with "…". The word is matched ignoring ASCII
// case, as the search's LIKE does.
func Snippet(content, word string) string {
	r := []rune(oneLine(content))
	w := []rune(word)
	at := 0
	for i := 0; i+len(w) <= len(r); i++ {
		if foldEqual(r[i:i+len(w)], w) {
			at = i
			break
		}
	}
	start, end := max(0, at-snippetBefore), min(len(r), at+len(w)+snippetAfter)
	out := string(r[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(r) {
		out += "…"
	}
	return out
}

func foldEqual(a, b []rune) bool {
	for i := range a {
		if asciiLower(a[i]) != asciiLower(b[i]) {
			return false
		}
	}
	return true
}

func asciiLower(c rune) rune {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// oneLine joins the lines of s with single spaces.
func oneLine(s string) string {
	return strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
}

// V2Theme is /api/v2/theme: the color scheme the web UI is drawn in, the
// TUI's ([tui] theme). Fixed is set for a scheme of its own, whose Colors
// (CSS custom properties without the --) the page takes, and Dark tells
// which it is; for auto, and for ansi, which a page cannot draw, the page
// keeps its own catppuccin-mocha and latte, and its dark and light switch.
type V2Theme struct {
	Name   string            `json:"name"`
	Fixed  bool              `json:"fixed"`
	Dark   bool              `json:"dark,omitempty"`
	Colors map[string]string `json:"colors,omitempty"`
}
