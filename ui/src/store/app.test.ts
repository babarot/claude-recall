import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Find, Session } from "../api";

const calls: { q: string; text: boolean }[] = [];
let fullResult: (q: string) => Find = (q) => ({ ids: [q], text: true, in: false });
let single: Session | null = null;

vi.mock("../api", () => ({
  api: {
    find: async (q: string, text: boolean, signal?: AbortSignal) => {
      calls.push({ q, text });
      await new Promise((r) => setTimeout(r, 10));
      if (signal?.aborted) throw new DOMException("aborted", "AbortError");
      return text ? fullResult(q) : { ids: [], text: false, in: false };
    },
    session: async () => single,
  },
}));

const app = await import("./app");

const row = (id: string, endedAt: string, extra: Partial<Session> = {}): Session => ({
  id, title: id, folder: "", groupKey: "k", groupName: "me/app", worktree: "", gone: false, branch: "", path: "",
  messages: 1, size: 1, startedAt: endedAt, endedAt, recallOnly: false, activity: [], ...extra,
});

describe("search", () => {
  beforeEach(() => {
    app.__resetForTests();
    calls.length = 0;
  });

  it("finds by the fields first, then the conversations, and a later query wins", async () => {
    app.setQuery("re", 0);
    app.setQuery("retry", 0);
    await vi.waitFor(() => expect(app.found.value?.text).toBe(true));
    expect(app.found.value?.ids).toEqual(["retry"]);
    expect(calls.filter((c) => c.q === "retry")).toEqual([{ q: "retry", text: false }, { q: "retry", text: true }]);
    expect(app.searching.value).toBe(false);
  });

  it("drops the result when the query is cleared", async () => {
    app.setQuery("retry", 0);
    await vi.waitFor(() => expect(app.found.value?.text).toBe(true));
    app.setQuery("");
    expect(app.found.value).toBeNull();
  });

  it("lets folder: in the query pick the folders over the one the list is narrowed to", async () => {
    app.sessions.value = [row("a", "2026-01-02T00:00:00Z", { groupKey: "other" }), row("b", "2026-01-01T00:00:00Z")];
    app.folder.value = "k";
    expect(app.visible.value.map((s) => s.id)).toEqual(["b"]);
    fullResult = () => ({ ids: ["a"], text: true, in: true });
    app.setQuery("in:other", 0);
    await vi.waitFor(() => expect(app.visible.value.map((s) => s.id)).toEqual(["a"]));
  });
});

describe("bodyWords", () => {
  it("takes the words a conversation is searched for", () => {
    expect(app.bodyWords("retry in:app text:x a branch:main policy")).toEqual(["retry", "x", "policy"]);
  });
});

describe("refreshSession", () => {
  it("puts an updated session first and marks it live", async () => {
    app.__resetForTests();
    app.sessions.value = [row("a", "2026-01-02T00:00:00Z"), row("b", "2026-01-01T00:00:00Z")];
    single = row("b", "2026-01-03T00:00:00Z", { messages: 9 });
    await app.refreshSession("b");
    expect(app.sessions.value.map((s) => [s.id, s.messages])).toEqual([["b", 9], ["a", 1]]);
    expect(app.isLive("b")).toBe(true);
    expect(app.isLive("a")).toBe(false);
  });
});
