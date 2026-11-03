import { computed, signal } from "@preact/signals";
import { api } from "../api";
import type { Stats } from "../api";
import { FLAGS, METRICS, PERIODS, noFilter, prepare, range } from "../lib/stats";
import type { Breakdown, Filter, Flag, Grain, Group, Metric, SessionSort } from "../lib/stats";

// The Stats page's state. The period, the filters and what the main chart
// shows are in the URL (/stats?period=30&repo=...), so back and reload
// return to the same page.

export const data = signal<Stats | null>(null);
export const error = signal<string | null>(null);
export const rows = computed(() => (data.value ? prepare(data.value) : []));

export const period = signal(30);
export const metric = signal<Metric>("sessions");
export const group = signal<Group>("repo");
export const grain = signal<Grain>("day");
export const showPrev = signal(true);
export const filter = signal<Filter>(noFilter());
export const table = signal<Breakdown>("repo");
export const sessionSort = signal<SessionSort>("cost");
export const limit = signal(10);

let loading: AbortController | null = null;

/** Reads the sessions of the last two years: a year's period and the one before it. */
export async function load(now = new Date()): Promise<void> {
  loading?.abort();
  const ctl = new AbortController();
  loading = ctl;
  try {
    data.value = await api.stats(range(365, now, 1)[0], ctl.signal);
    error.value = null;
  } catch (e) {
    if (!ctl.signal.aborted) error.value = (e as Error).message;
  }
}

export function toggle<K extends "repo" | "model" | "flag" | "weekday">(kind: K, v: Filter[K][number]): void {
  const f = filter.value;
  const list = f[kind] as (typeof v)[];
  filter.value = { ...f, [kind]: list.includes(v) ? list.filter((x) => x !== v) : [...list, v] };
  limit.value = 10;
}

export function setSlot(day: number, hour: number): void {
  const s = filter.value.slot;
  filter.value = { ...filter.value, slot: s && s[0] === day && s[1] === hour ? null : [day, hour] };
}

export function clear(kind?: keyof Filter): void {
  filter.value = kind ? { ...filter.value, [kind]: kind === "slot" ? null : [] } : noFilter();
}

export function stepPeriod(d: number): void {
  const i = PERIODS.indexOf(period.value);
  period.value = PERIODS[Math.max(0, Math.min(PERIODS.length - 1, (i < 0 ? 1 : i) + d))];
}

const GROUPS: Group[] = ["none", "repo", "model", "work"];
export const nextGroup = () => { group.value = GROUPS[(GROUPS.indexOf(group.value) + 1) % GROUPS.length]; };

const list = (v: string | null) => (v ? v.split(",").filter(Boolean) : []);

/** Sets the state from the query of a /stats URL. */
export function readURL(search: string): void {
  const q = new URLSearchParams(search);
  const p = Number(q.get("period"));
  period.value = PERIODS.includes(p) ? p : 30;
  const m = q.get("metric") as Metric;
  metric.value = METRICS.includes(m) ? m : "sessions";
  const g = q.get("group") as Group;
  group.value = GROUPS.includes(g) ? g : "repo";
  grain.value = q.get("grain") === "week" ? "week" : "day";
  const slot = q.get("slot")?.split("-").map(Number);
  filter.value = {
    repo: list(q.get("repo")),
    model: list(q.get("model")),
    flag: list(q.get("flag")).filter((x): x is Flag => FLAGS.includes(x as Flag)),
    weekday: list(q.get("weekday")).map(Number).filter((n) => n >= 0 && n < 7),
    slot: slot?.length === 2 && slot.every((n) => Number.isInteger(n)) ? [slot[0], slot[1]] : null,
  };
}

/** The query of the URL that shows the state. */
export function urlQuery(): string {
  const q = new URLSearchParams();
  const f = filter.value;
  if (period.value !== 30) q.set("period", String(period.value));
  if (metric.value !== "sessions") q.set("metric", metric.value);
  if (group.value !== "repo") q.set("group", group.value);
  if (grain.value !== "day") q.set("grain", grain.value);
  for (const k of ["repo", "model", "flag", "weekday"] as const) if (f[k].length) q.set(k, f[k].join(","));
  if (f.slot) q.set("slot", f.slot.join("-"));
  const s = q.toString();
  return s ? `?${s}` : "";
}
