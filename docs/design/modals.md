# Modals

What a modal is in `internal/tui`, the words used for the parts of the screen around it, which modals there are, what each holds, and how they are drawn and read keys. The rules are in [ADR-006](../adr/006-a-modal-reads-the-keys-of-what-it-holds.md) (a modal reads the keys of what it holds, and closes with `esc` alone); the keys of each place are in [keybindings.md](keybindings.md).

## Words

| Word | What it is | In the code |
|---|---|---|
| modal | What opens over the screen and takes every key while it shows, until `esc` closes it, but `ctrl+c`, which quits from anywhere (a running ask stopped first). It is drawn as a box | `modalSpec` in [modal.go](../../internal/tui/modal.go); each modal's declaration in its own file (`helpSpec`, `sortSpec`, `askTypingSpec` and the like); `specOf` and `openModal` in [uistate.go](../../internal/tui/uistate.go) |
| box | What a modal is drawn as: its lines inside a rounded frame, with a title and a hint on the top edge. The filter's suggestions and the user's messages in a conversation are drawn in boxes too. In the code and these docs, box is only the drawing: what opens, closes and takes keys is a modal. The user-facing docs call a modal a box | `modalBox` and `drawBox` in [modal.go](../../internal/tui/modal.go) |
| edge | The top line of a box, which holds its title and the hint made from the modal's declaration (`edgeHint`); Ask's answer holds the model, the time taken and the cost there instead (`askSummary`) | `edgeHint` in modal.go |
| overlay | Drawing lines over the screen's lines from a cell on, keeping the screen left and right of them. It is how a modal is put on the screen, and how the filter's suggestions are; it is not a modal | `overlay` in [query.go](../../internal/tui/query.go) |
| field | Where text is typed: the filter, the folder search, the conversation search, and the fields inside modals (Ask's question, the search of Ask's questions, the topic before recalling) | `textinput.Model` made by `newInput` in [input.go](../../internal/tui/input.go) |
| pane | An area that can have the focus: the session list, the folder list, the detail frames and the reading view | `focus`, `keyContext` |
| frame | One of the three parts of the detail pane (Conversation, What was done, Details), drawn with a border. A frame is a pane, not a box | `frameTitles`, `renderPane` |
| reading view | The Conversation frame over the detail pane, the list left a few rows | `expanded` |
| suggestions | The list the filter opens under its line while a `folder:` (or `branch:`, `worktree:`) term is typed. It is overlaid but takes no keys of its own beyond the field's, so it is part of the field, not a modal | `suggestRect`, `suggestBox` in query.go |
| toast | A line that tells something for a moment (copied, updated, a failure), drawn in place of the status line, not over the screen. It is not a modal and takes no keys | `toast`, `showToast`, `renderStatus` in [view.go](../../internal/tui/view.go) |

The state of the screen, which of these takes the keys, is `uiState` in [uistate.go](../../internal/tui/uistate.go): a modal first, then a field, then the focused pane. Which modal shows is one field, `Model.modal` (a `modalKind`), so two never show at once. A modal is opened through `openModalKind`, which closes the one that shows first, with what closing it takes (`closeModal`: a running ask stopped, a field put away), and Ask's stage and search are read only while Ask shows (`askUIState` in ask.go).

## The modals

| Modal | `uiState` | Opened by | Parts (its declaration, in its own file) | Placed | Width |
|---|---|---|---|---|---|
| key list | `uiHelp` | `help` | none; it does not scroll | centered top to bottom | up to 64 |
| sort menu | `uiSort` | `sort` | list; own keys `1` to `4` | centered top to bottom | up to 44 |
| Ask, typing | `uiAskTyping` | `ask` | field; recalls questions on `up` `down`; own keys `enter`, `ctrl+r`, `tab`, `right` | the third row | up to 96 |
| Ask, searching the questions | `uiAskSearch` | `ctrl+r` in Ask | field and list | the third row | up to 96 |
| Ask, running | `uiAskRunning` | `enter` in Ask | none | the third row | up to 96 |
| Ask, answered | `uiAskAnswered` | the answer | list; own keys `f`, `r` | the third row | up to 96 |
| Ask, failed | `uiAskFailed` | the failure | own key `r` | the third row | up to 96 |
| before recalling | `uiRecall` | `recall` | field; own key `enter` | the third row | up to 96 |
| what's new | `uiWhatsNew` | `whats_new` | body | centered top to bottom | up to 76 |

A width is the most a box takes, a constant; a narrower terminal narrows it, and a terminal narrower than a box can be (30 columns, 24 for the sort menu) leaves it undrawn, with the footer still saying how out (`esc` first).

The Ask box keeps one height from typing to running: its body is five rows in both, and the search of the questions fills the same five. The box of the answer grows to the height of the screen, which is why it sits near the top.

## Where a modal is written

- [modal.go](../../internal/tui/modal.go) is the mechanism every modal shares: the declaration's types, the layer that reads keys from it (`modalKeyFor`), the hints (`hints`, `edgeHint`) and what the conflict check takes (`fixedKeys`, `navOps`). It knows no modal: it reaches a `Model` only for the keymap, the key hints and its own methods. `TestModalGoKnowsNoModal` checks it by the names of the parameters and receivers declared as a `Model`, in functions, func literals and package variables; a `Model` reached through another name is left to review.
- Each modal declares itself in its own file, as a function (`helpSpec` in help.go, `sortSpec` in sortmenu.go, the Ask stages in ask.go and the search in askhistory.go, `recallSpec` in recall.go, `whatsNewSpec` in whatsnew.go). A function, not a package variable: a declaration's funcs reach Model methods, and as variables Go's initialization order would follow them into a cycle as soon as one reached the registry.
- [uistate.go](../../internal/tui/uistate.go) says which state is which modal (`specOf`), and which modal shows (`Model.modal`, `openModalKind`, `closeModal`). It is the one file that names every modal, as the list of states is. The conflict check in keys.go walks the states through `specOf`, with no Model.

## How a modal is drawn

A modal's declaration holds its box (`modalBox` in modal.go): its title, how wide it gets and the margin it leaves, the narrowest it is drawn at, its frame, where it is placed, what a click does, and its rows (`lines`), with what goes on its edges when not the hint. The boxes differ where the user sees it, and those differences are values of the declaration, not code of their own:

| | Frame | Edge end | Margin | Placed | Click |
|---|---|---|---|---|---|
| key list, sort menu, what's new | border color (`frameRule`) | `─╮` | 2 | centered (`placeCenter`) | the key list and what's new close; the sort menu chooses the row clicked and closes outside (`clickList`) |
| Ask, before recalling | accent color (`frameAccent`) | ` ╮` | 4 | third row (`placeTop`) | nothing |

A selected row is drawn by each modal in its rows, but for two things a row tells `drawBox`: the key list's mark goes in the left margin (`bar`), and the rest of the row may have the selection's background (`selected`).

1. `draw` in view.go draws the screen as if no modal were open (`renderScreen`).
2. If a modal shows, `drawModal` in modal.go places its box (`boxRect`) and draws it (`drawBox`) over the screen with `overlay`. The screen under it is not dimmed.
3. `View` takes the cursor's mark out of the drawn screen and puts the terminal's cursor there, when a field is typed in ([input.go](../../internal/tui/input.go)).

How many rows a box may hold (`boxRows`) is decided from where it is placed and the screen's height, before it is placed, since a centered box's row depends on its height. A centered box is centered on the terminal's height, not on how many lines the screen drew: below 16 rows the list draws more lines than the terminal shows, and centering on those put the bottom of the key list and of what's new past the last row. A click goes to the modal that shows (`clickModal`), with the same `boxRect`. What's new reads the same rows for its scrolling, its page and the position on its bottom edge (`whatsNewRows`).

Placing every modal by one rule, as gh-desk places its popups a quarter down the screen, was looked at and left for later: the Ask answer's box would show fewer sessions. It is now a change of `place` in the declarations.

## How a modal reads keys

`update` in model.go reads `ctrl+c` first, which quits from anywhere, and then gives a key to the modal of the state through `modalKeyFor` in modal.go, before any field or pane. The order, the parts and why are in [keybindings.md](keybindings.md#6-a-modals-keys-and-closing-it) and ADR-006. The footer while a modal shows, and the hint on its edge (but on Ask's answer), are made from the same declaration (`hints`), so they never name a key the modal does not read.

## Not modals

- The filter, the folder search and the conversation search are fields of the screen: they take the keys while typed in, but are drawn in their place, not over the screen.
- The suggestions are overlaid but belong to the filter's field.
- A toast replaces the status line for a moment and takes no keys.
- The detail frames and the reading view are panes.
