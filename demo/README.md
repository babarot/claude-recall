# Demo

The README's GIFs are recorded with [VHS](https://github.com/charmbracelet/vhs) on a demo archive, so nothing of your own archive, config or home shows up:

| GIF | Tape | Make |
|---|---|---|
| `tui.gif`, the TUI | `tui.tape` | `make demo-tui` |
| `claude-band.gif`, `claude-search.gif`, `claude-command.gif`: the plugin's mod in Claude Code | `claude-band.tape`, `claude-search.tape`, `claude-command.tape` | `make demo-claude` |

Each target builds `recall`, runs `demo/gen` and then VHS on its tape. Re-record a GIF after a change to what it shows, and commit the new one.

- `gen/` writes a demo home directory with a few git repositories and worktrees, Claude Code transcripts of the sessions in `gen/scenario.go`, and an archive imported from them. It lives under the temporary directory, outside any git repository, and `demo/.out/env.sh` points a shell at it.
- `bin/claude` stands in for Claude Code in the TUI demo: `a` in the TUI runs it, and it replays the answer `gen` wrote.
- `fakeapi/` stands in for the Anthropic API in the Claude Code demo: it answers the one question `claude-search.tape` asks by calling `recall_search`, then sums up the result, the same every time.
- Dates are relative to when `gen` runs, so the demo always reads "Today" and "Yesterday".

## Claude Code

`make demo-claude` runs the real Claude Code (`CLAUDE`, by default the `claude` on PATH; one that loads mods, such as 2.1.291) with the plugin from this checkout (`--plugin-dir plugin`), on a fresh demo for each tape.

- `gen` writes the demo's Claude Code config: the first-run setup, the release notes of that version and its notices done, every demo repository trusted, an API key that is no one's approved, and recall's tools allowed, so it opens straight at the prompt and asks nothing.
- `env.sh` points Claude Code at `fakeapi`, which the target runs on `DEMO_API` (127.0.0.1:47123) while it records. So the demo needs no login, costs nothing and reads the same on every run. It also drops the variables of a Claude Code session it may have been started from.
- Pressing an ID, which asks Claude for a recap, is not in the demo.

A new Claude Code version may bring a notice or a first-run screen of its own, or ask `fakeapi` for something it does not answer; if one shows in a GIF, mark it seen in `claudeConfig` in `gen/main.go`, or answer it in `fakeapi`.

## Japanese

The demos with the conversations, the question to Claude and the searches in Japanese:

```bash
make demo-tui-ja
make demo-claude-ja
```

They write `demo/ja/tui.gif` and `demo/ja/claude-*.gif`, which are not committed. The sessions are in `gen/scenario_ja.go`, with the code, commands and paths as in English. For the Claude Code demo, `gen -lang ja` sets Claude Code's `language` to Japanese, which the plugin's drawings follow, and `fakeapi -lang ja` searches for and answers in Japanese; Claude Code's own words stay English. The `-ja` tapes record them in IBM Plex Mono and IBM Plex Sans JP, which have to be installed (`brew install --cask font-ibm-plex-mono font-ibm-plex-sans-jp`).
