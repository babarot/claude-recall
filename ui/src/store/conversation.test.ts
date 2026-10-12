import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Message, Range } from "../api";

// A session of n messages, with cursors "<i>.0", served the way the
// server serves ranges.
let all: Message[] = [];
const msg = (i: number, uuid = `u${i}`): Message => ({
  cursor: `${i}.0`, uuid, role: i % 2 ? "assistant" : "user", type: "text", content: `m${i}`,
  toolName: null, toolInput: null, timestamp: null,
});
const cmp = (a: string, b: string) => Number(a.split(".")[0]) - Number(b.split(".")[0]);
const range = (msgs: Message[]): Range => ({ total: all.length, messages: msgs, first: msgs[0]?.cursor, last: msgs[msgs.length - 1]?.cursor });

vi.mock("../api", () => ({
  api: {
    header: async () => ({ id: "s" }),
    outline: async () => [],
    after: async (_id: string, cursor: string, limit: number, anchor?: { uuid: string; known: number }) => {
      if (anchor) {
        const at = all.findIndex((m) => m.cursor === cursor);
        if (at < 0 || all[at].uuid !== anchor.uuid || at + 1 !== anchor.known) return { ...range([]), stale: true };
      }
      const from = cursor === "start" ? 0 : all.findIndex((m) => cmp(m.cursor, cursor) > 0);
      return range(from < 0 ? [] : all.slice(from, from + limit));
    },
    before: async (_id: string, cursor: string, limit: number) => {
      const to = cursor === "end" ? all.length : all.findIndex((m) => cmp(m.cursor, cursor) >= 0);
      return range(all.slice(Math.max(0, to - limit), to));
    },
  },
}));

const conv = await import("./conversation");

describe("conversation ranges", () => {
  beforeEach(() => conv.__resetForTests());

  it("opens a short session whole", async () => {
    all = Array.from({ length: 10 }, (_, i) => msg(i));
    await conv.open("s");
    expect(conv.spans.value).toHaveLength(1);
    expect(conv.loadedCount()).toBe(10);
    expect(conv.spans.value[0].atStart && conv.spans.value[0].atEnd).toBe(true);
  });

  it("reads a long one's start and end, then what lies between", async () => {
    all = Array.from({ length: conv.PAGE * 3 + 5 }, (_, i) => msg(i));
    await conv.open("s");
    expect(conv.spans.value).toHaveLength(2);
    expect(conv.loadedCount()).toBe(conv.PAGE * 2);
    await conv.readOn(0);
    expect(conv.spans.value).toHaveLength(2);
    expect(conv.loadedCount()).toBe(conv.PAGE * 3);
    await conv.readOn(0);
    // The rest reaches the end span, and the two are one.
    expect(conv.spans.value).toHaveLength(1);
    expect(conv.loadedCount()).toBe(all.length);
    expect(conv.spans.value[0].msgs.map((m) => m.content)).toEqual(all.map((m) => m.content));
  });

  it("reads around a message far from both ends", async () => {
    all = Array.from({ length: conv.PAGE * 5 }, (_, i) => msg(i));
    await conv.open("s");
    const far = all[conv.PAGE * 2 + 7].cursor;
    expect(conv.has(far)).toBe(false);
    await conv.readAround(far);
    expect(conv.has(far)).toBe(true);
    expect(conv.spans.value).toHaveLength(3);
  });

  it("follows what is added, and opens again after a rewrite", async () => {
    all = Array.from({ length: 5 }, (_, i) => msg(i));
    await conv.open("s");
    all.push(msg(5), msg(6));
    await conv.follow("s", () => true);
    expect(conv.loadedCount()).toBe(7);
    expect(conv.unseen.value).toBe(0);
    all.push(msg(7));
    await conv.follow("s", () => false);
    expect(conv.unseen.value).toBe(1);
    // Rewritten: the same count, other messages.
    all = Array.from({ length: 8 }, (_, i) => msg(i, `x${i}`));
    await conv.follow("s", () => true);
    await vi.waitFor(() => expect(conv.spans.value[0].msgs[0].uuid).toBe("x0"));
  });
});

describe("follow", () => {
  beforeEach(() => conv.__resetForTests());

  it("adds each message once when events come in a burst", async () => {
    all = Array.from({ length: 5 }, (_, i) => msg(i));
    await conv.open("s");
    all.push(msg(5), msg(6));
    await Promise.all([conv.follow("s", () => true), conv.follow("s", () => true), conv.follow("s", () => true)]);
    await vi.waitFor(() => expect(conv.loadedCount()).toBe(7));
    await new Promise((r) => setTimeout(r, 20));
    expect(conv.spans.value[0].msgs.map((m) => m.cursor)).toEqual(all.map((m) => m.cursor));
    expect(conv.stickToEnd.value).toBe(true);
  });
});

describe("follow, after much came in", () => {
  beforeEach(() => conv.__resetForTests());

  it("reads the start and the end again rather than stop short of the end", async () => {
    all = Array.from({ length: 5 }, (_, i) => msg(i));
    await conv.open("s");
    for (let i = 5; i < 5 + conv.FOLLOW + 10; i++) all.push(msg(i));
    await conv.follow("s", () => true);
    await vi.waitFor(() => {
      const spans = conv.spans.value;
      expect(spans[spans.length - 1].msgs.at(-1)?.cursor).toBe(all[all.length - 1].cursor);
    });
  });
});

describe("joinSpans", () => {
  it("joins spans that overlap and keeps apart ones that do not", () => {
    const s = (from: number, to: number, atStart = false, atEnd = false) =>
      ({ msgs: Array.from({ length: to - from }, (_, i) => msg(from + i)), atStart, atEnd });
    const joined = conv.joinSpans([s(8, 12, false, true), s(0, 3, true), s(2, 5)]);
    expect(joined.map((x) => [x.msgs[0].cursor, x.msgs[x.msgs.length - 1].cursor])).toEqual([["0.0", "4.0"], ["8.0", "11.0"]]);
    expect(joined[0].atStart && !joined[0].atEnd && joined[1].atEnd).toBe(true);
  });
});
