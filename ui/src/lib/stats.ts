import type { Stats, StatSession } from "../api";

// What the Stats page sums, from /api/v2/stats: the sessions of a period
// and the one before it, narrowed by the filters, by day or week, split by
// repository, model or kind of work. Everything here is a pure function of
// the sessions, so every chart and table follows the filters at once.

export type Metric = "sessions" | "hours" | "tokens" | "cost" | "calls" | "failRate";
export const METRICS: Metric[] = ["sessions", "hours", "tokens", "cost", "calls", "failRate"];
export type Group = "none" | "repo" | "model" | "work";
export type Grain = "day" | "week";
export type Flag = "interrupted" | "failing" | "long";
export const FLAGS: Flag[] = ["interrupted", "failing", "long"];
export type Work = "explore" | "run" | "edit" | "subagent" | "web" | "other";
export const WORKS: Work[] = ["explore", "run", "edit", "subagent", "web", "other"];
export const PERIODS = [7, 30, 90, 365];

/** A session is failing with this many failed tool calls, and long once worked in for this many minutes. */
export const FAILING = 10;
export const LONG = 180;

export interface Filter {
  repo: string[];
  model: string[];
  flag: Flag[];
  /** Days of the week, 0 for Sunday, as Date.getDay. */
  weekday: number[];
  /** A day of the week and an hour the session started in. */
  slot: [number, number] | null;
}

export const noFilter = (): Filter => ({ repo: [], model: [], flag: [], weekday: [], slot: null });
export const filtered = (f: Filter) => f.repo.length + f.model.length + f.flag.length + f.weekday.length > 0 || f.slot !== null;

/** A session with what the sums need worked out once. */
export interface Row {
  s: StatSession;
  start: Date;
  tokens: number;
  calls: number;
  failed: number;
  known: number;
  work: Record<Work, number>;
  /** Its cost by model, at the models' usual rates, scaled to its cost. */
  costBy: Record<string, number>;
  flags: Flag[];
}

export function prepare(data: Stats): Row[] {
  const out: Row[] = [];
  for (const s of data.sessions) {
    if (!s.startedAt) continue;
    const work = Object.fromEntries(WORKS.map((w) => [w, 0])) as Record<Work, number>;
    let calls = 0, failed = 0, known = 0;
    for (const [tool, [c, f, k]] of Object.entries(s.tools)) {
      calls += c;
      failed += f;
      known += k;
      const w = (data.categories[tool] ?? "other") as Work;
      work[WORKS.includes(w) ? w : "other"] += c;
    }
    let tokens = 0;
    const usual: Record<string, number> = {};
    let usualSum = 0;
    for (const [m, t] of Object.entries(s.tokens)) {
      tokens += t[0] + t[1] + t[2] + t[3] + t[4];
      const r = data.prices[m];
      usual[m] = r ? t.reduce((a, n, i) => a + n * r[i], 0) : 0;
      usualSum += usual[m];
    }
    const costBy: Record<string, number> = {};
    for (const m of Object.keys(usual)) costBy[m] = usualSum > 0 ? (s.cost * usual[m]) / usualSum : 0;
    const flags: Flag[] = [];
    if (s.interrupted) flags.push("interrupted");
    if (failed >= FAILING) flags.push("failing");
    if (s.activeMinutes >= LONG) flags.push("long");
    out.push({ s, start: new Date(s.startedAt), tokens, calls, failed, known, work, costBy, flags });
  }
  return out;
}

const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate());
const addDays = (d: Date, n: number) => new Date(d.getFullYear(), d.getMonth(), d.getDate() + n);

/** The days of a period ending today, or of the one before it (offset 1), as [from, to). */
export function range(days: number, now: Date, offset = 0): [Date, Date] {
  const end = addDays(startOfDay(now), 1 - offset * days);
  return [addDays(end, -days), end];
}

export const within = (r: Row, [from, to]: [Date, Date]) => r.start >= from && r.start < to;

/** Whether a session passes the filters, leaving out one of them (for that filter's own counts). */
export function passes(r: Row, f: Filter, except?: keyof Filter): boolean {
  if (except !== "repo" && f.repo.length && !f.repo.includes(r.s.groupKey)) return false;
  if (except !== "model" && f.model.length && !f.model.includes(r.s.model ?? "")) return false;
  if (except !== "weekday" && f.weekday.length && !f.weekday.includes(r.start.getDay())) return false;
  if (except !== "flag" && f.flag.length && !r.flags.some((x) => f.flag.includes(x))) return false;
  if (except !== "slot" && f.slot && (r.start.getDay() !== f.slot[0] || r.start.getHours() !== f.slot[1])) return false;
  return true;
}

/** A metric over sessions. The failure rate is of the calls whose outcome is known. */
export function total(rows: Row[], m: Metric): number {
  switch (m) {
    case "sessions": return rows.length;
    case "hours": return rows.reduce((a, r) => a + r.s.activeMinutes, 0) / 60;
    case "tokens": return rows.reduce((a, r) => a + r.tokens, 0);
    case "cost": return rows.reduce((a, r) => a + r.s.cost, 0);
    case "calls": return rows.reduce((a, r) => a + r.calls, 0);
    case "failRate": {
      const known = rows.reduce((a, r) => a + r.known, 0);
      return known ? rows.reduce((a, r) => a + r.failed, 0) / known : 0;
    }
  }
}

/** The part of a metric a series has in one session: its model's tokens and cost, or its share of the calls. */
function partOf(r: Row, m: Metric, g: Group, key: string): number {
  if (g === "model" && (m === "tokens" || m === "cost")) {
    const t = r.s.tokens[key];
    if (!t) return 0;
    return m === "tokens" ? t[0] + t[1] + t[2] + t[3] + t[4] : r.costBy[key] ?? 0;
  }
  if (g === "work") {
    const n = r.work[key as Work] ?? 0;
    if (m === "calls") return n;
    const share = r.calls ? n / r.calls : 0;
    return share * total([r], m);
  }
  return keyOf(r, g) === key ? total([r], m) : 0;
}

export function keyOf(r: Row, g: Group): string {
  switch (g) {
    case "repo": return r.s.groupKey;
    case "model": return r.s.model ?? "";
    default: return "";
  }
}

export interface Series {
  key: string;
  name: string;
  /** A CSS color: the slot of the series' rank, --s-other past five. */
  color: string;
}

const SLOTS = ["var(--s1)", "var(--s2)", "var(--s3)", "var(--s4)", "var(--s5)"];
export const OTHER = "var(--s-other)";

/**
 * The colors of repositories and models, by how much each was used over
 * the year before now: a color stays with its repository whatever the
 * period and the filters.
 */
export function ranks(rows: Row[], now: Date): { repo: Series[]; model: Series[] } {
  const year = range(365, now);
  const repo = new Map<string, { name: string; n: number }>();
  const model = new Map<string, number>();
  for (const r of rows) {
    if (!within(r, year)) continue;
    const x = repo.get(r.s.groupKey) ?? { name: r.s.groupName, n: 0 };
    x.n += r.s.activeMinutes;
    repo.set(r.s.groupKey, x);
    for (const [m, t] of Object.entries(r.s.tokens)) model.set(m, (model.get(m) ?? 0) + t[1]);
  }
  const color = (i: number) => SLOTS[i] ?? OTHER;
  return {
    repo: [...repo.entries()].sort((a, b) => b[1].n - a[1].n || a[0].localeCompare(b[0])).map(([key, x], i) => ({ key, name: x.name, color: color(i) })),
    model: [...model.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).map(([key], i) => ({ key, name: key, color: color(i) })),
  };
}

/** The series a chart split by g shows: the first five by rank, then the rest as one. */
export function seriesOf(g: Group, ranked: { repo: Series[]; model: Series[] }, workNames: Record<Work, string>, otherName: string): Series[] {
  if (g === "none") return [{ key: "", name: "", color: "var(--accent)" }];
  if (g === "work") return WORKS.map((w, i) => ({ key: w, name: workNames[w], color: SLOTS[i] ?? OTHER }));
  const list = ranked[g];
  return list.length > 5 ? [...list.slice(0, 5), { key: "\0other", name: otherName, color: OTHER }] : list;
}

export interface Bucket {
  from: Date;
  to: Date;
  /** The sessions started in it, and its part for each series. */
  rows: Row[];
  parts: number[];
  /** The metric over the same stretch of the period before. */
  prev: number;
}

/** A period by day or by week, the last bucket ending today. */
export function buckets(rows: Row[], prevRows: Row[], days: number, grain: Grain, now: Date, m: Metric, g: Group, series: Series[]): Bucket[] {
  const step = grain === "day" ? 1 : 7;
  const n = Math.ceil(days / step);
  const [start, end] = range(days, now);
  const out: Bucket[] = [];
  for (let i = n - 1; i >= 0; i--) {
    const to = addDays(end, -i * step);
    // The first week starts with the period, however many days it has.
    const from = new Date(Math.max(addDays(to, -step).getTime(), start.getTime()));
    const inB = rows.filter((r) => r.start >= from && r.start < to);
    const prevB = prevRows.filter((r) => r.start >= addDays(from, -days) && r.start < addDays(to, -days));
    const parts = m === "failRate" || g === "none" ? [total(inB, m)] : split(inB, m, g, series);
    out.push({ from, to, rows: inB, parts, prev: total(prevB, m) });
  }
  return out;
}

/** A metric over sessions split into series; "other" takes what the named ones do not. */
function split(rows: Row[], m: Metric, g: Group, series: Series[]): number[] {
  const parts = series.map(() => 0);
  for (const r of rows) {
    let named = 0;
    series.forEach((s, i) => {
      if (s.key === "\0other") return;
      const v = partOf(r, m, g, s.key);
      parts[i] += v;
      named += v;
    });
    const o = series.findIndex((s) => s.key === "\0other");
    if (o >= 0) parts[o] += Math.max(0, total([r], m) - named);
  }
  return parts;
}

/** How many sessions started in each hour of each day of the week, [day][hour]. */
export function hours(rows: Row[]): number[][] {
  const out = Array.from({ length: 7 }, () => new Array<number>(24).fill(0));
  for (const r of rows) out[r.start.getDay()][r.start.getHours()]++;
  return out;
}

/** Bounds, in minutes worked in, of the length chart's bars. */
export const LENGTHS: [number, number][] = [[0, 10], [10, 30], [30, 60], [60, 120], [120, 240], [240, Infinity]];

export function lengths(rows: Row[]): number[] {
  return LENGTHS.map(([lo, hi]) => rows.filter((r) => r.s.activeMinutes >= lo && r.s.activeMinutes < hi).length);
}

export function median(ns: number[]): number {
  if (ns.length === 0) return 0;
  const s = [...ns].sort((a, b) => a - b);
  return s[Math.floor(s.length / 2)];
}

/** The share of tool calls of each kind of work. */
export function workShares(rows: Row[]): Record<Work, number> {
  const sum = Object.fromEntries(WORKS.map((w) => [w, 0])) as Record<Work, number>;
  let all = 0;
  for (const r of rows) for (const w of WORKS) {
    sum[w] += r.work[w];
    all += r.work[w];
  }
  for (const w of WORKS) sum[w] = all ? sum[w] / all : 0;
  return sum;
}

export interface Line {
  key: string;
  name: string;
  /** What the row narrows the page to, when it does. */
  filter?: { repo?: string; model?: string };
  values: number[];
  /** The first value in the period before, for the change. */
  prev: number;
}

export type Breakdown = "repo" | "tool" | "program" | "file" | "model";

/** The breakdown table of one kind, most first. */
export function breakdown(kind: Breakdown, rows: Row[], prev: Row[]): Line[] {
  const by = <K>(list: Row[], keys: (r: Row) => [string, K][]) => {
    const m = new Map<string, K[]>();
    for (const r of list) for (const [k, v] of keys(r)) m.set(k, [...(m.get(k) ?? []), v]);
    return m;
  };
  let out: Line[] = [];
  if (kind === "repo" || kind === "model") {
    const key = (r: Row): [string, Row][] => [[kind === "repo" ? r.s.groupKey : r.s.model ?? "", r]];
    const cur = by(rows, key), before = by(prev, key);
    out = [...cur.entries()].filter(([k]) => k !== "").map(([k, list]) => ({
      key: k,
      name: kind === "repo" ? list[0].s.groupName : k,
      filter: kind === "repo" ? { repo: k } : { model: k },
      values: [total(list, "hours"), list.length, total(list, "tokens"), total(list, "cost"), total(list, "failRate")],
      prev: total(before.get(k) ?? [], "hours"),
    }));
  } else if (kind === "tool") {
    const sums = (list: Row[]) => {
      const m = new Map<string, [number, number, number]>();
      for (const r of list) for (const [tool, [c, f, k]] of Object.entries(r.s.tools)) {
        const x = m.get(tool) ?? [0, 0, 0];
        m.set(tool, [x[0] + c, x[1] + f, x[2] + k]);
      }
      return m;
    };
    const cur = sums(rows), before = sums(prev);
    out = [...cur.entries()].map(([k, [c, f, known]]) => {
      const b = before.get(k);
      return { key: k, name: k, values: [c, f, known ? f / known : 0, b && b[2] ? b[1] / b[2] : 0], prev: b?.[0] ?? 0 };
    });
  } else if (kind === "program") {
    const sums = (list: Row[]) => {
      const m = new Map<string, number>();
      for (const r of list) for (const p of r.s.programs) m.set(p.name, (m.get(p.name) ?? 0) + p.n);
      return m;
    };
    const cur = sums(rows), before = sums(prev);
    out = [...cur.entries()].map(([k, n]) => ({ key: k, name: k, values: [n], prev: before.get(k) ?? 0 }));
  } else {
    const sums = (list: Row[]) => {
      const m = new Map<string, { name: string; repo: string; read: number; edited: number }>();
      for (const r of list) for (const f of r.s.files) {
        const k = `${r.s.groupKey}\0${f.path}`;
        const x = m.get(k) ?? { name: f.path, repo: r.s.groupName, read: 0, edited: 0 };
        x.read += f.read;
        x.edited += f.edited;
        m.set(k, x);
      }
      return m;
    };
    const cur = sums(rows), before = sums(prev);
    out = [...cur.entries()].map(([k, x]) => {
      const b = before.get(k);
      return { key: k, name: `${x.repo}  ${x.name}`, values: [x.read + x.edited, x.read, x.edited], prev: b ? b.read + b.edited : 0 };
    });
  }
  return out.sort((a, b) => b.values[0] - a.values[0] || a.name.localeCompare(b.name));
}

export type SessionSort = "cost" | "failed" | "length" | "recent";

export function sortSessions(rows: Row[], by: SessionSort): Row[] {
  const key: Record<SessionSort, (r: Row) => number> = {
    cost: (r) => r.s.cost,
    failed: (r) => r.failed,
    length: (r) => r.s.activeMinutes,
    recent: (r) => r.start.getTime(),
  };
  return [...rows].sort((a, b) => key[by](b) - key[by](a) || a.s.id.localeCompare(b.s.id));
}

/** The change from prev to cur: a share in %, or for a rate, its difference in points. */
export function change(cur: number, prev: number, m: Metric): { up: boolean; text: string } | null {
  if (m === "failRate") {
    const d = (cur - prev) * 100;
    return { up: d > 0, text: `${Math.abs(d).toFixed(1)}pt` };
  }
  if (!prev) return null;
  // Against a period with almost nothing, a share means little: say how many times.
  if (cur >= prev * 10) return { up: true, text: `×${Math.round(cur / prev).toLocaleString()}` };
  const p = Math.round(((cur - prev) / prev) * 100);
  return { up: p > 0, text: `${Math.abs(p)}%` };
}

/** A round top for an axis over max, and its step, for four or five grid lines. */
export function niceAxis(max: number): { top: number; step: number } {
  if (!(max > 0)) return { top: 1, step: 0.25 };
  const raw = max / 5;
  const mag = 10 ** Math.floor(Math.log10(raw));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((x) => x >= raw) ?? 10 * mag;
  return { top: Math.ceil(max / step) * step, step };
}

