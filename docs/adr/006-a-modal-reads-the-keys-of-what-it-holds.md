# ADR-006: A modal reads the keys of what it holds, and closes with `esc` alone

## Status

Accepted

## Context

A modal is a box over the screen that takes every key while it shows. Each modal read its keys in a `switch` of its own: some moved on keys written as strings, one scrolled on the shared verbs of the keymap, so a move set in the config file worked in one modal and not in the next. The hints on a box's edge and the footer under it were written by hand beside each `switch`.

The keys inside a modal had been made fixed so that a key set for an operation would not get in the way of typing: if Ask's field read the keymap first, a letter given to a move would move instead of being typed. That holds only while a field is read after the keymap.

A modal without a field also closed with `q` and with the key that opened it. The key that opened a modal can close it only where no part of the modal reads that key first, which a field or a list does not allow, so which keys closed which modal had to be learned modal by modal; and a `q` pressed once more than needed quit recall.

## Decision

- A modal declares the parts it holds: a list to choose from, a body to read, a field, keys of its own, and what `esc` does ([modal.go](../../internal/tui/modal.go)). One layer reads the keys of those parts, the same in every modal. The hints on its box, the footer under it and the conflict check of the config file are made from the same declaration, not written beside it.
- A field takes every key that types a character before anything else in the modal, so no setting can take a letter being typed.
- A list and a body move on the shared verbs of the keymap as the config file sets them, as the panes do. A modal reads no other operation of the keymap.
- `esc` alone steps out of a modal. Neither `quit` nor the key that opened a modal is read over it, so stepping out never quits.
- A modal's own keys and its list's choose keys are fixed, and the conflict check takes them as fixed keys wherever a modal moves on the shared verbs, so a move cannot be given one of them.
- Rejected: keeping the keys inside a modal fixed, as before. A move set in the config file would keep working in the panes and not in the boxes, and the reason for fixing them (typing) is met by the field reading first.
- Rejected: closing a modal on the key that opened it as well. It holds only where no part reads that key, so it differs from modal to modal, and a key pressed twice closes what it just opened.
- Rejected: closing a modal without a field on `q` as well. The same key would close in one place and quit one press later.

## Consequences

- A new modal is a declaration; its keys, hints and conflicts come from it. Reading a key in a modal outside the declaration loses the hint and the conflict check for it.
- Any key a modal reads as written must be in its declaration, or a move set to that key in the config file takes it without being reported.
- The design, with each modal's parts and keys, is in [keybindings.md](../design/keybindings.md).
