package theme

import (
	"regexp"
	"testing"
)

var hex = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Every scheme but ansi has the web UI's colors, so a new scheme cannot
// leave the page half drawn.
func TestWebColors(t *testing.T) {
	for _, name := range Names() {
		vars, dark, ok := WebColors(name)
		if name == Auto || name == "ansi" {
			if ok {
				t.Errorf("%s: the page keeps its own colors, got %v", name, vars)
			}
			continue
		}
		if !ok {
			t.Errorf("%s has no web colors", name)
			continue
		}
		for k, v := range vars {
			if !hex.MatchString(v) {
				t.Errorf("%s: %s = %q", name, k, v)
			}
		}
		if len(vars) != 19 {
			t.Errorf("%s: %d colors", name, len(vars))
		}
		if dark == (name == "catppuccin-latte") {
			t.Errorf("%s: dark = %v", name, dark)
		}
	}
	if v, _, _ := WebColors("catppuccin-latte"); v["prompt"] != "#b8720f" || v["mark-cur-text"] != "#ffffff" {
		t.Errorf("latte's prompt reads on white: %v", v)
	}
	if v, _, _ := WebColors("tokyo-night"); v["mark-cur-text"] != v["bg"] || v["surface-hi"] != palettes["tokyo-night"].Surface {
		t.Errorf("tokyo-night: %v", v)
	}
}
