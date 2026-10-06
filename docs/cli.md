# CLI

`recall` without a command opens the [TUI](tui.md). The other commands import, search, list and export from the terminal or a script, and start the MCP server and the web UI. `recall <command> --help` shows the same flags.

## Import

```
recall import [options]

  --session <uuid>    Import a specific session (an ID prefix works)
  --project <name>    Import sessions whose project matches
  -n, --dry-run       Show what would be imported without writing
```

It reads `~/.claude/projects` (`$CLAUDE_CONFIG_DIR/projects` when set) and the trees listed in `extra_projects_dirs` under `[core]` in the [config file](configuration.md). With more than one tree it first says what it found in each:

```console
$ recall import
~/.claude/projects: 812 session files
~/containers/claude/projects: 57 session files
Syncing 869 sessions...
Imported 12 sessions (3104 messages). 857 unchanged.
```

A session whose transcript is in two trees, copied from one to the other, is imported from the copy written last. `--dry-run` names the tree of a file outside `~/.claude/projects`, and the copies it was chosen over.

## Search

```
recall search <query> [options]

  --project <name>    Filter by project
  --limit <n>         Max results (default: 20)
  --from <date>       Start date (YYYY-MM-DD)
  --to <date>         End date (YYYY-MM-DD)
  --repo <dir>        Only the repository <dir> is in, its worktrees included
  --format text|json  Output format (default: text)
  --substring         Match anywhere in the text, newest first
```

Supports FTS5 query syntax: `"exact phrase"`, `term1 AND term2`, `term1 OR term2`, `term1 NOT term2`.

The full-text index splits words at spaces and punctuation, so it cannot find a word inside Japanese or other text written without spaces: `ロード` would not match `ロード時間`. A query in such a script (Japanese, Chinese, Korean) is therefore matched as plain text anywhere in a message, newest first, unless it uses `AND`, `OR` or `NOT`. `--substring` asks for the same with any query.

`--repo` takes the repository the way the TUI groups sessions: a linked worktree belongs to its main checkout, and a removed herdr or Claude Code worktree to the checkout its path names. With `--format json`, each result also has its session's `title`, `messageCount`, `firstPrompt`, `repository` and, in a worktree, `worktree`.

## List

```
recall list [options]

  --project <name>    Filter by project
  --repo <dir>        Only the repository <dir> is in, its worktrees included
  --limit <n>         Max sessions (default: 50)
  --format text|json  Output format (default: text)
```

With `--format json`, each session also has `repository` and, in a worktree, `worktree`.

## Export

```
recall export <session-id> [options]

  --format markdown|json|text  Output format (default: markdown)
  --output <file>              Write to file instead of stdout
```

A session ID prefix works: `recall export a1b2`.

## Stats

```
recall stats [--project <name>]
```

## Web UI

```
recall ui [--port <n>]        Start in the background (default port: port in the config file, or 6276)
recall ui --foreground        Run in the foreground
recall ui status              Show server status, and with extra_projects_dirs, the trees it watches
recall ui stop                Stop the server
```

## Global options

```
--db <path>   Database file (default: db in the config file, or ~/.claude/vault.db)
-h, --help    Show help; recall <command> --help shows a command's flags
```

A flag a command does not take is an error, not ignored.

## Shell completion

```bash
source <(recall completion zsh)    # also bash, fish and powershell
```
