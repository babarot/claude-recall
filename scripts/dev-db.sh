#!/usr/bin/env bash
# Copies the archive to .dev/vault.db, the copy mise run dev works on, so
# development never writes to the real one. The copy is the archive's as of
# now; run it again for a fresh one. RECALL_DEV_SOURCE names another archive
# to copy from than ~/.claude/vault.db.
set -euo pipefail

src=${RECALL_DEV_SOURCE:-$HOME/.claude/vault.db}
dst=.dev/vault.db

if [ ! -f "$src" ]; then
  echo "dev:db: no archive at $src (set RECALL_DEV_SOURCE)" >&2
  exit 1
fi
# A server on the copy would go on with a file replaced under it
if [ -f "$dst" ] && lsof "$dst" >/dev/null 2>&1; then
  echo "dev:db: $dst is open; stop mise run dev first" >&2
  exit 1
fi

mkdir -p .dev
echo "Copying $src to $dst..."
# .backup reads a consistent copy while recall may be writing; it goes to a
# temporary file first, so an interrupted copy leaves the old one in place.
# What an interrupted one left is removed, or .backup would fail on it.
rm -f "$dst.tmp" "$dst.tmp-wal" "$dst.tmp-shm"
sqlite3 "$src" ".backup '$dst.tmp'"
rm -f "$dst.tmp-wal" "$dst.tmp-shm"
rm -f "$dst-wal" "$dst-shm"
mv "$dst.tmp" "$dst"
