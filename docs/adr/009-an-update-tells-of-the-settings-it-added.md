# ADR-009: An update tells of the settings it added, once

## Status

Accepted

## Context

ADR-008 keeps the words of every setting in the running recall rather than in the config file, so a file no longer lists what an update adds. A user who reads the release notes may still miss a setting among the other changes, and nothing else said it was there.

The settings are known in code, as the keys of the config file's tables, and the TUI already tells once, after an update, that recall was updated.

## Decision

- The TUI keeps the settings it has known in a record of its own in the state directory, and compares the running version's with it on every run. The operations under `[keys]` are left out: they come with their keys, which the key list and the release notes show.
- With no record it keeps the settings as known and tells of nothing: a new install, or the update to the first version that keeps the record, has nothing it can tell apart as new.
- It tells of the settings a run adds only when that run is the first after an update, in the toast that says so and at the top of What's new. A run that adds settings without an update, as a build from source does, records them and tells of nothing.
- Names are never dropped from the record, so going back a version and forward again tells of nothing twice. The last settings added, and the version that found them, stay in the record, and `recall config reference` marks them until an update adds others.
- The mark is a comment line of its own above a setting, so a setting's line uncommented from the reference still reads.
- A record that cannot be read is left as it is, and tells of nothing.
- Rejected: comparing the config file with the settings. Nearly every setting is missing from a file that holds what the user changed (ADR-008).
- Rejected: a version written beside each setting by hand. It makes the author guess the next version, and a wrong guess tells of a setting with the wrong one, or twice.

## Consequences

- A setting is told of by being added to the config file's tables; nothing else is needed for it.
- A setting renamed is told of as one added, under its new name, and the old name stays known.
- The record is kept only where the TUI runs; the MCP server and the other commands neither read nor write it, save `recall config reference`, which reads it for the marks.
