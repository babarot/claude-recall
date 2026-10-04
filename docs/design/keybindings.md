# The TUI's key bindings

A design of the TUI's keys, starting from which key does what and where. Implementation builds on it in two stages: stage 1 moves key handling into a keymap (with the behavior made to match this design), and stage 2 lets users change keys from the config file.

## Goals

- Say which key does what, and where, without reading the code
- Make keys and their meanings agree across panes
- Build the handling, the footer, the `?` key list and the docs from one definition
- Let users change keys (for example, swap `enter` and `space`)

In scope: the keyboard in the TUI (`recall` / `recall tui`). Out of scope: the mouse and the CLI.

## Terms

| Term | Meaning |
|---|---|
| Modal | Something that opens over the screen and takes every key until it closes: the key list (`?`), the sort menu (`s`) and the Ask Claude box (`a`) |
| Field | Where text is typed: the filter (`/`), the folder search, the conversation search and Ask's question |
| Pane | An area that can have the focus: the session list, the folder list, the detail frames (Conversation, What was done, Details) and the spread conversation |
| Global | Keys that work whatever pane has the focus |
| Session operation | An operation on the selected session (resume, read the conversation, copy its ID and so on) |

## Before

Keys were compared as strings such as `"enter"` in a `switch` per place, and handled in this hand-written order:

1. If the Ask box, the sort menu or the key list was open, it took the key and that was it
2. `?` opened the key list unless a field was being typed in
3. While the filter was being typed in, the filter took the key
4. Otherwise `updateList` took it and looked, in order:
   - While the folder search or the conversation search was being typed in, that took it
   - `tab` `]` `[` `.` `a` worked anywhere
   - With the focus on the folder list, the folder list took it
   - `enter` `y` `Y` worked on the selected session
   - With the focus on a detail frame, the frame took the scrolling keys, and any other key was handled as a list key (`goto list`)
   - The list's keys

### What did not agree

1. The keys that close differed by place. The key list closed with `?` `esc` `q`, Ask's answer with `esc` `q`, a running ask with `esc` only, the sort menu with `esc` `s` `q`
2. Paging with `ctrl+d` `ctrl+u` worked in the frames and the folder list but not in the session list
3. A key a detail frame did not know acted as a list key (`q` quit, `space` spread the conversation, `/` opened the filter), while `s` alone was special-cased not to work while reading a frame. Which keys fell through and which did not could only be told from the code
4. `a` and `.` worked in the folder list and the frames, `s` did not
5. Quitting on `ctrl+c` was written separately in each place
6. Keys were described in four places, apart from one another: the handling, the footer (`renderHelp` in `view.go`), the `?` list (`help.go`), and the README and `docs/tui.md`

## Principles

### 1. Layers

Keys are looked up from the top layer down. A key a layer takes does not go to the layers below it.

| Order | Layer | Takes |
|---|---|---|
| 1 | Fixed | `ctrl+c` (quit, anywhere) |
| 2 | Modal | Every key, while a modal is open |
| 3 | Field | Every key, while a field is being typed in (typed as text, except its submit, cancel and own helper keys) |
| 4 | Pane | The keys the focused pane declares |
| 5 | Global | Keys no pane took |

Keys never go from one pane to another. A key a detail frame does not take no longer acts as a list key; it goes to the global layer only.

### 2. Two kinds of pane

- Session panes: the session list, the detail frames and the spread conversation. They all show the selected session, so the session operations work in them
- The folder list: where a folder is picked, so the session operations do not work (`enter` there means "back to the sessions")

The session operations are keys shared by the session panes, not global keys.

### 3. Shared verbs

A key means the same in every pane.

| Verb | Keys | Meaning |
|---|---|---|
| Down, up | `j` `k` `↓` `↑` `ctrl+n` `ctrl+p` | Move in a list, scroll a frame a line |
| Page | `ctrl+d` `ctrl+u` `pgdown` `pgup` `ctrl+f` `ctrl+b` | A screen's worth. The list moves by the rows it shows, a frame keeps 2 lines of the last screen, the folder list 1 (all as before) |
| Ends | `g` `G` `home` `end` | The top, the bottom |
| Search | `/` | Search within what has the focus (see "The `/` rule") |
| Next and previous match | `n` `N` | In a pane with search results, go to the next and previous match |
| Back | `esc` | Go back a step (see "The `esc` rule") |

### 4. The `/` rule

`/` searches within what has the focus.

- The session list: the filter
- The folder list: the folder search
- The spread conversation: the conversation search
- A pane with no search of its own (a detail frame at its small size): opens the session list's filter, leaving the focus where it is, as before

The last line keeps the old behavior (`/` in a frame opens the filter) as a rule for panes without a search, rather than as a key falling from one pane to another.

### 5. The `esc` rule

`esc` goes back a step: it clears what there is to clear, and otherwise leaves for the outside.

| Where | 1st | 2nd | 3rd |
|---|---|---|---|
| The session list | Clear the filter | Clear the list narrowed by Claude's answer | Nothing |
| The session list, with the conversation spread | Clear the filter | Clear the list narrowed by Claude's answer | Put the pane back ★ |
| The folder list | Clear the folder search | Back to the sessions | |
| A detail frame | Back to the session list | | |
| The spread conversation | Clear the conversation search | Put the pane back | |
| A modal | Close it | | |
| A field | Cancel (per field, in the table below) | | |

### 6. Closing a modal

- Every modal closes with `esc`
- One without a field also closes with `q` ★ (`q` is added to a running ask)
- The key that opened it closes it too (`help` for the key list, `sort` for the sort menu). When stage 2 changes that key, the key that closes changes with it. Ask's `a` is left out, since it is a letter while the question is typed

### 7. Keys that cannot change

These are fixed, and the config file in stage 2 cannot change them either:

- `ctrl+c`
- `esc` (back), so that it means "go back a step" everywhere
- The keys of a field being typed in (submit with `enter`, cancel with `esc`, the letters themselves, a field's own helper keys)
- The keys inside a modal, except the ones that close it with the key that opened it (`help` and `sort`), which follow that operation's keys

What can change are the global keys, the session panes' keys, the folder list's keys and the shared verbs (but `esc`). Modals and fields do not read the list's keymap, so `resume = "a"`, for example, does not get in the way of typing or of Ask.

## Keys by place

★ marks what changes from the behavior before.

### Global

| Key | What it does | Operation |
|---|---|---|
| `q` | Quit | `quit` |
| `?` | Open the key list | `help` |
| `tab` `]` | Focus the next pane | `focus_next` |
| `shift+tab` `[` | Focus the previous pane | `focus_prev` |
| `a` | Open Ask Claude | `ask` |
| `s` | Open the sort menu ★ (only from the session list before) | `sort` |
| `.` | Switch between the folder recall was started in and all folders | `scope` |

### Session panes (the list, the detail frames, the spread conversation)

| Key | What it does | Operation |
|---|---|---|
| `enter` | Resume the session | `resume` |
| `c` | Continue the session in a new claude | `continue` |
| `space` | Spread the conversation, put it back | `read` |
| `y` | Copy the session ID | `copy_id` |
| `Y` | Copy the resume command | `copy_command` |
| `+` `=` | Make the detail pane taller | `grow` |
| `-` | Make the detail pane shorter | `shrink` |

### The session list

The shared verbs (move, page ★ adding `ctrl+d` `ctrl+u`, ends, `/` for the filter, `esc`), and:

| Key | What it does | Operation |
|---|---|---|
| `←` `h` | Open the folder list; when it is open, move into it | `list.folders_open` |
| `→` `l` | Close the folder list | `list.folders_close` |

With the conversation spread and the focus on the list, `j` `k` read the next session's conversation in place (as before).

### The folder list

The shared verbs (move, page, ends, `/` for the folder search, `esc`), and:

| Key | What it does | Operation |
|---|---|---|
| `→` `l` `enter` | Back to the session list | `folders.back` |

The session operations (`enter` to resume, `y` and so on) do not work here (as before).

### A detail frame (Conversation at its small size, What was done, Details)

The shared verbs (scroll by a line, by a screen, to the ends, `esc` back to the list) and the session operations. `/` opens the list's filter, as "The `/` rule" says.

★ Before, a key a frame did not take acted as a list key. In this design it goes to the global layer only. What changes as a result:

- `h` `l` `←` `→`: did nothing in a frame before either (no change)
- `s`: moves to the global layer, so the sort menu opens from a frame too ★
- Keys only for the list (`h` `l` alone, before) do not work in a frame (no change)

### The spread conversation

The shared verbs (scroll, `/` for the conversation search, `n` `N`, `esc`) and the session operations, as before.

★ `s` moves to the global layer, so the sort menu opens from the spread conversation too (it did nothing before).

### Fields

| Field | Submit | Cancel | Helper keys |
|---|---|---|---|
| The filter | `enter` | `esc` (closes the suggestions if they show, otherwise clears the filter and closes it) | `↑` `↓` move in the list, or pick a suggestion while they show. `tab` `→` complete a key or a suggestion, `shift+tab` the other way |
| The folder search | `enter` | `esc` (clears the search) | `↑` `↓` `ctrl+n` `ctrl+p` pick a folder |
| The conversation search | `enter` | `esc` (clears the search) | None |
| Ask's question | `enter` (ask) | `esc` (closes the box) | None |

All as before.

### Modals

| Modal | Keys |
|---|---|
| The key list | Closes with `esc` `q` and the `help` key (`?` by default) |
| The sort menu | `j` `k` `↓` `↑` `ctrl+n` `ctrl+p` `tab` `shift+tab` pick, `enter` `space` choose, `1` to `4` choose directly; closes with `esc` `q` and the `sort` key (`s` by default) |
| Ask, running | `esc` ★`q` cancel |
| Ask, answered | `j` `k` `↓` `↑` `ctrl+n` `ctrl+p` `tab` `shift+tab` pick, `enter` jumps, `f` narrows the list, `r` asks again, `esc` `q` close |
| Ask, failed | `r` `enter` ask again, `esc` `q` close |

## What changes from the behavior before

1. A key a detail frame does not take goes to the global layer, not to the list (no more keys falling from pane to pane)
2. `s` (sort) moves to the global layer, so it opens from the detail frames, the spread conversation and the folder list too
3. `ctrl+d` `ctrl+u` work in the session list (a screen's worth)
4. `q` cancels a running ask too
5. With the conversation spread and the focus on the list, `esc` puts the pane back when there is nothing to clear
6. `ctrl+c` is handled in one place at the top (same behavior)

## Stage 1: a keymap

- `internal/tui/keys.go` defines the keymap per layer with bubbles' `key.Binding`. Each operation has its default keys, its name, and its description for the footer and the `?` list
- Keys are matched with `key.Matches`, in one function that looks through the layers in order. The hand-written chain and `goto list` go away
- The keys in the footer and the `?` list are built from the keymap, so changing a key changes what they show. They are built by these rules:
  - An entry's keys are written as a template. `{operation.n}` stands for the operation's n-th key (counting from 0), and anything else (spaces, `/` and so on) is shown as is. For example:
    - The `?` list's move line: `{up.0} {down.0}  {down.1} {up.1}` → `↑ ↓  j k` (with the default keys `up = [up, k, ...]`, `down = [down, j, ...]`; as before)
    - The footer's move in the list: `{up.0}{down.0}` → `↑↓`. Scrolling the spread conversation: `{down.1} {up.1}` → `j k`
    - Resizing: `{grow.0}/{shrink.0}` → `+/-` in the footer, `{grow.0} {shrink.0}` → `+ -` in the `?` list
  - Fixed keys that cannot change (`esc`, the keys of fields and modals) are written in the template as they are: `tab  →` on the filter's completion line, `↑ ↓  enter` on the line for picking a suggestion, `{help.0} esc q close` on the key list's frame
  - When a changed keymap has no key at the index a template names, that `{…}` is left out with the separator before it (after it, at the start). An entry with none of its keys left is left out whole
  - Key names are shown as symbols: `up` → `↑`, `down` → `↓`, `left` → `←`, `right` → `→`, `shift+tab` → `⇧tab`
  - A description is a string or a function returning one, given the model's state and the keymap (the current keys). So descriptions that change with the state (`.`'s this folder and all folders, `→ close folders` while the folder list is open, `esc clear search` during the folder search, `tab key <hint>` in the filter) and other keys named inside a description (the "`[` and `]` too" of `tab  ⇧tab`, the "g G top and bottom" of `move`) follow a changed key too
  - Lines that are not keys (the filter syntax in the `?` list, such as `folder:` `text:`, and the footer's `folder: text: title: ...`) are entries with a description only
  - The hint on a modal's frame is built by the same rules. A hint with the key that opened the modal (the key list's `? esc q close`) comes from the template `{help.0} esc q close` and follows the `help` key. A hint with fixed keys only (the sort menu's `enter esc`, the Ask box's `enter ask · esc close`, `esc cancel`, `r ask again · esc close`) stays a fixed string. Keys are written into hints in these three places only: the footer, the `?` list and the modals' frames
  - The default keymap orders each operation's keys so that these rules show what was shown before. Tests check that the `?` list and the footer show the same as before in every place
- Fields and modals have keymaps too, but stage 2's config does not cover them
- Tests
  - A table test from a place and a key to the operation expected pins down "Keys by place" above
  - A test that the default keymap has no conflicts (see "Conflicts" below)
  - The existing screen tests pass unchanged, except where the behavior changes (the 6 above). A test that hits one of those (such as `TestSortMenu`'s "`s` does nothing while reading a frame") is rewritten in the commit that changes the behavior
- The behavior changes (1 to 5 above) are commits of their own, apart from the move to a keymap

## Stage 2: keys from the config file

```toml
[keys]
resume = ["space"]
read   = ["enter"]
```

- Under `[keys]`, an operation name takes a list of keys, or a string for one. An operation written there has its default keys replaced
- An operation that works in one pane only is written in that pane's table: `folders_open` and `folders_close` in `[keys.list]`, `back` in `[keys.folders]`. They are named with the table: `list.folders_open`, `folders.back`. A line in the wrong place (`resume` under `[keys.list]`, `folders_open` directly under `[keys]`) is an error that says where it goes
- `[]` turns the operation off
- Keys are written as bubbletea writes them (what `KeyPressMsg.String()` returns: `enter`, `space`, `ctrl+d`, `shift+tab`, a single character and so on). Neither bubbles nor bubbletea has a list of key names or a parser, so the config reader keeps the list of names it accepts
  - Accepted: a printable character (capitals included), a named key (`enter` `space` `tab` `backspace` `up` `down` `left` `right` `home` `end` `pgup` `pgdown` `insert` `delete` `f1` to `f12`), and either with `ctrl+` or `alt+`. `shift+` goes with named keys only (`shift+tab`, `shift+up` and so on)
  - A shifted character is written as itself (`Y`, `?`). bubbletea sends shift+y as `Y`, not `shift+y`, so `shift+y` would never match. `shift+` with a printable character is an error whose message gives the way to write it (`Y`)
  - Anything else (`ctrl-d`, `Enter` and so on) is an error, rather than a key that silently never works
- The footer and the `?` list show the changed keys. The footer is ordered by key: a key shows where it is by default (with `continue = ["enter"]` and `resume = ["c"]`, `enter continue` still comes first), and an operation on a key that no operation has by default goes after the rest. The `?` list groups operations by meaning, so it keeps the operations' order
- These are errors when the file is read, reported rather than ignored, as an unknown setting is:
  - An unknown operation
  - A key that cannot be written (the fixed keys included)
  - A conflict (see below)

### Conflicts

In each place keys are taken, the keys of the operations that work there do not overlap.

- The session list: global + session panes + shared verbs + the list's keys
- A detail frame: global + session panes + shared verbs
- The spread conversation: global + session panes + shared verbs
- The folder list: global + shared verbs + the folder list's keys

Inside a modal, the key that opened it (`help` and `sort`) does not overlap that modal's fixed keys either. `sort = ["1"]`, for example, is an error, since it overlaps the sort menu's "choose the first". `help` and `sort` cannot take `esc` or `q` (they are fixed keys, so the rule above rejects them first).

The folder list's `enter` (back to the sessions) and the sessions' `enter` (resume) never show up in the same place, so they do not conflict. Swapping `resume = ["space"]` and `read = ["enter"]` overlaps nowhere, so it is valid.

A key written in the config file that a default key has is not an error: the key is taken from the default operation, so that what is written wins, as if the key itself had been given to that operation. `continue = ["enter"]` alone leaves `resume` with no key (and out of the footer and the `?` list). `resume = ["j"]` takes `j` from "down", which keeps its arrow and `ctrl+n`. The key is taken from the operation itself, so `j` is no longer "down" even where the two would not meet (the folder list, where `resume` does not work). Only operations written in the file that overlap one another (`resume = ["j"]` with `down = ["j"]`) and keys that overlap fixed keys are errors.

## Decided

Stage 1 landed in #39 and stage 2 in #41. The questions left open when this was designed were decided as follows:

1. `s` (sort) moved to the global layer; it opens from the frames, the spread conversation and the folder list too
2. With the conversation spread and the focus on the list, `esc` puts the pane back when there is nothing to clear
3. `esc` stays fixed; the config file cannot change it
4. The shared verbs (move, page, ends, search, next and previous match) can be changed in the config file

Added since:

- With the focus on the spread conversation, `q` (the `quit` key) puts the pane back instead of quitting (#45). With the focus on the list, it quits as before
- In a field, `ctrl+v` and the terminal's own paste (`cmd+v`) paste the clipboard (#47). `ctrl+v` is a fixed key of the field layer, outside the config file
- A mistake under `[keys]` is reported with where it is in the config file (#44)
- `c` (`continue`) continues the session in a new claude that recalls it, and a key written in the config file takes over from the default operation that has it, rather than being a conflict (#48)

For users, the keys are described in [docs/tui.md](../tui.md#changing-keys).
