import { describe, expect, it } from "vitest";
import { footer, HELP_GROUPS, KEYMAP, lookup } from "./keymap";
import type { Pane } from "./keymap";

const PANES: Pane[] = ["folders", "list", "reader"];

describe("keymap", () => {
  // As the TUI's uistate tests ask: every pane has a footer and a group in
  // the ? key list, so a new pane cannot leave its keys unsaid.
  it("gives every pane a footer and a group in the key list", () => {
    for (const p of PANES) {
      expect(footer(p).length, p).toBeGreaterThan(0);
      expect(HELP_GROUPS.some((g) => g.where === p), p).toBe(true);
    }
    expect(HELP_GROUPS.some((g) => g.where === "any")).toBe(true);
  });

  it("does not give one key two actions in one place", () => {
    for (const where of [...PANES, "any"]) {
      const seen = new Map<string, string>();
      for (const b of KEYMAP.filter((x) => x.where === where)) {
        for (const k of b.keys) {
          expect(seen.get(k), `${where} ${k}`).toBeUndefined();
          seen.set(k, b.action);
        }
      }
    }
  });

  it("looks a key up in the pane first, then everywhere", () => {
    expect(lookup("reader", "[")).toBe("prevPrompt");
    expect(lookup("list", "[")).toBe("folders");
    expect(lookup("reader", "/")).toBe("search");
    expect(lookup("folders", "Y")).toBe("copyResume");
    expect(lookup("list", "x")).toBeUndefined();
  });

  it("joins a pair of keys in the footer", () => {
    expect(footer("list")[0]).toEqual({ keys: "j k", label: "移動" });
    expect(footer("reader").find((f) => f.label === "前・次の発言")?.keys).toBe("[ ]");
    expect(footer("reader").find((f) => f.label === "一覧へ")?.keys).toBe("Esc");
  });
});
