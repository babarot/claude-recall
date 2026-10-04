# CLI

`recall` without a command opens the [TUI](tui.md). The other commands import, search, list and export from the terminal or a script, and start the MCP server and the web UI. `recall <command> --help` shows the same flags.

## Import

```
recall import [options]

  --session <uuid>    Import a specific session (an ID prefix works)
  --project <name>    Import sessions whose project matches
  -n, --dry-run       Show what would be imported without writing
```

## Search

```
recall search <query> [options]

  --project <name>    Filter by project
  --limit <n>         Max results (default: 20)
  --from <date>       Start date (YYYY-MM-DD)
  --to <date>         End date (YYYY-MM-DD)
  --format text|json  Output format (default: text)
```

Supports FTS5 query syntax: `"exact phrase"`, `term1 AND term2`, `term1 OR term2`, `term1 NOT term2`.

## List

```
recall list [options]

  --project <name>    Filter by project
  --limit <n>         Max sessions (default: 50)
  --format text|json  Output format (default: text)
```

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
recall ui status              Show server status
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
