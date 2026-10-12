import type { Session } from "../api";

// The folder list, as the TUI's: a repository with its worktrees, or a
// directory outside git, the most recently active first.

export interface Worktree {
  name: string;
  count: number;
  /** All of its sessions ran in a folder that is gone. */
  gone: boolean;
}

export interface Folder {
  key: string;
  name: string;
  count: number;
  worktrees: Worktree[];
  /** A directory outside git, as far as its name tells: not owner/repo. */
  other: boolean;
}

function ended(s: Session): number {
  return s.endedAt ? new Date(s.endedAt).getTime() : 0;
}

export function buildFolders(rows: Session[]): Folder[] {
  const byKey = new Map<string, { f: Folder; last: number; wts: Map<string, Worktree & { last: number }> }>();
  for (const s of rows) {
    let g = byKey.get(s.groupKey);
    if (!g) {
      const other = s.groupName.startsWith("~") || s.groupName.startsWith("/");
      g = { f: { key: s.groupKey, name: s.groupName, count: 0, worktrees: [], other }, last: 0, wts: new Map() };
      byKey.set(s.groupKey, g);
    }
    g.f.count++;
    g.last = Math.max(g.last, ended(s));
    if (s.worktree) {
      let w = g.wts.get(s.worktree);
      if (!w) {
        w = { name: s.worktree, count: 0, gone: true, last: 0 };
        g.wts.set(s.worktree, w);
      }
      w.count++;
      w.gone &&= s.gone;
      w.last = Math.max(w.last, ended(s));
    }
  }
  const out = [...byKey.values()].sort((a, b) => b.last - a.last || a.f.name.localeCompare(b.f.name));
  return out.map(({ f, wts }) => ({
    ...f,
    worktrees: [...wts.values()]
      .sort((a, b) => b.last - a.last)
      .map(({ name, count, gone }) => ({ name, count, gone })),
  }));
}
