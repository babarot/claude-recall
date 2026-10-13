import { groupMessages } from "./chat-utils";
import type { DisplayMessage, Message } from "./chat-utils";
import type { Message as APIMessage } from "../api";

// What the conversation shows, built on groupMessages: a run of tool
// calls one after another becomes one item, its calls paired with their
// results, so the conversation reads as what was said.

type ToolUse = Extract<DisplayMessage, { type: "tool_use" }>;
type ToolResult = Extract<DisplayMessage, { type: "tool_result" }>;

export interface Call {
  use?: ToolUse;
  result?: ToolResult;
}

export type Item =
  | { kind: "run"; key: string; calls: Call[]; start?: string; end?: string }
  | { kind: "thinking"; key: string; content: string }
  | { kind: "meta"; key: string; label: string; content: string }
  | { kind: "message"; key: string; msg: Exclude<DisplayMessage, ToolUse | ToolResult | { type: "thinking" } | { type: "meta" }> };

export function fromAPI(m: APIMessage): Message {
  return {
    uuid: m.uuid, role: m.role, blockType: m.type, content: m.content,
    toolName: m.toolName ?? undefined, toolInput: m.toolInput ?? undefined, timestamp: m.timestamp ?? undefined,
    cursor: m.cursor, contentLength: m.contentLength, toolInputLength: m.toolInputLength,
    toolUseId: m.toolUseId, isError: m.isError,
  };
}

/**
 * Builds the items of messages. A tool result is paired with the call
 * whose id it carries. A session stored before the archive kept ids has
 * none, and its results are paired with the calls in the order they came,
 * the order Claude Code writes them in when calls do not run in parallel.
 */
export function buildItems(messages: Message[]): Item[] {
  const out: Item[] = [];
  let run: Extract<Item, { kind: "run" }> | null = null;
  let waiting: Call[] = [];
  groupMessages(messages).forEach((m, i) => {
    const key = ("cursor" in m && m.cursor) || `i${i}`;
    if (m.type === "tool_use" || m.type === "tool_result") {
      if (!run) {
        run = { kind: "run", key, calls: [], start: m.timestamp };
        waiting = [];
        out.push(run);
      }
      run.end = m.timestamp ?? run.end;
      if (m.type === "tool_use") {
        const call: Call = { use: m };
        run.calls.push(call);
        waiting.push(call);
      } else {
        const at = m.toolUseId ? waiting.findIndex((c) => c.use?.toolUseId === m.toolUseId) : 0;
        const call = at >= 0 ? waiting.splice(at, 1)[0] : undefined;
        if (call) call.result = m;
        else run.calls.push({ result: m });
      }
      return;
    }
    run = null;
    if (m.type === "thinking") out.push({ kind: "thinking", key, content: m.content });
    else if (m.type === "meta") out.push({ kind: "meta", key, label: m.label, content: m.content });
    else out.push({ kind: "message", key, msg: m });
  });
  return out;
}

/** Counts a run's calls by tool, most used first, and its failures. */
export function tally(calls: Call[]): { counts: [string, number][]; failed: number } {
  const n = new Map<string, number>();
  let failed = 0;
  for (const c of calls) {
    const name = c.use?.toolName ?? "?";
    n.set(name, (n.get(name) ?? 0) + 1);
    if (c.result && failedResult(c.result)) failed++;
  }
  return { counts: [...n.entries()].sort((a, b) => b[1] - a[1]), failed };
}

/**
 * Whether a result was a failure: as the transcript said, or, for a
 * session stored before the archive kept it, as its text reads.
 */
export function failedResult(r: { content: string; isError?: boolean }): boolean {
  return r.isError ?? looksFailed(r.content);
}

/**
 * Whether a result reads as a failure, from what Claude Code writes for
 * one: "Error: ...", "<tool_use_error>", or "Exit code N".
 */
export function looksFailed(content: string): boolean {
  const s = content.trimStart();
  return s.startsWith("<tool_use_error>") || /^Error[:\s]/.test(s) || /^Exit code [1-9]/.test(s);
}

/** How a tool's name shows: an MCP tool (mcp__<server>__<tool>) by its tool. */
export function toolLabel(name: string): string {
  const m = name.match(/^mcp__.+?__(.+)$/);
  return m ? m[1] : name;
}

/**
 * Shortens the paths in text that are under dir, the session's folder, to
 * where they are in it: tool inputs name files by their absolute paths.
 */
export function relativeTo(text: string, dir: string | undefined): string {
  if (!dir) return text;
  const d = dir.endsWith("/") ? dir : dir + "/";
  return text.split(d).join("");
}
