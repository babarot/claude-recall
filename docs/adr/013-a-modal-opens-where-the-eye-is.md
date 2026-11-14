# ADR-013: A modal opens where the eye is

## Status

Accepted

## Context

A modal's box was placed in one of two ways, a value of its declaration (ADR-012). The key list, the sort menu and What's new were centered by their height each time they were drawn; Ask and the box before recalling stood on the third row. A centered box moved whenever its height changed: What's new jumped from the middle once its notes came. The third row is far above where the eye rests on a terminal, and a small box there, or a small box centered, left the eye to find it in a different place for each modal.

gh-desk places its popups by one rule (its ADR-127): a quarter down the screen, or centered for one too tall for that, decided once when the first opens and held while popups are open.

## Decision

- When a modal opens with none showing, its box is placed a quarter down the terminal. A box tall enough that, placed there, it would leave less room under it than above it is placed higher, centered, and never above the third row. A box never sits lower than centered.
- The row is decided once, by the height the box has when it opens. It holds while a modal shows: a modal taking the place of another opens on the same row, and a box whose height changes grows or shrinks downward from it. It is decided again on a resize, and when a modal opens after all have closed.
- A box holds the rows from its row down to above the status line and the footer, which stay in sight under it. Rows past them are cut, the last saying there is more.
- The row is decided after each message is taken in, once the modal opened has the state its box is drawn from.
- The quarter is a constant, not a setting.
- This replaces "placement" among what a modal's declaration holds, in ADR-012: no declaration says where its box goes.
- Rejected: every box a quarter down. A tall box (the key list) would be cut at the bottom and lose lines it shows now, with more space above it than under it.
- Rejected: centering each box by its height while it shows. It moves whenever its height changes.
- Rejected: deciding the row inside the function that opens a modal. The openers set the modal's state after opening it, so the box measured there is not the one drawn.

## Consequences

- Ask opens a quarter down rather than on the third row, and its answer has fewer rows to show sessions in on a short terminal; the list in the answer scrolls to keep the selected session in sight.
- A box that grows after it opens (What's new once its notes come, Ask's answer) can reach the status line and scroll there, though it would have fitted had it opened that tall; opening it again places it anew.
- A modal drawn without an update first, as a test that sets the modal's field directly does, is drawn from the third row.
