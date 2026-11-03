import { describe, expect, it } from "vitest";
import type { Stats, StatSession } from "../api";
import { breakdown, buckets, change, niceAxis, noFilter, passes, prepare, range, ranks, seriesOf, total, within, workShares } from "./stats";

const now = new Date(2026, 10, 14, 15, 0); // a Saturday

function session(id: string, day: number, hour: number, over: Partial<StatSession> = {}): StatSession {
  return {
    id, title: id, startedAt: new Date(2026, 10, day, hour, 30).toISOString(), groupKey: "repo-a", groupName: "acme/a",
    activeMinutes: 60, messages: 10, tokens: {}, cost: 0, tools: {}, programs: [], files: [], ...over,
  };
}

const data: Stats = {
  sessions: [
    session("s1", 14, 9, { model: "opus", tokens: { opus: [100, 10, 0, 0, 0], haiku: [50, 5, 0, 0, 0] }, cost: 3,
      tools: { Bash: [10, 4, 8], Read: [5, 0, 5], mcp: [5, 0, 0] }, programs: [{ name: "go test", n: 3 }],
      files: [{ path: "main.go", read: 2, edited: 1 }] }),
    session("s2", 13, 21, { groupKey: "repo-b", groupName: "acme/b", activeMinutes: 200, interrupted: true, tools: { Bash: [12, 10, 12] } }),
    // The period before a 7-day one.
    session("s3", 5, 9, { tools: { Bash: [4, 0, 4] }, programs: [{ name: "go test", n: 1 }] }),
    session("old", 1, 9, { startedAt: null }),
  ],
  categories: { Bash: "run", Read: "explore", mcp: "other" },
  prices: { opus: [1, 2, 0, 0, 0], haiku: [0.5, 1, 0, 0, 0] },
  unknownTokens: 0,
};

describe("stats", () => {
  const rows = prepare(data);
  const cur = rows.filter((r) => within(r, range(7, now)));
  const prev = rows.filter((r) => within(r, range(7, now, 1)));

  it("works out each session once", () => {
    expect(rows.map((r) => r.s.id)).toEqual(["s1", "s2", "s3"]);
    const s1 = rows[0];
    expect([s1.calls, s1.failed, s1.known, s1.tokens]).toEqual([20, 4, 13, 165]);
    expect(s1.work).toMatchObject({ run: 10, explore: 5, other: 5 });
    // The cost split by the models' usual rates: opus 120, haiku 30.
    expect(s1.costBy.opus).toBeCloseTo(2.4);
    expect(s1.costBy.haiku).toBeCloseTo(0.6);
    expect(rows[1].flags).toEqual(["interrupted", "failing", "long"]);
  });

  it("splits the periods by day", () => {
    expect(range(7, now)).toEqual([new Date(2026, 10, 8), new Date(2026, 10, 15)]);
    expect(range(7, now, 1)).toEqual([new Date(2026, 10, 1), new Date(2026, 10, 8)]);
    expect(cur.map((r) => r.s.id)).toEqual(["s1", "s2"]);
    expect(prev.map((r) => r.s.id)).toEqual(["s3"]);
  });

  it("sums the metrics, the failure rate over known outcomes", () => {
    expect(total(cur, "sessions")).toBe(2);
    expect(total(cur, "hours")).toBeCloseTo(260 / 60);
    expect(total(cur, "calls")).toBe(32);
    expect(total(cur, "failRate")).toBeCloseTo(14 / 25);
    expect(change(14 / 25, 0, "failRate")).toEqual({ up: true, text: "56.0pt" });
    expect(change(2, 1, "sessions")).toEqual({ up: true, text: "100%" });
    expect(change(2, 0, "sessions")).toBeNull();
    expect(change(222, 2, "sessions")).toEqual({ up: true, text: "×111" });
    expect(niceAxis(4501)).toEqual({ top: 5000, step: 1000 });
    expect(niceAxis(36)).toEqual({ top: 40, step: 10 });
    expect(niceAxis(0.04)).toEqual({ top: 0.04, step: 0.01 });
  });

  it("filters, leaving out a filter for its own counts", () => {
    const f = { ...noFilter(), repo: ["repo-b"], flag: ["long" as const] };
    expect(cur.filter((r) => passes(r, f)).map((r) => r.s.id)).toEqual(["s2"]);
    expect(cur.filter((r) => passes(r, f, "repo")).map((r) => r.s.id)).toEqual(["s2"]);
    expect(cur.filter((r) => passes(r, { ...noFilter(), slot: [6, 9] })).map((r) => r.s.id)).toEqual(["s1"]);
    expect(cur.filter((r) => passes(r, { ...noFilter(), weekday: [5] })).map((r) => r.s.id)).toEqual(["s2"]);
  });

  it("keeps a repository's color whatever is filtered", () => {
    const ranked = ranks(rows, now);
    // repo-a has 120 minutes in the year, repo-b 200.
    expect(ranked.repo.map((s) => [s.key, s.color])).toEqual([["repo-b", "var(--s1)"], ["repo-a", "var(--s2)"]]);
    expect(ranked.model.map((s) => s.key)).toEqual(["opus", "haiku"]);
  });

  it("splits a chart by series, each day of the period a bucket", () => {
    const ranked = ranks(rows, now);
    const series = seriesOf("model", ranked, { explore: "", run: "", edit: "", subagent: "", web: "", other: "" }, "other");
    const bs = buckets(cur, prev, 7, "day", now, "tokens", "model", series);
    expect(bs).toHaveLength(7);
    expect(bs[6].parts).toEqual([110, 55]); // the 14th: opus and haiku
    expect(bs[5].parts).toEqual([0, 0]);
    const weeks = buckets(cur, prev, 7, "week", now, "sessions", "none", seriesOf("none", ranked, {} as never, ""));
    expect(weeks.map((b) => [b.parts[0], b.prev])).toEqual([[2, 1]]);
    const month = buckets(cur, prev, 30, "week", now, "sessions", "none", seriesOf("none", ranked, {} as never, ""));
    expect(month).toHaveLength(5);
    expect(month[0].from).toEqual(range(30, now)[0]);
  });

  it("tells the share of each kind of work", () => {
    expect(workShares(cur)).toMatchObject({ run: 22 / 32, explore: 5 / 32, other: 5 / 32 });
  });

  it("breaks down by repository, tool, program and file", () => {
    expect(breakdown("repo", cur, prev).map((l) => [l.name, l.values[1], l.prev])).toEqual([["acme/b", 1, 0], ["acme/a", 1, 1]]);
    const bash = breakdown("tool", cur, prev).find((l) => l.key === "Bash")!;
    expect(bash.values).toEqual([22, 14, 14 / 20, 0]);
    expect(breakdown("program", cur, prev)).toEqual([{ key: "go test", name: "go test", values: [3], prev: 1 }]);
    expect(breakdown("file", cur, prev)[0].values).toEqual([3, 2, 1]);
  });
});
