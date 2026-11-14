# ADR-012: A modal is one declaration, and one shows at a time

## Status

Accepted

## Context

ADR-006 had each modal declare the keys it reads. The rest of a modal was written elsewhere: which modal showed was six fields, read in a fixed order, so two could be set and one would quietly win; the declarations sat in the shared modal.go and reached into each modal's fields, while each modal read its declaration back to draw its hint; each modal drew and placed its own box, in four ways, and the clicks were decided in the update loop by the fields. Adding a modal meant writing it in four places that nothing kept in step.

## Decision

- One field holds the modal that shows. A modal opens through one function, which first closes the one that shows with what closing it takes (a running ask stopped), so two never show at once and none is written over.
- A modal is one declaration: the keys it reads (ADR-006), its hints, and its box (rows, title, width, frame, placement, what a click does). Drawing, placing and clicking go through one mechanism that reads the declaration; no modal draws or places itself.
- The ways the boxes differ where the user sees it are values of the declaration, not code of each modal. Making them look alike is a change of those values.
- Each modal declares itself in its own file, as a function. The shared mechanism, [modal.go](../../internal/tui/modal.go), knows no modal: it reaches a Model only for the keymap, the key hints, the screen's size and styles, closing the modal that shows, and its own methods, which [modal_test.go](../../internal/tui/modal_test.go) checks by the names of the parameters it declares. Which state is which modal is said in one place, beside the list of states.
- Declarations are functions, not package variables: a declaration's funcs reach the model's methods, and Go's initialization order follows them into a cycle once one reaches the place that says which modal shows.
- Rejected: keeping each modal's drawing and placement in its own code with a shared helper. The places and margins would stay four kinds of code, and a click's hit-test would again repeat a modal's placement apart from its drawing.
- Rejected: drawing every box one way in the same change. The boxes differed where the user sees it, so drawing them one way would change what the user sees; that is left as its own decision.

## Consequences

- A new modal is a declaration and a state; its keys, hints, box, placement and clicks follow from it, and modal.go does not change.
- A Model reached in modal.go under another name than a declared parameter is not caught by the test, and is left to review.
- The file that says which state is which modal names every modal, as the list of states does.
