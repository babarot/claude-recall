import { useEffect, useRef, useState } from "preact/hooks";
import {
  bodyWords, folder, folders, found, isLive, loadError, loaded, now, query, searching, selected, sort, visible, worktree,
} from "../store/app";
import type { Sort } from "../store/app";
import type { Session } from "../api";
import { dayGroup, relative } from "../lib/format";
import { highlight } from "../lib/highlight";
import { t } from "../lib/i18n";

export const SORTS: { key: Sort; label: string }[] = [
  { key: "ended", label: t.sorts.ended },
  { key: "started", label: t.sorts.started },
  { key: "messages", label: t.sorts.messages },
  { key: "size", label: t.sorts.size },
];

export function Spark({ data }: { data: number[] }) {
  if (data.length < 2) return null;
  const w = 48, h = 14, max = Math.max(...data, 1);
  const pts = data.map((v, i) => `${(i / (data.length - 1)) * w},${h - (v / max) * (h - 2) - 1}`).join(" ");
  return (
    <svg class="spark" width={w} height={h} viewBox={`0 0 ${w} ${h}`} aria-hidden="true">
      <polyline points={pts} fill="none" stroke="var(--title)" stroke-opacity=".7" stroke-width="1.3" stroke-linejoin="round" />
    </svg>
  );
}

export function SessionList({ focused, onPick, sortOpen, setSortOpen }: {
  focused: boolean;
  /** A session was clicked. */
  onPick: (id: string) => void;
  sortOpen: boolean;
  setSortOpen: (v: boolean) => void;
}) {
  const list = visible.value;
  const scroller = useRef<HTMLDivElement>(null);
  const [, rerender] = useState(0);

  // Keep the selected row in view as the keys move it.
  useEffect(() => {
    scroller.current?.querySelector(".s-row.sel")?.scrollIntoView({ block: "nearest" });
  }, [selected.value]);
  useEffect(() => {
    const t = setInterval(() => rerender((n) => n + 1), 30000);
    return () => clearInterval(t);
  }, []);

  const f = found.value;
  const words = bodyWords(query.value);
  const where = folders.value.find((x) => x.key === folder.value)?.name;
  const scoped = !(f && f.in);
  let lastDay = "";
  return (
    <section class={`pane list ${focused ? "focused" : ""}`}>
      <div class="pane-head" style="position:relative">
        <span class="label">{scoped && where ? (worktree.value ? `${where} · ${worktree.value}` : where) : t.list.allFolders}</span>
        <span>{list.length}</span>
        <button class="sortbtn" onClick={(e) => { e.stopPropagation(); setSortOpen(!sortOpen); }}>
          {t.list.sortBy(SORTS.find((s) => s.key === sort.value)?.label ?? "")} ▾
        </button>
        <div class={`menu ${sortOpen ? "show" : ""}`}>
          {SORTS.map((s, i) => (
            <button key={s.key} class={s.key === sort.value ? "cur" : ""} onClick={() => { sort.value = s.key; setSortOpen(false); }}>
              <span class="k">{i + 1}</span>{s.label}
            </button>
          ))}
        </div>
      </div>
      <div class="scroll" ref={scroller}>
        {query.value && (
          <div class="result-head">
            {t.list.matchBefore}<span class="chip">{query.value}</span>{t.list.matchAfter}{t.list.sessions(list.length)}
            {searching.value && <span>{t.list.searching}</span>}
            <span style="margin-left:auto"><kbd>Esc</kbd>{t.list.escAfter}</span>
          </div>
        )}
        {!loaded.value && <div class="list-empty">{t.list.loading}</div>}
        {loadError.value && <div class="list-empty">{t.list.loadFailed(loadError.value)}</div>}
        {loaded.value && !loadError.value && list.length === 0 && (
          <div class="list-empty">
            {t.list.noMatch}
            {folder.value && scoped && <><br /><small>{t.list.noMatchHint}</small></>}
          </div>
        )}
        {list.flatMap((s) => {
          const day = sort.value === "ended" && !query.value ? dayGroup(s.endedAt, now.value) : "";
          const out = [];
          if (day && day !== lastDay) out.push(<div class="day" key={"d" + day}>{day}</div>);
          if (day) lastDay = day;
          out.push(<Row key={s.id} s={s} words={words} snippet={f?.snippets?.[s.id]?.text} onPick={onPick} />);
          return out;
        })}
      </div>
    </section>
  );
}

function Row({ s, words, snippet, onPick }: { s: Session; words: string[]; snippet?: string; onPick: (id: string) => void }) {
  return (
    <div class={`s-row ${s.id === selected.value ? "sel" : ""}`} onClick={() => onPick(s.id)}>
      <div class="s-top">
        <span class="s-title" dangerouslySetInnerHTML={{ __html: highlight(s.title, words) }} />
        {isLive(s.id) && <span class="badge live"><span class="dot pulse" />live</span>}
        <span class="s-time" title={s.endedAt ?? ""}>{relative(s.endedAt, now.value)}</span>
      </div>
      <div class="s-meta">
        <span class={`repo ${s.gone && !s.worktree ? "gone" : ""}`} title={s.groupName}>{s.folder}</span>
        {s.worktree
          ? <><span class="sep">·</span><span class="wt">{s.worktree}</span></>
          : s.branch && <><span class="sep">·</span><span>{s.branch}</span></>}
        {s.recallOnly && <span class="badge recall" title={t.list.recallOnly}>recall</span>}
        <span class="num"><Spark data={s.activity} /><span>{s.messages.toLocaleString()}</span></span>
      </div>
      {snippet && <div class="s-snip" dangerouslySetInnerHTML={{ __html: highlight(snippet, words) }} />}
    </div>
  );
}
