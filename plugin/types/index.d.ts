// A session as `recall list --format json` prints it.
export type ListedSession = {
  sessionId: string
  projectPath: string
  gitBranch?: string
  firstPrompt?: string
  messageCount: number
  startedAt: string
  endedAt?: string
  title?: string
  // What the session is called, as the TUI shows it: the title, or else
  // the first prompt made readable.
  displayTitle: string
  // The session called recall's MCP tools and no other tool, so it was
  // only a look back.
  recallOnly: boolean
  // The repository the session belongs to, as the TUI groups it, and the
  // worktree inside it, when it ran in one.
  repository: string
  worktree?: string
}

// One session of a search, ready to draw.
export type FoundRow = {
  id: string
  title: string
  place: string
  // When the session last ran, ISO 8601; drawn relative to now.
  at: string
  msgs?: number
  hits: number
  snippet: string
}

// What /recall drew for one run, keyed by its args.
export type SearchView = {
  query: string
  scope: 'repo' | 'all'
  total: number
  // The sessions drawn in full, at most a screenful.
  rows: FoundRow[]
  // The sessions that look like they were only a recall search, drawn
  // together on one line after the rows.
  searches: string[]
  // The terminal's width when the command ran.
  columns?: number
}

declare module 'claude-code' {
  interface PluginState {
    'claude-recall': {
      previous: ListedSession[] | null
      // How many recall searches the band left out to show work sessions.
      omitted: number
      isBandHidden: boolean
      queries: Record<string, string>
      found: string[]
      views: Record<string, SearchView>
      lang: 'en' | 'ja'
    }
  }
}
