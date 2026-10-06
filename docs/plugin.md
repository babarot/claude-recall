# Claude Code plugin

[`plugin/`](../plugin) is what Claude Code loads as the claude-recall plugin:

| Part | Files | What it does |
|---|---|---|
| MCP server | `.mcp.json` | Runs `recall mcp` |
| `SessionEnd` hook | `hooks/hooks.json` | Runs `recall import` when a session ends |
| Hooks module | `hooks/hooks.json` (`modules`), `hooks/register.tsx`, `hooks/recall.ts`, `hooks/i18n.ts` | Draws recall in the session and adds `/recall` |

The README's [In Claude Code](../README.md#in-claude-code) says what the hooks module shows. This page is how it does it.

## Which layer decides what

The hooks module draws; it does not work anything out about the data. What a session is (its repository and worktree, title, first prompt and message count) and how a query is matched (FTS5, or a substring for Japanese and other unspaced text) are decided in Go and reach every client the same way: the TUI, `recall search` and `recall list`, and the MCP server. The instruction to look back at a session is the MCP server's `recap` prompt. How far Claude goes after a recap is left to the user and Claude Code's own settings.

A new feature in the hooks module that needs to know something about the data starts as a field or a flag in the CLI and the MCP server, so that an agent using the MCP server without the plugin gets it too.

## Hooks module

`hooks/register.tsx` hooks these events:

| Event | What it does |
|---|---|
| `session.start` | Registers `/recall`, reads Claude Code's `language` setting, and lists the repository's sessions for the band (`recall list --repo . --format json`) |
| `prompt.submit` | Hides the band once the first prompt is sent |
| `ui.render` `AbovePrompt` | Draws the band of the latest sessions |
| `ui.render` `ToolGroup` | Unfolds a run of `recall_search` calls (and the `ToolSearch` that loads the tool first), which Claude Code folds into one "Called ..." line |
| `ui.render` `ToolUse`, `ToolResult` | Draws a `recall_search` result as a tree from the hits the MCP server returned |
| `tool.call` | Keeps the query of each `recall_search` call, for a standalone result row that does not carry it |
| `command.run` `recall` | Runs `recall search <query> --repo . --format json` (`--all` drops `--repo`) and answers with a plain list Claude reads |
| `ui.render` `CommandOutput` | Draws the same `/recall` result as a table |

Pressing a session's ID runs the `recap` prompt with `$.command.run`, as if the person typed it. The prompt is found among the commands Claude Code lists from MCP servers (`/plugin:claude-recall:claude-recall:recap` from the plugin); when a recall too old to have it is on PATH, a toast says to update recall.

`hooks/recall.ts` holds the parts with no `$`: reading the CLI's and the MCP server's JSON (they spell a few fields differently), grouping hits by session and repository, and laying out rows in terminal cells, where a CJK character takes two. Rows are padded by hand rather than with flex, which did not keep columns in place around wide characters. `hooks/i18n.ts` holds the words, English unless the `language` setting is Japanese; what Claude reads stays English.

### The one guess

A session that was only a recall search ("recall で uriba を検索") matches every later search for the same word and crowds out the sessions that did the work. The module puts such sessions together on one line, and the band says how many it left out, rather than hiding them. Which sessions those are is guessed from the title and the size: a title starting with "recall" and at most 50 messages. Nothing in the archive tells it yet; once something does (the tools a session used, say), the guess should move into Go like the rest.

## Developing

- `recall` on PATH must be one with `--repo` and the `recap` prompt, from the same commit as the plugin; `go build -o <dir>/recall ./cmd/recall` and put `<dir>` first on PATH.
- `claude --plugin-dir plugin` loads the plugin from the checkout in place of the installed one, and reloads the hooks module when its files change. A session started that way shows a line in the transcript when a hook fails or a drawing is refused.
- `claude plugin validate plugin` checks the manifest and what the module hooks and calls. It also reports that the name `claude-recall` is reserved for Anthropic's own plugins; that is known and does not stop the plugin from loading.
- `claude plugin test plugin` runs `tests/*.test.ts`. The tests answer `process.run`, `session.id`, `command.list` and the rest themselves, and mount the drawings to check that Claude Code accepts them and that pressing an ID runs `recap`.
- Claude Code writes the API's types to `.claude-plugin/types/` when it loads the plugin (ignored by git); after that, `npx -p typescript tsc -p plugin` type-checks the module.
- A drawing that passes the tests can still look different in a terminal: try it with `--plugin-dir` before sending a change.
