# Changelog

## [v1.13.0](https://github.com/babarot/claude-recall/compare/v1.12.0...v1.13.0) - 2026-10-28
### New Features
- Add a button to jump to the end or the start of a conversation in the web UI by @babarot in https://github.com/babarot/claude-recall/pull/113
- Show the web UI in English unless the browser prefers Japanese by @babarot in https://github.com/babarot/claude-recall/pull/115
### Improvements
- Reword unclear TUI toasts and call the spread conversation the reading view by @babarot in https://github.com/babarot/claude-recall/pull/116
- Run the tasks with mise instead of make by @babarot in https://github.com/babarot/claude-recall/pull/117
- Add mise run dev, the web UI and its API on a copy of the archive by @babarot in https://github.com/babarot/claude-recall/pull/118
- Show images in the TUI when the terminal draws Kitty graphics by @babarot in https://github.com/babarot/claude-recall/pull/121

## [v1.12.0](https://github.com/babarot/claude-recall/compare/v1.11.0...v1.12.0) - 2026-10-13
### New Features
- Store whether a tool result was an error by @babarot in https://github.com/babarot/claude-recall/pull/104
- Make the checkouts of one git remote one folder by @babarot in https://github.com/babarot/claude-recall/pull/108
### Bug fixes
- List only repositories under the web UI's repositories by @babarot in https://github.com/babarot/claude-recall/pull/111
### Improvements
- Pair tool results with their calls by id in the web UI by @babarot in https://github.com/babarot/claude-recall/pull/105
- Mark how each tool call went in the web UI by @babarot in https://github.com/babarot/claude-recall/pull/109
### Refactorings
- Wait for the conversation to redraw in its test by @babarot in https://github.com/babarot/claude-recall/pull/110

## [v1.11.0](https://github.com/babarot/claude-recall/compare/v1.10.0...v1.11.0) - 2026-10-13
### New Features
- Record the git remote of the directory a session ran in by @babarot in https://github.com/babarot/claude-recall/pull/101
- Store the tool_use_id that ties a tool_result to its call by @babarot in https://github.com/babarot/claude-recall/pull/102

## [v1.10.0](https://github.com/babarot/claude-recall/compare/v1.9.0...v1.10.0) - 2026-10-13
### New Features
- Draw the web UI in the TUI's color scheme by @babarot in https://github.com/babarot/claude-recall/pull/95

## [v1.9.0](https://github.com/babarot/claude-recall/compare/v1.8.2...v1.9.0) - 2026-10-12
### New Features
- Add /api/v2 for the web UI to come by @babarot in https://github.com/babarot/claude-recall/pull/91
- Rebuild the web UI to show sessions as the TUI does by @babarot in https://github.com/babarot/claude-recall/pull/93
### Bug fixes
- Fit the web UI on a phone and shorten paths in tool calls by @babarot in https://github.com/babarot/claude-recall/pull/94
### Refactorings
- Move code the web UI will share out of the TUI by @babarot in https://github.com/babarot/claude-recall/pull/90

## [v1.8.2](https://github.com/babarot/claude-recall/compare/v1.8.1...v1.8.2) - 2026-10-11
### Bug fixes
- Fix the Dependabot alerts in the web UI dependencies by @babarot in https://github.com/babarot/claude-recall/pull/82
- Stop watching when another recall mcp took over around this one by @babarot in https://github.com/babarot/claude-recall/pull/88
### Others
- Sync shared files from github-config by @babarot in https://github.com/babarot/claude-recall/pull/87

## [1.8.1](https://github.com/babarot/claude-recall/compare/1.8.0...1.8.1) - 2026-10-09
### Improvements
- Release with GoReleaser by @babarot in https://github.com/babarot/claude-recall/pull/80

## [1.8.0](https://github.com/babarot/claude-recall/compare/1.7.2...1.8.0) - 2026-10-08
### New Features
- Draw recall inside Claude Code with a Claude Mods by @babarot in https://github.com/babarot/claude-recall/pull/65
- Add recall update to replace recall with the latest release by @babarot in https://github.com/babarot/claude-recall/pull/75
- Tell of a newer release in the TUI and in recall version by @babarot in https://github.com/babarot/claude-recall/pull/76
- Show what's new in recall in the TUI by @babarot in https://github.com/babarot/claude-recall/pull/77

## [1.7.2](https://github.com/babarot/claude-recall/compare/1.7.1...1.7.2) - 2026-10-07
### Bug fixes
- Do not let an import replace a newer one with stale content by @babarot in https://github.com/babarot/claude-recall/pull/71

## [1.7.1](https://github.com/babarot/claude-recall/compare/1.7.0...1.7.1) - 2026-10-07
### Bug fixes
- Watch transcripts from one recall mcp at a time by @babarot in https://github.com/babarot/claude-recall/pull/69

## [1.7.0](https://github.com/babarot/claude-recall/compare/1.6.2...1.7.0) - 2026-10-06
### New Features
- Support importing transcripts from more than one directory by @babarot in https://github.com/babarot/claude-recall/pull/67
### Improvements
- Give search results their repository, title and size; find words in Japanese by @babarot in https://github.com/babarot/claude-recall/pull/62
- Add the first prompt to search results by @babarot in https://github.com/babarot/claude-recall/pull/64
- Name sessions, tell recall-only ones and narrow to a repository over MCP too by @babarot in https://github.com/babarot/claude-recall/pull/66
- Let the Details grid use the width it has by @babarot in https://github.com/babarot/claude-recall/pull/68

## [1.6.2](https://github.com/babarot/claude-recall/compare/1.6.1...1.6.2) - 2026-10-05
### Bug fixes
- Do not resume a session whose transcript Claude Code has deleted by @babarot in https://github.com/babarot/claude-recall/pull/56
- Keep the line right of a box in place over wide characters by @babarot in https://github.com/babarot/claude-recall/pull/59
### Improvements
- Show the keys of the pane ? is pressed in first by @babarot in https://github.com/babarot/claude-recall/pull/57
### Refactorings
- Read the TUI state from one place, and test every state's footer by @babarot in https://github.com/babarot/claude-recall/pull/58

## [1.6.1](https://github.com/babarot/claude-recall/compare/1.6.0...1.6.1) - 2026-10-04
### Improvements
- Show recall next to resume in the footer by @babarot in https://github.com/babarot/claude-recall/pull/52
- Draw tools and commands on one scale in What was done by @babarot in https://github.com/babarot/claude-recall/pull/54
- Say how to set up the archive on the first run by @babarot in https://github.com/babarot/claude-recall/pull/55

## [1.6.0](https://github.com/babarot/claude-recall/compare/1.5.0...1.6.0) - 2026-10-04
### New Features
- Continue a session in a new claude when it cannot be resumed by @babarot in https://github.com/babarot/claude-recall/pull/48
### Improvements
- Show every setting in docs/configuration.md and re-record the demo by @babarot in https://github.com/babarot/claude-recall/pull/50
- Rename the c operation from continue to recall by @babarot in https://github.com/babarot/claude-recall/pull/51

## [1.5.0](https://github.com/babarot/claude-recall/compare/1.4.0...1.5.0) - 2026-10-03
### New Features
- Change the TUI's keys under [keys] in the config file by @babarot in https://github.com/babarot/claude-recall/pull/41
- Show where a detail frame is scrolled to with a scrollbar by @babarot in https://github.com/babarot/claude-recall/pull/42
- Show pasted images in the spread Conversation by @babarot in https://github.com/babarot/claude-recall/pull/46
### Bug fixes
- Paste into the TUI's text fields with cmd+v and ctrl+v by @babarot in https://github.com/babarot/claude-recall/pull/47
### Improvements
- Look up the TUI's keys in a keymap, with keys that agree across panes by @babarot in https://github.com/babarot/claude-recall/pull/39
- Make scrolling a detail frame light on a long session by @babarot in https://github.com/babarot/claude-recall/pull/43
- Show where a mistake in the config file is, as linters do by @babarot in https://github.com/babarot/claude-recall/pull/44
- Show the focus on the spread Conversation and close it with q by @babarot in https://github.com/babarot/claude-recall/pull/45

## [1.4.0](https://github.com/babarot/claude-recall/compare/1.3.1...1.4.0) - 2026-10-03
### New Features
- Set the archive and the web UI port in the config file by @babarot in https://github.com/babarot/claude-recall/pull/31
- Read transcripts from CLAUDE_CONFIG_DIR, add recall --all, and test the CLI by @babarot in https://github.com/babarot/claude-recall/pull/32
- Add folder: to the TUI filter, with in: for short by @babarot in https://github.com/babarot/claude-recall/pull/35
- Search the spread conversation with / by @babarot in https://github.com/babarot/claude-recall/pull/36
### Improvements
- Parse the CLI with cobra by @babarot in https://github.com/babarot/claude-recall/pull/30
- Keep the TUI moving while a long session's detail loads by @babarot in https://github.com/babarot/claude-recall/pull/37
### Others
- Add a demo GIF of the TUI, recorded with VHS by @babarot in https://github.com/babarot/claude-recall/pull/33

## [1.3.1](https://github.com/babarot/claude-recall/compare/1.3.0...1.3.1) - 2026-10-03
### Bug fixes
- Draw the TUI once the terminal's size settles by @babarot in https://github.com/babarot/claude-recall/pull/26

## [1.3.0](https://github.com/babarot/claude-recall/compare/1.2.0...1.3.0) - 2026-10-02
### New Features
- Search the conversation from the TUI filter and add field keys by @babarot in https://github.com/babarot/claude-recall/pull/21
- Move focus with Tab in the TUI and list the keys with ? by @babarot in https://github.com/babarot/claude-recall/pull/23
- Read the conversation over the TUI detail pane and pick the sort from a menu by @babarot in https://github.com/babarot/claude-recall/pull/24
- Ask Claude to find sessions from the TUI by @babarot in https://github.com/babarot/claude-recall/pull/25

## [1.2.0](https://github.com/babarot/claude-recall/compare/1.1.0...1.2.0) - 2026-10-02
### New Features
- Narrow the TUI to a folder and add a folder list by @babarot in https://github.com/babarot/claude-recall/pull/19

## [1.1.0](https://github.com/babarot/claude-recall/compare/1.0.1...1.1.0) - 2026-10-02
### New Features
- Improve the TUI's look, detail pane and mouse support by @babarot in https://github.com/babarot/claude-recall/pull/17

## [1.0.1](https://github.com/babarot/claude-recall/compare/1.0.0...1.0.1) - 2026-10-02
### Bug fixes
- Keep MCP servers up when several import at once by @babarot in https://github.com/babarot/claude-recall/pull/15

## [1.0.0](https://github.com/babarot/claude-recall/compare/0.2.0...1.0.0) - 2026-10-02
### Breaking Changes
- Rewrite in Go, add a TUI and rename to claude-recall by @babarot in https://github.com/babarot/claude-recall/pull/13

## [0.2.0](https://github.com/babarot/agent-recall/compare/0.1.1...0.2.0) - 2026-09-29

## [0.1.1](https://github.com/babarot/agent-recall/compare/0.1.0...0.1.1) - 2026-09-29
### Bug fixes
- Fix timer handle types that broke the release build, and catch type errors in CI by @babarot in https://github.com/babarot/agent-recall/pull/8
- Fix a flaky mtime check in the import resync test by @babarot in https://github.com/babarot/agent-recall/pull/10

## [0.1.0](https://github.com/babarot/agent-recall/commits/0.1.0) - 2026-09-29
### Breaking Changes
- Fix silent message drops by switching to uuid-based natural-key dedup by @babarot in https://github.com/babarot/agent-recall/pull/6
### New Features
- Add web UI for browsing sessions and chat history by @babarot in https://github.com/babarot/agent-recall/pull/2
- Add real-time web UI with FS watcher + SSE + incremental imports by @babarot in https://github.com/babarot/agent-recall/pull/3
### Improvements
- UI improvements: chat rendering, session list, and settings by @babarot in https://github.com/babarot/agent-recall/pull/4
- Extract sessions data layer into a signals-backed store by @babarot in https://github.com/babarot/agent-recall/pull/5
