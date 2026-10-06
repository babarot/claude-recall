package title

import "testing"

func TestPrompt(t *testing.T) {
	cases := map[string]string{
		"fix the bug": "fix the bug",
		"<command-message>commit</command-message> <command-name>/commit</command-name> <command-args>staged only</command-args>": "/commit staged only",
		"<command-name>/clear</command-name>":                         "/clear",
		"<bash-input>git status</bash-input>":                         "! git status",
		"<pasted_content id=\"x\"> # Handoff\nnotes</pasted_content>": "# Handoff notes",
		"": "(no prompt)",
	}
	for in, want := range cases {
		if got := Prompt(in); got != want {
			t.Errorf("Prompt(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDisplay(t *testing.T) {
	if got := Display("  Fix the deploy ", "<command-name>/x</command-name>"); got != "Fix the deploy" {
		t.Errorf("a stored title wins, got %q", got)
	}
	if got := Display("", "<command-name>/x</command-name>"); got != "/x" {
		t.Errorf("no title falls back to the first prompt, got %q", got)
	}
}
