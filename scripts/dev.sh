#!/usr/bin/env bash
# Runs the API, rebuilt and restarted when Go code changes, and the web UI's
# dev server with hot reload. Both work on .dev/vault.db, a copy of the
# archive (see scripts/dev-db.sh), never on the real one. Each worktree gets
# its own pair of ports, the same each time, so several can run side by side
# and next to the usual recall ui on 6276. Ctrl-C stops both.
set -euo pipefail

free() { ! lsof -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1; }

# A pair of ports from the worktree's path: the API, then the UI
api=$((30000 + $(printf %s "$PWD" | cksum | cut -d' ' -f1) % 5000 * 2))
while ! free "$api" || ! free $((api + 1)); do api=$((api + 2)); done
ui=$((api + 1))

[ -f .dev/vault.db ] || scripts/dev-db.sh
[ -d ui/node_modules ] || (cd ui && npm ci)

# Stop everything this started, however this ends
cleanup() {
  trap - EXIT INT TERM HUP
  kill 0 2>/dev/null || true
}
trap cleanup EXIT INT TERM HUP

# The API, built again and restarted when Go code changes. It is built to a
# file and run, not go run, so a restart stops the server itself. It keeps
# the copy up to date with the transcripts, as recall ui does the archive.
watchexec --restart --exts go,mod,sum --watch cmd --watch internal --watch go.mod --debounce 500ms -- \
  "go build -o .dev/recall ./cmd/recall && exec .dev/recall --db .dev/vault.db ui --foreground --port $api" &

(cd ui && RECALL_API="http://127.0.0.1:$api" exec npx vite --port "$ui" --strictPort --clearScreen false) &

cat <<EOS

  recall dev
  UI   http://localhost:$ui/   (hot reload)
  API  http://localhost:$api/  (restarted when Go code changes)
  DB   .dev/vault.db           (a copy; mise run dev:db makes a fresh one)

EOS
wait
