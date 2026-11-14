# ADR-007: Ask keeps its questions in a file of its own, not in the archive

## Status

Accepted

## Context

The TUI's Ask box kept only the last question, in memory. A question is a line of natural language, often asked again with a few words changed, and retyping it costs more than in the filter. Ask runs `claude -p` with `--no-session-persistence`, so a question leaves no transcript and nothing of it reaches the archive.

The archive is the only copy of sessions whose transcripts Claude Code has deleted, so it is never rebuilt and changes to it are additive migrations. Several TUIs can run at once, in panes of one terminal, and each may ask.

## Decision

- The questions asked in Ask are kept in a file of JSON lines in the state directory, beside the TUI's other state, not in the archive. The history is the TUI's input, not a record of sessions, and keeping it out of the archive keeps the archive's migrations and its care to what only it holds ([askhistory](../../internal/askhistory)).
- A question is written as it is sent, and what its answer found in a line of its own when it comes, joined by ID when read; so a question stays when the TUI quits before the answer. An answer's line is written only after its question's: a read drops an answer with no question.
- Each line carries the version of its format. A read that meets a line of another version leaves the file as it is, so an older recall does not lose what a newer one wrote. A read otherwise writes the file back with the newest of each question, up to a number that is a constant, not a setting, since it only bounds the file.
- Every read and write holds a lock on a file of its own that is never replaced, and opens the history only once it holds it. The history is replaced when it is read, so a lock on the history itself, or a file kept open, would miss another TUI's write.
- Failing to read or keep the questions is told, and Ask goes on without them.
- Rejected: a table in the archive. It would be a migration of the one file that cannot be rebuilt, for data that is not a session and that no other part of recall reads.
- Rejected: keeping the scope a question was asked in. A question recalled is asked again about the list as it is now.

## Consequences

- A change to what a line holds must raise the version, and an older recall then leaves the file alone rather than rewriting it.
- The history does not travel with the archive: a copy of `vault.db` on another machine has no questions, and `--db` does not choose a history.
- Writes run in the background, so whatever writes a question's answer must wait for the question's own write.
