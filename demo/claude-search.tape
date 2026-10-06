# Records demo/claude-search.gif from the demo `make demo-claude` builds (see
# demo/README.md): Claude searching recall over MCP, its result drawn as a tree. The real Claude Code, RECALL_DEMO_CLAUDE, loads the
# plugin from RECALL_DEMO_PLUGIN and talks to demo/fakeapi.

Output demo/claude-search.gif

Set Shell "bash"
# Recorded at twice the size the README shows it, as tui.tape is.
Set FontSize 24
Set Width 1800
Set Height 1100
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms

Hide
Type 'source demo/.out/env.sh && cd "$HOME/src/github.com/acme/api" && clear'
Enter
Type 'claude() { "$RECALL_DEMO_CLAUDE" --plugin-dir "$RECALL_DEMO_PLUGIN" "$@"; } && clear'
Enter
Type 'claude'
Enter
Sleep 8s
Show

# Ask Claude: it calls recall_search, and the result is drawn as the
# repository, its sessions and the messages that matched.
Sleep 800ms
Type@40ms "Look up what we did about the token refresh with recall"
Sleep 500ms
Enter
Sleep 10s

Hide
Ctrl+C
Ctrl+C
