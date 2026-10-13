import { useEffect, useRef } from "preact/hooks";
import { copy, current, density, isLive, reading, saveSettings, selected, tab } from "../store/app";
import {
  convError, detail, findInConversation, findQuery, hitIndex, hits, header, loadDetail, opening,
} from "../store/conversation";
import { Conversation, goTo } from "./Conversation";
import { dateTime, hhmm, size } from "../lib/format";
import { toolLabel } from "../lib/items";

export const findInput: { current: HTMLInputElement | null } = { current: null };

export function copyResume(): void {
  const h = header.value;
  if (h && h.id === selected.value) void copy(h.resume.command, "再開コマンド");
}

export function copyID(): void {
  if (selected.value) void copy(selected.value, "セッションID");
}

const REASONS: Record<string, string> = {
  gone: "フォルダがもうないので、recall で読み直すコマンド",
  "other-tree": "transcript が claude -r の読まない場所にあるので、recall で読み直すコマンド",
  "no-transcript": "transcript が消えているので、recall で読み直すコマンド",
};

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
        <div class="list-empty">セッションを選ぶと、ここに会話が出ます</div>
      </section>
    );
  }
  return (
    <section class={`pane reader ${focused ? "focused" : ""}`}>
      <div class="r-head">
        <div class="r-titlebar">
          <button class="btn back" onClick={() => { reading.value = false; }}>← 一覧</button>
          <h1 class="r-title">{s.title}</h1>
          <div class="r-actions">
            <button class="btn primary" onClick={copyResume} disabled={!h}
              title={h ? (h.resume.resumable ? h.resume.command : REASONS[h.resume.reason ?? ""] ?? "") : ""}>
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
              <span class="lbl">{h && !h.resume.resumable ? "recall コマンド" : "再開コマンド"}</span><kbd>Y</kbd>
            </button>
            <button class="btn" onClick={copyID}><span class="lbl">IDをコピー</span><kbd>y</kbd></button>
          </div>
        </div>
        <div class="r-meta">
          <span class="repo" title={s.groupName}>{s.folder}</span>
          {s.worktree && <span class={`wt ${s.gone ? "gone" : ""}`}>{s.worktree}</span>}
          {s.branch && <span>⎇ {s.branch}</span>}
          <span title="開始 → 終了">{dateTime(s.startedAt)} → {hhmm(s.endedAt)}</span>
          <span>{s.messages.toLocaleString()} messages · {size(s.size)}</span>
          <span class="id mono" title="クリックでコピー" onClick={copyID}>{s.id.slice(0, 8)}</span>
          {isLive(s.id) && <span class="live-note"><span class="dot pulse" />書き込み中</span>}
        </div>
        <div class="tabs">
          <button class={`tab ${tab.value === "conv" ? "on" : ""}`} onClick={() => { tab.value = "conv"; }}>
            会話<span class="n">{s.messages.toLocaleString()}</span>
          </button>
          <button class={`tab ${tab.value === "done" ? "on" : ""}`} onClick={() => { tab.value = "done"; }}>やったこと</button>
          <div class="r-tools" style={{ visibility: tab.value === "conv" ? "visible" : "hidden" }}>
            <div class="seg" title="ツール呼び出しの見せ方">
              <button class={density.value === "talk" ? "on" : ""} onClick={() => { density.value = "talk"; saveSettings(); }}>会話だけ</button>
              <button class={density.value === "all" ? "on" : ""} onClick={() => { density.value = "all"; saveSettings(); }}>すべて</button>
            </div>
            <label class="find">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
              <input ref={ref} value={findQuery.value} placeholder="この会話を検索"
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
          ? <div class="list-empty">会話を読み込めませんでした：{convError.value}</div>
          : opening.value && !h ? <div class="list-empty">読み込んでいます…</div> : <Conversation sessionId={s.id} />)
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
  if (top.length === 0) return <div class="bar-row"><span class="k">なし</span></div>;
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
  if (!d) return <div class="done"><div class="list-empty">読み込んでいます…</div></div>;
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
        <div class="card"><h3>使ったツール</h3><Bars counts={d.topTools} /></div>
        <div class="card"><h3>走らせたコマンド（{d.commands}）</h3><Bars counts={d.programs} cls="cmdb" /></div>
        <div class="card">
          <h3>編集したファイル（{d.fileCount}）</h3>
          <div class="files">
            {d.fileCount === 0 && <div class="f"><span>なし</span></div>}
            {d.groups.map((g) => (
              <div key={g.place + (g.worktree ?? "")}>
                <div class="repo">{g.place === "" ? "このフォルダ" : g.worktree ? <>{g.place} <span class="wt">⌥{g.worktree}</span></> : g.place}</div>
                {g.files.map((f) => <div class="f" key={f.name}><span title={f.name}>{f.name}</span><span>{f.n > 1 ? `×${f.n}` : ""}</span></div>)}
              </div>
            ))}
            {d.temps > 0 && <div class="f"><span>+{d.temps} 個の一時ファイル</span></div>}
            {d.fileCount > d.groups.reduce((n, g) => n + g.files.length, 0) + d.temps && (
              <div class="f"><span>ほか {d.fileCount - d.groups.reduce((n, g) => n + g.files.length, 0) - d.temps} 個</span></div>
            )}
          </div>
        </div>
        <div class="card">
          <h3>Details</h3>
          <dl class="details">
            <dt>ID</dt><dd class="mono" style="color:var(--accent)">{s.id}</dd>
            <dt>開始</dt><dd>{dateTime(s.startedAt)}</dd>
            <dt>終了</dt><dd>{dateTime(s.endedAt)}</dd>
            <dt>メッセージ</dt><dd>{s.messages.toLocaleString()}（あなた {d.you}・Claude {d.claude}・ツール {d.tools}・thinking {d.thinking}）</dd>
            {d.images > 0 && <><dt>画像</dt><dd>{d.images}</dd></>}
            <dt>サイズ</dt><dd>{size(s.size)}</dd>
            <dt>ブランチ</dt><dd>{s.branch || "-"}</dd>
            <dt>フォルダ</dt><dd class="mono">{s.path}{s.gone ? "（もうない）" : ""}</dd>
            {h?.claudeVersion && <><dt>Claude Code</dt><dd>{h.claudeVersion}</dd></>}
            {h && <><dt>戻るには</dt><dd class="mono" title={h.resume.command}>{shownCommand(h.resume.command, h.fullPath, s.path)}</dd></>}
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
