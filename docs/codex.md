# Codex

claude-recall archives [Codex CLI](https://github.com/openai/codex) sessions beside Claude Code's, in the same archive, so one `recall` finds a session whichever agent ran it: the TUI lists both, the MCP server searches both from inside either agent, and the web UI reads both.

## What is read

Codex writes each thread as a rollout, `$CODEX_HOME/sessions/YYYY/MM/DD/rollout-<time>-<thread id>.jsonl` (`~/.codex` unless `CODEX_HOME` says otherwise), and moves the ones you archive with `codex archive` to `archived_sessions`. A week after a rollout was last written, Codex compresses it to `.jsonl.zst`; recall reads those too. Both trees are read whenever they are on disk, so nothing changes on a machine without Codex.

A rollout is one JSON object per line. What recall keeps of it, as the same rows a Claude Code transcript gives:

| Rollout line | Stored as |
|---|---|
| `session_meta` | The session: its ID, directory, branch, Codex version and start time |
| `response_item` message, role user or assistant | `text`; a user message that is context Codex put in (`<environment_context>`, `<user_instructions>` from AGENTS.md, and the like) is `meta`, as a Claude Code slash-command expansion is |
| `response_item` message, role developer or system | `meta` |
| `response_item` reasoning | `thinking`, one per summary paragraph |
| `response_item` function_call, custom_tool_call, local_shell_call, web_search_call | `tool_use`, named after the tool (`shell`, `apply_patch`, ...) |
| `response_item` function_call_output, custom_tool_call_output | `tool_result`, cut at 10,000 characters as Claude Code's are |
| An `input_image` in a user message | The image, from its data URL |
| `event_msg`, `turn_context`, `compacted`, token counts and the rest | Nothing: they say nothing the items above do not |

The rollouts of Codex's own sub-agents (a review, a compaction, a spawned agent) are not sessions of yours and are left out, as Claude Code's sidechains are. The title is the name you gave the thread (`/rename`, or the picker's), read from `$CODEX_HOME/session_index.jsonl`; without one, the first prompt stands in, as for Claude Code. The session's project name is its directory encoded the way Claude Code encodes it (`-Users-me-src-app`), so `--project` and the folder list treat a directory's Codex and Claude Code sessions alike.

Rollouts from before Codex wrote `session_meta` (mid 2025), whose first line is the meta itself, are read too.

## Set up

With `recall` on PATH, give Codex the MCP server:

```bash
codex mcp add claude-recall -- recall mcp
```

The [installer](../README.md#curl) does this when `codex` is on PATH, as it does for Claude Code. Then, in a Codex session, "what did we decide about the retry policy last week?" looks it up through `recall_search`, Claude Code sessions included. Each Codex session runs its own `recall mcp`, which imports everything when it starts and watches both agents' trees while the session runs, exactly as under Claude Code; see [Sync timing](architecture.md#sync-timing).

To import when a Codex session ends as well, as the Claude Code plugin's `SessionEnd` hook does, add a hook in `~/.codex/hooks.json`:

```json
{
  "hooks": {
    "SessionEnd": [
      { "hooks": [{ "type": "command", "command": "recall import" }] }
    ]
  }
}
```

Codex asks you to trust a command hook before it first runs it. The hook is optional: the watcher has usually imported the session by then.

## Going back to a session

In the TUI, `Enter` on a Codex session runs `codex resume <id>` from the session's folder, as it runs `claude -r` for a Claude Code session; `Y` copies that command. `codex resume` finds the rollout wherever it is under Codex's home, archived or not, so a Codex session is resumable as long as its folder is there and the rollout has not been deleted. `c` recalls a session in a new `codex` instead, with recall's MCP server passed as config overrides (`-c mcp_servers.recall.command=...`), for one that cannot be resumed; the web UI's "go back" command is the same.

## Telling them apart

| Where | How |
|---|---|
| TUI | An `Agent` column (`claude` or `codex`) on a list 96 columns or wider; the detail pane's Version line names the agent; `agent:codex` or `agent:claude` in the [filter](tui.md#filter), and the agent's name as a plain word |
| Web UI | A `codex` badge on the session; the reader's details name the agent; the same filter |
| CLI | `--agent claude` or `--agent codex` on `search` and `list`; `agent` in every JSON output |
| MCP | `agent` on `recall_search` and `recall_list`, and in each result |
| `history` package | `Session.Agent` |

## Configuration

Under `[codex]` in the [config file](configuration.md): `enabled = false` leaves Codex sessions out of imports, searches and the watcher (the ones already archived stay), and `dir` names Codex's home directory when it is not `$CODEX_HOME` or `~/.codex`. A Codex tree elsewhere, such as a container's, can also be listed in `extra_projects_dirs`: a tree is read as Codex's by its layout.

## Archive

Codex sessions go in the same `~/.claude/vault.db`, with `agent = 'codex'` on the session row (`NULL` or `'claude'` is Claude Code's). The [schema](architecture.md#database) is otherwise the same, so an archive from before Codex sessions were stored is migrated by adding the column, and a `recall` from before reads the Codex rows as if they were Claude Code's.
