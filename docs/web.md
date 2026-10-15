# Web UI

`recall ui` serves the web UI on http://localhost:6276 (127.0.0.1 only); `recall ui stop` stops it. It shows the sessions as the [TUI](tui.md) does, and reads long conversations comfortably, with the images you pasted.

The page has three columns: the folders, the sessions, and the conversation of the session selected. A narrower window hides the outline, then the folders (`[` shows them), and on a phone the list and the conversation take turns.

The page is in Japanese when the browser's first language is Japanese, and in English otherwise.

## Sessions

The title is the session's `/rename` name, or else the title Claude Code generated, or else its first prompt. Under it are the repository and the worktree (or the branch), the activity over the session and its message count. A session being written has `live`; one that only looked back through recall has `recall`.

The list goes by when sessions ended, under Today, Yesterday, This week and Earlier. `s` sorts it by when they ended or started, by message count or by size.

The folder list narrows the list to a repository, its worktrees included, or to one worktree; a worktree since removed is struck through.

## Search

`/` searches the sessions with the TUI's [filter](tui.md#filter): words in the title, folder, branch, ID or conversation, and `text:`, `title:`, `branch:`, `worktree:`, `id:` and `folder:` (`in:`). The fields are matched at once, and the conversations a moment later; a session found in its conversation shows where. `folder:` picks the folders itself, over the folder list. `Esc` clears the search.

Opening a session found in its conversation searches the conversation for the word and goes to where it was said.

## Conversation

- Tool calls one after another are one line, `12 tool calls · Bash 6 · Read 4`; open it for each call with its result. `Collapsed` and `Expanded` (`t`) show them folded or all open. Each call is paired with its result by the id the transcript gives them, and marked ✓ or ✗ as the transcript says it went. A session stored before the archive kept these, whose transcript is gone, pairs them by order, and marks ✗ only a result that reads as a failure (`Exit code 1`, `Error: ...`)
- The outline on the right lists what you wrote; click one to go there. `[` `]` go to the previous and next
- The search box over the conversation (`/` there) searches all of it, the parts not read yet too; `n` `N` and `Enter` go through the matches
- A long session is read in parts: its start and its end first, and `Load more` reads what lies between. A long message or tool result is cut, and `Show more` or `Load full text` reads the rest
- A button at the bottom of the conversation goes to its end (`Jump to latest`) when you are above it, and back to its start (`Back to top`) when you are at it
- While a session is written, its new messages come in. At the end of the conversation, the view follows them; further up, the button says how many came in
- `What was done` shows the activity, the tools and commands most used, the files edited by repository, and the details: times, counts, size, branch, folder, Claude Code version

`Y` copies the command that goes back to the session: `claude -r` in its folder, or, when its folder or transcript is gone, a new claude that recalls it through recall, as the TUI's `Y`. `y` copies the session ID.

## Keys

`?` lists every key, by where it works. The footer shows the keys of the column in focus; `Tab` moves the focus, and a click does too.

| Where | Key | Action |
|---|---|---|
| Anywhere | `/` | Search the sessions, or the conversation when it has the focus |
| | `Tab` `Shift+Tab` | Move the focus between the folders, the sessions and the conversation |
| | `[` | Show or hide the folders (in the conversation: the previous prompt) |
| | `s` | Sort |
| | `y` `Y` | Copy the session ID, or the command that goes back to it |
| | `Esc` | Back a step: clear the search, return to the list |
| Sessions | `j` `k` `↓` `↑`, `g` `G` | Move, to the top, to the bottom |
| | `Enter` | Read the conversation |
| Conversation | `j` `k`, `g` `G` | Scroll, to the start, to the end |
| | `[` `]` | The previous and next thing you wrote |
| | `n` `N` | The next and previous match |
| | `t` | Tool calls folded or all open |
| | `1` `2` | The conversation and what was done |

## Settings

The page is drawn in the TUI's color scheme, `theme` under `[tui]` in the [config file](configuration.md), read when `recall ui` starts. With `auto`, or `ansi`, which a page cannot draw, it is catppuccin-mocha or catppuccin-latte, and the switch at the top right picks `auto` (as the browser is), `dark` or `light`; with a scheme of its own, such as tokyo-night, the page takes it and the switch is gone.

The switch, how tool calls show, the sort order and whether the folders show are kept in the browser.

A page in a background tab lets go of its connection to the server and takes it again when shown, reading what changed meanwhile: a browser keeps at most six connections to one host.
