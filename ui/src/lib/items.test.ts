import { describe, expect, it } from "vitest";
import { buildItems, looksFailed, tally, toolLabel } from "./items";
import type { Message } from "./chat-utils";

let n = 0;
const m = (role: string, blockType: string, content: string, extra: Partial<Message> = {}): Message =>
  ({ uuid: `u${n}`, cursor: `${n++}.0`, role, blockType, content, ...extra });
const use = (name: string, input = "{}") => m("assistant", "tool_use", name, { toolName: name, toolInput: input, timestamp: `2026-01-01T00:00:0${n % 10}Z` });
const result = (content: string) => m("user", "tool_result", content);

describe("buildItems", () => {
  it("folds a run of tool calls into one item, each call with its result", () => {
    const items = buildItems([
      m("user", "text", "fix it"),
      use("Read"), use("Grep"), result("file"), result("matches"),
      use("Bash"), result("Exit code 1\nFAIL"),
      m("assistant", "text", "done"),
    ]);
    expect(items.map((i) => i.kind)).toEqual(["message", "run", "message"]);
    const run = items[1];
    if (run.kind !== "run") throw new Error("not a run");
    expect(run.calls.map((c) => [c.use?.toolName, c.result?.content])).toEqual([
      ["Read", "file"], ["Grep", "matches"], ["Bash", "Exit code 1\nFAIL"],
    ]);
    expect(tally(run.calls)).toEqual({ counts: [["Read", 1], ["Grep", 1], ["Bash", 1]], failed: 1 });
    expect(run.start).toBeDefined();
  });

  it("keeps a result with no call, and splits runs at text and thinking", () => {
    const items = buildItems([
      result("orphan"), m("assistant", "thinking", "hmm"), use("Edit"), m("assistant", "text", "ok"), use("Write"),
    ]);
    expect(items.map((i) => i.kind)).toEqual(["run", "thinking", "run", "message", "run"]);
    const first = items[0];
    if (first.kind !== "run") throw new Error("not a run");
    expect(first.calls[0].use).toBeUndefined();
    expect(first.calls[0].result?.content).toBe("orphan");
  });

  it("keys items by cursor", () => {
    const items = buildItems([m("user", "text", "a"), m("assistant", "text", "b")]);
    expect(new Set(items.map((i) => i.key)).size).toBe(2);
    expect(items[0].key).toMatch(/^\d+\.0$/);
  });
});

describe("looksFailed", () => {
  it("tells what Claude Code writes for a failed call", () => {
    expect(looksFailed("Exit code 2\nboom")).toBe(true);
    expect(looksFailed("<tool_use_error>File has not been read yet</tool_use_error>")).toBe(true);
    expect(looksFailed("Error: no such file")).toBe(true);
    expect(looksFailed("Exit code 0")).toBe(false);
    expect(looksFailed("errors []")).toBe(false);
    expect(looksFailed("ok")).toBe(false);
  });
});

describe("toolLabel", () => {
  it("names an MCP tool by its tool", () => {
    expect(toolLabel("mcp__claude-in-chrome__navigate")).toBe("navigate");
    expect(toolLabel("mcp__plugin_claude-recall_claude-recall__recall_search")).toBe("recall_search");
    expect(toolLabel("Bash")).toBe("Bash");
  });
});
