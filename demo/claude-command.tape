# Records demo/claude-command.gif from the demo `make demo-claude` builds (see
# demo/README.md): /recall, searching without a model turn. The real Claude Code, RECALL_DEMO_CLAUDE, loads the
# plugin from RECALL_DEMO_PLUGIN and talks to demo/fakeapi.

Output demo/claude-command.gif

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

# Search this repository's sessions without asking Claude.
Sleep 800ms
Type@60ms "/recall token"
Sleep 600ms
Enter
Sleep 4.5s

# Nothing about staging here, so search every repository.
Type@60ms "/recall staging --all"
Sleep 600ms
Enter
Sleep 4.5s

Hide
Ctrl+C
Ctrl+C
