# ADR-008: The config file holds what the user changed, and the running recall describes every setting

## Status

Accepted

## Context

The TUI's first run wrote a config file holding every setting at its default, commented out, with the words that describe each. The words and defaults came from the version that wrote the file and stayed as they were. A file written a month before lacked six settings added since, and listed a key under the name it had before it was renamed: uncommenting that line named an unknown operation, and another line gave its key to a different operation than the one it described.

The words of every setting are kept once, in the template in [internal/config](../../internal/config), and tests keep the template, the defaults, the keymap and `docs/configuration.md` in step. Only the running version's copy is current; the docs on GitHub describe the main branch.

## Decision

- The file the TUI writes on its first run says where the settings are described, and sets nothing. A file that is there is never written to, by a first run or by an update.
- `recall config reference [table]` prints the template, or one table's part of it, as the running version reads it. It does not read the config file, so a file with mistakes does not stop it. Redirected into a new file, it gives every setting commented out, for those who edit that way.
- The template stays the one place the words of the settings are written; the reference, the docs and the defaults' tests read it.
- Rejected: a table of settings that the template, the checks and the known keys are made from, as gh-desk has. The words and defaults are already kept in step by tests, nothing else in recall (a settings page, a JSON Schema) would be made from such a table, and its kinds would not keep the choices written in the words in step either.
- Rejected: keeping the full template as the first-run file. It goes stale as settings are added, renamed and change their defaults, and its lines are not the user's.
- Rejected: writing new settings into an existing file, in place or by a command. A file may be managed elsewhere and read-only, and what was written would go stale in turn.
- Not decided here: narrowing the reference to one key. The template shares its words among keys, so that would start with splitting them into blocks.

## Consequences

- A setting added is described by being added to the template; the tests then require it in the docs and as a default.
- Files written by earlier first runs keep their stale words. Users learn of the reference from the docs and the release notes, since they do not see the new first-run file.
- A user who wants the old full file runs the reference into it, and then keeps a copy of that version's words.
