import { useEffect, useRef } from "preact/hooks";
import { copy, current, density, isLive, reading, saveSettings, selected, tab } from "../store/app";
import {
  convError, detail, findInConversation, findQuery, hitIndex, hits, header, loadDetail, opening,
} from "../store/conversation";
import { Conversation, goTo } from "./Conversation";
import { dateTime, hhmm, size } from "../lib/format";
import { toolLabel } from "../lib/items";
import { t } from "../lib/i18n";

export const findInput: { current: HTMLInputElement | null } = { current: null };

export function copyResume(): void {
  const h = header.value;
  if (h && h.id === selected.value) void copy(h.resume.command, t.copy.resume);
}

export function copyID(): void {
  if (selected.value) void copy(selected.value, t.copy.id);
}

const REASONS = t.reader.reasons;

export function Reader({ focused }: { focused: boolean }) {
  const s = current.value;
  const h = header.value?.id === s?.id ? header.value : null;
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => {
    findInput.current = ref.current;
  }, []);
  useEffect(() => {
    if (tab.value === "done") void loadDetail();
  }, [tab.value, s?.id]);

  if (!s) {
    return (
      <section class={`pane reader ${focused ? "focused" : ""}`}>
        <div class="list-empty">{t.reader.empty}</div>
      </section>
    );
  }
  return (
    <section class={`pane reader ${focused ? "focused" : ""}`}>
      <div class="r-head">
        <div class="r-titlebar">
          <button class="btn back" onClick={() => { reading.value = false; }}>{t.reader.back}</button>
          <h1 class="r-title">{s.title}</h1>
          <div class="r-actions">
            <button class="btn primary" onClick={copyResume} disabled={!h}
              title={h ? (h.resume.resumable ? h.resume.command : REASONS[h.resume.reason ?? ""] ?? "") : ""}>
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
              <span class="lbl">{h && !h.resume.resumable ? t.reader.recallCommand : t.reader.resumeCommand}</span><kbd>Y</kbd>
            </button>
            <button class="btn" onClick={copyID}><span class="lbl">{t.reader.copyId}</span><kbd>y</kbd></button>
          </div>
        </div>
        <div class="r-meta">
          <span class="repo" title={s.groupName}>{s.folder}</span>
          {s.worktree && <span class={`wt ${s.gone ? "gone" : ""}`}>{s.worktree}</span>}
          {s.branch && <span>⎇ {s.branch}</span>}
          <span title={t.reader.span}>{dateTime(s.startedAt)} → {hhmm(s.endedAt)}</span>
          <span>{t.reader.messages(s.messages)} · {size(s.size)}</span>
          <span class="id mono" title={t.reader.clickToCopy} onClick={copyID}>{s.id.slice(0, 8)}</span>
          {isLive(s.id) && <span class="live-note"><span class="dot pulse" />{t.reader.writing}</span>}
        </div>
        <div class="tabs">
          <button class={`tab ${tab.value === "conv" ? "on" : ""}`} onClick={() => { tab.value = "conv"; }}>
            {t.reader.conv}<span class="n">{s.messages.toLocaleString()}</span>
          </button>
          <button class={`tab ${tab.value === "done" ? "on" : ""}`} onClick={() => { tab.value = "done"; }}>{t.reader.done}</button>
          <div class="r-tools" style={{ visibility: tab.value === "conv" ? "visible" : "hidden" }}>
            <div class="seg" title={t.reader.densityTitle}>
              <button class={density.value === "talk" ? "on" : ""} onClick={() => { density.value = "talk"; saveSettings(); }}>{t.reader.talk}</button>
              <button class={density.value === "all" ? "on" : ""} onClick={() => { density.value = "all"; saveSettings(); }}>{t.reader.all}</button>
            </div>
            <label class="find">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
              <input ref={ref} value={findQuery.value} placeholder={t.reader.find}
                onInput={(e) => void findInConversation((e.target as HTMLInputElement).value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    stepHit(e.shiftKey ? -1 : 1);
                  } else if (e.key === "Escape") {
                    e.stopPropagation();
                    void findInConversation("");
                    (e.target as HTMLInputElement).blur();
                  }
                }} />
              <span class="count">{findQuery.value ? (hits.value.length ? `${hitIndex.value + 1}/${hits.value.length}` : "0") : ""}</span>
            </label>
          </div>
        </div>
      </div>
      {tab.value === "conv"
        ? (convError.value
          ? <div class="list-empty">{t.reader.convFailed(convError.value)}</div>
          : opening.value && !h ? <div class="list-empty">{t.reader.loading}</div> : <Conversation sessionId={s.id} />)
        : <Done />}
    </section>
  );
}

/** Moves to the next (1) or previous (-1) hit of the conversation search. */
export function stepHit(d: number): void {
  const n = hits.value.length;
  if (!n) return;
  hitIndex.value = (hitIndex.value + d + n) % n;
  void goTo(hits.value[hitIndex.value].cursor);
}

function Bars({ counts, cls }: { counts: { name: string; n: number }[]; cls?: string }) {
  const top = counts.slice(0, 8);
  const max = Math.max(1, ...top.map((c) => c.n));
  if (top.length === 0) return <div class="bar-row"><span class="k">{t.done.none}</span></div>;
  return (
    <>
      {top.map((c) => (
        <div class="bar-row" key={c.name}>
          <span class="k mono" title={c.name}>{toolLabel(c.name)}</span>
          <div><div class={`bar ${cls ?? ""}`} style={{ width: `${(c.n / max) * 100}%` }} /></div>
          <span class="v">{c.n}</span>
        </div>
      ))}
    </>
  );
}

function Done() {
  const s = current.value!;
  const h = header.value?.id === s.id ? header.value : null;
  const d = detail.value;
  if (!d) return <div class="done"><div class="list-empty">{t.reader.loading}</div></div>;
  return (
    <div class="done">
      <div class="done-grid">
        <div class="card" style="grid-column:1/-1">
          <h3>Activity</h3>
          <div class="activity">
            {d.activity.map((v, i) => <i key={i} style={{ height: `${(v / Math.max(1, ...d.activity)) * 100}%` }} />)}
          </div>
          <div class="axis"><span>{dateTime(s.startedAt)}</span><span>{dateTime(s.endedAt)}</span></div>
        </div>
        <div class="card"><h3>{t.done.tools}</h3><Bars counts={d.topTools} /></div>
        <div class="card"><h3>{t.done.commands(d.commands)}</h3><Bars counts={d.programs} cls="cmdb" /></div>
        <div class="card">
          <h3>{t.done.files(d.fileCount)}</h3>
          <div class="files">
            {d.fileCount === 0 && <div class="f"><span>{t.done.none}</span></div>}
            {d.groups.map((g) => (
              <div key={g.place + (g.worktree ?? "")}>
                <div class="repo">{g.place === "" ? t.done.thisFolder : g.worktree ? <>{g.place} <span class="wt">⌥{g.worktree}</span></> : g.place}</div>
                {g.files.map((f) => <div class="f" key={f.name}><span title={f.name}>{f.name}</span><span>{f.n > 1 ? `×${f.n}` : ""}</span></div>)}
              </div>
            ))}
            {d.temps > 0 && <div class="f"><span>{t.done.temps(d.temps)}</span></div>}
            {d.fileCount > d.groups.reduce((n, g) => n + g.files.length, 0) + d.temps && (
              <div class="f"><span>{t.done.more(d.fileCount - d.groups.reduce((n, g) => n + g.files.length, 0) - d.temps)}</span></div>
            )}
          </div>
        </div>
        <div class="card">
          <h3>Details</h3>
          <dl class="details">
            <dt>ID</dt><dd class="mono" style="color:var(--accent)">{s.id}</dd>
            <dt>{t.done.started}</dt><dd>{dateTime(s.startedAt)}</dd>
            <dt>{t.done.ended}</dt><dd>{dateTime(s.endedAt)}</dd>
            <dt>{t.done.messages}</dt><dd>{s.messages.toLocaleString()}{t.done.breakdown(d.you, d.claude, d.tools, d.thinking)}</dd>
            {d.images > 0 && <><dt>{t.done.images}</dt><dd>{d.images}</dd></>}
            <dt>{t.done.size}</dt><dd>{size(s.size)}</dd>
            <dt>{t.done.branch}</dt><dd>{s.branch || "-"}</dd>
            <dt>{t.done.folder}</dt><dd class="mono">{s.path}{s.gone ? t.done.gone : ""}</dd>
            {h?.claudeVersion && <><dt>Claude Code</dt><dd>{h.claudeVersion}</dd></>}
            {h && <><dt>{t.done.goBack}</dt><dd class="mono" title={h.resume.command}>{shownCommand(h.resume.command, h.fullPath, s.path)}</dd></>}
          </dl>
        </div>
      </div>
    </div>
  );
}

/** The command as the details show it: the folder as ~, cut when long. */
function shownCommand(cmd: string, fullPath: string, path: string): string {
  const short = fullPath && path ? cmd.split(fullPath).join(path) : cmd;
  return short.length > 200 ? short.slice(0, 200) + "…" : short;
}
