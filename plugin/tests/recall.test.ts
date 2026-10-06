import { expect, mock, test } from 'claude-code/testing'

import { langOf, relativeTime, words } from '../hooks/i18n'
import {
  cells,
  cleanTitle,
  fitTree,
  groupByRepo,
  groupHits,
  highlight,
  isRecallSearch,
  padCells,
  padStartCells,
  placeOf,
  previousSessions,
  queryTerms,
  resultText,
  rowLayout,
  snippet,
  truncateCells,
} from '../hooks/recall'

const RECAP = 'plugin:claude-recall:claude-recall:recap (MCP)'
const SEARCH_TOOL = 'mcp__plugin_claude-recall_claude-recall__recall_search'

// Hits as recall_search returns them: short IDs, `branch`, `date`, `messages`.
const MCP_HITS = [
  {
    sessionId: 'ef7eecb9',
    project: '~/.herdr/worktrees/dd-slo-reporter/worktree-brave-cloud-4f88',
    branch: 'worktree/brave-cloud-4f88',
    date: '2026-10-02',
    role: 'assistant',
    content: 'Slack App は team_uriba に入っていません。',
    title: 'uribaチームの送信先確認',
    messages: 206,
    repository: 'myorg/dd-slo-reporter',
    worktree: 'brave-cloud-4f88',
  },
]

// Hits as `recall search --format json` prints them.
const cliHit = (id: string, repository: string, worktree: string, title: string, messageCount: number) => ({
  sessionId: id,
  projectPath: `/w/${repository}/${worktree}`,
  gitBranch: 'main',
  startedAt: '2026-10-01T00:00:00Z',
  role: 'user',
  content: 'linkify here',
  title,
  messageCount,
  repository,
  worktree,
})

const group = (id: string, repository: string, title: string, msgs: number, hits = 1) => ({
  sessionId: id,
  repository,
  worktree: '',
  branch: '',
  date: '',
  title,
  msgs,
  hits: Array.from({ length: hits }, () => ({ sessionId: id, role: 'user', content: '' })),
})

test('previousSessions passes over the running session and stubs, and counts the recall searches it leaves out', async () => {
  const s = (id: string, messageCount: number, title = 'Fix the deploy') => ({
    sessionId: id,
    projectPath: '/r',
    messageCount,
    startedAt: '',
    repository: 'r',
    title,
  })
  const sessions = [s('now', 10), s('a', 10), s('stub', 1), s('search', 9, 'recall で uriba を検索'), s('b', 10)]
  const prev = previousSessions(sessions, 'now')
  expect(prev.shown.map(x => x.sessionId)).toEqual(['a', 'b'])
  expect(prev.omitted).toBe(1)
})

test('placeOf names the worktree or branch in one repository, the repository across them', async () => {
  expect(placeOf({ repository: 'me/app', worktree: 'fix', branch: 'main' }, true)).toBe('fix')
  expect(placeOf({ repository: 'me/app', branch: 'main' }, true)).toBe('main')
  expect(placeOf({ repository: 'me/app', worktree: 'fix' }, false)).toBe('me/app (fix)')
  expect(placeOf({ repository: 'me/app' }, false)).toBe('me/app')
})

test('isRecallSearch takes short sessions titled with a recall search only', async () => {
  expect(isRecallSearch('recall で uriba を検索', 6)).toBe(true)
  expect(isRecallSearch('Recall で uriba を検索', 49)).toBe(true)
  // Work in the claude-recall repository mentions recall but is long.
  expect(isRecallSearch('recall の TUI に mod を足す', 300)).toBe(false)
  expect(isRecallSearch('Fix the recall TUI footer', 8)).toBe(false)
  expect(isRecallSearch('recall で uriba を検索', undefined)).toBe(false)
})

test('queryTerms and highlight', async () => {
  expect(queryTerms('"claude mod" OR linkify')).toEqual(['claude mod', 'linkify'])
  expect(highlight('about Claude Mods', ['claude mod'])).toEqual([
    { text: 'about ', isHit: false },
    { text: 'Claude Mod', isHit: true },
    { text: 's', isHit: false },
  ])
})

test('snippet centers on the first hit', async () => {
  const s = snippet(`${'x'.repeat(100)} needle ${'y'.repeat(100)}`, ['needle'], 40)
  expect(s.includes('needle')).toBe(true)
  expect(s.startsWith('…')).toBe(true)
})

test('resultText reads strings, blocks and results', async () => {
  expect(resultText('[]')).toBe('[]')
  expect(resultText([{ type: 'text', text: '[1]' }])).toBe('[1]')
  expect(resultText({ content: [{ type: 'text', text: '[2]' }], isError: false })).toBe('[2]')
  expect(resultText(42)).toBe(undefined)
})

test('groupHits reads both the CLI and the MCP spelling', async () => {
  const [cli] = groupHits([cliHit('a', 'me/app', 'fix', 'Fix it', 12)])
  expect([cli?.repository, cli?.worktree, cli?.branch, cli?.title, cli?.msgs]).toEqual(['me/app', 'fix', 'main', 'Fix it', 12])
  const [untitled] = groupHits([{ ...cliHit('b', 'me/app', '', '', 2), content: '<command-name>/recall</command-name><command-args>x</command-args>' }])
  expect(untitled?.title).toBe('/recall x')
  const [named] = groupHits([{ ...cliHit('c', 'me/app', '', '', 2), firstPrompt: 'https://example.com/issues/1 catch up on this' }])
  expect(named?.title).toBe('https://example.com/issues/1 catch up on this')
  const [m] = groupHits(MCP_HITS)
  expect([m?.repository, m?.branch, m?.date, m?.msgs]).toEqual(['myorg/dd-slo-reporter', 'worktree/brave-cloud-4f88', '2026-10-02', 206])
})

test('cells counts wide characters as two', async () => {
  expect(cells('ユーザーのprompt')).toBe(16)
  expect(truncateCells('ユーザーのprompt傾向分析', 10)).toBe('ユーザー…')
  expect(padCells('前回', 6)).toBe('前回  ')
  expect(padStartCells('4日前', 7)).toBe('  4日前')
})

test('cleanTitle reads a slash command record and drops markup', async () => {
  expect(
    cleanTitle('<command-message>x</command-message>\n<command-name>/claude-recall:recall</command-name>\n<command-args>DORA</command-args>'),
  ).toBe('/claude-recall:recall DORA')
  expect(cleanTitle('<b>hello</b>  world')).toBe('hello world')
})

test('rowLayout fills the line exactly, dropping the place when narrow', async () => {
  for (const [inner, numbered, hits, msgs] of [
    [120, true, true, true],
    [90, true, true, false],
    [100, false, false, true],
  ] as const) {
    const l = rowLayout(inner, numbered, hits, msgs)
    expect(l.indent + l.title + (l.place ? l.place + 2 : 0) + 2 + 8 + (hits ? 6 : 0) + (msgs ? 12 : 0)).toBe(inner)
  }
  for (const when of ['11時間前', 'just now', '59m ago', '10/02']) {
    expect(cells(when) <= 8).toBe(true)
  }
  expect(rowLayout(60, true, true).place).toBe(0)
})

test('searches gather on one line per repository, after the full sessions', async () => {
  const repos = groupByRepo(
    [group('s1', 'slo', 'recall で a を検索', 6), group('i1', 'infra', 'Apply', 300), group('f1', 'slo', 'Fix', 200), group('s2', 'only', 'recall で b', 4)],
    g => isRecallSearch(g.title, g.msgs),
  )
  expect(repos.map(r => [r.repo, r.sessions.length, r.searches.length])).toEqual([
    ['infra', 1, 0],
    ['slo', 1, 1],
    ['only', 0, 1],
  ])
  // infra: 1 + 2; slo: 1 + 2 + searches 1; only: 1 + 1 = 9
  expect(fitTree(repos, 9, 1).hidden).toBe(0)
  const cut = fitTree(repos, 7, 1)
  expect(cut.shown.map(r => r.repo)).toEqual(['infra', 'slo'])
  expect(cut.hidden).toBe(1)
})

test('langOf reads the language setting, English unless Japanese', async () => {
  expect(langOf('japanese')).toBe('ja')
  expect(langOf('日本語')).toBe('ja')
  expect(langOf('English')).toBe('en')
  expect(langOf(undefined)).toBe('en')
  expect(words('ja').sessions(3)).toBe('3 セッション')
  expect(words('en').sessions(1)).toBe('1 session')
  const now = Date.parse('2026-10-06T12:00:00Z')
  expect(relativeTime('2026-10-06T09:00:00Z', now)).toBe('3h ago')
  expect(relativeTime('2026-10-06T09:00:00Z', now, words('ja'))).toBe('3時間前')
})

test('/recall searches this repository through recall and recaps a result', async ($, on) => {
  mock.clock(on, { now: Date.parse('2026-10-02T00:00:00Z') })
  on('session.id', () => ({ value: 'current' }) as never)
  const argvs: string[][] = []
  on('process.run', (_$, e) => {
    const argv = (e as { argv: string[] }).argv
    argvs.push(argv)
    const hits = [
      cliHit('ssssssss-1', 'me/app', '', 'recall で linkify を検索', 6),
      cliHit('aaaaaaaa-1', 'me/app', 'fix', 'Fix the linkify', 10),
      cliHit('current', 'me/app', '', 'now', 3),
    ]
    return { value: { exitCode: 0, stdout: JSON.stringify(hits), stderr: '' } } as never
  })
  on('command.list', () => ({ value: [{ name: RECAP, description: '', source: 'mcp' }] }) as never)
  let ran = ''
  on('command.run', (_$, e) => {
    ran = `${(e as { command: string }).command} ${(e as { args: string }).args}`
    return {} as never
  })
  const run = (args: string) =>
    $.command.run({ command: 'recall', args, origin: { kind: 'composer' }, presentation: { isFullscreen: false, columns: 120 } })

  const listed = await run('linkify')
  expect(argvs[0]).toEqual(['recall', 'search', 'linkify', '--format', 'json', '--limit', '1000', '--repo', '.'])
  // What the model reads: English, every session but the running one, the
  // recall search after the work session and numbered so.
  expect(listed.text?.includes('"linkify" · 2 sessions · this repository')).toBe(true)
  expect(listed.text?.includes(' 1  aaaaaaaa  Fix the linkify')).toBe(true)
  expect(listed.text?.includes(' 2  ssssssss  recall で linkify を検索')).toBe(true)
  expect(listed.text?.includes('current')).toBe(false)

  const ui = await $.ui.mount({
    plugin: 'claude-recall',
    surface: 'terminal',
    component: 'CommandOutput',
    props: { command: 'recall', args: 'linkify', text: listed.text ?? '', isErrored: false },
    viewport: { columns: 120, rows: 40 },
  } as never)
  expect(await ui.find({ type: 'Text', text: /Fix the linkify/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /10 msgs/ })).toBeDefined()
  // The recall search is not a row of its own but is on the screen.
  expect(await ui.find({ type: 'Text', text: /1 recall search {2}ssssssss/ })).toBeDefined()
  await ui.press({ key: 'refer-aaaaaaaa-1' })
  expect(ran).toBe(`${RECAP} aaaaaaaa-1`)
  await ui.unmount()

  expect((await run('1')).text).toBe('Recapping aaaaaaaa.')
  await run('linkify --all')
  expect(argvs.at(-1)?.includes('--repo')).toBe(false)
})

test('pressing an ID without the recap prompt says to update recall', async ($, on) => {
  on('command.list', () => ({ value: [] }) as never)
  let toast = ''
  on('ui.toast', (_$, e) => {
    toast = String((e as { text?: unknown }).text ?? '')
    return {} as never
  })
  const ui = await $.ui.mount({
    plugin: 'claude-recall',
    surface: 'terminal',
    component: 'ToolUse',
    props: {
      tool_use_id: 't1',
      tool: SEARCH_TOOL,
      input: { query: 'uriba' },
      isRunning: false,
      isErrored: false,
      isInterrupted: false,
      output: [{ type: 'text', text: JSON.stringify(MCP_HITS) }],
    },
    viewport: { columns: 120, rows: 40 },
  } as never)
  await ui.press({ key: 'refer-ef7eecb9' })
  expect(toast.includes('recap')).toBe(true)
  await ui.unmount()
})

test('a recall_search row draws the query, the repository and the sessions', async $ => {
  const ui = await $.ui.mount({
    plugin: 'claude-recall',
    surface: 'terminal',
    component: 'ToolUse',
    props: {
      tool_use_id: 't2',
      tool: SEARCH_TOOL,
      input: { query: 'uriba', limit: 20 },
      isRunning: false,
      isErrored: false,
      isInterrupted: false,
      output: [{ type: 'text', text: JSON.stringify(MCP_HITS) }],
    },
    viewport: { columns: 120, rows: 40 },
  } as never)
  expect(await ui.find({ type: 'Text', text: /1 session · 1 hit/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /myorg\/dd-slo-reporter/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /uribaチームの送信先確認/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /206 msgs/ })).toBeDefined()
  await ui.unmount()
})

test('a group of recall_search calls, with the ToolSearch before them, unfolds', async ($, on) => {
  let expanded: boolean | undefined
  on('ui.render', ($e, e) => {
    expanded = (e.props as { isExpanded?: boolean }).isExpanded
    const { Text } = $e.ui.resolve(e) as never as { Text: unknown }
    return h(Text as never, {}, 'group') as never
  })
  const call = { tool: SEARCH_TOOL, input: { query: 'x' }, isRunning: false, isErrored: false, isInterrupted: false }
  const mount = (calls: unknown[]) =>
    $.ui.mount({
      plugin: 'claude-recall',
      surface: 'terminal',
      component: 'ToolGroup',
      props: { calls, isActive: false, isExpanded: false },
    } as never)
  await (await mount([call])).unmount().catch(() => {})
  expect(expanded).toBe(true)
  await (await mount([{ ...call, tool: 'ToolSearch' }, call])).unmount().catch(() => {})
  expect(expanded).toBe(true)
  await (await mount([call, { ...call, tool: 'Read' }])).unmount().catch(() => {})
  expect(expanded).toBe(false)
})

// Starts a session the way Claude Code does, with recall answering from
// `sessions`, and returns the argv recall was run with. Everything beneath
// the plugin is answered here, before the test first calls $; a drawing the
// plugin passes on is answered with the text "engine".
// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function startSession($: any, on: any, sessions: unknown[], language?: string) {
  mock.clock(on, { now: Date.parse('2026-10-02T12:00:00Z') })
  const argvs: string[][] = []
  on('session.start', (_$: unknown, e: { cwd: string }) => ({ cwd: e.cwd }))
  on('session.id', () => ({ value: 'current' }))
  on('command.register', () => ({ value: { command: 'recall' } }))
  on('settings.read', () => ({ value: language ? { language } : {} }))
  on('prompt.submit', (_$: unknown, e: { text: string }) => ({ text: e.text }))
  on('process.run', (_$: unknown, e: { argv: string[] }) => {
    argvs.push(e.argv)
    return { value: { exitCode: 0, stdout: JSON.stringify(sessions), stderr: '' } }
  })
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  on('ui.render', ($e: any, e: any) => h($e.ui.resolve(e).Text, {}, 'engine'))
  await $.session.start({ cwd: '/w/me/app', surface: 'terminal', isInteractive: true })
  return argvs
}

// The band once its sessions are in: they are listed after the session
// starts, without holding it, so the drawing is tried until it shows them.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function mountBand($: any, until: RegExp) {
  for (let i = 0; i < 50; i++) {
    const ui = await $.ui.mount(BAND)
    if (await ui.find({ type: 'Text', text: until })) return ui
    await ui.unmount()
    await new Promise<void>(resolve =>
      (globalThis as unknown as { setTimeout: (f: () => void, ms: number) => void }).setTimeout(() => resolve(), 20),
    )
  }
  return $.ui.mount(BAND)
}

const listed = (id: string, title: string, messageCount: number) => ({
  sessionId: id,
  projectPath: '/w/me/app',
  gitBranch: 'main',
  messageCount,
  startedAt: '2026-10-02T09:00:00Z',
  endedAt: '2026-10-02T09:00:00Z',
  title,
  repository: 'me/app',
})

const BAND = { plugin: 'claude-recall', surface: 'terminal', component: 'AbovePrompt', props: { bodyColumns: 120 } } as const

test('the band lists the repository\'s work sessions and says how many searches it left out', async ($, on) => {
  const argvs = await startSession($, on, [
    listed('current', 'now', 10),
    listed('search01', 'recall で uriba を検索', 6),
    listed('work0001', 'Fix the deploy', 40),
  ])
  const ui = await mountBand($, /Fix the deploy/)
  expect(argvs[0]).toEqual(['recall', 'list', '--repo', '.', '--format', 'json', '--limit', '50'])
  expect(await ui.find({ type: 'Text', text: /Pick up where you left off/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /Fix the deploy/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /1 recall search left out/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /recall で uriba/ })).toBe(undefined)
  await ui.unmount()
})

test('the band goes once the first prompt is sent', async ($, on) => {
  await startSession($, on, [listed('work0001', 'Fix the deploy', 40)])
  await (await mountBand($, /Fix the deploy/)).unmount()
  await $.prompt.submit({ text: 'hello', wait: false, origin: { kind: 'composer' } } as never)
  const ui = await $.ui.mount(BAND as never)
  expect(await ui.find({ type: 'Text', text: /Fix the deploy/ })).toBe(undefined)
  expect(await ui.find({ type: 'Text', text: /engine/ })).toBeDefined()
  await ui.unmount()
})

test('drawings use Japanese when the language setting is Japanese', async ($, on) => {
  await startSession($, on, [listed('work0001', 'Fix the deploy', 40)], 'japanese')
  const band = await mountBand($, /Fix the deploy/)
  expect(await band.find({ type: 'Text', text: /前回の続き/ })).toBeDefined()
  await band.unmount()
  const ui = await $.ui.mount({
    plugin: 'claude-recall',
    surface: 'terminal',
    component: 'ToolUse',
    props: {
      tool_use_id: 'tj',
      tool: SEARCH_TOOL,
      input: { query: 'uriba' },
      isRunning: false,
      isErrored: false,
      isInterrupted: false,
      output: [{ type: 'text', text: JSON.stringify(MCP_HITS) }],
    },
    viewport: { columns: 120, rows: 40 },
  } as never)
  expect(await ui.find({ type: 'Text', text: /1 セッション · 1 件/ })).toBeDefined()
  await ui.unmount()
})

test('a standalone recall_search result row draws the tree with the query of its call', async ($, on) => {
  on('tool.call', () => ({ result: [] }) as never)
  await $.tool.call({ tool: SEARCH_TOOL, query: 'uriba' } as never).catch(() => {})
  const ui = await $.ui.mount({
    plugin: 'claude-recall',
    surface: 'terminal',
    component: 'ToolResult',
    props: {
      tool_use_id: 'standalone',
      tool: SEARCH_TOOL,
      output: [{ type: 'text', text: JSON.stringify(MCP_HITS) }],
      isErrored: false,
    },
    viewport: { columns: 120, rows: 40 },
  } as never)
  expect(await ui.find({ type: 'Text', text: /1 session · 1 hit/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /uribaチームの送信先確認/ })).toBeDefined()
  await ui.unmount()
})

test('the tree puts sessions that were a recall search on one line', async $ => {
  const search = { ...MCP_HITS[0], sessionId: '5ea4c400', title: 'recall で uriba を検索', messages: 6, content: 'recall で uriba を検索して', role: 'user' }
  const ui = await $.ui.mount({
    plugin: 'claude-recall',
    surface: 'terminal',
    component: 'ToolUse',
    props: {
      tool_use_id: 't4',
      tool: SEARCH_TOOL,
      input: { query: 'uriba' },
      isRunning: false,
      isErrored: false,
      isInterrupted: false,
      output: [{ type: 'text', text: JSON.stringify([...MCP_HITS, search]) }],
    },
    viewport: { columns: 120, rows: 40 },
  } as never)
  expect(await ui.find({ type: 'Text', text: /1 recall search {2}5ea4c400/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /uribaチームの送信先確認/ })).toBeDefined()
  await ui.unmount()
})

test('/recall says how to use it, an unknown number and a failed search', async ($, on) => {
  on('session.id', () => ({ value: 'current' }) as never)
  on('process.run', () => ({ value: { exitCode: 1, stdout: '', stderr: 'boom\n' } }) as never)
  const run = (args: string) =>
    $.command.run({ command: 'recall', args, origin: { kind: 'composer' }, presentation: { isFullscreen: false, columns: 120 } })
  expect((await run('')).text).toBe('Usage: /recall <query> [--all] | /recall <n>')
  expect((await run('9')).text).toBe('There is no result 9. Search first with /recall <query>.')
  expect((await run('uriba')).text).toBe('recall search failed: boom')
})
