package web

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/babarot/claude-recall/internal/api"
	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/importer"
	"github.com/babarot/claude-recall/internal/pricing"
)

// statsArchive is an archive of one session that ran a command which
// failed, read and edited files, was interrupted, paused for an hour, and
// used two models.
func statsArchive(t *testing.T) *Server {
	t.Helper()
	tree := t.TempDir()
	cwd := "/home/me/src/app"
	var lines []string
	add := func(v map[string]any) {
		v["sessionId"], v["cwd"] = "stats-1", cwd
		b, _ := json.Marshal(v)
		lines = append(lines, string(b))
	}
	at := func(min int) string { return "2026-05-01T10:00:00.000Z"[:14] + twoDigits(min) + ":00.000Z" }
	user := func(uuid string, min int, content any) {
		add(map[string]any{"type": "user", "uuid": uuid, "timestamp": at(min), "message": map[string]any{"role": "user", "content": content}})
	}
	assistant := func(uuid string, min int, id, model string, out int, blocks ...any) {
		add(map[string]any{"type": "assistant", "uuid": uuid, "timestamp": at(min), "message": map[string]any{"role": "assistant",
			"id": id, "model": model, "content": blocks,
			"usage": map[string]any{"input_tokens": 1000, "output_tokens": out, "cache_read_input_tokens": 0, "cache_creation_input_tokens": 0}}})
	}
	use := func(id, name string, input map[string]any) map[string]any {
		return map[string]any{"type": "tool_use", "id": id, "name": name, "input": input}
	}
	result := func(id string, failed bool) []any {
		return []any{map[string]any{"type": "tool_result", "tool_use_id": id, "content": "x", "is_error": failed}}
	}
	user("u1", 0, "run the tests")
	assistant("a1", 1, "msg_1", "claude-opus-5-5", 100, use("t1", "Bash", map[string]any{"command": "go test ./..."}))
	user("u2", 2, result("t1", true))
	assistant("a2", 3, "msg_2", "claude-opus-5-5", 100,
		use("t2", "Read", map[string]any{"file_path": cwd + "/main.go"}), use("t3", "Edit", map[string]any{"file_path": cwd + "/main.go"}))
	user("u3", 4, result("t2", false))
	user("u4", 5, "[Request interrupted by user]")
	// An hour's pause, which does not count as working in it.
	user("u5", 59, "go on")
	assistant("a3", 59, "msg_3", "claude-haiku-5-5", 10, map[string]any{"type": "text", "text": "done"})
	dir := filepath.Join(tree, "-home-me-src-app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stats-1.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := db.Open(filepath.Join(t.TempDir(), "vault.db"), db.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err := importer.Run(d, importer.Options{ProjectsDirs: []string{tree}}, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	s := New(d)
	s.trees, s.Home = []string{tree}, "/home/me"
	s.Prices = pricing.New(map[string]pricing.Price{"claude-opus-5-5": {Input: 10, Output: 100}})
	return s
}

func twoDigits(n int) string { return string(rune('0'+n/10)) + string(rune('0'+n%10)) }

func TestV2Stats(t *testing.T) {
	s := statsArchive(t)
	got := decode[api.V2Stats](t, s, "/api/v2/stats?since=2026-04-01T00:00:00Z")
	if len(got.Sessions) != 1 {
		t.Fatalf("sessions %+v", got.Sessions)
	}
	x := got.Sessions[0]
	// 0 to 5 minutes, then 59: the pause is left out.
	if x.ActiveMinutes != 5 {
		t.Errorf("active minutes %v, want 5", x.ActiveMinutes)
	}
	if x.Tools["Bash"] != [3]int{1, 1, 1} || x.Tools["Read"] != [3]int{1, 0, 1} || x.Tools["Edit"] != [3]int{1, 0, 0} {
		t.Errorf("tools %v", x.Tools)
	}
	if got.Categories["Bash"] != "run" || got.Categories["Read"] != "explore" || got.Categories["Edit"] != "edit" {
		t.Errorf("categories %v", got.Categories)
	}
	if len(x.Programs) != 1 || x.Programs[0].Name != "go test" {
		t.Errorf("programs %v", x.Programs)
	}
	if len(x.Files) != 1 || x.Files[0] != (api.V2StatFile{Path: "main.go", Read: 1, Edited: 1}) {
		t.Errorf("files %v", x.Files)
	}
	if !x.Interrupted {
		t.Error("not interrupted")
	}
	if x.Model != "claude-opus-5-5" || x.Tokens["claude-opus-5-5"] != [5]int64{2000, 200} || x.Tokens["claude-haiku-5-5"] != [5]int64{1000, 10} {
		t.Errorf("model %q tokens %v", x.Model, x.Tokens)
	}
	// Opus at the config's price, Haiku at its family's.
	want := 2000*10e-6 + 200*100e-6 + 1000*1e-6 + 10*5e-6
	if math.Abs(x.Cost-want) > 1e-12 || x.CostPartial {
		t.Errorf("cost %v (partial %v), want %v", x.Cost, x.CostPartial, want)
	}
	if got.UnknownTokens != 0 || len(got.Prices) != 2 {
		t.Errorf("unknown %d, prices %v", got.UnknownTokens, got.Prices)
	}

	if got := decode[api.V2Stats](t, s, "/api/v2/stats?since=2026-06-01T00:00:00Z"); len(got.Sessions) != 0 {
		t.Errorf("a session started before since: %+v", got.Sessions)
	}
	if rec := get(t, s, "/api/v2/stats?since=yesterday"); rec.Code != 400 {
		t.Errorf("a bad since: %d", rec.Code)
	}
}
