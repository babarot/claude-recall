import { describe, expect, it } from "vitest";
import { buildFolders } from "./folders";
import type { Session } from "../api";

const s = (id: string, groupKey: string, groupName: string, endedAt: string, worktree = "", gone = false): Session => ({
  id, title: id, folder: groupName, groupKey, groupName, worktree, gone, branch: "", path: "", messages: 1, size: 1,
  startedAt: endedAt, endedAt, recallOnly: false, activity: [],
});

describe("buildFolders", () => {
  it("groups by repository, the most recent first, with its worktrees", () => {
    const folders = buildFolders([
      s("a", "/r/app", "me/app", "2026-01-01T00:00:00Z"),
      s("b", "/r/app", "me/app", "2026-01-03T00:00:00Z", "calm-sea"),
      s("c", "/r/app", "me/app", "2026-01-02T00:00:00Z", "old", true),
      s("d", "/tmp", "/private/tmp", "2026-01-02T12:00:00Z"),
    ]);
    expect(folders.map((f) => [f.name, f.count, f.other])).toEqual([["me/app", 3, false], ["/private/tmp", 1, true]]);
    expect(folders[0].worktrees).toEqual([{ name: "calm-sea", count: 1, gone: false }, { name: "old", count: 1, gone: true }]);
  });

  it("files a folder under the repositories as the server tells, whatever its name reads as", () => {
    const folders = buildFolders([
      { ...s("a", "/src/github.com/me", "me", "2026-01-02T00:00:00Z"), repository: false },
      { ...s("b", "remote:git.example.com/team/tool", "git.example.com/team/tool", "2026-01-01T00:00:00Z"), repository: true },
      { ...s("c", "/work/app", "~/work/app", "2026-01-03T00:00:00Z"), repository: true },
    ]);
    expect(folders.map((f) => [f.name, f.other])).toEqual([["~/work/app", false], ["me", true], ["git.example.com/team/tool", false]]);
  });
});
