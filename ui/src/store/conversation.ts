import { signal } from "@preact/signals";
import { api } from "../api";
import type { Detail, Hit, Message, Prompt, SessionDetail } from "../api";

// The open session's conversation, read in ranges: its start and its end
// when it opens, what lies between when asked, and what is added while it
// is open. A long session holds tens of megabytes, so it is never read
// whole.

export const PAGE = 300;
/** How many new messages a live update reads at most. */
export const FOLLOW = PAGE * 2;

export interface Span {
  msgs: Message[];
  /** The span begins at the session's first message, or ends at its last. */
  atStart: boolean;
  atEnd: boolean;
}

export const header = signal<SessionDetail | null>(null);
export const spans = signal<Span[]>([]);
export const total = signal(0);
export const outline = signal<Prompt[]>([]);
export const detail = signal<Detail | null>(null);
export const convError = signal<string | null>(null);
export const opening = signal(false);
/** Messages that came in while the reader was not at the end. */
export const unseen = signal(0);
/** Scroll to the end once the next messages are drawn. */
export const stickToEnd = signal(false);

export const findQuery = signal("");
export const hits = signal<Hit[]>([]);
export const hitIndex = signal(0);

/** The whole of messages the server clipped, by cursor, once read. */
export const whole = signal<Record<string, Message>>({});

let openID: string | null = null;
let openAbort: AbortController | null = null;
/** How many messages there are up to the end span's last, as the server counted. */
let endCount = 0;

/** Orders two cursors ("<turn>.<block>"). */
export function compareCursors(a: string, b: string): number {
  const [at, ab] = a.split(".").map(Number);
  const [bt, bb] = b.split(".").map(Number);
  return at - bt || ab - bb;
}

const first = (s: Span) => s.msgs[0]?.cursor;
const last = (s: Span) => s.msgs[s.msgs.length - 1]?.cursor;

/**
 * Puts spans in order and joins the ones that touch: one that was read on
 * from the other's last message, or that overlaps it.
 */
export function joinSpans(list: Span[]): Span[] {
  const sorted = list.filter((s) => s.msgs.length > 0 || s.atStart || s.atEnd)
    .sort((a, b) => (a.atStart ? -1 : b.atStart ? 1 : compareCursors(first(a) ?? "0.0", first(b) ?? "0.0")));
  const out: Span[] = [];
  for (const s of sorted) {
    const prev = out[out.length - 1];
    if (prev && (prev.msgs.length === 0 || s.msgs.length === 0 || compareCursors(first(s)!, last(prev)!) <= 0)) {
      const seen = new Set(prev.msgs.map((m) => m.cursor));
      prev.msgs = [...prev.msgs, ...s.msgs.filter((m) => !seen.has(m.cursor))].sort((a, b) => compareCursors(a.cursor, b.cursor));
      prev.atStart ||= s.atStart;
      prev.atEnd ||= s.atEnd;
      continue;
    }
    out.push({ ...s, msgs: [...s.msgs] });
  }
  return out;
}

export function loadedCount(list = spans.value): number {
  return list.reduce((n, s) => n + s.msgs.length, 0);
}

/** Opens a session: its header, outline, start and end. */
export async function open(id: string): Promise<void> {
  openAbort?.abort();
  const ctl = new AbortController();
  openAbort = ctl;
  openID = id;
  header.value = null;
  spans.value = [];
  outline.value = [];
  detail.value = null;
  convError.value = null;
  unseen.value = 0;
  whole.value = {};
  hits.value = [];
  hitIndex.value = 0;
  opening.value = true;
  try {
    const [h, head, tail, o] = await Promise.all([
      api.header(id, ctl.signal),
      api.after(id, "start", PAGE, undefined, ctl.signal),
      api.before(id, "end", PAGE, ctl.signal),
      api.outline(id, ctl.signal),
    ]);
    if (openID !== id) return;
    header.value = h;
    total.value = tail.total;
    endCount = tail.total;
    outline.value = o;
    spans.value = joinSpans([
      { msgs: head.messages, atStart: true, atEnd: head.messages.length < PAGE },
      { msgs: tail.messages, atStart: tail.messages.length < PAGE, atEnd: true },
    ]);
  } catch (e) {
    if (openID === id && (e as Error).name !== "AbortError") convError.value = (e as Error).message;
  } finally {
    if (openID === id) opening.value = false;
  }
}

export function close(): void {
  openAbort?.abort();
  openID = null;
}

/** Reads on after span i, toward the next. */
export async function readOn(i: number): Promise<void> {
  const id = openID;
  const s = spans.value[i];
  if (!id || !s || !last(s)) return;
  const r = await api.after(id, last(s)!, PAGE);
  if (openID !== id) return;
  const list = spans.value.map((x) => ({ ...x }));
  list[i] = { ...list[i], msgs: [...list[i].msgs, ...r.messages], atEnd: list[i].atEnd || r.messages.length < PAGE };
  spans.value = joinSpans(list);
}

/** Makes sure the message at cursor is read, with some before it. */
export async function readAround(cursor: string): Promise<void> {
  const id = openID;
  if (!id || has(cursor)) return;
  const before = await api.before(id, cursor, 20);
  const from = before.last ?? "start";
  const after = await api.after(id, from, PAGE);
  if (openID !== id) return;
  spans.value = joinSpans([...spans.value, {
    msgs: [...before.messages, ...after.messages], atStart: !before.last, atEnd: after.messages.length < PAGE,
  }]);
}

export function has(cursor: string): boolean {
  return spans.value.some((s) => s.msgs.some((m) => m.cursor === cursor));
}

let following: Promise<void> | null = null;
let followAgain: { id: string; atBottom: () => boolean } | null = null;

/**
 * Adds what came in since the end span's last message. If the messages up
 * to it are not there as they were (the transcript was rewritten), the
 * session is opened again. Events come in bursts while a session is being
 * written: one read runs at a time, and those asked for meanwhile become
 * one more read after it, so no message is added twice. atBottom tells,
 * when the read is done, whether the reader is at the end.
 */
export function follow(id: string, atBottom: () => boolean): Promise<void> {
  if (following) {
    followAgain = { id, atBottom };
    return following;
  }
  following = followOnce(id, atBottom).finally(() => {
    following = null;
    const next = followAgain;
    followAgain = null;
    if (next) void follow(next.id, next.atBottom);
  });
  return following;
}

async function followOnce(id: string, atBottom: () => boolean): Promise<void> {
  if (openID !== id) return;
  const list = spans.value;
  const end = list[list.length - 1];
  const at = end?.msgs[end.msgs.length - 1];
  if (!end?.atEnd || !at) return void open(id);
  const r = await api.after(id, at.cursor, FOLLOW, { uuid: at.uuid, known: endCount });
  if (openID !== id) return;
  if (r.stale) return void open(id);
  if (r.messages.length === 0) return;
  // More came in than one read holds, as while the tab was hidden: read the
  // start and the end again, and leave what lies between to read on.
  if (r.messages.length >= FOLLOW) return void open(id);
  const bottom = atBottom();
  endCount += r.messages.length;
  total.value = r.total;
  const next = list.map((x) => ({ ...x }));
  next[next.length - 1].msgs = [...end.msgs, ...r.messages];
  spans.value = next;
  const prompts = r.messages.filter((m) => m.role === "user" && m.type === "text" && !m.content.startsWith("<"));
  if (prompts.length) {
    outline.value = [...outline.value, ...prompts.map((m) => ({ cursor: m.cursor, timestamp: m.timestamp, text: m.content.slice(0, 200) }))];
  }
  if (bottom) stickToEnd.value = true;
  else unseen.value += r.messages.length;
  const h = await api.header(id).catch(() => null);
  if (h && openID === id) header.value = h;
}

/** Reads the whole of a clipped message. */
export async function readWhole(cursor: string): Promise<void> {
  const id = openID;
  if (!id || whole.value[cursor]) return;
  const m = await api.message(id, cursor);
  if (openID === id) whole.value = { ...whole.value, [cursor]: m };
}

export async function loadDetail(): Promise<void> {
  const id = openID;
  if (!id || detail.value) return;
  const d = await api.detail(id).catch(() => null);
  if (openID === id) detail.value = d;
}

let findSeq = 0;

/** Searches the open conversation, all of it. */
export async function findInConversation(q: string): Promise<void> {
  const id = openID;
  findQuery.value = q;
  const seq = ++findSeq;
  if (!id || !q.trim()) {
    hits.value = [];
    return;
  }
  const r = await api.findIn(id, q).catch(() => []);
  if (seq !== findSeq) return;
  hits.value = r;
  hitIndex.value = 0;
}

/** Test-only. */
export function __resetForTests(): void {
  following = null;
  followAgain = null;
  openID = null;
  spans.value = [];
  endCount = 0;
}
