// Pure helpers for the plugin's hooks module: reading what the recall CLI
// and MCP server return, and laying it out in terminal cells. What a
// session is (its repository, title, size) comes from recall; nothing here
// works it out. Nothing here touches `$`.

import type { ListedSession } from '../types'

export type { ListedSession }

// A search hit, from `recall search --format json` or recall_search; the
// two spell a few fields differently.
export type SearchHit = {
  sessionId: string
  repository?: string
  worktree?: string
  gitBranch?: string
  branch?: string
  startedAt?: string
  date?: string
  title?: string
  firstPrompt?: string
  messageCount?: number
  messages?: number
  role: string
  content: string
}

// The hits of one session, in result order.
export type HitGroup = {
  sessionId: string
  repository: string
  worktree: string
  branch: string
  date: string
  title: string
  msgs?: number
  hits: SearchHit[]
}

export function parseJSON<T>(text: string): T | undefined {
  try {
    return JSON.parse(text) as T
  } catch {
    return undefined
  }
}

// The text of an MCP tool result as the transcript hands it over: a string,
// a list of content blocks, or `{ content: [...] }`.
export function resultText(output: unknown): string | undefined {
  if (typeof output === 'string') return output
  const blocks = Array.isArray(output)
    ? output
    : output && typeof output === 'object' && Array.isArray((output as { content?: unknown }).content)
      ? (output as { content: unknown[] }).content
      : undefined
  if (!blocks) return undefined
  const texts = blocks
    .map(b => (b && typeof b === 'object' && typeof (b as { text?: unknown }).text === 'string' ? (b as { text: string }).text : ''))
    .filter(Boolean)
  return texts.length ? texts.join('\n') : undefined
}

export function groupHits(hits: SearchHit[]): HitGroup[] {
  const groups = new Map<string, HitGroup>()
  for (const hit of hits) {
    let g = groups.get(hit.sessionId)
    if (!g) {
      g = {
        sessionId: hit.sessionId,
        repository: hit.repository ?? '',
        worktree: hit.worktree ?? '',
        branch: hit.gitBranch ?? hit.branch ?? '',
        date: hit.startedAt ?? hit.date ?? '',
        // A session with no stored title (a `claude -p` run, say) is named
        // by its first prompt, as the TUI names it; failing that, by the hit.
        title: cleanTitle(hit.title ?? '') || cleanTitle(hit.firstPrompt ?? '') || cleanTitle(hit.content),
        msgs: hit.messageCount ?? hit.messages,
        hits: [],
      }
      groups.set(hit.sessionId, g)
    }
    g.hits.push(hit)
  }
  return [...groups.values()]
}

// A repository's sessions in a result: the ones drawn in full, and the ones
// that were only a recall search themselves, drawn together on one line.
export type RepoGroup = { repo: string; sessions: HitGroup[]; searches: HitGroup[] }

// Sessions grouped by repository, in result order within each. Repositories
// go in the order their first full session appears; those holding only
// searches follow, in the order they appear.
export function groupByRepo(groups: HitGroup[], isSearch: (g: HitGroup) => boolean = () => false): RepoGroup[] {
  const repos = new Map<string, RepoGroup>()
  for (const g of groups) {
    let r = repos.get(g.repository)
    if (!r) {
      r = { repo: g.repository, sessions: [], searches: [] }
      repos.set(g.repository, r)
    }
    ;(isSearch(g) ? r.searches : r.sessions).push(g)
  }
  const firstFull = (r: RepoGroup) => {
    const g = r.sessions[0]
    return g ? groups.indexOf(g) : groups.length
  }
  return [...repos.values()].sort((a, b) => firstFull(a) - firstFull(b))
}

// What fits in `maxLines`: a repository header takes a line, a session one
// plus one per hit shown (at most `hitsPerSession`), a repository's
// searches one line together. Taken in order until the next does not fit;
// the sessions left are counted as hidden.
export function fitTree(
  repos: RepoGroup[],
  maxLines: number,
  hitsPerSession: number,
): { shown: RepoGroup[]; hidden: number } {
  const shown: RepoGroup[] = []
  let lines = 0
  let hidden = 0
  let isFull = false
  const fits = (cost: number) => {
    if (isFull || lines + cost > maxLines) {
      isFull = true
      return false
    }
    lines += cost
    return true
  }
  for (const r of repos) {
    const taken: HitGroup[] = []
    for (const g of r.sessions) {
      if (fits(1 + Math.min(g.hits.length, hitsPerSession) + (taken.length === 0 ? 1 : 0))) taken.push(g)
      else hidden++
    }
    let searches: HitGroup[] = []
    if (r.searches.length) {
      if (fits(1 + (taken.length === 0 ? 1 : 0))) searches = r.searches
      else hidden += r.searches.length
    }
    if (taken.length || searches.length) shown.push({ repo: r.repo, sessions: taken, searches })
  }
  return { shown, hidden }
}

// Where a session ran, short: inside the repository being looked at, the
// worktree or the branch; across repositories, the repository and its
// worktree.
export function placeOf(s: { repository: string; worktree?: string; branch?: string }, isOneRepo: boolean): string {
  if (isOneRepo) return s.worktree || s.branch || s.repository
  return s.worktree ? `${s.repository} (${s.worktree})` : s.repository
}

// Whether a session looks like it was only a recall search itself (titled
// "recall で uriba を検索"): such sessions match every later search for the
// same word. A stopgap guess from the title and the size until the archive
// can tell which tools a session used.
export const isRecallSearch = (title: string, msgs: number | undefined) =>
  /^recall\b/i.test(title) && msgs !== undefined && msgs <= 50

export const shortId = (id: string) => id.slice(0, 8)

export function oneLine(text: string): string {
  return text.replace(/\s+/g, ' ').trim()
}

// The part of `text` around the first match of any query term, at most
// `width` characters, with "…" where it was cut.
export function snippet(text: string, terms: string[], width = 80): string {
  const flat = oneLine(text)
  if (flat.length <= width) return flat
  const lower = flat.toLowerCase()
  let at = -1
  for (const t of terms) {
    const i = lower.indexOf(t.toLowerCase())
    if (i >= 0 && (at < 0 || i < at)) at = i
  }
  if (at < 0) return flat.slice(0, width - 1) + '…'
  const start = Math.max(0, at - Math.floor(width / 3))
  const end = Math.min(flat.length, start + width)
  return (start > 0 ? '…' : '') + flat.slice(start, end) + (end < flat.length ? '…' : '')
}

// Splits `text` into runs, marking the ones that match a query term.
export function highlight(text: string, terms: string[]): { text: string; isHit: boolean }[] {
  const words = terms.filter(t => t.length > 0)
  if (words.length === 0) return [{ text, isHit: false }]
  const escaped = words.map(w => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
  const re = new RegExp(`(${escaped.join('|')})`, 'gi')
  return text
    .split(re)
    .filter(s => s !== '')
    .map(s => ({ text: s, isHit: words.some(w => w.toLowerCase() === s.toLowerCase()) }))
}

// The words of an FTS5 query worth highlighting: phrases without their
// quotes, operators dropped.
export function queryTerms(query: string): string[] {
  const terms: string[] = []
  const re = /"([^"]+)"|(\S+)/g
  let m: RegExpExecArray | null
  while ((m = re.exec(query)) !== null) {
    const t = m[1] ?? m[2]
    if (!t || ['AND', 'OR', 'NOT'].includes(t)) continue
    terms.push(t.replace(/[*()]/g, ''))
  }
  return terms.filter(Boolean)
}

export function shortDate(iso: string): string {
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return iso.slice(0, 10)
  const d = new Date(t)
  return `${String(d.getMonth() + 1).padStart(2, '0')}/${String(d.getDate()).padStart(2, '0')}`
}

// The sessions to offer at start, from `recall list --repo .`, newest
// first, without the running one and one-message stubs. Sessions that look
// like a recall search are passed over and counted, so the band can say it
// left them out: `omitted` counts the ones newer than the last session shown.
export function previousSessions(
  sessions: ListedSession[],
  currentId: string,
  limit = 3,
): { shown: ListedSession[]; omitted: number } {
  const shown: ListedSession[] = []
  let omitted = 0
  for (const s of sessions) {
    if (shown.length === limit) break
    if (s.sessionId === currentId || s.messageCount <= 2) continue
    if (isRecallSearch(sessionTitle(s), s.messageCount)) omitted++
    else shown.push(s)
  }
  return { shown, omitted }
}

// A title fit for one line: a slash command's record
// (`<command-name>/x</command-name><command-args>y</command-args>`) reads as
// `/x y`, and any other markup is dropped.
export function cleanTitle(text: string): string {
  const name = /<command-name>([\s\S]*?)<\/command-name>/.exec(text)?.[1]
  if (name !== undefined) {
    const args = /<command-args>([\s\S]*?)<\/command-args>/.exec(text)?.[1] ?? ''
    const cmd = name.trim().startsWith('/') ? name.trim() : `/${name.trim()}`
    return oneLine(`${cmd} ${args}`)
  }
  return oneLine(text.replace(/<[^>]+>/g, ' '))
}

export const sessionTitle = (s: Pick<ListedSession, 'title' | 'firstPrompt'>) =>
  cleanTitle(s.title || s.firstPrompt || '') || '(no title)'

// Terminal cells a character takes: two for East Asian wide and fullwidth
// characters (CJK, kana, hangul, fullwidth forms), one otherwise.
function charCells(cp: number): number {
  if (cp < 0x1100) return 1
  if (
    (cp >= 0x1100 && cp <= 0x115f) ||
    (cp >= 0x2e80 && cp <= 0x303e) ||
    (cp >= 0x3041 && cp <= 0x33ff) ||
    (cp >= 0x3400 && cp <= 0x4dbf) ||
    (cp >= 0x4e00 && cp <= 0x9fff) ||
    (cp >= 0xa000 && cp <= 0xa4cf) ||
    (cp >= 0xac00 && cp <= 0xd7a3) ||
    (cp >= 0xf900 && cp <= 0xfaff) ||
    (cp >= 0xfe30 && cp <= 0xfe4f) ||
    (cp >= 0xff00 && cp <= 0xff60) ||
    (cp >= 0xffe0 && cp <= 0xffe6) ||
    (cp >= 0x1f300 && cp <= 0x1faff) ||
    (cp >= 0x20000 && cp <= 0x3fffd)
  )
    return 2
  return 1
}

export function cells(text: string): number {
  let n = 0
  for (const ch of text) n += charCells(ch.codePointAt(0) ?? 0)
  return n
}

// `text` cut to at most `max` cells, ending in "…" when it was cut.
export function truncateCells(text: string, max: number): string {
  if (cells(text) <= max) return text
  if (max <= 0) return ''
  let out = ''
  let n = 0
  for (const ch of text) {
    const w = charCells(ch.codePointAt(0) ?? 0)
    if (n + w > max - 1) break
    out += ch
    n += w
  }
  return out + '…'
}

// `text` cut or padded with spaces to exactly `width` cells.
export function padCells(text: string, width: number): string {
  const cut = truncateCells(text, width)
  return cut + ' '.repeat(Math.max(0, width - cells(cut)))
}

// `text` cut to `width` cells and right-aligned in them.
export function padStartCells(text: string, width: number): string {
  const cut = truncateCells(text, width)
  return ' '.repeat(Math.max(0, width - cells(cut))) + cut
}

export type RowLayout = { title: number; place: number; indent: number }

// Cells of the message count column, "12345 msgs" at most.
export const MSGS_CELLS = 10

// Cells of the time column: "11時間前" and "just now" at most.
export const WHEN_CELLS = 8

// Column widths for a session row `inner` cells wide: number (when the rows
// are numbered), ID, title, place, message count, time, hit count (when
// counted). The place goes first when the row is narrow, then the title
// shrinks to its floor.
export function rowLayout(inner: number, isNumbered: boolean, hasHits: boolean, hasMsgs = false): RowLayout {
  const lead = (isNumbered ? 4 : 0) + 8 + 2
  const tail = 2 + WHEN_CELLS + (hasHits ? 2 + 4 : 0) + (hasMsgs ? 2 + MSGS_CELLS : 0)
  const room = inner - lead - tail
  const place = room >= 48 ? Math.min(28, Math.max(16, Math.floor(room * 0.3))) : 0
  const title = Math.max(10, room - (place ? place + 2 : 0))
  return { title, place, indent: lead }
}
