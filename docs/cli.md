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
  --repo <dir|remote> Only one repository, by a directory in it or its remote
  --format text|json  Output format (default: text)
  --substring         Match anywhere in the text, newest first
```

Supports FTS5 query syntax: `"exact phrase"`, `term1 AND term2`, `term1 OR term2`, `term1 NOT term2`.

The full-text index splits words at spaces and punctuation, so it cannot find a word inside Japanese or other text written without spaces: `ロード` would not match `ロード時間`. A query in such a script (Japanese, Chinese, Korean) is therefore matched as plain text anywhere in a message, newest first, unless it uses `AND`, `OR` or `NOT`. `--substring` asks for the same with any query. Matching as plain text reads every message, so on an archive of a few hundred thousand messages it takes about a second where the index answers at once.

`--repo` takes a repository as a directory in it or as its git remote. A remote may be written as a URL (`https://github.com/owner/repo`, `ssh://git@github.com/owner/repo`), as `git@github.com:owner/repo.git`, or as `github.com/owner/repo`; a directory of that name on disk is taken as the directory. Remotes are compared as host and path, ignoring case, so a host other than github.com is matched the same way.

A session belongs to the repository by the origin recorded when it was imported, wherever it ran, so every checkout and worktree of one repository is one. A session that recorded no origin (outside git, or in a checkout with none) belongs to it by its directory, the way the TUI groups sessions: a linked worktree belongs to its main checkout, a removed Claude Code worktree (`<repo>/.claude/worktrees/<name>`) to the `<repo>` its path names, and a removed herdr worktree (`~/.herdr/worktrees/<name>/...`), whose path names the repository but not its owner, to the one checkout in the archive with that directory name; with none or several, it stays a repository of its own. A session that recorded another origin is left out even when it ran in the same directory, as when a path was used again for another repository.

`<dir>` may be the checkout, a worktree, a directory inside either, or a worktree since removed. Its origin is read from disk; for a directory since removed, it is the one its sessions recorded. An origin, once recorded for a session, is kept when the session is imported again.

With `--format json`, each result also has its session's `title`, `messageCount`, `firstPrompt`, `displayTitle` (the title, or else the first prompt made readable, as the TUI shows it), `recallOnly` (the session called recall's MCP tools and no other tool, so it was only a look back), `repository`, in a worktree `worktree`, and `remote`, the origin of the repository recorded when the session was imported (`github.com/owner/repo` whether the URL is https or ssh), when there is one.

## List

```
recall list [options]

  --project <name>    Filter by project
  --repo <dir|remote> Only one repository, by a directory in it or its remote
  --limit <n>         Max sessions (default: 50)
  --format text|json  Output format (default: text)
```

With `--format json`, each session also has `displayTitle`, `recallOnly`, `repository`, in a worktree `worktree`, and `remote`, as `search` has.

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
recall ui [--port <n>]        Start in the background (default port: port in the config file, or 6276); restart an older one
recall ui --foreground        Run in the foreground
recall ui status              Show server status, and with extra_projects_dirs, the trees it watches
recall ui stop                Stop the server
```

## Update

```
recall update                 Replace recall with the latest release, and restart the web UI if it runs an older one
recall update --check         Only say whether a newer release is out
```

`recall update` downloads the release binary for this platform from GitHub, checks it against the release's `checksums.txt`, runs it once and renames it over the installed one. It updates only the binary the curl installer put in place: an install from Nix or Homebrew, or a build from source, is left alone, and `recall update` says how to update it. The web UI comes back on the port in the config file and the `--db` given (or the config file's), not those of a UI started by hand with other ones.

`recall ui` also restarts a running web UI that is an older release than itself, so the UI follows an update made another way (Nix, for one). It leaves a newer one alone.

## MCP server

```
recall mcp [options]

  --repo <remote|dir> Hold every tool to one repository
```

Serves the MCP tools over stdio; Claude Code starts it. With `--repo`, every tool sees only the sessions of that repository: `repo` from the client narrows within it, `recall_export` does not find a session outside it, and `recall_stats` counts only it. The repository is a git remote, written as `--repo` of `search` takes one, or a directory on disk whose origin is read when the server starts; a directory that is gone, or whose git config cannot be read, is refused. Held to a remote, the server matches sessions by the origin they recorded and nothing else (see [ADR-005](adr/005-repository-by-remote.md)).

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
