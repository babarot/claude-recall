# Records the Japanese demo, demo/ja/claude-command.gif, from the demo `make
# demo-claude-ja` builds (see demo/README.md): claude-command.tape with Claude
# Code's language set to Japanese. It is not committed; make it when you need
# it.

Output demo/ja/claude-command.gif

Set Shell "bash"
# IBM Plex Mono, with IBM Plex Sans JP for the Japanese it does not have,
# taller than the English tape for the same rows, as tui-ja.tape is.
Set FontFamily "IBM Plex Mono, IBM Plex Sans JP"
Set FontSize 24
Set Width 1800
Set Height 1240
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
Type@60ms "/recall トークン"
Sleep 600ms
Enter
Sleep 4.5s

# Nothing about staging here, so search every repository.
Type@60ms "/recall ステージング --all"
Sleep 600ms
Enter
Sleep 4.5s

Hide
Ctrl+C
Ctrl+C
