package web

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/babarot/claude-recall/internal/api"
	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/filter"
	"github.com/babarot/claude-recall/internal/repos"
	"github.com/babarot/claude-recall/internal/resume"
	"github.com/babarot/claude-recall/internal/theme"
	"github.com/babarot/claude-recall/internal/tidy"
	"github.com/babarot/claude-recall/internal/title"
	"github.com/babarot/claude-recall/internal/worktree"
)

// /api/v2 is what the web UI reads: the sessions as the TUI lists them,
// the conversation in ranges, and the TUI's filter.

const (
	listBuckets = 12
	// defaultRange and maxRange are how many messages a range of the
	// conversation holds unless the reader asks for fewer.
	defaultRange, maxRange = 300, 2000
)

// v2 holds what the API keeps between requests. What is kept per session
// (whether it was only a look back through recall, its activity) is
// stamped with the session's end, message count and transcript size, and
// computed again when they change, as they do when it is imported again. The sessions each searched word is in are dropped when
// the archive changes: through the watcher (changed), or by another
// connection's commit, which changes the data version.
type v2 struct {
	mu       sync.Mutex
	stamps   map[string]string
	recall   map[string]bool
	activity map[string][]int
	mentions map[string]map[string]db.Mention
	version  int64
	changed  bool
	repos    *repos.Index
	paths    int
}

func newV2() *v2 {
	return &v2{stamps: map[string]string{}, recall: map[string]bool{}, activity: map[string][]int{},
		mentions: map[string]map[string]db.Mention{}}
}

// Changed tells the server the archive changed, as an import in this
// process does, which the data version does not show.
func (s *Server) Changed() {
	s.v2.mu.Lock()
	s.v2.changed = true
	s.v2.mu.Unlock()
}

// warm fills what the list keeps for every session.
func (s *Server) warm() {
	s.v2.mu.Lock()
	defer s.v2.mu.Unlock()
	if _, err := s.rows(""); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
}

// fresh drops the searched words when the archive changed since they were
// looked up. Call with mu held.
func (s *Server) fresh() error {
	v, err := s.DB.DataVersion()
	if err != nil {
		return err
	}
	if v != s.v2.version || s.v2.changed {
		s.v2.version, s.v2.changed = v, false
		clear(s.v2.mentions)
	}
	return nil
}

// index is the repository of every session directory, built again when a
// directory it does not know appears.
func (s *Server) index() (*repos.Index, error) {
	paths, err := s.DB.ProjectPaths()
	if err != nil {
		return nil, err
	}
	if s.v2.repos == nil || len(paths) != s.v2.paths {
		s.v2.repos, s.v2.paths = repos.New(paths, s.home()), len(paths)
	}
	return s.v2.repos, nil
}

func (s *Server) home() string {
	if s.Home != "" {
		return s.Home
	}
	h, _ := os.UserHomeDir()
	return h
}

// row is a session with what the list and the filter need.
type row struct {
	s    db.Session
	repo worktree.Repo
	out  api.V2Session
}

func (r *row) fields() filter.Fields {
	return filter.Fields{Title: r.out.Title, Folder: r.out.GroupName, Worktree: r.repo.Worktree, Branch: r.s.GitBranch, ID: r.s.ID,
		Search: filter.SearchText(r.out.Title, r.repo.Folder, r.repo.Worktree, r.s.GitBranch, r.s.ID)}
}

func isoTime(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := t.UTC().Format("2006-01-02T15:04:05.000Z")
	return &s
}

// rows lists the sessions, most recently ended first, or the one of id when
// it is not empty. Call with mu held.
func (s *Server) rows(id string) ([]row, error) {
	all, err := s.DB.Sessions()
	if err != nil {
		return nil, err
	}
	x, err := s.index()
	if err != nil {
		return nil, err
	}
	// Which folder a checkout goes to by its remote depends on every
	// session of it, so this reads them all even for one.
	members := make([]worktree.Member, len(all))
	for i, ses := range all {
		members[i] = worktree.Member{Key: x.Of(ses.ProjectPath).Key, Remote: ses.Remote, Ended: ses.EndedAt}
	}
	byRemote := worktree.RemoteGroups(members)
	if id != "" {
		all = slices.DeleteFunc(all, func(x db.Session) bool { return x.ID != id })
	}
	home := s.home()
	var stale []string
	out := make([]row, len(all))
	for i, ses := range all {
		repo := x.Of(ses.ProjectPath)
		group := worktree.Group{Key: repo.Key, Name: repo.Name}
		if g, ok := byRemote[repo.Key]; ok {
			group = g
		}
		stamp := ses.EndedAt.String() + "/" + strconv.Itoa(ses.MessageCount) + "/" + strconv.FormatInt(ses.FileSize, 10)
		if s.v2.stamps[ses.ID] != stamp {
			stale = append(stale, ses.ID)
			s.v2.stamps[ses.ID] = stamp
		}
		out[i] = row{s: ses, repo: repo, out: api.V2Session{ID: ses.ID, Title: title.Display(ses.Title, ses.FirstPrompt),
			Folder: repo.Folder, GroupKey: group.Key, GroupName: group.Name, Worktree: repo.Worktree, Gone: !repo.Exists,
			Branch: ses.GitBranch, Path: worktree.TildePath(ses.ProjectPath, home), Messages: ses.MessageCount,
			Size: ses.FileSize, StartedAt: isoTime(ses.StartedAt), EndedAt: isoTime(ses.EndedAt)}}
	}
	if len(stale) > 0 {
		recall, err := s.DB.RecallOnly(stale)
		if err != nil {
			return nil, err
		}
		act, err := s.DB.SessionActivities(stale, listBuckets)
		if err != nil {
			return nil, err
		}
		for _, id := range stale {
			s.v2.recall[id], s.v2.activity[id] = recall[id], act[id]
		}
	}
	for i := range out {
		r := &out[i]
		r.out.RecallOnly = s.v2.recall[r.s.ID]
		r.out.Activity = s.v2.activity[r.s.ID]
		if r.out.Activity == nil {
			r.out.Activity = make([]int, listBuckets)
		}
	}
	return out, nil
}

func badRequest(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	writeJSON(w, api.V2Error{Error: err.Error()})
}

func notFound(w http.ResponseWriter, what string) {
	w.WriteHeader(http.StatusNotFound)
	writeJSON(w, api.V2Error{Error: what + " not found"})
}

// v2API serves a path under /api/v2/.
func (s *Server) v2API(w http.ResponseWriter, r *http.Request, path string) error {
	q := r.URL.Query()
	rest := strings.TrimPrefix(path, "/api/v2/")
	switch {
	case rest == "sessions":
		return s.v2Sessions(w, q.Get("id"))
	case rest == "find":
		return s.v2Find(w, q.Get("q"), q.Get("text") == "1")
	case rest == "theme":
		name := s.Theme
		if !theme.Valid(name) {
			name = theme.Auto
		}
		out := api.V2Theme{Name: name}
		if colors, dark, ok := theme.WebColors(name); ok {
			out.Fixed, out.Dark, out.Colors = true, dark, colors
		}
		writeJSON(w, out)
		return nil
	case strings.HasPrefix(rest, "sessions/"):
		parts := strings.Split(strings.TrimPrefix(rest, "sessions/"), "/")
		id := parts[0]
		switch {
		case len(parts) == 1:
			return s.v2Session(w, id)
		case len(parts) == 2 && parts[1] == "conversation":
			return s.v2Conversation(w, id, q)
		case len(parts) == 3 && parts[1] == "messages":
			return s.v2Message(w, id, parts[2])
		case len(parts) == 2 && parts[1] == "outline":
			prompts, err := s.DB.Prompts(id)
			if err != nil {
				return err
			}
			writeJSON(w, api.V2Outline(prompts))
			return nil
		case len(parts) == 2 && parts[1] == "find":
			if q.Get("q") == "" {
				writeJSON(w, []api.V2Hit{})
				return nil
			}
			hits, err := s.DB.FindInConversation(id, q.Get("q"))
			if err != nil {
				return err
			}
			writeJSON(w, api.V2Hits(hits))
			return nil
		case len(parts) == 2 && parts[1] == "detail":
			return s.v2Detail(w, id)
		}
	}
	notFound(w, "API "+path)
	return nil
}

func (s *Server) v2Sessions(w http.ResponseWriter, id string) error {
	s.v2.mu.Lock()
	defer s.v2.mu.Unlock()
	rows, err := s.rows(id)
	if err != nil {
		return err
	}
	out := make([]api.V2Session, len(rows))
	for i, r := range rows {
		out[i] = r.out
	}
	writeJSON(w, out)
	return nil
}

func (s *Server) v2Session(w http.ResponseWriter, id string) error {
	s.v2.mu.Lock()
	rows, err := s.rows(id)
	s.v2.mu.Unlock()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		notFound(w, "session "+id)
		return nil
	}
	r := rows[0]
	other, missing := resume.Locate(s.trees, r.s.ID, r.s.Project)
	res := api.V2Resume{Resumable: r.repo.Exists && !missing && other == "", OtherTree: other}
	switch {
	case !r.repo.Exists:
		res.Reason = "gone"
	case other != "":
		res.Reason = "other-tree"
	case missing:
		res.Reason = "no-transcript"
	}
	if res.Resumable {
		res.Command = resume.Command(r.s.ProjectPath, r.s.ID)
	} else {
		res.Command = resume.RecallCommand(s.self(), r.s.ID)
	}
	writeJSON(w, api.V2SessionDetail{V2Session: r.out, FullPath: r.s.ProjectPath, ClaudeVersion: r.s.ClaudeVersion, Resume: res})
	return nil
}

// self is the command line of recall's MCP server that a recalled session
// gets.
func (s *Server) self() []string {
	if len(s.Self) > 0 {
		return s.Self
	}
	return []string{"recall", "mcp"}
}

// cursorParam reads a cursor parameter: "" or the given word for none.
func cursorParam(v, none string) (*db.Cursor, error) {
	if v == "" || v == none {
		return nil, nil
	}
	c, err := db.ParseCursor(v)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// v2Conversation serves a range: after=<cursor> (or "start") for the
// messages after it, before=<cursor> (or "end") for those before it, the
// last ones by default. With after, uuid=<the message at it> and
// known=<how many messages up to it the reader has> check that they are
// still there as they were; the range is stale if not.
func (s *Server) v2Conversation(w http.ResponseWriter, id string, q map[string][]string) error {
	get := func(k string) string {
		if v := q[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	limit := defaultRange
	if v := get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			badRequest(w, fmt.Errorf("limit: %q", v))
			return nil
		}
		limit = min(n, maxRange)
	}
	total, err := s.DB.MessageTotal(id)
	if err != nil {
		return err
	}
	var msgs []db.ConvMessage
	if v := get("after"); v != "" {
		c, err := cursorParam(v, "start")
		if err != nil {
			badRequest(w, err)
			return nil
		}
		if c != nil && get("uuid") != "" {
			known, err := strconv.Atoi(get("known"))
			if err != nil {
				badRequest(w, fmt.Errorf("known: %q", get("known")))
				return nil
			}
			ok, err := s.DB.Anchor(id, *c, get("uuid"), known)
			if err != nil {
				return err
			}
			if !ok {
				writeJSON(w, api.V2Conversation{Total: total, Messages: []api.V2Message{}, Stale: true})
				return nil
			}
		}
		if msgs, err = s.DB.ConversationAfter(id, c, limit); err != nil {
			return err
		}
	} else {
		c, err := cursorParam(get("before"), "end")
		if err != nil {
			badRequest(w, err)
			return nil
		}
		if msgs, err = s.DB.ConversationBefore(id, c, limit); err != nil {
			return err
		}
	}
	writeJSON(w, api.V2ConversationResult(total, msgs))
	return nil
}

func (s *Server) v2Message(w http.ResponseWriter, id, cursor string) error {
	c, err := db.ParseCursor(cursor)
	if err != nil {
		badRequest(w, err)
		return nil
	}
	m, err := s.DB.MessageAt(id, c)
	if err != nil {
		return err
	}
	if m == nil {
		notFound(w, "message "+cursor)
		return nil
	}
	writeJSON(w, api.V2Messages([]db.ConvMessage{*m}, 0)[0])
	return nil
}

func (s *Server) v2Detail(w http.ResponseWriter, id string) error {
	d, err := s.DB.SessionDetail(id)
	if err != nil {
		return err
	}
	var folder string
	s.v2.mu.Lock()
	rows, err := s.rows(id)
	s.v2.mu.Unlock()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		notFound(w, "session "+id)
		return nil
	}
	folder = rows[0].s.ProjectPath
	groups, temps := tidy.GroupFiles(d.Files, folder, s.home())
	out := api.V2Detail{You: d.You, Claude: d.Claude, Tools: d.Tools, Thinking: d.Thinking, Images: d.Images,
		TopTools: api.V2Counts(d.TopTools), Commands: len(d.Commands), Programs: api.V2Counts(tidy.CommandCounts(d.Commands)),
		FileCount: d.FileCount, Groups: make([]api.V2FileGroup, len(groups)), Temps: temps, Activity: d.Activity}
	if out.Activity == nil {
		out.Activity = []int{}
	}
	for i, g := range groups {
		out.Groups[i] = api.V2FileGroup{Place: g.Place, Worktree: g.Worktree, Files: api.V2Counts(g.Files)}
	}
	writeJSON(w, out)
	return nil
}

// v2Find applies the TUI's filter to every session. Without text, a word
// matches the sessions' fields only, which is quick; with it, the
// conversations too, as the TUI's filter does once it has looked the words
// up.
func (s *Server) v2Find(w http.ResponseWriter, query string, text bool) error {
	s.v2.mu.Lock()
	defer s.v2.mu.Unlock()
	q := filter.Parse(query)
	rows, err := s.rows("")
	if err != nil {
		return err
	}
	words := q.BodyWords()
	found := map[string]map[string]db.Mention{}
	if text {
		if err := s.fresh(); err != nil {
			return err
		}
		for _, w := range words {
			m, ok := s.v2.mentions[w]
			if !ok {
				if m, err = s.DB.Mentions(w); err != nil {
					return err
				}
				s.v2.mentions[w] = m
			}
			found[w] = m
		}
	}
	out := api.V2Find{IDs: []string{}, Text: text, In: len(q.In) > 0}
	for i := range rows {
		r := &rows[i]
		said := func(w string) bool { _, ok := found[w][r.s.ID]; return ok }
		if !filter.Match(q, r.fields(), said) {
			continue
		}
		out.IDs = append(out.IDs, r.s.ID)
		for _, w := range words {
			if m, ok := found[w][r.s.ID]; ok {
				if out.Snippets == nil {
					out.Snippets = map[string]api.V2Snippet{}
				}
				out.Snippets[r.s.ID] = api.V2Snippet{Cursor: m.Cursor.String(), Role: m.Role, Text: api.Snippet(m.Content, w)}
				break
			}
		}
	}
	writeJSON(w, out)
	return nil
}
