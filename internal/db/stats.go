package db

import (
	"cmp"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// IdleGap is the longest pause between two messages of a session that
// still counts as using it: a session left open counts only the time it
// was worked in.
const IdleGap = 15 * time.Minute

// ToolCount is how a tool fared in a session.
type ToolCount struct {
	Calls int
	// Failed is how many of its results the transcript marked an error, of
	// Known, the results whose outcome it says. A session stored before
	// outcomes were has none known.
	Failed, Known int
}

// SessionFacts is what the Stats page reads from a session's messages.
type SessionFacts struct {
	// Active is the time the session was worked in: the gaps between its
	// messages, leaving out those longer than IdleGap.
	Active time.Duration
	// Tools are the session's tool calls by tool name.
	Tools map[string]*ToolCount
	// Commands are its Bash commands; Read and Edited count the files it
	// read (Read) and edited (Edit, Write, MultiEdit, NotebookEdit), by
	// path as the tool was given it.
	Commands     []string
	Read, Edited map[string]int
	// Interrupted is whether the user stopped Claude in it.
	Interrupted bool
}

func placeholders(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }

func idArgs(ids []string) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}

// statsChunk is how many sessions one query reads, well under SQLite's
// limit on parameters.
const statsChunk = 500

// SessionFacts reads the facts of the sessions ids. A session without
// messages gets none.
func (d *DB) SessionFacts(ids []string) (map[string]*SessionFacts, error) {
	out := map[string]*SessionFacts{}
	for len(ids) > 0 {
		n := min(len(ids), statsChunk)
		if err := d.sessionFacts(ids[:n], out); err != nil {
			return nil, fmt.Errorf("session facts: %w", err)
		}
		ids = ids[n:]
	}
	return out, nil
}

func (d *DB) sessionFacts(ids []string, out map[string]*SessionFacts) error {
	isError, err := d.optionalColumnOf("messages", "is_error", "is_error")
	if err != nil {
		return err
	}
	useID, err := d.optionalColumnOf("messages", "tool_use_id", "tool_use_id")
	if err != nil {
		return err
	}
	rows, err := d.sql.Query(`SELECT session_id, block_type, COALESCE(timestamp, ''), COALESCE(tool_name, ''),
             CASE WHEN tool_name IN ('Bash', 'Read', 'Edit', 'Write', 'MultiEdit', 'NotebookEdit') THEN COALESCE(tool_input, '') ELSE '' END,
             COALESCE(`+useID+`, ''), `+isError+`,
             role = 'user' AND block_type = 'text' AND content LIKE '[Request interrupted by user%'
      FROM messages WHERE session_id IN (`+placeholders(len(ids))+`)
      ORDER BY session_id, turn_index`, idArgs(ids)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	type state struct {
		f    *SessionFacts
		last time.Time
		uses map[string]string // tool_use id to tool name
	}
	byID := map[string]*state{}
	for rows.Next() {
		var id, kind, ts, tool, input, use string
		var failed *bool
		var interrupted bool
		if err := rows.Scan(&id, &kind, &ts, &tool, &input, &use, &failed, &interrupted); err != nil {
			return err
		}
		st, ok := byID[id]
		if !ok {
			st = &state{f: &SessionFacts{Tools: map[string]*ToolCount{}, Read: map[string]int{}, Edited: map[string]int{}}, uses: map[string]string{}}
			byID[id], out[id] = st, st.f
		}
		f := st.f
		if t := parseTime(ts); !t.IsZero() {
			if gap := t.Sub(st.last); !st.last.IsZero() && gap > 0 && gap <= IdleGap {
				f.Active += gap
			}
			if t.After(st.last) {
				st.last = t
			}
		}
		f.Interrupted = f.Interrupted || interrupted
		switch kind {
		case "tool_use":
			c := f.Tools[tool]
			if c == nil {
				c = &ToolCount{}
				f.Tools[tool] = c
			}
			c.Calls++
			if use != "" {
				st.uses[use] = tool
			}
			if input == "" {
				continue
			}
			var in struct {
				FilePath     string `json:"file_path"`
				NotebookPath string `json:"notebook_path"`
				Command      string `json:"command"`
			}
			if json.Unmarshal([]byte(input), &in) != nil {
				continue
			}
			switch tool {
			case "Bash":
				if in.Command != "" {
					f.Commands = append(f.Commands, in.Command)
				}
			case "Read":
				if in.FilePath != "" {
					f.Read[in.FilePath]++
				}
			default:
				if p := cmp.Or(in.FilePath, in.NotebookPath); p != "" {
					f.Edited[p]++
				}
			}
		case "tool_result":
			if failed == nil {
				continue
			}
			if c := f.Tools[st.uses[use]]; use != "" && c != nil {
				c.Known++
				if *failed {
					c.Failed++
				}
			}
		}
	}
	return rows.Err()
}

// ModelUsage is the tokens a session used with one model, its subagents'
// included, in responses of one kind: in fast mode or not, and with a long
// prompt or not.
type ModelUsage struct {
	Model                                                string
	Fast, Long                                           bool
	Input, Output, CacheRead, CacheWrite5m, CacheWrite1h int64
}

// UsageByModel sums the usage of the sessions ids by model and kind of
// response; a long prompt is one of over longPrompt tokens, cache reads and
// writes included. It is empty on an archive opened read-only before usage
// was stored.
func (d *DB) UsageByModel(ids []string, longPrompt int64) (map[string][]ModelUsage, error) {
	out := map[string][]ModelUsage{}
	var n int
	if err := d.sql.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'usage'`).Scan(&n); err != nil || n == 0 {
		return out, err
	}
	for len(ids) > 0 {
		k := min(len(ids), statsChunk)
		args := append([]any{longPrompt}, idArgs(ids[:k])...)
		rows, err := d.sql.Query(`SELECT session_id, model, speed = 'fast', input + cache_read + cache_write_5m + cache_write_1h > ?,
                 SUM(input), SUM(output), SUM(cache_read), SUM(cache_write_5m), SUM(cache_write_1h)
          FROM usage WHERE session_id IN (`+placeholders(k)+`) GROUP BY 1, 2, 3, 4 ORDER BY 1, 2, 3, 4`, args...)
		if err != nil {
			return nil, fmt.Errorf("usage by model: %w", err)
		}
		for rows.Next() {
			var id string
			var u ModelUsage
			if err := rows.Scan(&id, &u.Model, &u.Fast, &u.Long, &u.Input, &u.Output, &u.CacheRead, &u.CacheWrite5m, &u.CacheWrite1h); err != nil {
				rows.Close()
				return nil, fmt.Errorf("usage by model: %w", err)
			}
			out[id] = append(out[id], u)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("usage by model: %w", err)
		}
		ids = ids[k:]
	}
	return out, nil
}
