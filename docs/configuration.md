# Configuration

`~/.config/claude-recall/config.toml` (or `$XDG_CONFIG_HOME/claude-recall/config.toml`). `recall` writes it the first time the TUI runs, with every setting at its default and commented out; uncomment a line to change it. A setting left out, or commented out, is its default, so the file needs only the settings you change. [Every setting](#every-setting) is below with its default.

| Section | What it sets |
|---|---|
| `[core]` | `db`, the archive, and `extra_projects_dirs`, more transcript trees to read, for every command, the MCP server and the web UI |
| `[ui]` | `port`, where the web UI listens |
| `[tui]` | The TUI: the detail pane, the color scheme, which sessions to start with, `a` (ask Claude), the scrollbar and images |
| `[keys]` | Which keys do what in the TUI; see [Changing keys](tui.md#changing-keys) |

`db` and `port` are the defaults of `--db` and `--port`, so the MCP server and the `SessionEnd` import use the archive you set here too. A flag on the command line wins over the file.

## Mistakes

An unknown key, a key in the wrong section, a value of the wrong type and, under `[keys]`, a key written another way or two operations that share a key are reported rather than ignored, each where it is in the file:

```console
$ recall
/Users/you/.config/claude-recall/config.toml:5:18: "shift+y" is never read; write "Y"
  |
5 | read = ["enter", "shift+y"]
  |                  ^~~~~~~~~
```

A mistake under `[tui]` or `[keys]` stops only the TUI; the MCP server, the web UI and the other commands still run.

## Every setting

Each at its default, so a line copied from here changes nothing until its value is edited. Under `[keys]`, copy only the lines you change: a key you give an operation is taken from the operations that have it by default, but two operations written in the file cannot share one, so `recall = "enter"` alone works, while the same next to `resume = "enter"` is an error.

```toml
[core]
# The archive database, for every command, the MCP server and the web UI,
# unless --db says otherwise: an absolute path or one starting with ~/.
db = "~/.claude/vault.db"
# Transcript trees to import, search and watch beside ~/.claude/projects (or
# $CLAUDE_CONFIG_DIR/projects): the projects directory of each, absolute or
# starting with ~/, such as "~/containers/claude/projects" for Claude Code
# run in a container whose config directory is bind-mounted from the host.
# claude -r does not read them; c recalls their sessions in a new claude.
extra_projects_dirs = []

[ui]
# Where the web UI (recall ui) listens, and where recall ui stop and
# recall ui status look for it, unless --port says otherwise.
port = 6276

[tui]
# Where the detail pane goes: "bottom" (default), "right", or "auto" to put it
# on the right when the terminal is at least detail_auto_width columns wide.
detail_position = "bottom"
detail_auto_width = 160
# Initial height of the detail pane below the list, in lines (at least 10).
detail_height = 16
# Color scheme: "auto" (default) picks catppuccin-mocha on a dark terminal and
# catppuccin-latte on a light one. Also: tokyo-night, dracula, nord,
# gruvbox-dark, and ansi (the terminal's own 16 colors).
theme = "auto"
# Which sessions to start with: "folder" (default) for the repository recall is
# started in, when it has sessions, or "all".
scope = "folder"
# a asks Claude Code (claude -p, on your Claude plan) to find sessions.
# The model: a family and version such as "sonnet-5.5", "opus-5.5" or
# "haiku-4.5", or a full model ID. Whether to show what an answer cost (the
# price claude reports; on a Claude plan it counts toward your usage rather
# than being billed), and why Claude picked each session.
ask_model = "sonnet-5.5"
ask_show_cost = true
ask_reasons = true
# The scrollbar on the right edge of a detail frame whose content scrolls. The
# thumb: "thin" (│), "heavy" (┃, default) or "block" (█). Its color: a hex
# color such as "#f5a3b5" or an ANSI color number (0-255); empty (default) is
# the frame's border color, so "thin" needs a color to stand out.
scrollbar_thumb = "heavy"
scrollbar_color = ""
# Show the images pasted into a session in the spread Conversation (Space),
# where they were pasted. Needs a terminal that draws Kitty graphics with
# Unicode placeholders, such as Ghostty or Kitty; elsewhere they come out as
# stray characters, so it is off by default.
images = false

[keys]
# Which keys do what in the TUI, by operation: a key or a list of keys,
# replacing the operation's own, or [] to turn it off. A key given to an
# operation leaves the ones that have it by default: recall = "enter"
# takes enter from resume. Every operation is below with its keys;
# docs/tui.md says how keys are written. ctrl+c and esc are fixed. For
# example, to resume with space and read with enter, swap the keys of
# resume and read.
#
# Anywhere:
quit = "q"
help = "?"
focus_next = ["tab", "]"]
focus_prev = ["shift+tab", "["]
ask = "a"
sort = "s"
scope = "."
#
# The selected session, from the list, a frame or the spread conversation:
resume = "enter"
recall = "c"
read = "space"
copy_id = "y"
copy_command = "Y"
grow = ["+", "="]
shrink = "-"
#
# Moving and searching, in every pane:
up = ["up", "k", "ctrl+p"]
down = ["down", "j", "ctrl+n"]
page_up = ["pgup", "ctrl+b", "ctrl+u"]
page_down = ["pgdown", "ctrl+f", "ctrl+d"]
top = ["home", "g"]
bottom = ["end", "G"]
search = "/"
next_match = "n"
prev_match = "N"
#
# Keys that work in one pane go in its table, after the lines above.
# The session list:
[keys.list]
folders_open = ["left", "h"]
folders_close = ["right", "l"]
#
# The folder list:
[keys.folders]
back = ["right", "l", "enter"]
```
