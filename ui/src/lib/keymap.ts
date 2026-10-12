// Every key of the page, in one table: the key handling, the footer and
// the ? key list all read it, as the TUI's do its keymap. A new key is a
// line here and a case in the action handler (App's runAction).

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
  { where: "any", keys: ["/"], action: "search", label: "セッションを検索（会話を読んでいるときはその会話を検索）" },
  { where: "any", keys: ["Tab"], action: "focusNext", label: "フォルダ・一覧・会話のあいだでフォーカスを移す" },
  { where: "any", keys: ["S-Tab"], action: "focusPrev", label: "フォーカスを逆に移す" },
  { where: "any", keys: ["["], action: "folders", label: "フォルダ一覧を出す・しまう" },
  { where: "any", keys: ["s"], action: "sort", label: "並べ替え" },
  { where: "any", keys: ["y"], action: "copyId", label: "セッションIDをコピー" },
  { where: "any", keys: ["Y"], action: "copyResume", label: "再開コマンドをコピー" },
  { where: "any", keys: ["Escape"], action: "back", label: "一歩戻る：検索を消す、一覧に戻る" },
  { where: "any", keys: ["?"], action: "help", label: "この一覧" },

  { where: "list", keys: ["j", "ArrowDown"], action: "down", label: "移動", footer: true },
  { where: "list", keys: ["k", "ArrowUp"], action: "up", label: "移動" },
  { where: "list", keys: ["g"], action: "top", label: "先頭へ" },
  { where: "list", keys: ["G"], action: "bottom", label: "末尾へ" },
  { where: "list", keys: ["Enter"], action: "open", label: "会話へ", footer: true },
  { where: "list", keys: ["/"], action: "search", label: "検索", footer: true },
  { where: "list", keys: ["s"], action: "sort", label: "並べ替え", footer: true },
  { where: "list", keys: ["y"], action: "copyId", label: "ID", footer: true },
  { where: "list", keys: ["Y"], action: "copyResume", label: "再開コマンド", footer: true },
  { where: "list", keys: ["["], action: "folders", label: "フォルダ", footer: true },

  { where: "reader", keys: ["j", "ArrowDown"], action: "scrollDown", label: "スクロール", footer: true },
  { where: "reader", keys: ["k", "ArrowUp"], action: "scrollUp", label: "スクロール" },
  { where: "reader", keys: ["["], action: "prevPrompt", label: "前・次の発言", footer: true },
  { where: "reader", keys: ["]"], action: "nextPrompt", label: "次の自分の発言へ" },
  { where: "reader", keys: ["/"], action: "search", label: "この会話を検索", footer: true },
  { where: "reader", keys: ["n"], action: "nextHit", label: "次・前の一致", footer: true },
  { where: "reader", keys: ["N"], action: "prevHit", label: "前の一致へ" },
  { where: "reader", keys: ["t"], action: "density", label: "ツールの見せ方", footer: true },
  { where: "reader", keys: ["1"], action: "tabConv", label: "会話・やったことのタブ", footer: true },
  { where: "reader", keys: ["2"], action: "tabDone", label: "やったことのタブ" },
  { where: "reader", keys: ["Escape"], action: "back", label: "一覧へ", footer: true },

  { where: "folders", keys: ["j", "ArrowDown"], action: "down", label: "移動", footer: true },
  { where: "folders", keys: ["k", "ArrowUp"], action: "up", label: "移動" },
  { where: "folders", keys: ["Enter"], action: "pickFolder", label: "絞り込んで一覧へ", footer: true },
  { where: "folders", keys: ["Escape"], action: "back", label: "一覧へ", footer: true },
];

/** The ? key list's groups, the pane's own first. */
export const HELP_GROUPS: { where: Where; title: string }[] = [
  { where: "any", title: "どこでも" },
  { where: "list", title: "一覧" },
  { where: "reader", title: "会話" },
  { where: "folders", title: "フォルダ" },
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
    // Bindings with the same label read as one: "j k 移動".
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
