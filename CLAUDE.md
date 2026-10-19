# CLAUDE.md

## Database

- The SQLite DB started as a derived cache of the JSONL transcripts, but it is now the only copy of sessions whose JSONL has been deleted (Claude Code removes old transcripts). Treat it as irreplaceable
- Never delete or rebuild `~/.claude/vault.db`. Schema changes are migrations in `internal/db/db.go` that only add (`ALTER TABLE ... ADD COLUMN`, new tables or indexes), tracked by `PRAGMA user_version`
- `internal/db/schema.go` uses `CREATE ... IF NOT EXISTS`, so it is safe to execute on every startup
- Develop and test against a copy, not the live file: `mise run dev` works on `.dev/vault.db`, which `mise run dev:db` copies afresh (`sqlite3 ~/.claude/vault.db ".backup '/path/to/copy.db'"` for a copy elsewhere)

## Compatibility

- MCP tool names, inputs and results, and the HTTP API the web UI reads, are interfaces other tools depend on. Add fields rather than change or remove them
- The `history` package is the Go API other modules import, under v1 semver. Add to it rather than change or remove what it exports, keep it read-only, and keep its imports to the standard library, `modernc.org/sqlite` and the `internal/` packages that need nothing more
- `internal/jscompat` keeps stored text identical to what the earlier TypeScript importer wrote (JavaScript trim, UTF-16 lengths, JSON.stringify). Keep using it in the parser so re-imports do not rewrite existing rows

## Agents

- Sessions come from two agents: Claude Code's JSONL (`internal/parser`) and Codex's rollouts (`internal/codex`), told apart by the tree's layout in `parser.Discover` and by the file name (`rollout-...`) in the importer. Both become the same `parser.Session`; `sessions.agent` ('claude' or 'codex', NULL meaning claude) says which. Anything that runs or names the agent goes through `internal/resume` (`Program`, `ResumeArgs`, `AgentRecallArgs`) and `parser.AgentName`, not a literal `claude`
- The JSON of the CLI, the MCP tools and the HTTP API carries `agent`; filters take it as `--agent`, the `agent` input and `agent:` in the TUI filter. See docs/codex.md

## TUI

- A new pane, modal or field is a value of `uiState` in `internal/tui/uistate.go`, so keys reach it and the footer and the `?` key list know it. Then fill in what the tests in `internal/tui/uistate_test.go` ask for: a footer case in `renderHelp`, a case in `footerCases`, whether `?` opens over it, and for a pane its group in `helpGroups`
