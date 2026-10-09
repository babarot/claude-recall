# Updating recall and telling users about new releases

A design for `recall update`, which replaces the installed binary with the latest release, and for telling users that a new release is out. It answers [#73](https://github.com/babarot/claude-recall/issues/73).

## Goals

- Upgrade a curl install without re-running `install.sh`, which also imports sessions and registers the MCP server again
- Never overwrite a binary another installer manages (Nix, Homebrew) or one the user built
- Keep running processes coherent after an update: the background web UI serves the new version
- Tell users of every install method that a release is out, in the TUI and in `recall version`, without slowing them down

Out of scope: Windows (no release builds), signatures beyond SHA256, updating the Claude Code plugin, updating from inside the TUI.

## Decisions taken

| Question | Decision |
|---|---|
| Notify only, or replace the binary | Replace. A curl install has no other upgrade path than the installer, which is what #73 is about |
| Check for new releases by default | Yes. A config key turns it off |
| Builds recall did not install | `recall update` refuses Nix, Homebrew and source builds, and says how to update them. Nix and Homebrew installs are still told a release is out |
| Verification | SHA256 against the release's `checksums.txt`, as `install.sh` does |

## Where recall stands today

| Install | Binary | Path | `version.Version` |
|---|---|---|---|
| curl | release asset | `~/.local/bin/recall` (or `RECALL_INSTALL_DIR`) | the release |
| Nix | the same binary, in the release's `claude-recall_<os>_<arch>.tar.gz` that the derivation GoReleaser writes fetches | `/nix/store/...` | the release |
| `make install` | built locally | `~/.local/bin/recall` | the last release, whatever the checkout |
| `go install` | built locally, no web UI | `$GOBIN` | the last release |
| Homebrew | none yet; babarot/homebrew-tap has no formula | | |

- `version.Version` is a constant tagpr rewrites. Nothing tells a release build from a local one, and curl and `make install` share a path
- Tags have no `v` prefix (`1.7.2`)
- The release workflow creates the release (tagpr) before the `goreleaser` job uploads assets, a few minutes later. Meanwhile `releases/latest` names a release with no binaries
- `recall ui` runs detached and stays up. `recall mcp` runs once per Claude Code session. Neither notices the binary changing
- `/api/status` reports `status`, `pid`, `port`, `sseClients` and `watcher`
- `install.sh` writes the download straight over the old binary (`curl -o`)

## Design

### 1. Telling a release build apart

`internal/version` gains a variable set at link time:

```go
// Source is "release" in the binaries the release workflow builds, and
// empty in any other build.
var Source string
```

The release workflow adds `-X github.com/babarot/claude-recall/internal/version.Source=release` to its `-ldflags`. `make install` and `go install` leave it empty. Nix gets `release`, because it ships the release asset; the path check below catches it.

### 2. How recall was installed

`internal/update` decides from the build and the resolved path of the running binary (`os.Executable`, then `filepath.EvalSymlinks`):

| Order | Condition | Method | `recall update` | Notice tells the user to run |
|---|---|---|---|---|
| 1 | `Source != "release"` | source | refuses | nothing (no check) |
| 2 | path under `/nix/store/` | nix | refuses | `nix profile upgrade claude-recall` (wording to settle, see open questions) |
| 3 | path contains `/Cellar/`, or starts with `/opt/homebrew/` or `/home/linuxbrew/` (decided from the path alone, without running `brew`) | homebrew | refuses | `brew upgrade claude-recall` |
| 4 | otherwise | binary | replaces | `recall update` |

The function takes the source and the path as arguments, so tests cover each row without a real install. Symlinks are resolved before matching, so `~/.local/bin/recall -> /nix/store/...` is Nix. A writable directory is checked when updating, not here: a binary in a root-owned directory is still a binary install, and the error says so.

### 3. Finding the latest release

`GET https://github.com/babarot/claude-recall/releases/latest` without following the redirect; the `Location` header ends in `/releases/tag/<tag>`. Unlike `api.github.com`, this is not subject to the 60 requests an hour an unauthenticated API caller gets, which a shared NAT can use up. The assets are at fixed URLs, `https://github.com/babarot/claude-recall/releases/download/<tag>/<asset>`, so nothing else is needed.

Versions compare as three numbers. A tag that does not parse is an error. A latest release lower than or equal to the running one is "up to date" (a release that was pulled must not offer a downgrade).

The base URL is a field on the checker so tests point it at `httptest`.

### 4. `recall update`

```
recall update [--check]
```

1. Decide the install method (section 2). Anything other than binary prints what to run instead and exits with an error
2. Find the latest release (section 3). If it is not newer, print `recall 1.7.2 is the latest.` and exit 0
3. With `--check`, print `recall 1.8.0 is available (you have 1.7.2).` and exit 0
4. Download `checksums.txt` and `claude-recall-<os>-<arch>` (arch `arm64` or `x86_64`, as in `install.sh`). A 404 on either means the release is still being built: print `recall 1.8.0 is still being published; try again in a few minutes.` and exit with an error
5. Write the asset to a temporary file in the directory of the resolved path (`os.CreateTemp(filepath.Dir(resolved), ".recall-update-*")`; the directory of a symlink may be on another filesystem, where step 8's rename fails with EXDEV), hashing it as it is written. A directory that cannot be written is reported with its path and a hint to re-run the installer with `RECALL_INSTALL_DIR` or with the right permissions
6. Compare the SHA256 with the line for the asset in `checksums.txt`. A mismatch or a missing line removes the file and fails
7. Close the file (on Linux, executing a file still open for writing fails with ETXTBSY), give it the old binary's mode, and run `<tmp> version` with `RECALL_NO_UPDATE_CHECK=1` and no terminal on stderr, so it does not check for releases itself. Its output must be `recall 1.8.0`
8. `os.Rename` the file over the resolved path. The old binary's inode stays open for any process running it
9. Restart the web UI if it is running (section 5)
10. Print `Updated recall 1.7.2 → 1.8.0`, and `Running MCP servers keep 1.7.2 until their Claude Code session ends.`

Any failure before step 8 removes the temporary file and leaves the installed binary as it was, so there is nothing to roll back. Finding the latest release uses a client that does not follow redirects; downloads use one that does (`releases/download/...` redirects to `objects.githubusercontent.com`). Both use the default transport (proxies from the environment), under one deadline of 2 minutes for the whole command.

Once the cache of section 6 exists (PR 2), `recall update` writes the result of step 2 to it, so the TUI stops showing the notice right after.

### 5. Restarting the web UI

`/api/status` gains `version` (added field; the web UI and other readers ignore it).

A shared helper, `restartUI(port, db, exe)`. Its caller works out one port and passes it; the helper uses it both to find the running UI and to start the new one:

1. Ask `/api/status` on `port`. `recall ui` passes `o.port`; `recall update` passes `[ui] port` from the config file (`cfg.UI.Port`), because the root command fills `o.port` only for commands that have a `--port` flag and `update` has none. Not running: nothing to do
2. `POST /api/shutdown`, then poll `/api/status` every 100 ms until it stops answering, for up to 5 s. Still answering: report that the old UI is still running and stop
3. Start `exe` (the resolved path from section 4, rather than `os.Executable()`) as `startBackground` does, with `--port port --db db`. `startBackground` takes the port, db and executable as arguments instead of reading `o`

It is called from:

- `recall update`, after the rename
- `recall ui`, when a UI is already running and its `version` is older than this binary's, or is missing (a server from before this change). A binary built without the web UI (`webui.Embedded == false`, a `go install` build) never restarts one, since it would serve only 404s. A running UI that is newer or the same is left alone: an older `recall` earlier on PATH (a stale Nix profile beside a curl install, say) must not replace a newer server with itself. Today `recall ui` starts a second process, which fails to bind, and prints the running UI's URL. It now checks first. This also restarts the UI after a Nix upgrade. A source build keeps the last release's version, so after `make install` the UI is restarted by hand with `recall ui stop`, as today

The restarted UI uses this invocation's `--port` and `--db` (or the config's), not the old process's: `/api/status` does not report the database. A UI started by hand with a non-default `--db` comes back on the configured one. This is noted in docs/cli.md; reporting `db` in `/api/status` would remove the gap if it matters.

Every request to the local UI here (status, shutdown, the polls) uses a client with a 1 s timeout, replacing the bare `http.Get` and `http.Post` of `getStatus` and `stopUI`, so a hung server cannot hold up `recall update` after the binary has been replaced.

Open browser tabs reconnect their SSE stream on their own, and the new server answers on the same port.

### 6. Checking in the background and the cache

`~/.local/state/claude-recall/update.json`, beside the TUI's `state.json` (a separate file, because the TUI rewrites `state.json` whole when it exits):

```json
{"checked_at": "2026-10-09T03:00:00Z", "latest": "1.8.0"}
```

`update.Cached(path)` reads it. `update.Refresh(ctx, path)` asks GitHub (section 3) with a 3 s timeout and writes the file atomically. A stale cache is one older than 24 hours. A failed check writes `checked_at` with the previous `latest`, so an offline machine does not try on every start.

No check at all when:

- `version.Source != "release"`
- `[core] update_check = false` in the config file
- `RECALL_NO_UPDATE_CHECK` is set (for CI and scripts, where the config file is not at hand)

Where a check runs and what it shows:

| Where | When it checks | What it shows |
|---|---|---|
| TUI | at start, if the cache is stale, as a `tea.Cmd`; never blocks the first frame | the status line (below) |
| `recall version` | if the cache is stale, synchronously with the 3 s timeout, only when stderr is a terminal | `A new release of recall is available: 1.7.2 → 1.8.0` and how to update this install on stderr; stdout stays `recall 1.7.2` for scripts |
| `recall update` | always (section 4) | its own output |
| `recall mcp`, `import`, `list`, `search`, `export`, `stats`, `ui` | never | nothing. `mcp` speaks the protocol on stdout, `import` runs from a hook, the others are often piped |

### 7. The notice in the TUI

The status line above the footer (`renderStatus`) is empty except while a toast shows. When a newer release is known, it shows:

```
 recall 1.8.0 is available · recall update
```

in the subtle color, with the version in the OK color and a command in the color of the footer's keys. What follows the dot depends on the install method: `recall update`, `brew upgrade claude-recall` (both commands), or the words `update it with Nix`. A toast replaces it for its 2.5 s and it comes back. It stays until the binary is updated (the TUI compares the cache with its own `version.Version`, so a TUI left open after `recall update` keeps showing it until restarted, which is correct).

A one-off toast was considered and dropped: it disappears in 2.5 s, competes with copy toasts, and the status line is otherwise unused.

This is text on an existing line, not a pane, modal or field, so it adds no `uiState`.

### 8. `install.sh`

Download to a temporary file in `INSTALL_DIR`, verify, `chmod`, then `mv` over `recall`. This keeps a running `recall ui` or `recall mcp` alive and avoids writing into the inode of a running, signed binary on macOS.

## Files

| File | Change |
|---|---|
| `internal/version/version.go` | `Source` |
| `internal/update/` (new) | install method, latest release, version compare, download and verify, replace, cache |
| `cmd/recall/main.go` | `update` command; `restartUI`; `recall ui` checks the running version; `recall version` notice |
| `internal/web/web.go` | `version` in `/api/status` |
| `internal/config/file.go` | `[core] update_check` (default true; a missing key means true) |
| `internal/tui/` | read the cache, run the refresh `tea.Cmd`, render the notice in `renderStatus` |
| `.github/workflows/release.yaml` | `-X ...version.Source=release` |
| `bin/install.sh` | temporary file and `mv` |
| `README.md`, `docs/cli.md`, `docs/configuration.md`, `docs/tui.md` | Upgrading, `recall update`, `update_check`, the notice |

## Tests

- Install method: each row of section 2, symlinks into `/nix/store`
- Version compare: newer, equal, older, malformed
- Latest release: `httptest` returning a redirect; no redirect; a 404
- Update end to end against `httptest` serving a fake asset and `checksums.txt` into a temp directory: success replaces the file and keeps its mode; a checksum mismatch, a missing checksum line, a 404 asset and an unwritable directory each leave the original file untouched and no temporary file behind
- Cache: fresh, stale, missing, unreadable; a failed refresh keeps `latest`
- `/api/status` includes `version`
- TUI: the status line shows the notice when the cache names a newer release and not otherwise; a toast replaces it

The step that runs `<tmp> version` needs a real executable; tests build a tiny program with `go build` into the temp directory, or the step is a function field the test replaces.

## The first release with `recall update`

Installs from before it have no `update` command, so their users upgrade once more the old way, by re-running `install.sh` (or through Nix). From then on `recall update` works. The release notes and README's Upgrading section say so. Nothing in the binaries before it can be changed to avoid this.

## Delivery

1. PR 1, closes #73: sections 1 to 5 and 8, the `version` field, docs for `recall update`
2. PR 2: sections 6 and 7, `update_check`, the `recall version` notice, docs
3. Later, separately designed: a What's new view of the release notes in the TUI (a new `uiState`), and a banner in the web UI from `/api/status`

## Open questions

- The Nix hint. `nix profile upgrade` takes an index or a name that depends on how the user installed (`nix profile install github:babarot/nur-packages#claude-recall`); home-manager and NixOS users update their flake instead. A generic `Update it with Nix (nix profile upgrade, or your flake).` may be more honest than a precise command
- Whether `recall update` should also offer `--force` for source builds later. Not now
