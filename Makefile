.DEFAULT_GOAL := build

.PHONY: build install ui test clean demo-tui demo-tui-ja demo-claude

# The web UI is built with npm and embedded into the binary (build tag
# embedui). A plain `go build ./cmd/recall` works too; it leaves the UI out.
build: ui
	go build -tags embedui -o recall ./cmd/recall

install: build
	install -m 755 -v recall $(or $(RECALL_INSTALL_DIR),$(HOME)/.local/bin)/recall

ui:
	cd ui && npm ci && npm run build
	rm -rf internal/webui/dist
	cp -R ui/dist internal/webui/dist

test:
	go vet ./...
	go test ./...
	cd ui && npm test

clean:
	rm -rf recall ui/dist internal/webui/dist

# The demo GIFs are recorded with VHS from a demo archive (see demo/README.md).

# demo/tui.gif: the TUI.
demo-tui:
	go build -o demo/.out/bin/recall ./cmd/recall
	go run ./demo/gen
	vhs demo/tui.tape

# The same in Japanese, to demo/ja/tui.gif (not committed).
demo-tui-ja:
	go build -o demo/.out/bin/recall ./cmd/recall
	go run ./demo/gen -lang ja
	mkdir -p demo/ja
	vhs demo/tui-ja.tape

# demo/claude-*.gif: the plugin's mod in the real Claude Code (CLAUDE, by
# default the claude on PATH), started on the same demo and talking to
# demo/fakeapi instead of the Anthropic API.
CLAUDE ?= $(shell command -v claude)
DEMO_API ?= 127.0.0.1:47123
demo-claude:
	@test -n "$(CLAUDE)" || { echo "demo-claude needs Claude Code: set CLAUDE or put claude on PATH" >&2; exit 1; }
	go build -o demo/.out/bin/recall ./cmd/recall
	go build -o demo/.out/fakeapi ./demo/fakeapi
	go build -o demo/.out/gen ./demo/gen
	demo/.out/fakeapi -addr $(DEMO_API) & api=$$!; trap 'kill $$api' EXIT; \
	  export RECALL_DEMO_CLAUDE="$(CLAUDE)" RECALL_DEMO_PLUGIN="$(CURDIR)/plugin"; \
	  version=$$("$(CLAUDE)" --version | cut -d' ' -f1); \
	  for scene in band search command; do \
	    demo/.out/gen -api $(DEMO_API) -claude-version "$$version" >/dev/null && vhs demo/claude-$$scene.tape || exit 1; \
	  done
