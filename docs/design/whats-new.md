# What's new in the TUI

A design for showing the release notes in the TUI, and for saying so once after recall is updated. It follows [self-update.md](self-update.md): #75 updates recall, #76 tells of a newer release.

## Goals

- Read what changed without leaving the TUI: in a release not installed yet, and in the one just installed
- Say once, on the first start after an update, that recall was updated
- Keep the TUI from waiting for the network

Out of scope: a banner in the web UI (see open questions), rendering Markdown beyond the shape of CHANGELOG.md.

## Where the notes come from

tagpr writes `CHANGELOG.md` on every release, one section per version, newest first:

```markdown
## [1.7.2](https://github.com/babarot/claude-recall/compare/1.7.1...1.7.2) - 2026-10-07
### Bug fixes
- Do not let an import replace a newer one with stale content by @babarot in https://github.com/babarot/claude-recall/pull/71
```

The TUI reads it from `https://raw.githubusercontent.com/babarot/claude-recall/<tag>/CHANGELOG.md`, where `<tag>` is the newer release when one is known (its file has every section up to it), else the running version. raw.githubusercontent.com is not subject to the API's limit of 60 requests an hour.

It is fetched when the view opens, with a 5 s timeout, and kept in memory for the rest of the run. While it loads the view says so; a failure says why and gives the releases page, `https://github.com/babarot/claude-recall/releases`.

Embedding `CHANGELOG.md` in the binary was considered: it would work offline after an update, but `go:embed` cannot reach the repository root from a package below it, so it needs a Go package at the module root, and it cannot show a release not installed yet, so the fetch is needed anyway. One source is simpler.

Parsing, in `internal/update` (`ParseChangelog`): a `## [<version>](<url>) - <date>` line starts a version, a `### <category>` line a category, a `- <title> by @<user> in <url>/pull/<n>` line an entry, shown as `<title> (#<n>)`. Any other line is skipped, so a hand edit to the file does not break the view. Builds that set `version.Source` to anything but `release` still read it; the view does not depend on the update check.

## The view

A modal, opened with `w` (`whats_new` under `[keys]`), a global key, so from every pane, the spread conversation included. It lists every version, newest first, scrolled to the top:

```
╭ What's new ─────────────────────────────────────────────────╮
│ 1.8.0  2026-10-12  not installed · recall update            │
│   New Features                                              │
│   · Add recall update to replace recall with the latest ... │
│                                                             │
│ 1.7.2  2026-10-07  installed                                │
│   Bug fixes                                                 │
│   · Do not let an import replace a newer one with stale ... │
│ ...                                                         │
╰─────────────────────────────────────────────────────── 1-12/80 ╯
```

- A version newer than the running one is marked `not installed`, with how to update this install (the text of #76's notice); the running one `installed`
- A long title wraps under itself, so nothing is cut; the box is at most 76 columns wide
- `j` `k`, the page keys, `g` `G` scroll; `esc` `q` `w` close, as the other modals close
- It is a new `uiState`, `uiWhatsNew`, with a footer case, a `?` entry and no `?` over it, as `internal/tui/uistate_test.go` asks

`w` works whether or not a newer release is known: with none, it shows the notes up to the running version.

## After an update

`~/.local/state/claude-recall/last_version` holds the version of the last TUI run, as one line. It is a file of its own, not a field of `state.json`: the TUI rewrites `state.json` whole from what it knows, and so would an older recall that has never heard of the field, which would lose it and show the toast again. On start:

| `last_version` | Then |
|---|---|
| missing (first run, or a recall from before this) | record the running version, say nothing |
| older than the running version | toast `Updated to 1.8.0 · w what's new` for 6 s, then record |
| the same, or newer (a downgrade) | record, say nothing |

`showToast` gains a variant that takes the duration. The toast lasts longer than the usual 2.5 s, since it is read once and its key is the point. It is shown after the size settles, so it is not lost behind the first frame. The TUI writes the file at start, atomically (a temporary file renamed over it).

#76's notice and this toast share the status line: the toast takes it, the notice comes back after.

## The key in hints

`w` is the default of a new operation, `whats_new`, registered with the others in `keys.go` and `keyconfig.go`, so `[keys]` can move it or turn it off and a clash with another operation is reported. Every hint that names it (the toast, the notice, the footer, the `?` list) renders it through `hintKeys("{whats_new.0}")`, so a moved key is shown as moved; with the operation turned off (`whats_new = []`), the toast and the notice leave out `· w what's new`.

## The notice gains the key

#76's status line becomes `recall 1.8.0 is available · recall update · w what's new`, so the notes of the release on offer are one key away. When the line is too narrow, `· w what's new` is dropped before the rest is cut.

## Files

| File | Change |
|---|---|
| `internal/update/changelog.go` (new) | `ParseChangelog`, `FetchChangelog(ctx, tag)` |
| `internal/config/state.go` | `LastVersionPath`, `LoadLastVersion`, `SaveLastVersion` |
| `internal/tui/whatsnew.go` (new) | the modal: open, load, scroll, render |
| `internal/tui/uistate.go`, `keys.go`, `keyconfig.go`, `help.go`, `view.go` | `uiWhatsNew`, the `w` key, footer, `?` list, the notice's key |
| `internal/tui/model.go` | `last_version` on start, the toast |
| `cmd/recall/main.go` | hand the TUI the fetch and the running version |
| `docs/tui.md`, `docs/configuration.md` | the view, the key |

## Tests

- `ParseChangelog` on the real `CHANGELOG.md` and on a file with stray lines
- The modal: loading, loaded, failed; markers on newer and installed versions; scrolling; closing keys
- `uistate_test.go` cases for `uiWhatsNew`
- The start: each row of the `last_version` table
- The notice with and without room for `w what's new`, with the key moved, and with it turned off

## Open questions

- A banner in the web UI. It needs the server to read the update cache and the UI (React) to show it, and the web UI is opened from a terminal where `recall ui` could say it instead. Leave it out unless asked for
- The key: `w` is free in every pane. `?` lists it under the global keys
