// Command gen builds the demo that demo/tui.tape and demo/claude-*.tape (and
// tui-ja.tape) record: a home directory with a few git repositories
// and worktrees, Claude Code transcripts of the sessions in scenario.go
// (scenario_ja.go with -lang ja), an archive imported from them, a config
// file, the answer the stand-in claude (demo/bin/claude) gives to `a`, and
// a Claude Code config that lets the real claude start in the demo without
// its first-run screens.
//
// The demo lives in claude-recall-demo under the temporary directory, which
// it replaces: outside any git repository, so the TUI sees only the demo's,
// and by its real path, since the TUI resolves symlinks. demo/.out/env.sh
// points a shell at it. Run it from the repository root: go run ./demo/gen
package main

import (
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/babarot/claude-recall/internal/db"
	"github.com/babarot/claude-recall/internal/importer"
)

func main() {
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		log.Fatal(err)
	}
	out := flag.String("out", filepath.Join(tmp, "claude-recall-demo"), "directory to build the demo in")
	env := flag.String("env", "demo/.out/env.sh", "where to write the script that points a shell at the demo")
	langName := flag.String("lang", "en", "language of the conversations: en or ja")
	claudeVersion := flag.String("claude-version", "", "version of the real claude the demo starts (default: claude --version)")
	apiAddr := flag.String("api", "127.0.0.1:47123", "address of demo/fakeapi, which the real claude is pointed at")
	flag.Parse()
	if *claudeVersion == "" {
		*claudeVersion = installedClaudeVersion()
	}
	lang, ok := languages[*langName]
	if !ok {
		log.Fatalf("unknown -lang %q: en or ja", *langName)
	}
	if err := build(*out, *env, lang, *claudeVersion, *apiAddr, time.Now()); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("demo built in %s: source %s\n", *out, *env)
}

func build(root, envPath string, lang language, claudeVersion, apiAddr string, now time.Time) error {
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	home := filepath.Join(root, "home")
	for _, repo := range []string{api, web, infra, cli, dots} {
		if err := gitRepo(home, repo); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(home, notes), 0o755); err != nil {
		return err
	}

	projects := filepath.Join(root, "claude", "projects")
	for _, s := range lang.sessions {
		if err := writeTranscript(projects, home, lang, s, now); err != nil {
			return err
		}
	}

	dbPath := filepath.Join(root, "demo.db")
	d, err := db.Open(dbPath, db.Options{})
	if err != nil {
		return err
	}
	err = importer.Run(d, importer.Options{ProjectsDirs: []string{projects}}, os.Stdout)
	d.Close()
	if err != nil {
		return err
	}

	repoRoot, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	claudeJSON, err := claudeConfig(home, claudeVersion)
	if err != nil {
		return err
	}
	files := map[string]string{
		"config/claude-recall/config.toml": fmt.Sprintf("[core]\ndb = %q\n\n[tui]\ntheme = \"catppuccin-mocha\"\nscope = \"all\"\n", dbPath),
		"claude/.claude.json":              claudeJSON,
		// The demo answers its one question without asking: recall's tools
		// run with no permission prompt.
		"claude/settings.json": claudeSettings,
		envPath: fmt.Sprintf(`# Sourced by the demo tapes: a shell that sees only the demo.
# Started from inside Claude Code, the shell would hand its session to the
# claude it runs; it starts a session of its own.
unset CLAUDECODE CLAUDE_CODE_CHILD_SESSION CLAUDE_CODE_SESSION_ID CLAUDE_CODE_ENTRYPOINT CLAUDE_CODE_EXECPATH \
  CLAUDE_CODE_SESSION_ATTENDED CLAUDE_CODE_MESSAGING_SOCKET CLAUDE_CODE_MESSAGING_TOKEN CLAUDE_PID
export RECALL_DEMO=%[1]q
export HOME="$RECALL_DEMO/home"
export XDG_CONFIG_HOME="$RECALL_DEMO/config"
export XDG_STATE_HOME="$RECALL_DEMO/state"
export CLAUDE_CONFIG_DIR="$RECALL_DEMO/claude"
export PATH=%[2]q:%[3]q:"$PATH"
export PS1='$ '
# The real claude of the Claude Code demo talks to demo/fakeapi, with a key
# that is no one's, and to nothing else.
export ANTHROPIC_BASE_URL="http://%[4]s"
export ANTHROPIC_API_KEY=%[5]q
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
`, root, filepath.Join(repoRoot, "demo", "bin"), filepath.Join(repoRoot, "demo", ".out", "bin"), apiAddr, demoAPIKey),
		"ask.jsonl": askStream(lang),
	}
	for name, body := range files {
		p := name
		if !filepath.IsAbs(p) && name != envPath {
			p = filepath.Join(root, name)
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// demoAPIKey is the key the real claude sends demo/fakeapi: no one's, and
// approved in the demo's config so Claude Code does not ask about it.
const demoAPIKey = "sk-ant-api03-claude-recall-demo-not-a-real-key-0000"

// claudeSettings is the demo's Claude Code settings: recall's tools allowed.
const claudeSettings = `{
  "permissions": {
    "allow": ["mcp__plugin_claude-recall_claude-recall__recall_search", "mcp__plugin_claude-recall_claude-recall__recall_export"]
  }
}
`

// claudeConfig is the Claude Code config (CLAUDE_CONFIG_DIR/.claude.json)
// of the demo: first-run setup and the release notes of claudeVersion done,
// the notice that auto mode is the default seen, the demo's API key
// approved, and every repository and worktree trusted, so the real claude
// opens straight at its prompt.
func claudeConfig(home, claudeVersion string) (string, error) {
	projects := map[string]any{}
	for _, repo := range []string{api, web, infra, cli, dots, notes} {
		projects[filepath.Join(home, repo)] = map[string]any{"hasTrustDialogAccepted": true}
		for _, wt := range worktrees[repo] {
			projects[filepath.Join(home, wt.path)] = map[string]any{"hasTrustDialogAccepted": true}
		}
	}
	b, err := json.MarshalIndent(map[string]any{
		"hasCompletedOnboarding":   true,
		"lastOnboardingVersion":    claudeVersion,
		"lastReleaseNotesSeen":     claudeVersion,
		"hasSeenAutoDefaultNotice": true,
		"customApiKeyResponses":    map[string]any{"approved": []string{demoAPIKey[len(demoAPIKey)-20:]}, "rejected": []string{}},
		"projects":                 projects,
	}, "", "  ")
	return string(b) + "\n", err
}

// installedClaudeVersion is the version `claude --version` reports, or ""
// without a claude on PATH.
func installedClaudeVersion() string {
	out, err := exec.Command("claude", "--version").Output()
	if err != nil {
		return ""
	}
	return strings.Fields(string(out) + " ")[0]
}

// gitRepo creates a repository with one commit, and its linked worktrees.
func gitRepo(home, repo string) error {
	dir := filepath.Join(home, repo)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# "+filepath.Base(repo)+"\n"), 0o644); err != nil {
		return err
	}
	steps := [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"commit", "-q", "-m", "Initial commit"}}
	for _, wt := range worktrees[repo] {
		steps = append(steps, []string{"worktree", "add", "-q", "-b", wt.branch, filepath.Join(home, wt.path)})
	}
	for _, args := range steps {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=Demo", "-c", "user.email=demo@example.com",
			"-c", "commit.gpgsign=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s in %s: %v: %s", strings.Join(args, " "), repo, err, out)
		}
	}
	return nil
}

// sessionID is a stable UUID for a session, from its title.
func sessionID(s session) string {
	x := fmt.Sprintf("%x", sha1.Sum([]byte(s.title)))
	return x[0:8] + "-" + x[8:12] + "-4" + x[13:16] + "-8" + x[17:20] + "-" + x[20:32]
}

type obj = map[string]any

func writeTranscript(projects, home string, lang language, s session, now time.Time) error {
	id := sessionID(s)
	cwd := filepath.Join(home, s.dir)
	// Messages come in bursts with pauses between them, as real work does,
	// so the activity bars are not flat: gap(i) is the time before the i-th.
	gap := func(i int) time.Duration {
		switch {
		case i%37 == 36:
			return time.Duration(4+i%7) * time.Minute
		case i%11 == 10:
			return 90 * time.Second
		}
		return time.Duration(6+i*7%17) * time.Second
	}
	var total time.Duration
	for i := 1; i < s.msgs; i++ {
		total += gap(i)
	}
	t := now.Add(-s.ago).Add(-total)

	var lines []obj
	n := 0
	at := func() string {
		if n > 0 {
			t = t.Add(gap(n))
		}
		n++
		return t.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	user := func(content any) {
		lines = append(lines, obj{"type": "user", "uuid": fmt.Sprintf("%s-%d", id[:8], len(lines)), "sessionId": id,
			"timestamp": at(), "cwd": cwd, "gitBranch": s.branch, "version": "2.1.87",
			"message": obj{"role": "user", "content": content}})
	}
	assistant := func(blocks ...obj) {
		lines = append(lines, obj{"type": "assistant", "uuid": fmt.Sprintf("%s-%d", id[:8], len(lines)), "sessionId": id,
			"timestamp": at(), "cwd": cwd, "gitBranch": s.branch, "version": "2.1.87",
			"message": obj{"role": "assistant", "content": blocks}})
	}
	text := func(t string) obj { return obj{"type": "text", "text": t} }

	// Each thing the user says gets Claude's work and then its answer, as in
	// a real session: a short note before its first tool calls, the calls,
	// and the reply. The tool calls are spread over the turns so the session
	// reaches about msgs messages.
	turns := [][2]string{{s.prompt, s.reply}}
	for i := 0; i+1 < len(s.talk); i += 2 {
		turns = append(turns, [2]string{s.talk[i], s.talk[i+1]})
	}
	turns = append(turns, [2]string{s.last, s.answer})
	perTurn := max(1, (s.msgs-3*len(turns))/(2*len(turns)))
	step := 0
	for _, turn := range turns {
		user(turn[0])
		for i := range perTurn {
			tool, input := toolStep(s, cwd, step)
			toolID := fmt.Sprintf("toolu_%s_%d", id[:8], step)
			call := obj{"type": "tool_use", "id": toolID, "name": tool, "input": input}
			if i < 2 {
				assistant(text(lang.narrate(tool, input, step)), call)
			} else {
				assistant(call)
			}
			user([]obj{{"type": "tool_result", "tool_use_id": toolID, "content": "ok"}})
			step++
		}
		assistant(text(turn[1]))
	}
	lines = append(lines, obj{"type": "ai-title", "aiTitle": s.title, "sessionId": id})

	var b strings.Builder
	for _, l := range lines {
		j, err := json.Marshal(l)
		if err != nil {
			return err
		}
		b.Write(j)
		b.WriteByte('\n')
	}
	// Claude Code names a project's directory after its path, / and . as -.
	dir := filepath.Join(projects, strings.NewReplacer("/", "-", ".", "-").Replace(cwd))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte(b.String()), 0o644)
}

// toolStep is the step-th tool call of a session: reading and editing its
// files and running its commands, the first file and command most often.
func toolStep(s session, cwd string, step int) (string, obj) {
	pick := func(list []string) string {
		if step%3 == 0 || len(list) == 1 {
			return list[0]
		}
		return list[step%len(list)]
	}
	switch {
	case len(s.files) > 0 && step%4 == 1:
		return "Edit", obj{"file_path": filepath.Join(cwd, pick(s.files)), "old_string": "-", "new_string": "+"}
	case len(s.commands) > 0 && step%4 == 2:
		return "Bash", obj{"command": pick(s.commands)}
	case len(s.files) > 0:
		return "Read", obj{"file_path": filepath.Join(cwd, pick(s.files))}
	case len(s.commands) > 0:
		return "Bash", obj{"command": pick(s.commands)}
	}
	return "Grep", obj{"pattern": "retry", "path": cwd}
}

// narrate is the note Claude writes before a tool call.
func (l language) narrate(tool string, input obj, step int) string {
	pick := func(forms []string) string { return forms[step%len(forms)] }
	switch tool {
	case "Read":
		return fmt.Sprintf(pick(l.read), filepath.Base(input["file_path"].(string)))
	case "Edit":
		return fmt.Sprintf(pick(l.edit), filepath.Base(input["file_path"].(string)))
	case "Bash":
		return fmt.Sprintf(pick(l.bash), input["command"])
	}
	return l.grep
}

// askStream is what the stand-in claude prints for a: its searches, then
// the sessions it found with why each matches.
func askStream(l language) string {
	find := func(title string) string {
		for _, s := range l.sessions {
			if s.title == title {
				return sessionID(s)[:8]
			}
		}
		panic("no session titled " + title)
	}
	events := []obj{{"type": "system", "subtype": "init"}}
	for _, q := range l.searches {
		events = append(events, obj{"type": "assistant", "message": obj{"content": []obj{
			{"type": "tool_use", "name": "mcp__recall__recall_search", "input": obj{"query": q}}}}})
	}
	var found []obj
	for _, f := range l.found {
		found = append(found, obj{"session_id": find(f[0]), "why": f[1]})
	}
	events = append(events,
		obj{"type": "assistant", "message": obj{"content": []obj{{"type": "tool_use", "name": "StructuredOutput", "input": obj{}}}}},
		obj{"type": "result", "subtype": "success", "is_error": false, "total_cost_usd": 0.0412,
			"modelUsage":        obj{"claude-sonnet-5-5": obj{"costUSD": 0.0412}},
			"structured_output": obj{"sessions": found}},
	)
	var b strings.Builder
	for _, e := range events {
		j, _ := json.Marshal(e)
		b.Write(j)
		b.WriteByte('\n')
	}
	return b.String()
}
