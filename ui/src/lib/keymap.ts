// Every key of the page, in one table: the key handling, the footer and
// the ? key list all read it, as the TUI's do its keymap. A new key is a
// line here and a case in the action handler (App's runAction).

import { t } from "./i18n";

export type Pane = "folders" | "list" | "reader";
export type Where = Pane | "any";

export type Action =
  | "search" | "focusNext" | "focusPrev" | "folders" | "sort" | "back" | "help"
  | "down" | "up" | "top" | "bottom" | "open" | "copyId" | "copyResume"
  | "scrollDown" | "scrollUp" | "prevPrompt" | "nextPrompt" | "nextHit" | "prevHit" | "density" | "tabConv" | "tabDone"
  | "pickFolder";

export interface Binding {
  where: Where;
  /** KeyboardEvent.key values. */
  keys: string[];
  action: Action;
  /** What the footer and the key list say; the footer shows it when footer is set. */
  label: string;
  footer?: boolean;
}

export const KEYMAP: Binding[] = [
  { where: "any", keys: ["/"], action: "search", label: t.keys.search },
  { where: "any", keys: ["Tab"], action: "focusNext", label: t.keys.focusNext },
  { where: "any", keys: ["S-Tab"], action: "focusPrev", label: t.keys.focusPrev },
  { where: "any", keys: ["["], action: "folders", label: t.keys.folders },
  { where: "any", keys: ["s"], action: "sort", label: t.keys.sort },
  { where: "any", keys: ["y"], action: "copyId", label: t.keys.copyId },
  { where: "any", keys: ["Y"], action: "copyResume", label: t.keys.copyResume },
  { where: "any", keys: ["Escape"], action: "back", label: t.keys.back },
  { where: "any", keys: ["?"], action: "help", label: t.keys.help },

  { where: "list", keys: ["j", "ArrowDown"], action: "down", label: t.keys.move, footer: true },
  { where: "list", keys: ["k", "ArrowUp"], action: "up", label: t.keys.move },
  { where: "list", keys: ["g"], action: "top", label: t.keys.top },
  { where: "list", keys: ["G"], action: "bottom", label: t.keys.bottom },
  { where: "list", keys: ["Enter"], action: "open", label: t.keys.open, footer: true },
  { where: "list", keys: ["/"], action: "search", label: t.keys.searchShort, footer: true },
  { where: "list", keys: ["s"], action: "sort", label: t.keys.sort, footer: true },
  { where: "list", keys: ["y"], action: "copyId", label: t.keys.id, footer: true },
  { where: "list", keys: ["Y"], action: "copyResume", label: t.keys.resume, footer: true },
  { where: "list", keys: ["["], action: "folders", label: t.keys.foldersShort, footer: true },

  { where: "reader", keys: ["j", "ArrowDown"], action: "scrollDown", label: t.keys.scroll, footer: true },
  { where: "reader", keys: ["k", "ArrowUp"], action: "scrollUp", label: t.keys.scroll },
  { where: "reader", keys: ["g"], action: "top", label: t.keys.convTop },
  { where: "reader", keys: ["G"], action: "bottom", label: t.keys.convEnd },
  { where: "reader", keys: ["["], action: "prevPrompt", label: t.keys.prompts, footer: true },
  { where: "reader", keys: ["]"], action: "nextPrompt", label: t.keys.nextPrompt },
  { where: "reader", keys: ["/"], action: "search", label: t.keys.searchConv, footer: true },
  { where: "reader", keys: ["n"], action: "nextHit", label: t.keys.hits, footer: true },
  { where: "reader", keys: ["N"], action: "prevHit", label: t.keys.prevHit },
  { where: "reader", keys: ["t"], action: "density", label: t.keys.density, footer: true },
  { where: "reader", keys: ["1"], action: "tabConv", label: t.keys.tabs, footer: true },
  { where: "reader", keys: ["2"], action: "tabDone", label: t.keys.tabDone },
  { where: "reader", keys: ["Escape"], action: "back", label: t.keys.toList, footer: true },

  { where: "folders", keys: ["j", "ArrowDown"], action: "down", label: t.keys.move, footer: true },
  { where: "folders", keys: ["k", "ArrowUp"], action: "up", label: t.keys.move },
  { where: "folders", keys: ["Enter"], action: "pickFolder", label: t.keys.pickFolder, footer: true },
  { where: "folders", keys: ["Escape"], action: "back", label: t.keys.toList, footer: true },
];

/** The ? key list's groups, the pane's own first. */
export const HELP_GROUPS: { where: Where; title: string }[] = [
  { where: "any", title: t.keyGroups.any },
  { where: "list", title: t.keyGroups.list },
  { where: "reader", title: t.keyGroups.reader },
  { where: "folders", title: t.keyGroups.folders },
];

/** The key a keydown event names in the table: "S-Tab" for shift+tab. */
export function eventKey(e: KeyboardEvent): string {
  return e.key === "Tab" && e.shiftKey ? "S-Tab" : e.key;
}

/** The action a key does in a pane: the pane's own binding first, then "any". */
export function lookup(pane: Pane, key: string): Action | undefined {
  return (KEYMAP.find((b) => b.where === pane && b.keys.includes(key)) ??
    KEYMAP.find((b) => b.where === "any" && b.keys.includes(key)))?.action;
}

/** What the footer shows in a pane: its keys, joined as the TUI's footer joins them. */
export function footer(pane: Pane): { keys: string; label: string }[] {
  const out: { keys: string; label: string }[] = [];
  for (const b of KEYMAP) {
    if (b.where !== pane || !b.footer) continue;
    // Bindings with the same label read as one: "j k Move".
    const pair = KEYMAP.find((o) => o !== b && o.where === pane && !o.footer && isPair(b, o));
    out.push({ keys: [b.keys[0], pair?.keys[0]].filter(Boolean).join(" ").replace("Escape", "Esc"), label: b.label });
  }
  return out;
}

const PAIRS: [Action, Action][] = [["down", "up"], ["scrollDown", "scrollUp"], ["nextPrompt", "prevPrompt"], ["nextHit", "prevHit"], ["tabConv", "tabDone"]];

function isPair(a: Binding, b: Binding): boolean {
  return PAIRS.some(([x, y]) => (a.action === x && b.action === y) || (a.action === y && b.action === x));
}

/** How a key shows in the key list. */
export function keyName(k: string): string {
  return { Escape: "Esc", ArrowDown: "↓", ArrowUp: "↑", "S-Tab": "Shift+Tab" }[k] ?? k;
}
