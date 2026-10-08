# Records demo/claude-band.gif from the demo `make demo-claude` builds (see
# demo/README.md): the band of the repository's last sessions as a session starts. The real Claude Code, RECALL_DEMO_CLAUDE, loads the
# plugin from RECALL_DEMO_PLUGIN and talks to demo/fakeapi.

Output demo/claude-band.gif

Set Shell "bash"
# Recorded at twice the size the README shows it, as tui.tape is.
Set FontSize 24
Set Width 1800
Set Height 760
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms

Hide
Type 'source demo/.out/env.sh && cd "$HOME/src/github.com/acme/api" && clear'
Enter
Type 'claude() { "$RECALL_DEMO_CLAUDE" --plugin-dir "$RECALL_DEMO_PLUGIN" "$@"; } && clear'
Enter
Show

# Start Claude Code in the api repository: the band above the prompt offers
# the sessions that last ran here, its worktrees included.
Sleep 600ms
Type "claude"
Sleep 400ms
Enter
Sleep 9s

Hide
Ctrl+C
Ctrl+C
