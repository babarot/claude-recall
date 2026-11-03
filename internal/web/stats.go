package web

import (
	"cmp"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/babarot/claude-recall/internal/api"
	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/pricing"
	"github.com/babarot/claude-recall/internal/tidy"
	"github.com/babarot/claude-recall/internal/worktree"
)

// statTop is how many programs and files a session of /api/v2/stats lists.
const statTop = 10

// categories are the kinds of work the Stats page splits tool calls into.
var categories = map[string]string{
	"Read": "explore", "Grep": "explore", "Glob": "explore", "LS": "explore", "NotebookRead": "explore",
	"Bash": "run", "BashOutput": "run", "KillShell": "run",
	"Edit": "edit", "MultiEdit": "edit", "Write": "edit", "NotebookEdit": "edit",
	"Agent": "subagent", "Task": "subagent", "SendMessage": "subagent",
	"WebFetch": "web", "WebSearch": "web",
}

func category(tool string) string {
	if c, ok := categories[tool]; ok {
		return c
	}
	if strings.HasPrefix(tool, "mcp__claude-in-chrome__") {
		return "web"
	}
	return "other"
}

// statFacts is what /api/v2/stats keeps of a session's messages until it
// changes.
type statFacts struct {
	active      float64
	tools       map[string][3]int
	programs    []api.V2Count
	files       []api.V2StatFile
	interrupted bool
}

func newStatFacts(f *db.SessionFacts, folder, home string) statFacts {
	out := statFacts{active: f.Active.Minutes(), tools: map[string][3]int{}, interrupted: f.Interrupted,
		programs: api.V2Counts(tidy.CommandCounts(f.Commands))}
	if len(out.programs) > statTop {
		out.programs = out.programs[:statTop]
	}
	for name, c := range f.Tools {
		out.tools[name] = [3]int{c.Calls, c.Failed, c.Known}
	}
	files := map[string]*api.V2StatFile{}
	add := func(p string, read, edited int) {
		if tidy.IsTemp(p) {
			return
		}
		if rel, ok := strings.CutPrefix(p, folder+"/"); ok && folder != "" {
			p = rel
		} else {
			p = worktree.TildePath(p, home)
		}
		x := files[p]
		if x == nil {
			x = &api.V2StatFile{Path: p}
			files[p] = x
		}
		x.Read += read
		x.Edited += edited
	}
	for p, n := range f.Read {
		add(p, n, 0)
	}
	for p, n := range f.Edited {
		add(p, 0, n)
	}
	for _, x := range files {
		out.files = append(out.files, *x)
	}
	slices.SortFunc(out.files, func(a, b api.V2StatFile) int {
		return cmp.Or(cmp.Compare(b.Read+b.Edited, a.Read+a.Edited), cmp.Compare(a.Path, b.Path))
	})
	if len(out.files) > statTop {
		out.files = out.files[:statTop]
	}
	return out
}

// v2Stats serves the sessions started since the time since (RFC 3339),
// every session without it.
func (s *Server) v2Stats(w http.ResponseWriter, since string) error {
	var from time.Time
	if since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			badRequest(w, fmt.Errorf("since: %q", since))
			return nil
		}
		from = t
	}
	s.v2.mu.Lock()
	defer s.v2.mu.Unlock()
	all, err := s.rows("")
	if err != nil {
		return err
	}
	var picked []row
	var missing []string
	for _, r := range all {
		if r.s.StartedAt.IsZero() || r.s.StartedAt.Before(from) {
			continue
		}
		picked = append(picked, r)
		if _, ok := s.v2.facts[r.s.ID]; !ok {
			missing = append(missing, r.s.ID)
		}
	}
	if len(missing) > 0 {
		facts, err := s.DB.SessionFacts(missing)
		if err != nil {
			return err
		}
		home := s.home()
		for _, r := range picked {
			if f, ok := facts[r.s.ID]; ok {
				s.v2.facts[r.s.ID] = newStatFacts(f, r.s.ProjectPath, home)
			} else if _, ok := s.v2.facts[r.s.ID]; !ok {
				s.v2.facts[r.s.ID] = statFacts{tools: map[string][3]int{}}
			}
		}
	}
	ids := make([]string, len(picked))
	for i, r := range picked {
		ids[i] = r.s.ID
	}
	usage, err := s.DB.UsageByModel(ids)
	if err != nil {
		return err
	}

	out := api.V2Stats{Sessions: make([]api.V2StatSession, len(picked)), Categories: map[string]string{}, Prices: map[string][5]float64{}}
	for i, r := range picked {
		f := s.v2.facts[r.s.ID]
		x := api.V2StatSession{ID: r.s.ID, Title: r.out.Title, StartedAt: r.out.StartedAt, GroupKey: r.out.GroupKey, GroupName: r.out.GroupName,
			ActiveMinutes: f.active, Messages: r.s.MessageCount, Tokens: map[string][5]int64{}, Tools: f.tools,
			Programs: f.programs, Files: f.files, Interrupted: f.interrupted}
		if x.Programs == nil {
			x.Programs = []api.V2Count{}
		}
		if x.Files == nil {
			x.Files = []api.V2StatFile{}
		}
		var most int64 = -1
		for _, u := range usage[r.s.ID] {
			t := pricing.Tokens{u.Input, u.Output, u.CacheRead, u.CacheWrite5m, u.CacheWrite1h}
			x.Tokens[u.Model] = t
			if u.Output > most {
				x.Model, most = u.Model, u.Output
			}
			if rates, ok := s.Prices.Rates(u.Model); ok {
				x.Cost += rates.Cost(t)
				out.Prices[u.Model] = rates
			} else {
				x.CostPartial = true
			}
		}
		if len(usage[r.s.ID]) == 0 {
			out.UnknownTokens++
		}
		for name := range f.tools {
			out.Categories[name] = category(name)
		}
		out.Sessions[i] = x
	}
	writeJSON(w, out)
	return nil
}
