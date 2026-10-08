// The words the drawings show, in English unless Claude Code's `language`
// setting names Japanese. What the model reads (a command's output) stays
// English whatever the setting.

export type Lang = 'en' | 'ja'

// The language of Claude Code's `language` setting, which people write as
// they like ("japanese", "Japanese", "日本語", "ja").
export function langOf(setting: unknown): Lang {
  return typeof setting === 'string' && /^(ja|japanese|日本語)/i.test(setting.trim()) ? 'ja' : 'en'
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

const en = {
  bandTitle: 'Pick up where you left off',
  bandHint: '(press an ID to resume)',
  close: 'Close',
  sessions: (n: number) => plural(n, 'session', 'sessions'),
  hits: (n: number) => plural(n, 'hit', 'hits'),
  more: (n: number) => plural(n, 'more session', 'more sessions'),
  msgs: (n: number) => `${n} msgs`,
  searches: (n: number) => plural(n, 'recall search', 'recall searches'),
  searchesOmitted: (n: number) => `${plural(n, 'recall search', 'recall searches')} left out`,
  pressToResume: 'press an ID to resume',
  idToResume: 'ID to resume',
  noRecap: "recall's recap prompt is not available here; update recall to resume a session",
  scope: { repo: 'this repository', all: 'all' },
  justNow: 'just now',
  minutesAgo: (n: number) => `${n}m ago`,
  hoursAgo: (n: number) => `${n}h ago`,
  daysAgo: (n: number) => `${n}d ago`,
}

const ja: typeof en = {
  bandTitle: '前回の続き',
  bandHint: '(ID を押すと再開できます)',
  close: '閉じる',
  sessions: n => `${n} セッション`,
  hits: n => `${n} 件`,
  more: n => `ほか ${n} セッション`,
  msgs: n => `${n} msgs`,
  searches: n => `recall の検索 ${n} 件`,
  searchesOmitted: n => `recall の検索 ${n} 件は省略`,
  pressToResume: 'ID を押すと再開',
  idToResume: 'ID で再開',
  noRecap: 'recall の recap prompt が見つかりません。セッションを再開するには recall を更新してください',
  scope: { repo: 'このリポジトリ', all: 'すべて' },
  justNow: 'たった今',
  minutesAgo: n => `${n}分前`,
  hoursAgo: n => `${n}時間前`,
  daysAgo: n => `${n}日前`,
}

export type Words = typeof en

export const words = (lang: Lang): Words => (lang === 'ja' ? ja : en)

// How long ago `iso` was, in the drawing's words; a month and day past a week.
export function relativeTime(iso: string, now: number, w: Words = en): string {
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return ''
  const min = Math.round((now - t) / 60000)
  if (min < 1) return w.justNow
  if (min < 60) return w.minutesAgo(min)
  const hours = Math.round(min / 60)
  if (hours < 24) return w.hoursAgo(hours)
  const days = Math.round(hours / 24)
  if (days < 7) return w.daysAgo(days)
  const d = new Date(t)
  return `${d.getMonth() + 1}/${d.getDate()}`
}
