# ADR-011: Every recall process logs to one file, and never what is said in a session

## Status

Accepted

## Context

Much of recall runs where no one watches it. Every Claude Code session starts its own `recall mcp`, several at once, one of which watches the transcripts and imports them; the web UI runs in the background; the `SessionEnd` hook imports when a session ends. Their failures went to standard error, which Claude Code and the background process keep out of sight, so an import failing over and over could go unnoticed. recall did not log.

What recall handles is the user's conversations with Claude, which are not to be copied into another file.

## Decision

- Every recall process appends JSON lines, through `log/slog`, to one file in the state directory, which `recall log` names ([internal/logging](../../internal/logging)). A line names the kind of process and an ID for it, so the processes running at once can be told apart and read in the order things happened.
- A process that starts moves the file aside when it is over a size, keeping a few old files. Only the process holding the file's lock moves it. A process that runs for long checks, before every line, that the file it has open is still the one at the path, and opens it again if not; checking at intervals instead would let it write on to a file moved aside past the old ones kept, which is gone. The size and the number kept are constants: they only bound the space the log takes.
- The level is `info` unless `core.log_level`, or `RECALL_LOG_LEVEL` over it, says otherwise. It is under `[core]`, which every command reads, as the log is every process's.
- The logger is made in `cmd/recall` and passed down; no package keeps one in a variable, and `slog.SetDefault` is not used.
- Never logged: what is said in a session, the questions asked in Ask, the words searched for through the MCP tools or the UI, and a session's title, which all come from the conversation. A session is named by its ID. Paths are logged, since they are where the transcripts are on this machine. Tests check that what was said does not reach the log.
- A log file that cannot be opened, or a level in the environment that cannot be read, leaves the command running with no log: the log is for finding out what went wrong, never a reason to stop.
- Rejected: a file for each kind of process. Several `recall mcp` run at once, so it would not keep processes from writing one file, and the order of what happened across them would be split.
- Rejected: taking the lock for every line. The `recall mcp` running at once would wait on one another to log.

## Consequences

- Something new to log names a session by its ID; a message or attribute holding text from a conversation is a breach of this decision, however useful.
- The old standard error output stays where something reads it (the hook's output, `recall import`); the log is in addition to it.
- A line written in the instant between a long-running process checking the file and writing to it can be lost, if the file is moved aside past the old ones kept in that instant.
