import { atom, read, update } from 'claude-code'
import type { EngineInterface, Register, Elements } from 'claude-code'

import type { FoundRow, ListedSession, SearchView } from '../types'
import {
  cells,
  fitTree,
  groupByRepo,
  groupHits,
  highlight,
  MSGS_CELLS,
  WHEN_CELLS,
  oneLine,
  padCells,
  padStartCells,
  parseJSON,
  placeOf,
  previousSessions,
  queryTerms,
  resultText,
  rowLayout,
  shortDate,
  shortId,
  snippet,
  truncateCells,
  type SearchHit,
} from './recall'
import { langOf, relativeTime, words, type Words } from './i18n'

const previous = atom({ plugin: 'claude-recall', key: 'previous' } as const, null)
const omitted = atom({ plugin: 'claude-recall', key: 'omitted' } as const, 0)
const isBandHidden = atom({ plugin: 'claude-recall', key: 'isBandHidden' } as const, false)
const queries = atom({ plugin: 'claude-recall', key: 'queries' } as const, {})
const found = atom({ plugin: 'claude-recall', key: 'found' } as const, [])
const views = atom({ plugin: 'claude-recall', key: 'views' } as const, {})
const lang = atom({ plugin: 'claude-recall', key: 'lang' } as const, 'en')

// Theme keys, so the drawing follows the person's theme.
const BRAND = 'merged'
const META = 'subtle'
const HIT = 'warning'
// A match inside a dim snippet, and a repository's name: gray, the match bold.
const MATCH = 'inactive'

// Sessions the /recall table draws in full, and the ones its text lists for
// the model.
const MAX_ROWS = 15
const MAX_TEXT_ROWS = 30
// Lines a recall_search tree may take, its repository headers included.
const MAX_TREE_LINES = 24
const MAX_HITS = 2

const isRecallTool = (tool: string, name: string) =>
  tool.startsWith('mcp__') && tool.includes('recall') && tool.endsWith(`__${name}`)

// The recall CLI's JSON for argv, or undefined when it failed or found
// nothing (it then prints a sentence, not JSON).
async function recallJSON<T>($: EngineInterface, argv: string[]): Promise<{ value?: T; error?: string }> {
  const { exitCode, stdout, stderr } = await $.process.run(['recall', ...argv])
  if (exitCode !== 0) return { error: oneLine(stderr) }
  return { value: parseJSON<T>(stdout) }
}

// Resumes a past session by running the MCP server's recap prompt, as if
// the person typed it: the server words the instruction, and the model
// reads the session with recall_export, sums it up and asks how to go on.
async function resumeSession($: EngineInterface, id: string) {
  const commands = await $.command.list()
  const recap = commands.find(c => c.source === 'mcp' && c.name.includes('recall') && /\brecap\b/.test(c.name))
  if (!recap) {
    $.ui.toast(words(await read($, lang)).noRecap)
    return
  }
  await $.command.run({ command: recap.name, args: id })
}

// Not awaited: the turn starts once the session is idle, and a command or a
// press waiting on it would hold the session until then. A failure is said
// in a toast rather than lost.
function resume($: EngineInterface, id: string) {
  void resumeSession($, id).catch((err: unknown) => $.ui.toast(`recall: could not resume ${shortId(id)}: ${String(err)}`))
}

type Table = Elements['terminal']

// One session as a row of fixed columns, laid out here rather than by
// flex so wide characters never push a column off the line: number, ID
// (pressing it recaps that session), title, place, message count, time,
// hit count; then the first hit under the title, the query highlighted.
function sessionRow(
  { Box, Button, Text }: Pick<Table, 'Box' | 'Button' | 'Text'>,
  $: EngineInterface,
  row: { n?: number; id: string; title: string; place: string; when: string; msgs?: number; hits?: number; snippet?: string },
  terms: string[],
  inner: number,
  w: Words,
) {
  const layout = rowLayout(inner, row.n !== undefined, row.hits !== undefined, row.msgs !== undefined)
  const snippetText = row.snippet ? truncateCells(row.snippet, inner - layout.indent) : ''
  return (
    <Box key={`row-${row.id}`} flexDirection="column">
      <Box flexDirection="row">
        {row.n !== undefined && <Text color={META}>{padStartCells(String(row.n), 2) + '  '}</Text>}
        <Button key={`refer-${row.id}`} plain dimColor label={shortId(row.id)} onPress={() => resume($, row.id)} />
        <Text wrap="truncate-end">
          {'  '}
          <Text bold>{padCells(row.title, layout.title)}</Text>
          {layout.place > 0 && <Text color={META}>{'  ' + padCells(row.place, layout.place)}</Text>}
          {row.msgs !== undefined && <Text color={META}>{'  ' + padStartCells(w.msgs(row.msgs), MSGS_CELLS)}</Text>}
          <Text color={META}>{'  ' + padStartCells(row.when, WHEN_CELLS)}</Text>
          {row.hits !== undefined && <Text color={HIT}>{'  ' + padStartCells(String(row.hits), 4)}</Text>}
        </Text>
      </Box>
      {snippetText && (
        <Text wrap="truncate-end" color={META}>
          {' '.repeat(layout.indent)}
          {highlight(snippetText, terms).map(run =>
            run.isHit ? (
              <Text color={MATCH} bold>
                {run.text}
              </Text>
            ) : (
              run.text
            ),
          )}
        </Text>
      )}
    </Box>
  )
}

// The sessions that were only a look back through recall, on one line:
// how many, then their IDs, each pressed like any other to recap it, as
// many as fit in `inner` cells and a count of the rest.
function searchesLine(
  { Box, Button, Text }: Pick<Table, 'Box' | 'Button' | 'Text'>,
  $: EngineInterface,
  lead: string,
  ids: string[],
  inner: number,
  w: Words,
) {
  const label = lead + w.searches(ids.length) + '  '
  // An ID takes eight cells, and two more for the ", " after it.
  const fit = Math.max(1, Math.floor((inner - cells(label) - 4) / 10))
  const shown = ids.slice(0, fit)
  return (
    <Box flexDirection="row">
      <Text color={META}>{label}</Text>
      {shown.map((id, i) => (
        <Box key={`search-${id}`} flexDirection="row">
          <Button key={`refer-${id}`} plain dimColor label={shortId(id)} onPress={() => resume($, id)} />
          {i < shown.length - 1 && <Text color={META}>{', '}</Text>}
        </Box>
      ))}
      {ids.length > shown.length && <Text color={META}>{` +${ids.length - shown.length}`}</Text>}
    </Box>
  )
}

// A header line: `parts` cut to fit, then `hint` at the right edge when
// there is room for it.
function headerLine(
  { Text }: Pick<Table, 'Text'>,
  parts: { text: string; color?: string; bold?: boolean }[],
  hint: string,
  inner: number,
) {
  const hintCells = cells(hint)
  const showHint = hint !== '' && inner >= 50 + hintCells
  let room = inner - (showHint ? hintCells + 2 : 0)
  const drawn: { text: string; color?: string; bold?: boolean }[] = []
  for (const part of parts) {
    if (room <= 0) break
    const text = truncateCells(part.text, room)
    drawn.push({ ...part, text })
    room -= cells(text)
  }
  return (
    <Text wrap="truncate-end">
      {drawn.map(part => (
        <Text color={part.color} bold={part.bold}>
          {part.text}
        </Text>
      ))}
      {showHint && <Text color={META}>{' '.repeat(Math.max(2, room + 2)) + hint}</Text>}
    </Text>
  )
}

// A recall_search result as a tree: repositories, their sessions (ID,
// title, size, date) and up to two hits of each, the query in gray bold.
// Sessions that were only a recall search share one line per repository.
function searchBody(
  $: EngineInterface,
  el: Pick<Table, 'Box' | 'Button' | 'Text'>,
  hits: SearchHit[],
  query: string,
  inner: number,
  w: Words,
) {
  const { Box, Button, Text } = el
  const terms = queryTerms(query)
  const { shown, hidden } = fitTree(
    groupByRepo(groupHits(hits), g => g.recallOnly),
    MAX_TREE_LINES,
    MAX_HITS,
  )
  const titleCells = Math.max(10, inner - 4 - 8 - 2 - 2 - MSGS_CELLS - 2 - 5)
  const snippetCells = Math.max(10, inner - 8)
  return (
    <Box flexDirection="column">
      {shown.map((r, ri) => (
        <Box key={`repo-${r.repo}`} flexDirection="column">
          <Text wrap="truncate-end">
            <Text color={META}>{ri === 0 ? '⎿ ' : '├ '}</Text>
            <Text color={MATCH}>{truncateCells(r.repo, inner - 2)}</Text>
          </Text>
          {r.sessions.map((g, si) => {
            const isLast = si === r.sessions.length - 1 && r.searches.length === 0
            return (
              <Box key={`s-${g.sessionId}`} flexDirection="column">
                <Box flexDirection="row">
                  <Text color={META}>{isLast ? '│ └ ' : '│ ├ '}</Text>
                  <Button key={`refer-${g.sessionId}`} plain dimColor label={shortId(g.sessionId)} onPress={() => resume($, g.sessionId)} />
                  <Text wrap="truncate-end">
                    {'  '}
                    <Text>{padCells(g.title, titleCells)}</Text>
                    <Text color={META}>
                      {'  ' + padStartCells(g.msgs === undefined ? '' : w.msgs(g.msgs), MSGS_CELLS)}
                      {'  ' + padStartCells(shortDate(g.date), 5)}
                    </Text>
                  </Text>
                </Box>
                {g.hits.slice(0, MAX_HITS).map((hit, hi) => (
                  <Text key={`h-${g.sessionId}-${hi}`} wrap="truncate-end" color={META}>
                    {(isLast ? '│     ' : '│ │   ') + (hit.role === 'user' ? '❯ ' : '⏺ ')}
                    {highlight(truncateCells(snippet(hit.content, terms, snippetCells * 2), snippetCells), terms).map(run =>
                      run.isHit ? (
                        <Text color={MATCH} bold>
                          {run.text}
                        </Text>
                      ) : (
                        run.text
                      ),
                    )}
                  </Text>
                ))}
              </Box>
            )
          })}
          {r.searches.length > 0 &&
            searchesLine(
              el,
              $,
              '│ └ ',
              r.searches.map(g => g.sessionId),
              inner,
              w,
            )}
        </Box>
      ))}
      <Text color={META} wrap="truncate-end">
        {'└ '}
        {hidden > 0 ? `${w.more(hidden)} · ` : ''}
        {w.pressToResume}
      </Text>
    </Box>
  )
}

function searchHits(output: unknown): SearchHit[] | undefined {
  const text = resultText(output)
  const hits = text ? parseJSON<SearchHit[]>(text) : undefined
  return Array.isArray(hits) ? hits : undefined
}

function inputQuery(input: unknown): string {
  return input && typeof input === 'object' && typeof (input as { query?: unknown }).query === 'string'
    ? (input as { query: string }).query
    : ''
}

export const register: Register = on => {
  // The previous sessions of this repository, above the prompt until the
  // first prompt is sent.
  on('session.start', async ($, e, next) => {
    const started = await next(e)
    // A refused name (another plugin's /recall) leaves the command out, not
    // the band.
    await $.command
      .register({
        name: 'recall',
        description: 'Search past sessions of this repository with claude-recall (--all for every one)',
        argumentHint: '<query> [--all] | <n>',
      })
      .catch((err: unknown) => $.ui.log(`claude-recall: /recall not registered: ${String(err)}`, { to: 'debug' }))
    const settings = await $.settings.read().catch(() => ({}))
    await update($, lang, () => langOf((settings as { language?: unknown }).language))
    if (!e.isInteractive) return started
    void (async () => {
      const [{ value: sessions }, id] = await Promise.all([
        recallJSON<ListedSession[]>($, ['list', '--repo', '.', '--format', 'json', '--limit', '50']),
        $.session.id(),
      ])
      const prev = previousSessions(sessions ?? [], id)
      await update($, omitted, () => prev.omitted)
      await update($, previous, () => (prev.shown.length ? prev.shown : null))
    })().catch(() => {})
    return started
  })

  on('prompt.submit', async ($, e, next) => {
    if (!(await read($, isBandHidden))) await update($, isBandHidden, () => true)
    return next(e)
  }).catch(($, e, next) => next(e))

  on('ui.render', { component: 'AbovePrompt' }, async ($, e, next) => {
    const prev = await read($, previous)
    if (!prev || (await read($, isBandHidden)) || e.props.hasSurvey) return next(e)
    if (e.surface !== 'terminal') return next(e)
    const { Box, Button, Text } = $.ui.resolve(e)
    const now = await $.clock.now()
    const w = words(await read($, lang))
    const left = await read($, omitted)
    // The border and its padding take two cells a side.
    const inner = (e.props.bodyColumns ?? e.viewport?.columns ?? 80) - 4
    return (
      <Box flexDirection="column" borderStyle="round" borderColor={BRAND} paddingX={1}>
        <Box flexDirection="row">
          <Box flexGrow={1}>
            {headerLine(
              { Text },
              [
                { text: 'recall', color: BRAND, bold: true },
                { text: ` ${w.bandTitle}`, bold: true },
                { text: ` ${w.bandHint}`, color: META },
                ...(left > 0 ? [{ text: ` · ${w.searchesOmitted(left)}`, color: META }] : []),
              ],
              '',
              inner - 8,
            )}
          </Box>
          <Button key="hide" plain label={w.close} onPress={() => update($, isBandHidden, () => true)} />
        </Box>
        {prev.map(s =>
          sessionRow(
            { Box, Button, Text },
            $,
            {
              id: s.sessionId,
              title: oneLine(s.displayTitle),
              place: placeOf({ repository: s.repository, worktree: s.worktree, branch: s.gitBranch }, true),
              when: relativeTime(s.endedAt ?? s.startedAt, now, w),
              msgs: s.messageCount,
            },
            [],
            inner,
            w,
          ),
        )}
      </Box>
    )
  })

  // recall_search results: the query of each call, for the drawing of a
  // standalone result row, which does not carry the call's input.
  on('tool.call', async ($, e, next) => {
    if (!isRecallTool(e.tool, 'recall_search')) return next(e)
    const query = typeof (e as { query?: unknown }).query === 'string' ? (e as { query: string }).query : ''
    await update($, queries, q => ({ ...q, [e.tool_use_id]: query }))
    return next(e)
  }).catch(($, e, next) => next(e))

  // The transcript folds MCP calls into one "Called ..." line. A run of
  // recall_search calls unfolds, so each draws as its own row below; the
  // ToolSearch that loads the tool's schema first may sit in the same run.
  on('ui.render', { component: 'ToolGroup' }, ($, e, next) => {
    const calls = e.props.calls
    const isSearch = (tool: string) => isRecallTool(tool, 'recall_search')
    const isSearchesOnly =
      calls.some(c => isSearch(c.tool)) &&
      calls.every(c => (isSearch(c.tool) || c.tool === 'ToolSearch') && !c.isRunning && !c.isErrored)
    if (e.props.isExpanded || !isSearchesOnly) return next(e)
    return next({ ...e, props: { ...e.props, isExpanded: true } })
  })

  // A recall_search row, unfolded or under ctrl+o: the call as one line
  // (the query, the counts), then its result as a tree. Only the drawing
  // changes; the model reads the JSON.
  on('ui.render', { component: 'ToolUse' }, async ($, e, next) => {
    const p = e.props
    if (!isRecallTool(p.tool, 'recall_search') || p.isRunning || p.isErrored || p.isInterrupted) return next(e)
    if (e.surface !== 'terminal') return next(e)
    const hits = searchHits(p.output)
    if (!hits) return next(e)
    const { Box, Button, Text } = $.ui.resolve(e)
    const query = inputQuery(p.input)
    const w = words(await read($, lang))
    const inner = (e.viewport?.columns ?? 100) - 4
    return (
      <Box flexDirection="column">
        {headerLine(
          { Text },
          [
            { text: '⏺ ', color: 'success' },
            { text: 'recall', color: BRAND },
            { text: ` ${query}`, color: MATCH },
            { text: `  ${w.sessions(groupHits(hits).length)} · ${w.hits(hits.length)}`, color: META },
          ],
          '',
          inner,
        )}
        {hits.length > 0 && <Box paddingLeft={2}>{searchBody($, { Box, Button, Text }, hits, query, inner - 2, w)}</Box>}
      </Box>
    )
  })

  on('ui.render', { component: 'ToolResult' }, async ($, e, next) => {
    if (!isRecallTool(e.props.tool, 'recall_search') || e.props.isErrored) return next(e)
    if (e.surface !== 'terminal') return next(e)
    const hits = searchHits(e.props.output)
    if (!hits) return next(e)
    const { Box, Button, Text } = $.ui.resolve(e)
    const query = (await read($, queries))[e.props.tool_use_id] ?? ''
    const w = words(await read($, lang))
    // The transcript indents a tool's result by about five cells.
    const inner = (e.viewport?.columns ?? 100) - 6
    return (
      <Box flexDirection="column">
        {headerLine(
          { Text },
          [{ text: `${w.sessions(groupHits(hits).length)} · ${w.hits(hits.length)}`, color: META }],
          '',
          inner,
        )}
        {hits.length > 0 && searchBody($, { Box, Button, Text }, hits, query, inner, w)}
      </Box>
    )
  })

  // /recall: search without a model turn. The text is the command's output
  // the model also reads (so "read number 2" works next), in English; the
  // terminal draws the same result as a table.
  on('command.run', { command: 'recall' }, async ($, e) => {
    const args = e.args.trim()
    if (args === '') return { text: `Usage: /${e.command} <query> [--all] | /${e.command} <n>` }

    if (/^\d+$/.test(args)) {
      const id = (await read($, found))[Number(args) - 1]
      if (!id) return { text: `There is no result ${args}. Search first with /${e.command} <query>.` }
      resume($, id)
      return { text: `Recapping ${shortId(id)}.` }
    }

    const isAll = /(^|\s)--all(\s|$)/.test(args)
    const query = args.replace(/(^|\s)--all(\s|$)/g, ' ').trim()
    // `--` keeps a query that starts with "-" from being read as a flag.
    const argv = ['search', '--format', 'json', '--limit', '1000', ...(isAll ? [] : ['--repo', '.']), '--', query]
    const [{ value, error }, current] = await Promise.all([recallJSON<SearchHit[]>($, argv), $.session.id()])
    if (error !== undefined) return { text: `recall search failed: ${error}` }
    const groups = groupHits((value ?? []).filter(hit => hit.sessionId !== current))
    // Sessions that look like a recall search go last, on one line in the
    // table; nothing is left out, and the numbers follow this order.
    const isSearch = (g: (typeof groups)[number]) => g.recallOnly
    const work = groups.filter(g => !isSearch(g))
    const searches = groups.filter(isSearch)
    const ordered = [...work, ...searches]
    await update($, found, () => ordered.map(g => g.sessionId))
    const scope = isAll ? 'all' : 'repo'
    const terms = queryTerms(query)
    const toRow = (g: (typeof groups)[number]): FoundRow => ({
      id: g.sessionId,
      title: g.title,
      place: placeOf(g, !isAll),
      at: g.date,
      msgs: g.msgs,
      hits: g.hits.length,
      snippet: snippet(g.hits[0]?.content ?? '', terms, 160),
    })
    const view: SearchView = {
      query,
      scope,
      total: groups.length,
      rows: work.slice(0, MAX_ROWS).map(toRow),
      searches: searches.map(g => g.sessionId),
      columns: e.presentation.columns,
    }
    await update($, views, v => ({ ...v, [args]: view }))

    const en = words('en')
    if (groups.length === 0) {
      return { text: `"${query}" · no sessions · ${en.scope[scope]}${isAll ? '' : ' (--all searches every session)'}` }
    }
    // The model reads every session, recall searches included, numbered as
    // `/recall <n>` takes them.
    const now = await $.clock.now()
    const lines = ordered
      .slice(0, MAX_TEXT_ROWS)
      .map(toRow)
      .map(
        (r, i) =>
          `${String(i + 1).padStart(2)}  ${shortId(r.id)}  ${r.title}  (${r.place} · ${relativeTime(r.at, now)} · ${en.hits(r.hits)})`,
      )
    const more = ordered.length > MAX_TEXT_ROWS ? [`    ${en.more(ordered.length - MAX_TEXT_ROWS)}`] : []
    return {
      text: [`"${query}" · ${en.sessions(groups.length)} · ${en.scope[scope]}`, '', ...lines, ...more].join('\n'),
    }
  })

  on('ui.render', { component: 'CommandOutput', props: { command: 'recall' } }, async ($, e, next) => {
    if (e.surface !== 'terminal' || e.props.isErrored) return next(e)
    const view = (await read($, views))[e.props.args.trim()]
    if (!view || view.total === 0) return next(e)
    const { Box, Button, Text } = $.ui.resolve(e)
    const terms = queryTerms(view.query)
    const w = words(await read($, lang))
    const now = await $.clock.now()
    // The command's row is indented by about four cells, the border and its
    // padding take two more a side.
    const inner = (view.columns ?? e.viewport?.columns ?? 100) - 8
    return (
      <Box flexDirection="column" borderStyle="round" borderColor={BRAND} paddingX={1}>
        <Box marginBottom={1}>
          {headerLine(
            { Text },
            [
              { text: 'recall', color: BRAND, bold: true },
              { text: ` ${view.query}`, bold: true },
              { text: ` · ${w.sessions(view.total)} · ${w.scope[view.scope]}`, color: META },
            ],
            w.idToResume,
            inner,
          )}
        </Box>
        {view.rows.map((r, i) =>
          sessionRow({ Box, Button, Text }, $, { ...r, n: i + 1, when: relativeTime(r.at, now, w) }, terms, inner, w),
        )}
        {view.searches.length > 0 && (
          <Box marginTop={view.rows.length > 0 ? 1 : 0}>
            {searchesLine({ Box, Button, Text }, $, '', view.searches, inner, w)}
          </Box>
        )}
        {view.total > view.rows.length + view.searches.length && (
          <Box marginTop={1}>
            <Text color={META}>{w.more(view.total - view.rows.length - view.searches.length)}</Text>
          </Box>
        )}
      </Box>
    )
  })
}
