# ADR-010: Packages import only what their layer allows, and history depends on little

## Status

Accepted

## Context

recall is one module holding a TUI, a web UI, an MCP server, the importer and watcher, the CLI, and `history`, a Go API other modules import under v1 semver. CLAUDE.md promised that `history` imports only the standard library, `modernc.org/sqlite` and the internal packages that need nothing more, and `docs/architecture.md` described the parts by hand. Nothing checked either: a package could reach another part, or start a process or a network connection, and a review would have to notice it.

The packages as they were already formed layers: every import pointed to a package below it.

## Decision

- Every package of the module has a row in a table in [arch_test.go](../../arch_test.go): its layer, the packages of the module it may import, the outside modules it may import, and what it does. A test checks every import of every package against it, so a package not in the table, or an import not allowed, fails. A package imports only from lower layers.
- The table holds no import a package does not make, so it tells what each package depends on.
- The parts of the standard library that reach outside the process (`os/exec`, `net`, `net/http`, `syscall`) are used only by the packages the table lists for each.
- Every package of the module that `history` reaches imports only the standard library, `modernc.org/sqlite` and packages of the module; a test follows every step, not only the first.
- `docs/architecture.md` shows the table, written from it by the test, which fails when the two differ.
- Test files are not checked: a test may import what helps it.
- Rejected: moving the TUI's running of `claude` and its clipboard copy out of `internal/tui` so that it no longer uses `os/exec`. Nothing needs the TUI without them yet; the table makes a new user of `os/exec` show in a diff, which is what the limit is for.

## Consequences

- A new package, or a new import between packages, is a row or an entry in the table in the same change, and `go test -run TestArchitectureDoc -update` writes the docs.
- A package that wants to use one above it has to be split or moved, not imported upward.
- A dependency added to a package `history` reaches fails the test, even when `history` itself does not change.
