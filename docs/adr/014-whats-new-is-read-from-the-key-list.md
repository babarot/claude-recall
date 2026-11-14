# ADR-014: What's new is read from the key list

## Status

Accepted

## Context

What's new, the release notes, opened on `w` from every pane: a key that works anywhere, taken from every screen for something read rarely, after being told of a release or an update, or out of curiosity about recall itself. The key list is where the user looks at recall's own keys, and since it scrolls and tells the version and a newer release at its top, it is where reading about releases starts.

gh-desk reads its release notes from its key list for the same reason (its ADR-088).

## Decision

- What's new opens from the key list, on `whats_new` (`w` by default), and from there only. `esc` goes back to the key list, on the same row and where it was scrolled to.
- The setting stays `whats_new` under `[keys]`, written as before, so a config file that sets it keeps working. A table `[keys.help]` cannot hold it: `[keys]` already has `help`, the key that opens the key list, and in TOML one name cannot be both a value and a table.
- A modal's own key may be bound to an operation of the keymap (`modalKey.op`): it is read on that operation's keys and named by them, and the conflict check takes the operation in that modal and not in the panes. This replaces, in ADR-006, "A modal reads no other operation of the keymap" and, for such a key, "A modal's own keys ... are fixed".
- The key list tells `w what's new` at its top. The status line's notice of a newer release and the toast after an update name both keys, `? w what's new`, and leave them out when either is turned off.
- Rejected: keeping `w` in every pane as well. It would keep a key taken from every screen.
- Rejected: moving the setting to a table of its own (`[keys.key_list]`) and reading the old name with a warning. The config file has no warnings, only errors, and the old name would need one for as long as it is read.

## Consequences

- With `help = []`, What's new cannot be opened, and nothing tells of it.
- `w` does nothing in the panes now. Given to an operation of the panes, it is taken from `whats_new` too, as a key given is taken from the operation that has it by default; `whats_new` then needs a key of its own.
