# Records the Japanese demo, demo/ja/claude-band.gif, from the demo `make
# demo-claude-ja` builds (see demo/README.md): claude-band.tape with Claude
# Code's language set to Japanese. It is not committed; make it when you need
# it.

Output demo/ja/claude-band.gif

Set Shell "bash"
# IBM Plex Mono, with IBM Plex Sans JP for the Japanese it does not have,
# taller than the English tape for the same rows, as tui-ja.tape is.
Set FontFamily "IBM Plex Mono, IBM Plex Sans JP"
Set FontSize 24
Set Width 1800
Set Height 860
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
