// The /api/v2 the server gives the web UI (internal/api/v2.go).

export interface Session {
  id: string;
  title: string;
  folder: string;
  groupKey: string;
  groupName: string;
  worktree: string;
  gone: boolean;
  branch: string;
  path: string;
  messages: number;
  size: number;
  startedAt: string | null;
  endedAt: string | null;
  recallOnly: boolean;
  activity: number[];
}

export interface Resume {
  resumable: boolean;
  command: string;
  reason?: "gone" | "other-tree" | "no-transcript";
  otherTree?: string;
}

export interface SessionDetail extends Session {
  fullPath: string;
  claudeVersion: string;
  resume: Resume;
}

export interface Message {
  cursor: string;
  uuid: string;
  role: string;
  type: string;
  content: string;
  contentLength?: number;
  toolName: string | null;
  toolInput: string | null;
  toolInputLength?: number;
  timestamp: string | null;
  /** Ties a tool_result to its tool_use; absent when the archive does not know. */
  toolUseId?: string;
  /** Whether a tool_result was an error, when the transcript said. */
  isError?: boolean;
}

export interface Range {
  total: number;
  messages: Message[];
  first?: string;
  last?: string;
  stale?: boolean;
}

export interface Prompt {
  cursor: string;
  timestamp: string | null;
  text: string;
}

export interface Hit {
  cursor: string;
  uuid: string;
}

export interface Count {
  name: string;
  n: number;
}

export interface Detail {
  you: number;
  claude: number;
  tools: number;
  thinking: number;
  images: number;
  topTools: Count[];
  commands: number;
  programs: Count[];
  fileCount: number;
  groups: { place: string; worktree?: string; files: Count[] }[];
  temps: number;
  activity: number[];
}

export interface Snippet {
  cursor: string;
  role: string;
  text: string;
}

/** The TUI's color scheme, as the page is to draw it (see internal/api.V2Theme). */
export interface Theme {
  name: string;
  fixed: boolean;
  dark?: boolean;
  colors?: Record<string, string>;
}

export interface Find {
  ids: string[];
  text: boolean;
  in: boolean;
  snippets?: Record<string, Snippet>;
}

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(path, { signal });
  if (!res.ok) {
    let msg = `${res.status}`;
    try {
      msg = (await res.json()).error ?? msg;
    } catch {
      // not JSON
    }
    throw new Error(msg);
  }
  return res.json() as Promise<T>;
}

const enc = encodeURIComponent;

export const api = {
  sessions: (signal?: AbortSignal) => get<Session[]>("/api/v2/sessions", signal),
  session: (id: string) => get<Session[]>(`/api/v2/sessions?id=${enc(id)}`).then((r) => r[0] ?? null),
  header: (id: string, signal?: AbortSignal) => get<SessionDetail>(`/api/v2/sessions/${enc(id)}`, signal),
  /** The messages after cursor ("start" for the first ones). */
  after: (id: string, cursor: string, limit: number, anchor?: { uuid: string; known: number }, signal?: AbortSignal) => {
    const q = new URLSearchParams({ after: cursor, limit: String(limit) });
    if (anchor) {
      q.set("uuid", anchor.uuid);
      q.set("known", String(anchor.known));
    }
    return get<Range>(`/api/v2/sessions/${enc(id)}/conversation?${q}`, signal);
  },
  /** The messages before cursor ("end" for the last ones). */
  before: (id: string, cursor: string, limit: number, signal?: AbortSignal) =>
    get<Range>(`/api/v2/sessions/${enc(id)}/conversation?${new URLSearchParams({ before: cursor, limit: String(limit) })}`, signal),
  message: (id: string, cursor: string) => get<Message>(`/api/v2/sessions/${enc(id)}/messages/${enc(cursor)}`),
  outline: (id: string, signal?: AbortSignal) => get<Prompt[]>(`/api/v2/sessions/${enc(id)}/outline`, signal),
  findIn: (id: string, q: string, signal?: AbortSignal) => get<Hit[]>(`/api/v2/sessions/${enc(id)}/find?q=${enc(q)}`, signal),
  detail: (id: string, signal?: AbortSignal) => get<Detail>(`/api/v2/sessions/${enc(id)}/detail`, signal),
  find: (q: string, text: boolean, signal?: AbortSignal) =>
    get<Find>(`/api/v2/find?q=${enc(q)}${text ? "&text=1" : ""}`, signal),
  theme: () => get<Theme>("/api/v2/theme"),
  status: () => get<{ version: string; watcher: { running: boolean } }>("/api/status"),
};
