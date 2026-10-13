import { computed, signal } from "@preact/signals";
import { api } from "../api";
import type { Find, Session } from "../api";
import { buildFolders } from "../lib/folders";
import type { Pane } from "../lib/keymap";

// The page's state: the sessions, where the list is narrowed to, the
// search, which session is open, and how things show. Components read the
// signals and call the functions below.

export type Sort = "ended" | "started" | "messages" | "size";
export type Theme = "auto" | "dark" | "light";
export type Density = "talk" | "all";
export type Tab = "conv" | "done";

export const sessions = signal<Session[]>([]);
export const loaded = signal(false);
export const loadError = signal<string | null>(null);

/** The folder (a groupKey) and worktree the list is narrowed to. */
export const folder = signal<string | null>(null);
export const worktree = signal<string | null>(null);
export const sort = signal<Sort>("ended");

export const query = signal("");
/** What the server found for query; null while there is no query. */
export const found = signal<Find | null>(null);
/** The conversations are still being searched. */
export const searching = signal(false);

export const selected = signal<string | null>(null);
export const focus = signal<Pane>("list");
export const tab = signal<Tab>("conv");
export const showFolders = signal(true);
export const theme = signal<Theme>("auto");
/** The TUI's scheme is one of its own: the page is drawn in it, with no switch. */
export const fixedTheme = signal<string | null>(null);
export const density = signal<Density>("talk");
/** On a narrow screen, the conversation is shown in place of the list. */
export const reading = signal(false);

/** When each session last had an event, in ms: it is live for a while after. */
export const lastEvent = signal<Record<string, number>>({});
export const LIVE_FOR = 2 * 60 * 1000;
export const now = signal(new Date());

export const toast = signal<{ html: string; at: number } | null>(null);

export const folders = computed(() => buildFolders(sessions.value));

const sorts: Record<Sort, (a: Session, b: Session) => number> = {
  ended: (a, b) => (b.endedAt ?? "").localeCompare(a.endedAt ?? ""),
  started: (a, b) => (b.startedAt ?? "").localeCompare(a.startedAt ?? ""),
  messages: (a, b) => b.messages - a.messages,
  size: (a, b) => b.size - a.size,
};

/** The sessions the list shows, in order. */
export const visible = computed(() => {
  const f = found.value;
  const ids = f ? new Set(f.ids) : null;
  // folder: in the query picks the folders itself, as in the TUI.
  const scoped = !(f && f.in);
  const list = sessions.value.filter((s) => {
    if (ids && !ids.has(s.id)) return false;
    if (scoped && folder.value && s.groupKey !== folder.value) return false;
    if (scoped && worktree.value && s.worktree !== worktree.value) return false;
    return true;
  });
  return list.sort(sorts[sort.value]);
});

export const current = computed(() => sessions.value.find((s) => s.id === selected.value) ?? null);

export function isLive(id: string): boolean {
  const at = lastEvent.value[id];
  return at !== undefined && now.value.getTime() - at < LIVE_FOR;
}

// --- settings kept in the browser ---------------------------------------

const KEY = "recall-web";

export function loadSettings(): void {
  try {
    const s = JSON.parse(localStorage.getItem(KEY) ?? "{}");
    if (["auto", "dark", "light"].includes(s.theme)) theme.value = s.theme;
    if (["talk", "all"].includes(s.density)) density.value = s.density;
    if (s.sort in sorts) sort.value = s.sort;
    if (typeof s.showFolders === "boolean") showFolders.value = s.showFolders;
  } catch {
    // private window, or nothing saved
  }
}

export function saveSettings(): void {
  try {
    localStorage.setItem(KEY, JSON.stringify({
      theme: theme.value, density: density.value, sort: sort.value, showFolders: showFolders.value,
    }));
  } catch {
    // not kept; the defaults come back next time
  }
}

// --- loading -------------------------------------------------------------

export async function loadSessions(): Promise<void> {
  try {
    sessions.value = await api.sessions();
    loadError.value = null;
  } catch (e) {
    loadError.value = String((e as Error).message ?? e);
  } finally {
    loaded.value = true;
  }
}

/** Reads one session again after an event, adding it if it is new. */
export async function refreshSession(id: string): Promise<void> {
  lastEvent.value = { ...lastEvent.value, [id]: Date.now() };
  const s = await api.session(id).catch(() => null);
  if (!s) return;
  const list = sessions.value.filter((x) => x.id !== id);
  sessions.value = [s, ...list];
  // A search's result set stays as it was found; a new session in it
  // shows the next time the query changes.
}

// --- search --------------------------------------------------------------

let searchSeq = 0;
let searchAbort: AbortController | null = null;
let searchTimer: ReturnType<typeof setTimeout> | undefined;

/**
 * Sets the query and searches: the fields first, which is quick, then the
 * conversations too. A later query cancels an earlier one.
 */
export function setQuery(q: string, delay = 150): void {
  query.value = q;
  clearTimeout(searchTimer);
  searchAbort?.abort();
  if (!q.trim()) {
    found.value = null;
    searching.value = false;
    return;
  }
  searchTimer = setTimeout(() => void runSearch(q), delay);
}

async function runSearch(q: string): Promise<void> {
  const seq = ++searchSeq;
  const ctl = new AbortController();
  searchAbort = ctl;
  searching.value = true;
  try {
    const quick = await api.find(q, false, ctl.signal);
    if (seq !== searchSeq) return;
    found.value = quick;
    const full = await api.find(q, true, ctl.signal);
    if (seq !== searchSeq) return;
    found.value = full;
  } catch {
    // canceled by a later query, or failed: what was found stays
  } finally {
    if (seq === searchSeq) searching.value = false;
  }
}

/** The words of the query a conversation is searched for: the plain and text: words. */
export function bodyWords(q: string): string[] {
  const out: string[] = [];
  for (const w of q.toLowerCase().split(/\s+/).filter(Boolean)) {
    const m = w.match(/^([a-z]+):(.*)$/);
    if (m && ["folder", "in", "title", "branch", "worktree", "id"].includes(m[1])) continue;
    if (m && m[1] === "text") {
      if (m[2]) out.push(m[2]);
      continue;
    }
    if ([...w].length >= 2) out.push(w);
  }
  return out;
}

// --- toast ---------------------------------------------------------------

export function say(html: string): void {
  toast.value = { html, at: Date.now() };
}

export async function copy(text: string, what: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text);
    say(`${what}をコピーしました：<code>${escapeHTML(text)}</code>`);
  } catch {
    say(`${what}をコピーできませんでした`);
  }
}

export function escapeHTML(s: string): string {
  return s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]!));
}

/** Test-only: put every signal back. */
export function __resetForTests(): void {
  sessions.value = [];
  loaded.value = false;
  loadError.value = null;
  folder.value = worktree.value = selected.value = null;
  sort.value = "ended";
  query.value = "";
  found.value = null;
  searching.value = false;
  lastEvent.value = {};
  searchSeq++;
  clearTimeout(searchTimer);
}
