#!/usr/bin/env bash
# Records demo/claude-*.gif: the plugin's mod in the real Claude Code (CLAUDE,
# by default the claude on PATH), started on the demo archive and talking to
# demo/fakeapi instead of the Anthropic API. With "ja", the conversations are
# in Japanese and the GIFs go to demo/ja (not committed). See demo/README.md.
set -euo pipefail

lang=${1:-en}
case $lang in
  en) suffix= ;;
  ja) suffix=-ja; mkdir -p demo/ja ;;
  *) echo "usage: $0 [en|ja]" >&2; exit 2 ;;
esac

claude=${CLAUDE:-$(command -v claude || true)}
if [ -z "$claude" ]; then
  echo "demo:claude needs Claude Code: set CLAUDE or put claude on PATH" >&2
  exit 1
fi
api=${DEMO_API:-127.0.0.1:47123}

go build -o demo/.out/bin/recall ./cmd/recall
go build -o demo/.out/fakeapi ./demo/fakeapi
go build -o demo/.out/gen ./demo/gen

demo/.out/fakeapi -addr "$api" -lang "$lang" &
fakeapi=$!
trap 'kill $fakeapi' EXIT

export RECALL_DEMO_CLAUDE="$claude" RECALL_DEMO_PLUGIN="$PWD/plugin"
version=$("$claude" --version | cut -d' ' -f1)
for scene in band search command; do
  demo/.out/gen -lang "$lang" -api "$api" -claude-version "$version" >/dev/null
  vhs "demo/claude-$scene$suffix.tape"
done
