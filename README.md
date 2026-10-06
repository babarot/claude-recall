# claude-recall

[![Test](https://github.com/babarot/claude-recall/actions/workflows/test.yaml/badge.svg)](https://github.com/babarot/claude-recall/actions/workflows/test.yaml)

Recall any past Claude Code session: ask Claude to look into it, or find it yourself and go back to it.

![The recall TUI: looking through sessions, narrowing to a folder and what was said, reading a conversation, and asking Claude](demo/demo.gif)

claude-recall archives every Claude Code session into SQLite, including the ones whose JSONL Claude Code has since deleted, and gives you three ways back into them from one binary called `recall`:

- MCP server: Claude searches past sessions and reads what was done in them, from inside the session you are working in
- TUI: find a session yourself and resume it where it left off
- Web UI: read past conversations comfortably in the browser, live as they are written

With the [Claude Code plugin](#claude-code-plugin), recall also shows up [in Claude Code](#in-claude-code) itself: the repository's last sessions when you start, search results you can read at a glance, and `/recall`.

## Quick start

```bash
curl -fsSL https://raw.githubusercontent.com/babarot/claude-recall/main/bin/install.sh | bash
recall
```

The installer puts `recall` in `~/.local/bin`, imports your sessions and registers the MCP server with Claude Code. Then ask Claude about a past session ("how did we set up the staging deploy last month?"), or run `recall` to browse them. See [Install](#install) for Nix, building from source and the Claude Code plugin.

## Use cases

### Pick up where an earlier task left off

At work, a project rarely fits in one session. Project A gets split into tasks 1 to 10, and each one runs in its own session. While working on task 5, you need what was decided in task 3: ask Claude "what did we decide about the retry policy in the task 3 session?" and it looks that session up through MCP and answers from it. Or, to carry on with task 3 itself, find it in the TUI and press Enter to resume it.

### Answer a question you already answered months ago

A support question comes in, you open a session, and it is settled in ten minutes. Months later a similar question arrives. By then Claude Code has deleted that transcript, but claude-recall still has it: ask Claude to find how it was handled last time, or search for it yourself and read it again.

## How to recall

| You want to | Use | What only it gives you |
|---|---|---|
| Have Claude remember what happened in a past session | [MCP server](#mcp-server) | The answer lands in the session you are working in, as context Claude can use right away |
| Find a session yourself and go back to it | [TUI](#tui) | Resume it in place (`claude -r`), recall it in a new claude, or copy its ID to hand to another agent |
| Find a session yourself and read it | [Web UI](#web-ui) | A comfortable reader in the browser, for long conversations and images |
| Look back without leaving the session | [In Claude Code](#in-claude-code) | The repository's last sessions as you start, a search without a model turn, and a recap of any session with one press |

The [CLI](#cli) underneath starts each of them (`recall mcp`, `recall`, `recall ui`) and also searches, lists and exports from the terminal or a script.

### MCP server

Each Claude Code session runs its own `recall mcp`, so Claude can look into past sessions whenever it needs to. Nothing is injected into the prompt: Claude calls these tools when you ask about the past or when it realizes something was discussed before.

| Tool | Description |
|------|-------------|
| `recall_search` | Full-text search across past sessions, optionally within one repository (`repo`); each hit names its session's title, size and repository |
| `recall_list` | List archived sessions, optionally of one repository (`repo`) |
| `recall_export` | Export a session's full conversation, or with `tail` only its last messages |
| `recall_stats` | Show archive statistics |

It also offers one prompt, `recap`, which has Claude read where a session ended, sum up what was done, decided and left, and ask how to go on. Claude Code lists it as a command named after the server: `/claude-recall:recap <session-id>` with `claude mcp add`, `/plugin:claude-recall:claude-recall:recap <session-id>` from the plugin.

A session ID from the TUI (`y`) works too: "look up session a1b2c3 with claude-recall and continue from there".

### TUI

`recall` lists every archived session with its title, folder (a git worktree is shown under the repository it belongs to), branch, message count, size and ID. Started inside a repository, it shows only that repository's sessions; `.` switches to all of them, and `recall --all` starts with all of them.

| Key | Action |
|-----|--------|
| `Enter` | Resume the session: `claude -r <id>` from the session's folder |
| `c` | Recall the session in a new claude through MCP: for a session `claude -r` cannot resume, such as one whose worktree was removed |
| `y` / `Y` | Copy the session ID / the resume command (the recall command when it cannot be resumed) |
| `/` | Filter by title, folder, branch, ID or what was said; `text:`, `title:`, `folder:` and others narrow it to one field |
| `a` | Ask Claude to find sessions, when you remember what it was about but not what to type. It runs your own `claude -p`, so no API key is needed |
| `Space` | Read the conversation over the detail pane |
| `?` | Show every key, the ones for where you are first |

Its settings are under `[tui]` in the [config file](#configuration), and `[keys]` changes which keys do what. See [docs/tui.md](docs/tui.md) for every key, the filter, the folder list, the detail pane and the mouse.

### Web UI

```bash
recall ui            # http://localhost:6276, in the background
recall ui stop
```

A session browser, a chat viewer and search. While it runs, new and changed sessions are imported within about half a second, the session list moves them to the top, and the chat view of a running session follows new messages. The server listens on 127.0.0.1 only.

### In Claude Code

The [plugin](#claude-code-plugin) carries a Claude Code hooks module (a mod) that draws recall in the session you are working in:

- When a session starts, the latest sessions of the same repository, its worktrees included, show above the prompt until you send the first prompt.
- When Claude calls `recall_search`, the result shows as a tree of repositories, sessions (title, message count, date) and the messages that matched, instead of JSON. Claude still reads the JSON.
- `/recall <query>` searches this repository's sessions on the spot, without a model turn, and shows them as a table; `--all` searches every session. Claude reads the same list, so "read number 2" works next.
- Pressing a session's ID in any of them, or `/recall <n>`, runs the `recap` prompt on it: Claude reads where that session ended, sums up what was done, decided and left, and asks how to go on.

Sessions that only looked back through recall (a search for a word, a recap) are put together on one line instead of a row each; their IDs can still be pressed. The words are English, or Japanese when Claude Code's `language` setting is Japanese.

It needs a Claude Code that loads hooks modules (tested on 2.1.291). [docs/plugin.md](docs/plugin.md) covers how it works.

## Why

claude-recall is a recall tool, not a memory system. The goal is to make `grep ~/.claude/projects/**/*.jsonl` a better experience, and to keep those files around after Claude Code deletes them. When you or the agent realize something was discussed before, you look it up. See [ADR-001](docs/adr/001-recall-not-memory-extension.md).

### Why not just [claude-mem](https://github.com/thedotmack/claude-mem)?

[claude-mem](https://github.com/thedotmack/claude-mem) is an excellent project solving a related problem, and if it fits your workflow, you should use it. claude-recall deliberately solves a different one.

claude-mem extends the agent's memory, for Claude Code and many other agents. It captures observations on every tool use, summarizes them with an LLM into structured facts, stores them in a vector DB, and injects the result into the next session's prompt. claude-recall doesn't touch the memory layer. It stores the JSONL files Claude Code already writes and searches them with SQLite FTS5.

|  | claude-mem | claude-recall |
|---|---|---|
| Goal | Extend the agent's memory | Help you and the agent recall |
| Injection | Push (auto-injected at `SessionStart`) | Pull (looked up when needed) |
| What's stored | LLM-summarized observations | Raw conversation, noise-stripped |
| Search | FTS5 + Chroma vector hybrid | FTS5 only (deterministic) |
| LLM calls | During indexing, through a hosted observer, your own OpenRouter or Gemini key, or your Anthropic plan | Never while storing or searching; only when you ask Claude from the TUI (`a`), through your own `claude` |
| Where data lives | Local, with optional cloud sync | Local only |
| Agents | Claude Code, Codex, Gemini, OpenCode and more | Claude Code |
| Runtime | Node + Bun + Python (uv) + Chroma, resident worker | One static binary, no daemon |

The tradeoff claude-recall picks:

- Raw logs don't drift. What's stored is what happened, not a summary of it.
- The agent says when it doesn't know. Past context comes from a tool call, not from memory it may misremember.
- Idle means idle. No worker, no background LLM calls.
- One binary, works offline.

## Install

Getting started takes three steps: put `recall` on PATH, import your sessions into the archive, and connect Claude Code to the MCP server. What each way of installing does for you:

| Install | `recall` on PATH | Import | MCP server |
|---------|------------------|--------|------------|
| [curl](#curl) | Yes | Yes | Yes, when `claude` is on PATH |
| [Nix](#nix) | Yes | No | No |
| [Build from source](#build-from-source) | Yes | No | No |
| [Claude Code plugin](#claude-code-plugin) | No | When a Claude Code session starts | Yes |

Whatever is left is in [Set up](#set-up).

### curl

```bash
curl -fsSL https://raw.githubusercontent.com/babarot/claude-recall/main/bin/install.sh | bash
```

Installs `recall` to `~/.local/bin` (set `RECALL_INSTALL_DIR` to change it), imports your sessions and registers the MCP server.

### Nix

Each release is published to [babarot/nur-packages](https://github.com/babarot/nur-packages):

```bash
nix profile install github:babarot/nur-packages#claude-recall
```

The package also carries the [Claude Code plugin](#claude-code-plugin) under `share/claude-plugin/claude-recall`. Then [set up](#set-up).

### Build from source

Requires Go and Node.js (for the web UI):

```bash
git clone https://github.com/babarot/claude-recall.git
cd claude-recall
make install   # builds the UI, embeds it, installs recall to ~/.local/bin
```

`go install github.com/babarot/claude-recall/cmd/recall@latest` also works; that build leaves the web UI out. Then [set up](#set-up).

### Claude Code plugin

The plugin is the recommended way to connect claude-recall to Claude Code. [`plugin/`](plugin) bundles, with `recall` on PATH:

| Component | What it does |
|-----------|--------------|
| MCP server | Runs `recall mcp` |
| `SessionEnd` hook | Runs `recall import` when a session ends |
| Hooks module | Draws recall [in Claude Code](#in-claude-code) and adds `/recall` |

An older Claude Code, which does not load hooks modules, still gets the MCP server and the hook.

Each release ships it as `claude-recall-plugin.tar.gz`. A plugin directory under `~/.claude/skills/` loads as `claude-recall@skills-dir`, so with Nix:

```bash
ln -s ~/.nix-profile/share/claude-plugin/claude-recall ~/.claude/skills/claude-recall
```

Without the plugin, register just the MCP server, as in [Set up](#set-up).

### Set up

After installing with Nix or from source, import your sessions and connect Claude Code:

```bash
recall import                                        # create the archive from ~/.claude/projects
claude mcp add claude-recall -s user -- recall mcp   # or install the plugin
```

Until the archive exists, `recall` and its `search`, `list`, `export` and `stats` commands say how to set it up and exit with status 1. The MCP server and the web UI create the archive and import into it when they start, so with the MCP server connected, the first Claude Code session you start does the import too.

## Your archive

The archive is `~/.claude/vault.db`. Sessions whose JSONL Claude Code has deleted stay in it, so it is the only copy of them: back it up, and never delete it to rebuild it.

```bash
sqlite3 ~/.claude/vault.db ".backup '/path/to/backup.db'"
```

Sessions are imported from `~/.claude/projects` (`$CLAUDE_CONFIG_DIR/projects` when Claude Code runs with `CLAUDE_CONFIG_DIR`), and from the trees `extra_projects_dirs` in the [config file](#configuration) lists, such as a container's; [docs/architecture.md](docs/architecture.md#sync-timing) says when. Symlinked project directories and transcripts are followed. The archive stays in `~/.claude` either way; `db` in the [config file](#configuration) moves it.

| Stored | Excluded |
|--------|----------|
| User and assistant text | System events (turn_duration and others) |
| Thinking | File history snapshots |
| Tool calls and their results | Sidechains |
| Slash-command expansions, task notifications | Progress, queue operations and other bookkeeping |

## Configuration

`~/.config/claude-recall/config.toml` (or `$XDG_CONFIG_HOME/claude-recall/config.toml`). `recall` writes it the first time the TUI runs, with every setting at its default and commented out; a setting left out is its default, so set only what you change:

```toml
[core]
db = "~/.claude/vault.db"   # the archive, unless --db says otherwise

[ui]
port = 6276                 # the web UI, unless --port says otherwise

[tui]
theme = "auto"              # or tokyo-night, dracula, nord, gruvbox-dark, ansi
detail_position = "bottom"  # or "right", or "auto" on a wide terminal

[keys]
resume = "space"            # swap the keys of resume and read
read = "enter"
```

A mistake in it is reported with the line it is on, not ignored. See [docs/configuration.md](docs/configuration.md) for every setting, and [Changing keys](docs/tui.md#changing-keys) for `[keys]`.

## CLI

```bash
recall                      # Browse sessions (same as recall tui)
recall import               # Import all sessions
recall search "terraform module"
recall search "deploy" --project oksskolten --from 2026-03-01
recall list --project gh-infra --format json
recall search "staging" --repo .            # this repository, its worktrees included
recall export <session-id> --format json --output session.json
recall stats
recall ui                   # Web UI in the background (http://localhost:6276)
recall mcp                  # MCP server over stdio (started by Claude Code)
recall version
```

Each command's flags are in [docs/cli.md](docs/cli.md) and `recall <command> --help`.

## Development

```bash
make test                      # go vet, go test, UI tests
make build                     # recall with the web UI embedded
go run ./cmd/recall search "query" --db /tmp/vault-copy.db

# Web UI: Vite dev server on 5173, proxying /api to the Go server on 6276
cd ui && npm run dev
go run ./cmd/recall ui --foreground
```

Work against a copy of the archive (`sqlite3 ~/.claude/vault.db ".backup '/tmp/vault-copy.db'"`), not the live file.

After a change to how the TUI looks, re-record the demo GIF with `make demo` (see [demo/README.md](demo/README.md)).

For the plugin's hooks module, `claude --plugin-dir plugin` loads it from the checkout and reloads it as you edit, and `claude plugin test plugin` runs its tests (see [docs/plugin.md](docs/plugin.md#developing)).

[docs/architecture.md](docs/architecture.md) covers how sessions flow into the archive, the schema and the web UI's live updates. The [ADRs](docs/adr) record the larger decisions, such as why claude-recall moved from Deno to Go ([ADR-004](docs/adr/004-go-port.md)).

### Tech stack

- Go, [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (SQLite without CGo) with FTS5
- [Bubble Tea](https://github.com/charmbracelet/bubbletea), Bubbles and Lip Gloss (TUI)
- The [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [Preact](https://preactjs.com/), [Vite](https://vitejs.dev/), [Tailwind CSS](https://tailwindcss.com/) and [marked](https://marked.js.org/) (web UI)

## License

MIT
