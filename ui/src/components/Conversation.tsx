import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "preact/hooks";
import { signal } from "@preact/signals";
import { density, isLive } from "../store/app";
import {
  compareCursors, findQuery, has, header, hitIndex, hits, outline, readAround, readOn, readWhole, spans, stickToEnd, total, unseen,
  whole,
} from "../store/conversation";
import type { Span } from "../store/conversation";
import { buildItems, failedResult, fromAPI, relativeTo, tally, toolLabel } from "../lib/items";
import type { Call, Item } from "../lib/items";
import { getToolInputPreview, renderImages, stripAnsi } from "../lib/chat-utils";
import { renderMarkdown } from "../lib/markdown";
import { ansiToHtml, hasAnsi } from "../lib/ansi";
import { duration, hhmm } from "../lib/format";

/** The scrolling conversation, for the keys and the live updates to reach. */
export const conversationEl: { current: HTMLDivElement | null } = { current: null };
/** The prompt (its cursor) the reader is at, as the outline marks it. */
export const currentPrompt = signal<string | null>(null);
/** A message to bring into view once it is drawn. */
export const goal = signal<string | null>(null);

const CLAMP = 1000;

export function atBottom(): boolean {
  const el = conversationEl.current;
  return !!el && el.scrollHeight - el.scrollTop - el.clientHeight < 80;
}

/** Brings the message at cursor into view, reading it first if it is not. */
export async function goTo(cursor: string): Promise<void> {
  if (!has(cursor)) await readAround(cursor);
  goal.value = cursor;
}

export function Conversation({ sessionId }: { sessionId: string }) {
  const list = spans.value;
  const scroller = useRef<HTMLDivElement>(null);
  const inner = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState<Set<string>>(new Set());
  const all = density.value === "all";

  useEffect(() => setOpen(new Set()), [density.value, sessionId]);
  useEffect(() => {
    conversationEl.current = scroller.current;
    return () => { conversationEl.current = null; };
  }, []);

  const toggle = (key: string) => setOpen((s) => {
    const n = new Set(s);
    if (n.has(key)) n.delete(key);
    else n.add(key);
    return n;
  });

  const items = useMemo(() => list.map((s) => buildItems(s.msgs.map(fromAPI))), [list]);

  useLayoutEffect(() => {
    const el = scroller.current;
    if (!el) return;
    if (stickToEnd.value) {
      el.scrollTop = el.scrollHeight;
      stickToEnd.value = false;
    }
  }, [list]);

  // Marks what the conversation search found, and brings the current hit
  // or a goal into view.
  useEffect(() => {
    const root = inner.current;
    if (!root) return;
    markWords(root, findQuery.value);
    const hit = hits.value[hitIndex.value];
    const target = goal.value ?? hit?.cursor;
    if (!target) return;
    const el = root.querySelector<HTMLElement>(`[data-cursor="${target}"]`);
    if (!el) return;
    const mark = el.querySelector("mark");
    root.querySelectorAll("mark.cur").forEach((m) => m.classList.remove("cur"));
    mark?.classList.add("cur");
    (mark ?? el).scrollIntoView({ block: goal.value && !mark ? "start" : "center" });
    goal.value = null;
  }, [list, findQuery.value, hits.value, hitIndex.value, goal.value, open]);

  const onScroll = () => {
    const el = scroller.current;
    if (!el) return;
    const prompts = el.querySelectorAll<HTMLElement>(".turn");
    let cur: string | null = prompts[0]?.dataset.cursor ?? null;
    for (const p of prompts) {
      if (p.offsetTop - el.offsetTop <= el.scrollTop + 40) cur = p.dataset.cursor ?? cur;
    }
    currentPrompt.value = cur;
    if (atBottom()) unseen.value = 0;
  };

  const loaded = list.reduce((n, s) => n + s.msgs.length, 0);
  const gaps = list.length - 1;
  return (
    <div class="r-body">
      <div class="conv" ref={scroller} onScroll={onScroll} onClick={(e) => {
        const t = e.target as HTMLElement;
        if (t.tagName === "IMG" && t.closest(".body")) zoom.value = (t as HTMLImageElement).src;
      }}>
        <div class="conv-inner" ref={inner}>
          {list.length === 0 && <div class="list-empty">メッセージはありません</div>}
          {items.map((its, i) => (
            <SpanView key={i} span={list[i]} items={its} sessionId={sessionId} all={all} open={open} toggle={toggle}
              gap={i < gaps ? (gaps === 1 ? total.value - loaded : null) : undefined} onReadOn={() => readOn(i)} />
          ))}
          {isLive(sessionId) && <div class="typing"><span class="dot pulse" />書き込み中</div>}
        </div>
      </div>
      <Outline />
      {unseen.value > 0 && (
        <button class="jump show" onClick={() => {
          const el = conversationEl.current;
          if (el) el.scrollTop = el.scrollHeight;
          unseen.value = 0;
        }}>新しいメッセージ {unseen.value}件 ↓</button>
      )}
    </div>
  );
}

export const zoom = signal<string | null>(null);

function SpanView({ span, items, sessionId, all, open, toggle, gap, onReadOn }: {
  span: Span;
  items: Item[];
  sessionId: string;
  all: boolean;
  open: Set<string>;
  toggle: (key: string) => void;
  /** After this span: undefined for none, a count, or null when it is not known. */
  gap: number | null | undefined;
  onReadOn: () => void;
}) {
  const [busy, setBusy] = useState(false);
  return (
    <>
      {items.map((it) => <ItemView key={it.key} it={it} sessionId={sessionId} open={all || open.has(it.key)} toggle={toggle} />)}
      {gap !== undefined && !span.atEnd && (
        <div class="gap-marker">
          ⋮ {gap !== null ? `${gap.toLocaleString()} 件のメッセージ` : "読み込んでいないメッセージ"}
          <button disabled={busy} onClick={async () => {
            setBusy(true);
            await onReadOn();
            setBusy(false);
          }}>{busy ? "読み込み中…" : "さらに読み込む"}</button>
        </div>
      )}
    </>
  );
}

function ItemView({ it, sessionId, open, toggle }: { it: Item; sessionId: string; open: boolean; toggle: (k: string) => void }) {
  switch (it.kind) {
    case "run":
      return <Run it={it} open={open} toggle={() => toggle(it.key)} />;
    case "thinking":
      return (
        <div class={`run ${open ? "open" : ""}`}>
          <button class="run-head" onClick={() => toggle(it.key)}><span class="chev">▶</span><span style="font-style:italic">考えていたこと</span></button>
          <div class="think-body">{it.content}</div>
        </div>
      );
    case "meta":
      return (
        <div class={`run meta ${open ? "open" : ""}`}>
          <button class="run-head" onClick={() => toggle(it.key)} title="Claude Code が差し込んだ内容（スキルの展開、文脈など）">
            <span class="chev">▶</span><span class="badge">meta</span><span>{it.label}</span>
          </button>
          <div class="meta-body">{it.content}</div>
        </div>
      );
  }
  const m = it.msg;
  if (m.type === "bash") {
    return (
      <div class="cmd">
        <div class="cl"><span>$ </span>{m.command}</div>
        {(m.stdout || m.stderr) && <pre>{stripAnsi(m.stdout || m.stderr)}</pre>}
      </div>
    );
  }
  if (m.type === "command") {
    return (
      <div class="cmd">
        <div class="cl">{m.name}{m.args ? ` ${m.args}` : ""}</div>
        {m.stdout && <pre>{stripAnsi(m.stdout)}</pre>}
      </div>
    );
  }
  return <Chat sessionId={sessionId} msg={m} />;
}

function Chat({ sessionId, msg }: { sessionId: string; msg: Extract<Item, { kind: "message" }>["msg"] & { type: "chat" } }) {
  const [expanded, setExpanded] = useState(false);
  const full = msg.cursor ? whole.value[msg.cursor] : undefined;
  const content = full?.content ?? msg.content;
  const clipped = !!msg.contentLength && !full;
  const user = msg.role === "user";
  const html = useMemo(() => renderMarkdown(renderImages(content, sessionId, msg.uuid)), [content, sessionId, msg.uuid]);
  const long = !user && content.length > CLAMP;
  const body = (
    <div class="body">
      <div class={long && !expanded ? "clamp" : ""} dangerouslySetInnerHTML={{ __html: html }} />
      {(long || clipped) && (
        <button class="more" onClick={() => {
          if (clipped && msg.cursor) void readWhole(msg.cursor);
          setExpanded(!expanded || clipped);
        }}>{long && expanded && !clipped ? "たたむ" : "続きを読む"}</button>
      )}
    </div>
  );
  const head = <div class="who">{user ? "You" : "Claude"}<small>{hhmm(msg.timestamp)}</small></div>;
  if (user) {
    return <div class="turn" data-cursor={msg.cursor}><div class="msg user">{head}{body}</div></div>;
  }
  return <div class="msg claude" data-cursor={msg.cursor}>{head}{body}</div>;
}

function Run({ it, open, toggle }: { it: Extract<Item, { kind: "run" }>; open: boolean; toggle: () => void }) {
  const { counts, failed } = tally(it.calls);
  return (
    <div class={`run ${open ? "open" : ""}`}>
      <button class="run-head" onClick={toggle}>
        <span class="chev">▶</span>
        <span>{it.calls.length} 件のツール呼び出し</span>
        <span class="tally">
          {counts.map(([n, c]) => <span key={n} title={n}><b>{toolLabel(n)}</b> {c}</span>)}
          {failed > 0 && <span class="fail">✗ {failed} 失敗</span>}
        </span>
        <span class="dur">{duration(it.start, it.end)}</span>
      </button>
      {open && <div class="run-body">{it.calls.map((c, i) => <CallView key={c.use?.cursor ?? c.result?.cursor ?? i} c={c} />)}</div>}
    </div>
  );
}

function CallView({ c }: { c: Call }) {
  const [open, setOpen] = useState(false);
  const input = c.use?.cursor ? whole.value[c.use.cursor]?.toolInput ?? c.use.toolInput : c.use?.toolInput ?? "";
  const result = c.result?.cursor ? whole.value[c.result.cursor]?.content ?? c.result.content : c.result?.content ?? "";
  const clipped = (!!c.use?.toolInputLength && !whole.value[c.use.cursor!]) || (!!c.result?.contentLength && !whole.value[c.result.cursor!]);
  const failed = c.result ? failedResult(c.result) : false;
  const preview = relativeTo(getToolInputPreview(c.use?.toolInput ?? "") || (c.use?.toolInput ?? "").slice(0, 120), header.value?.fullPath);
  const lines = result.split("\n").length;
  return (
    <div class={`call ${open ? "open" : ""}`}>
      <button class="call-head" onClick={() => setOpen(!open)}>
        <span class="chev">▶</span>
        <span class="call-name" title={c.use?.toolName ?? ""}>{c.use ? toolLabel(c.use.toolName) : "result"}</span>
        <span class="call-in">{preview}</span>
        <span class={`call-st ${failed ? "err" : ""}`}>{failed ? "失敗" : c.result ? (lines > 1 ? `${lines} 行` : "") : "結果なし"}</span>
      </button>
      {open && (
        <div class="call-body">
          {c.use && <><span class="lbl">input</span><pre>{pretty(input)}</pre></>}
          {c.result && <><span class="lbl">result</span><Result content={result} /></>}
          {clipped && (
            <button class="more" onClick={() => {
              if (c.use?.toolInputLength && c.use.cursor) void readWhole(c.use.cursor);
              if (c.result?.contentLength && c.result.cursor) void readWhole(c.result.cursor);
            }}>全文を読み込む</button>
          )}
        </div>
      )}
    </div>
  );
}

function Result({ content }: { content: string }) {
  if (hasAnsi(content)) return <pre dangerouslySetInnerHTML={{ __html: ansiToHtml(content) }} />;
  return <pre>{content}</pre>;
}

function pretty(input: string): string {
  try {
    return JSON.stringify(JSON.parse(input), null, 2);
  } catch {
    return input;
  }
}

function Outline() {
  const prompts = outline.value;
  const cur = currentPrompt.value;
  const hitPrompts = useMemo(() => {
    // The prompt each hit falls under.
    const out = new Set<string>();
    for (const h of hits.value) {
      let p: string | undefined;
      for (const o of prompts) {
        if (compareCursors(o.cursor, h.cursor) <= 0) p = o.cursor;
        else break;
      }
      if (p) out.add(p);
    }
    return out;
  }, [hits.value, prompts]);
  return (
    <nav class="outline">
      <div class="label">自分の発言</div>
      {prompts.map((p, i) => (
        <div key={p.cursor} class={`o-item ${p.cursor === cur ? "cur" : ""} ${hitPrompts.has(p.cursor) ? "hit" : ""}`}
          onClick={() => void goTo(p.cursor)} title={p.text}>
          <span class="t">{hhmm(p.timestamp) || `#${i + 1}`}</span>
          <span class="x">{p.text}</span>
        </div>
      ))}
    </nav>
  );
}

/**
 * Marks q in the messages' text under root, ignoring case. Only the
 * rendered Markdown is touched: Preact sets it as HTML and does not keep
 * the text nodes in it, so marking them does not upset what it draws.
 */
export function markWords(root: HTMLElement, q: string): void {
  // Unmarked, and joined again only inside the Markdown: normalizing the
  // rest would merge text nodes Preact holds, which it then updates no more.
  root.querySelectorAll("mark").forEach((m) => {
    const parent = m.parentElement;
    m.replaceWith(document.createTextNode(m.textContent ?? ""));
    parent?.normalize();
  });
  const word = q.trim();
  if (!word) return;
  const re = new RegExp(word.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), "gi");
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode: (n) => (n.parentElement?.closest(".body > div") ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT),
  });
  const nodes: Text[] = [];
  while (walker.nextNode()) nodes.push(walker.currentNode as Text);
  for (const n of nodes) {
    re.lastIndex = 0;
    if (!re.test(n.data)) continue;
    re.lastIndex = 0;
    const frag = document.createDocumentFragment();
    let at = 0;
    for (const m of n.data.matchAll(re)) {
      frag.append(n.data.slice(at, m.index));
      const mark = document.createElement("mark");
      mark.textContent = m[0];
      frag.append(mark);
      at = m.index! + m[0].length;
    }
    frag.append(n.data.slice(at));
    n.replaceWith(frag);
  }
}
