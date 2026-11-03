# ADR-005: A repository is its origin, and `recall mcp --repo` holds a server to one

## Status

Accepted

## Context

`--repo` and the MCP `repo` input took a directory and narrowed to the sessions of its main checkout and worktrees. A repository was the directory it was checked out in, so two clones of one repository were two. A tool that reviews a pull request by running Claude in a worktree of its own clone reached only the sessions run in that clone, not those in the user's checkout.

The scope was also only what the model asked for. A Claude reading a pull request reads text someone else wrote, which may tell it to search every repository and put what it finds in an answer the user then posts. A tool that starts `recall mcp` for such a Claude can only pass it a command line, so the server has to hold the scope itself.

recall already recorded the origin of each session's directory when importing it (`sessions.remote`, as host and path), and the TUI and web UI grouped the checkouts of one remote as one folder.

## Decision

- A session belongs to the repository of the origin it recorded, wherever it ran. A session that recorded none (outside git, a checkout with no origin, or a directory gone before recall read it) belongs to the repository of its directory, as before. A session that recorded another origin is left out even when it ran in the same directory, so a path used again for another repository does not bring the earlier one's sessions. Matching is per session, not per directory.
- Only `origin` counts. Matching every remote would join a fork with its upstream, and a server held to one would reach the other's sessions.
- Remotes are compared as host and path, ignoring case. A port is not part of it: one host serving two repositories of the same path on different ports is taken as one.
- `repo` takes a remote as well as a directory. For a directory on disk, its origin is read from disk now; for one since removed, it is the one its sessions recorded.
- An origin recorded for a session is kept when the session is imported again; one recorded as none, or not known, takes what is read later.
- `recall mcp --repo <remote-or-dir>` holds every tool to one repository, fixed when the server starts. A directory must be on disk with a readable git config; its origin is read then. A held server matches by that remote alone and never by directory, so sessions that recorded no origin, and removed worktrees only guessed by name to belong to a checkout, stay out. Without an origin, it holds to the directory's own checkout and worktrees. `repo` from the client narrows within the held repository; `recall_export` does not find a session outside it; `recall_stats` counts only it. Without the flag nothing changes; a flag given as an empty value is refused rather than taken for none. Results on a server held to a remote name their repository by that remote, since the name the index guesses for a removed worktree is drawn from every directory of the archive.

## Consequences

- Two checkouts of one repository, and the worktrees of either, are one repository to `--repo`, the plugin's band and the MCP tools, as they already were one folder in the TUI and web UI. Sessions a reviewing tool ran in its own clone show with the user's own.
- A held server misses the sessions of the repository that recorded no origin, mostly ones imported after their directory was gone. That is the price of not widening the scope by guesses.
- `FillRemotes` still fills a session that recorded no origin from what its directory holds now. If the user checks out another repository at that path first, the session is taken for that one. Only the user can cause this, not text a held Claude reads, and telling such a value apart would need another column.
- A held server still ranks full-text matches with the whole archive's FTS5 statistics, so how often a word appears outside the repository can shift the order of results inside it. No session outside it is returned or named, and reading word counts across the archive through result order would take many crafted queries over sessions already in the repository. Ranking within the repository alone would mean leaving FTS5's ranking, at a cost to every held search.
