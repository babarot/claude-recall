import { useState } from "preact/hooks";
import { folder, folders, sessions, worktree } from "../store/app";
import type { Folder } from "../lib/folders";

export function pick(key: string | null, wt: string | null = null): void {
  folder.value = key;
  worktree.value = wt;
}

/** The rows the folder list shows, in order, for moving through them with keys. */
export function folderRows(list: Folder[], q: string): { key: string | null; wt: string | null }[] {
  const rows: { key: string | null; wt: string | null }[] = [{ key: null, wt: null }];
  for (const f of filterFolders(list, q)) {
    rows.push({ key: f.key, wt: null });
    if (folder.value === f.key) for (const w of f.worktrees) rows.push({ key: f.key, wt: w.name });
  }
  return rows;
}

function filterFolders(list: Folder[], q: string): Folder[] {
  const s = q.trim().toLowerCase();
  return s ? list.filter((f) => f.name.toLowerCase().includes(s)) : list;
}

export function Folders({ focused }: { focused: boolean }) {
  const [q, setQ] = useState("");
  const list = filterFolders(folders.value, q);
  const repos = list.filter((f) => !f.other);
  const others = list.filter((f) => f.other);
  const row = (f: Folder) => {
    const open = folder.value === f.key;
    return [
      <div
        key={f.key}
        class={`f-row ${open && !worktree.value ? "sel" : ""}`}
        onClick={() => pick(open && !worktree.value ? null : f.key)}
        title={f.name}
      >
        <span class="caret">{f.worktrees.length ? (open ? "▾" : "▸") : ""}</span>
        <span class="name">{f.name}</span>
        <span class="n">{f.count}</span>
      </div>,
      ...(open ? f.worktrees.map((w) => (
        <div
          key={f.key + "/" + w.name}
          class={`f-row wt ${w.gone ? "gone" : ""} ${worktree.value === w.name ? "sel" : ""}`}
          onClick={() => pick(f.key, w.name)}
          title={w.gone ? "削除済みの worktree" : w.name}
        >
          <span class="name">{w.name}</span>
          <span class="n">{w.count}</span>
        </div>
      )) : []),
    ];
  };
  return (
    <aside class={`pane folders ${focused ? "focused" : ""}`}>
      <div class="pane-head"><span class="label">Folders</span><span style="margin-left:auto"><kbd>[</kbd></span></div>
      <label class="folder-search">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
        <input value={q} onInput={(e) => setQ((e.target as HTMLInputElement).value)} placeholder="フォルダを絞り込む" />
      </label>
      <div class="scroll">
        <div class={`f-row ${folder.value === null ? "sel" : ""}`} onClick={() => pick(null)}>
          <span class="caret" /><span class="name">すべてのフォルダ</span><span class="n">{sessions.value.length}</span>
        </div>
        {repos.length > 0 && <div class="f-group">Repositories</div>}
        {repos.flatMap(row)}
        {others.length > 0 && <div class="f-group">Other</div>}
        {others.flatMap(row)}
      </div>
    </aside>
  );
}
