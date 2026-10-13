package web

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/babarot/claude-recall/internal/api"
	"github.com/babarot/claude-recall/internal/fixture"
	"github.com/babarot/claude-recall/internal/importer"
	"github.com/babarot/claude-recall/pkg/db"
)

// archive imports the fixture's transcripts, and returns a server on the
// archive, its path and the transcript tree.
func archive(t *testing.T) (*Server, string, string) {
	t.Helper()
	projects := fixture.Projects(t)
	path := filepath.Join(t.TempDir(), "vault.db")
	d, err := db.Open(path, db.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err := importer.Run(d, importer.Options{ProjectsDirs: []string{projects}}, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	s := New(d)
	s.trees, s.Home = []string{projects}, "/home/me"
	return s, path, projects
}

func decode[T any](t *testing.T, s *Server, path string) T {
	t.Helper()
	rec := get(t, s, path)
	if rec.Code != 200 {
		t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
	}
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("%s: %v\n%s", path, err, rec.Body.String())
	}
	return v
}

// transcript writes a session's JSONL under the tree: user and assistant
// lines in turn, with the texts given.
func transcript(t *testing.T, tree, project, id, cwd string, texts ...string) {
	t.Helper()
	var b strings.Builder
	for i, text := range texts {
		role := []string{"user", "assistant"}[i%2]
		var content any = text
		if role == "assistant" {
			content = []any{map[string]any{"type": "text", "text": text}}
		}
		line, _ := json.Marshal(map[string]any{"type": role, "uuid": fmt.Sprintf("%s-%d", id, i), "sessionId": id,
			"timestamp": fmt.Sprintf("2026-05-01T10:%02d:00.000Z", i), "cwd": cwd, "gitBranch": "main",
			"message": map[string]any{"role": role, "content": content}})
		b.Write(line)
		b.WriteByte('\n')
	}
	dir := filepath.Join(tree, project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestV2EmptyArchive(t *testing.T) {
	s := newServer(t)
	cases := map[string]string{
		"/api/v2/sessions":                           `[]`,
		"/api/v2/find?q=x":                           `{"ids":[],"text":false,"in":false}`,
		"/api/v2/find?q=in:x&text=1":                 `{"ids":[],"text":true,"in":true}`,
		"/api/v2/sessions/nope":                      `{"error":"session nope not found"}`,
		"/api/v2/sessions/nope/conversation":         `{"total":0,"messages":[]}`,
		"/api/v2/sessions/nope/outline":              `[]`,
		"/api/v2/sessions/nope/find?q=x":             `[]`,
		"/api/v2/sessions/nope/messages/0.0":         `{"error":"message 0.0 not found"}`,
		"/api/v2/sessions/nope/messages/x":           `{"error":"not a cursor: \"x\""}`,
		"/api/v2/nope":                               `{"error":"API /api/v2/nope not found"}`,
		"/api/v2/sessions/nope/conversation?after=x": `{"error":"not a cursor: \"x\""}`,
	}
	for path, want := range cases {
		if got := strings.TrimSpace(get(t, s, path).Body.String()); got != want {
			t.Errorf("%s: got %s, want %s", path, got, want)
		}
	}
}

func TestV2Sessions(t *testing.T) {
	s, _, _ := archive(t)
	list := decode[[]api.V2Session](t, s, "/api/v2/sessions")
	if len(list) != 2 || list[0].ID != fixture.APISession || list[1].ID != fixture.AppSession {
		t.Fatalf("most recently ended first: %+v", list)
	}
	a := list[0]
	if a.Title != "Fix the login bug where the session cookie expires early" || a.Branch != "feature/login" ||
		a.Folder != "/work/api" || a.GroupName != "/work/api" || !a.Gone || a.Messages != 3 || len(a.Activity) != listBuckets ||
		a.StartedAt == nil || *a.StartedAt != "2026-04-02T09:00:00.000Z" {
		t.Errorf("entry %+v", a)
	}
	one := decode[[]api.V2Session](t, s, "/api/v2/sessions?id="+fixture.AppSession)
	if len(one) != 1 || one[0].ID != fixture.AppSession {
		t.Errorf("?id= gives that one: %+v", one)
	}
}

func TestV2SessionResume(t *testing.T) {
	s, _, projects := archive(t)
	gone := decode[api.V2SessionDetail](t, s, "/api/v2/sessions/"+fixture.AppSession)
	if gone.Resume.Resumable || gone.Resume.Reason != "gone" || !strings.HasPrefix(gone.Resume.Command, "claude 'Use the recall tools to recall session "+fixture.AppSession) {
		t.Errorf("a session whose folder is gone is recalled: %+v", gone.Resume)
	}
	if gone.FullPath != "/work/app" || gone.ClaudeVersion != "2.1.87" {
		t.Errorf("header %+v", gone)
	}

	dir := t.TempDir()
	transcript(t, projects, "-here", "33333333-here", dir, "hello", "hi")
	if _, err := importer.ImportFile(s.DB, filepath.Join(projects, "-here", "33333333-here.jsonl"), nil, s.trees); err != nil {
		t.Fatal(err)
	}
	here := decode[api.V2SessionDetail](t, s, "/api/v2/sessions/33333333-here")
	if !here.Resume.Resumable || here.Resume.Command != "cd "+dir+" && claude -r 33333333-here" {
		t.Errorf("a session in a folder that is there is resumed: %+v", here.Resume)
	}
	os.Remove(filepath.Join(projects, "-here", "33333333-here.jsonl"))
	if r := decode[api.V2SessionDetail](t, s, "/api/v2/sessions/33333333-here").Resume; r.Resumable || r.Reason != "no-transcript" {
		t.Errorf("a session whose transcript is gone is recalled: %+v", r)
	}
}

func TestV2ConversationRanges(t *testing.T) {
	s, _, projects := archive(t)
	texts := make([]string, 9)
	for i := range texts {
		texts[i] = fmt.Sprintf("message %d", i)
	}
	transcript(t, projects, "-long", "44444444-long", "/work/long", texts...)
	if _, err := importer.ImportFile(s.DB, filepath.Join(projects, "-long", "44444444-long.jsonl"), nil, s.trees); err != nil {
		t.Fatal(err)
	}
	base := "/api/v2/sessions/44444444-long/conversation"
	all := decode[api.V2Conversation](t, s, base)
	if all.Total != 9 || len(all.Messages) != 9 || *all.First != all.Messages[0].Cursor || *all.Last != all.Messages[8].Cursor {
		t.Fatalf("the whole: %+v", all)
	}
	// Ranges from the start, read on, join up with no message twice or
	// missed.
	var joined []string
	after := "start"
	for {
		r := decode[api.V2Conversation](t, s, base+"?limit=4&after="+after)
		for _, m := range r.Messages {
			joined = append(joined, m.Content)
		}
		if r.Last == nil {
			break
		}
		after = *r.Last
	}
	if !slices.Equal(joined, texts) {
		t.Errorf("after ranges: %q", joined)
	}
	tail := decode[api.V2Conversation](t, s, base+"?limit=3")
	if len(tail.Messages) != 3 || tail.Messages[0].Content != "message 6" || tail.Messages[2].Content != "message 8" {
		t.Errorf("the end: %+v", tail.Messages)
	}
	before := decode[api.V2Conversation](t, s, base+"?limit=2&before="+*tail.First)
	if len(before.Messages) != 2 || before.Messages[0].Content != "message 4" || before.Messages[1].Content != "message 5" {
		t.Errorf("before the end: %+v", before.Messages)
	}

	// Going on after the third message, with it and the count up to it.
	third := all.Messages[2]
	ok := decode[api.V2Conversation](t, s, base+"?after="+third.Cursor+"&uuid="+third.UUID+"&known=3")
	if ok.Stale || len(ok.Messages) != 6 {
		t.Errorf("as it was: %+v", ok)
	}
	for _, q := range []string{"&uuid=other&known=3", "&uuid=" + third.UUID + "&known=2"} {
		if r := decode[api.V2Conversation](t, s, base+"?after="+third.Cursor+q); !r.Stale || len(r.Messages) != 0 {
			t.Errorf("%s: stale %+v", q, r)
		}
	}

	// The transcript rewritten with as many messages, other ones: the
	// reader's place is stale.
	for i := range texts {
		texts[i] = fmt.Sprintf("rewritten %d", i)
	}
	transcript(t, projects, "-long", "44444444-long", "/work/long", texts...)
	f := filepath.Join(projects, "-long", "44444444-long.jsonl")
	b, _ := os.ReadFile(f)
	os.WriteFile(f, []byte(strings.ReplaceAll(string(b), `"44444444-long-`, `"x-`)), 0o644)
	if _, err := importer.ImportFile(s.DB, f, nil, s.trees); err != nil {
		t.Fatal(err)
	}
	if r := decode[api.V2Conversation](t, s, base+"?after="+third.Cursor+"&uuid="+third.UUID+"&known=3"); !r.Stale {
		t.Errorf("after a rewrite: %+v", r)
	}
}

func TestV2ClipAndWhole(t *testing.T) {
	s, _, projects := archive(t)
	long := strings.Repeat("あ", api.Clip+10)
	transcript(t, projects, "-clip", "55555555-clip", "/work/clip", "short", long)
	if _, err := importer.ImportFile(s.DB, filepath.Join(projects, "-clip", "55555555-clip.jsonl"), nil, s.trees); err != nil {
		t.Fatal(err)
	}
	r := decode[api.V2Conversation](t, s, "/api/v2/sessions/55555555-clip/conversation")
	m := r.Messages[1]
	if len([]rune(m.Content)) != api.Clip || m.ContentLength != api.Clip+10 || r.Messages[0].ContentLength != 0 {
		t.Fatalf("clipped to %d of %d", len([]rune(m.Content)), m.ContentLength)
	}
	whole := decode[api.V2Message](t, s, "/api/v2/sessions/55555555-clip/messages/"+m.Cursor)
	if whole.Content != long || whole.ContentLength != 0 {
		t.Errorf("whole %d runes", len([]rune(whole.Content)))
	}
}

func TestV2OutlineFindDetail(t *testing.T) {
	s, _, _ := archive(t)
	outline := decode[[]api.V2Prompt](t, s, "/api/v2/sessions/"+fixture.AppSession+"/outline")
	if len(outline) != 2 || outline[1].Text != "And for production?" || outline[1].Timestamp == nil {
		t.Errorf("outline %+v", outline)
	}
	hits := decode[[]api.V2Hit](t, s, "/api/v2/sessions/"+fixture.AppSession+"/find?q=TERRAFORM")
	if len(hits) != 3 {
		t.Errorf("terraform is in three messages: %+v", hits)
	}
	d := decode[api.V2Detail](t, s, "/api/v2/sessions/"+fixture.APISession+"/detail")
	if d.You != 1 || d.Claude != 1 || d.Tools != 1 || d.Commands != 1 || len(d.Programs) != 1 || d.Programs[0] != (api.V2Count{Name: "go test", N: 1}) ||
		d.Groups == nil || len(d.Activity) != 24 {
		t.Errorf("detail %+v", d)
	}
}

func TestV2Find(t *testing.T) {
	s, path, projects := archive(t)
	// workspace is said in the app session, not in its title.
	if f := decode[api.V2Find](t, s, "/api/v2/find?q=workspace"); len(f.IDs) != 0 || f.Text {
		t.Errorf("without text, the fields only: %+v", f)
	}
	f := decode[api.V2Find](t, s, "/api/v2/find?q=workspace&text=1")
	if !slices.Equal(f.IDs, []string{fixture.AppSession}) || !f.Text ||
		!strings.Contains(f.Snippets[fixture.AppSession].Text, "separate workspace so") || f.Snippets[fixture.AppSession].Role != "assistant" {
		t.Fatalf("with text: %+v", f)
	}
	if f := decode[api.V2Find](t, s, "/api/v2/find?q=login+branch:feature"); !slices.Equal(f.IDs, []string{fixture.APISession}) || f.In {
		t.Errorf("fields: %+v", f)
	}
	if f := decode[api.V2Find](t, s, "/api/v2/find?q=in:wapp&text=1"); !slices.Equal(f.IDs, []string{fixture.AppSession}) || !f.In {
		t.Errorf("in: %+v", f)
	}

	// Another process imports a session that says it too: the word is
	// looked up again.
	other, err := db.Open(path, db.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	transcript(t, projects, "-more", "66666666-more", "/work/more", "where does it go?", "into the workspace")
	if _, err := importer.ImportFile(other, filepath.Join(projects, "-more", "66666666-more.jsonl"), nil, s.trees); err != nil {
		t.Fatal(err)
	}
	if f := decode[api.V2Find](t, s, "/api/v2/find?q=workspace&text=1"); len(f.IDs) != 2 {
		t.Errorf("after another process imported: %+v", f)
	}
}

func TestV2Theme(t *testing.T) {
	s := newServer(t)
	for theme, want := range map[string]string{
		"":                 `{"name":"auto","fixed":false}`,
		"auto":             `{"name":"auto","fixed":false}`,
		"ansi":             `{"name":"ansi","fixed":false}`,
		"no-such-scheme":   `{"name":"auto","fixed":false}`,
		"tokyo-night":      `{"name":"tokyo-night","fixed":true,"dark":true,"colors":{`,
		"catppuccin-latte": `{"name":"catppuccin-latte","fixed":true,"colors":{`,
	} {
		s.Theme = theme
		if got := strings.TrimSpace(get(t, s, "/api/v2/theme").Body.String()); !strings.HasPrefix(got, want) {
			t.Errorf("%q: %s, want %s…", theme, got, want)
		}
	}
	s.Theme = "dracula"
	got := decode[api.V2Theme](t, s, "/api/v2/theme")
	if got.Colors["bg"] != "#282a36" || got.Colors["accent"] != "#8be9fd" {
		t.Errorf("dracula: %v", got.Colors)
	}
}
