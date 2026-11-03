import { useEffect, useLayoutEffect, useRef, useState } from "preact/hooks";
import { signal } from "@preact/signals";
import type { ComponentChildren } from "preact";
import { t } from "../lib/i18n";
import {
  FLAGS, LENGTHS, METRICS, OTHER, PERIODS, WORKS, breakdown, buckets, change, filtered, hours, lengths, median, niceAxis, passes, range,
  ranks, seriesOf, sortSessions, total, within, workShares,
} from "../lib/stats";
import type { Breakdown, Filter, Line, Metric, Row, Series, SessionSort } from "../lib/stats";
import * as st from "../store/stats";

// The Stats page: the filters on the left, everything they narrow on the
// right. Every chart, table and count reads the same filtered sessions, and
// pressing a bar, a legend entry, a row or a cell narrows them further.

const s = t.stats;

const num = (n: number) => Math.round(n).toLocaleString(t.lang);
const compact = (n: number) =>
  n >= 1e9 ? `${(n / 1e9).toFixed(1)}B` : n >= 1e6 ? `${(n / 1e6).toFixed(1)}M` : n >= 1e4 ? `${(n / 1e3).toFixed(1)}K` : num(n);
const money = (n: number) => `$${n >= 100 ? num(n) : n.toFixed(n >= 10 ? 1 : 2)}`;
const minutes = (m: number) => (m >= 60 ? `${(m / 60).toFixed(1)}h` : `${Math.round(m)}m`);

export function fmt(m: Metric, v: number): string {
  switch (m) {
    case "hours": return `${v < 10 ? v.toFixed(1) : num(v)}h`;
    case "tokens": case "calls": return compact(v);
    case "cost": return money(v);
    case "failRate": return `${(v * 100).toFixed(1)}%`;
    default: return num(v);
  }
}

const day = (d: Date) => `${d.getMonth() + 1}/${d.getDate()}`;

/** The tooltip, shared by every chart: a title, colored rows, and what a press does. */
interface Tip { x: number; y: number; title: string; rows?: [string, string, string][]; hint?: string }
const tip = signal<Tip | null>(null);
const showTip = (e: MouseEvent, v: Omit<Tip, "x" | "y">) => { tip.value = { x: e.clientX, y: e.clientY, ...v }; };
const hideTip = () => { tip.value = null; };

function Tooltip() {
  const ref = useRef<HTMLDivElement>(null);
  const v = tip.value;
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el || !v) return;
    el.style.left = `${Math.min(v.x + 14, innerWidth - el.offsetWidth - 8)}px`;
    el.style.top = `${Math.min(v.y + 14, innerHeight - el.offsetHeight - 8)}px`;
  });
  if (!v) return null;
  return (
    <div class="st-tip" ref={ref}>
      <div class="tl">{v.title}</div>
      {v.rows?.map(([name, value, color]) => (
        <div class="row" key={name}><i style={{ background: color }} /><span class="tl">{name}</span><span class="tv">{value}</span></div>
      ))}
      {v.hint && <div class="hint">{v.hint}</div>}
    </div>
  );
}

/** The width of an element, followed as it changes. */
function useWidth(): [preact.RefObject<HTMLDivElement>, number] {
  const ref = useRef<HTMLDivElement>(null);
  const [w, setW] = useState(0);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setW(el.clientWidth));
    ro.observe(el);
    setW(el.clientWidth);
    return () => ro.disconnect();
  }, []);
  return [ref, w];
}

export function StatsView({ onOpenSession, onOpenFolder }: { onOpenSession: (id: string) => void; onOpenFolder: (key: string) => void }) {
  useEffect(() => {
    void st.load();
  }, []);
  // The state in the URL, for back and reload.
  useEffect(() => {
    const q = st.urlQuery();
    if (location.search !== q) history.replaceState(null, "", `/stats${q}`);
  });

  const data = st.data.value;
  if (st.error.value) return <div class="stats-page"><div class="list-empty">{s.failed(st.error.value)}</div></div>;
  if (!data) return <div class="stats-page"><div class="list-empty">{s.loading}</div></div>;

  const now = new Date();
  const all = st.rows.value;
  const f = st.filter.value;
  const days = st.period.value;
  const cur = all.filter((r) => within(r, range(days, now)));
  const prev = all.filter((r) => within(r, range(days, now, 1)));
  const curF = cur.filter((r) => passes(r, f));
  const prevF = prev.filter((r) => passes(r, f));
  const ranked = ranks(all, now);

  return (
    <div class="stats-page" onMouseLeave={hideTip}>
      <Facets cur={cur} ranked={ranked} now={now} />
      <div class="st-canvas">
        <Kpis cur={curF} prev={prevF} now={now} />
        <MainChart cur={curF} prev={prevF} ranked={ranked} now={now} />
        <div class="st-row3">
          <Panel title={s.hoursTitle} hint={s.hoursHint}><Hours rows={cur.filter((r) => passes(r, f, "slot"))} /></Panel>
          <Panel title={s.lengthTitle} hint={s.lengthHint}><Lengths rows={curF} /></Panel>
          <Panel title={s.workTitle} hint={s.workHint}><WorkPanel cur={curF} prev={prevF} /></Panel>
        </div>
        <BreakdownTable cur={curF} prev={prevF} ranked={ranked} />
        <Sessions rows={curF} ranked={ranked} onOpen={onOpenSession} onOpenFolder={onOpenFolder} />
        {data.unknownTokens > 0 && <div class="st-note">{s.unknownTokens(data.unknownTokens)}</div>}
      </div>
      <Tooltip />
    </div>
  );
}

function Panel({ title, hint, right, children }: { title: ComponentChildren; hint?: string; right?: ComponentChildren; children: ComponentChildren }) {
  return (
    <div class="st-panel">
      <div class="st-ph"><h3>{title}</h3>{hint && <span class="hint">{hint}</span>}{right && <div class="right">{right}</div>}</div>
      <div class="st-pb">{children}</div>
    </div>
  );
}

function Seg<T extends string | number>({ value, options, onPick }: { value: T; options: [T, string][]; onPick: (v: T) => void }) {
  return (
    <div class="seg">
      {options.map(([v, label]) => <button key={String(v)} class={v === value ? "on" : ""} onClick={() => onPick(v)}>{label}</button>)}
    </div>
  );
}

/** How many repositories the filters list before "show all". */
const FACET_TOP = 10;

function Facets({ cur, ranked, now }: { cur: Row[]; ranked: ReturnType<typeof ranks>; now: Date }) {
  const [allRepos, setAllRepos] = useState(false);
  const f = st.filter.value;
  const [from, to] = range(st.period.value, now);
  const last = new Date(to.getTime() - 1);
  const count = (kind: keyof Filter, test: (r: Row) => boolean) => cur.filter((r) => passes(r, f, kind) && test(r)).length;
  const group = <K extends "repo" | "model" | "flag" | "weekday">(kind: K, all: [Filter[K][number], string][], test: (r: Row, v: Filter[K][number]) => boolean, sorted = false) => {
    let items = all.map(([v, label]): [Filter[K][number], string, number] => [v, label, count(kind, (r) => test(r, v))]);
    // Repositories and models by how many sessions of the period they have, the chosen ones kept.
    if (sorted) items = items.filter(([v, , n]) => n > 0 || (f[kind] as unknown[]).includes(v)).sort((a, b) => b[2] - a[2]);
    const hidden = kind === "repo" && !allRepos ? Math.max(0, items.length - FACET_TOP) : 0;
    if (hidden) items = items.filter(([v], i) => i < FACET_TOP || (f[kind] as unknown[]).includes(v));
    const counts = items.map((x) => x[2]);
    const max = Math.max(1, ...counts);
    return (
      <div class={`st-fgroup ${kind === "flag" || kind === "weekday" ? "collapsible" : ""}`}>
        <h4>{s.facets[kind]}{(f[kind] as unknown[]).length > 0 && <button onClick={() => st.clear(kind)}>{s.clear}</button>}</h4>
        {items.map(([v, label], i) => (
          <div key={String(v)} class={`st-fi ${(f[kind] as unknown[]).includes(v) ? "on" : ""} ${counts[i] ? "" : "zero"}`} onClick={() => st.toggle(kind, v)}>
            <span class="box" /><span class="nm" title={label}>{label}</span><span class="ct">{counts[i]}</span>
            <span class="mini" style={{ width: `${(counts[i] / max) * 100}%` }} />
          </div>
        ))}
        {kind === "repo" && (hidden > 0 || allRepos) && (
          <button class="st-all" onClick={() => setAllRepos(!allRepos)}>{allRepos ? s.showLess : s.showAll(all.length)}</button>
        )}
      </div>
    );
  };
  const chips: [keyof Filter, string | number, string][] = [
    ...f.repo.map((v): [keyof Filter, string, string] => ["repo", v, ranked.repo.find((x) => x.key === v)?.name ?? v]),
    ...f.model.map((v): [keyof Filter, string, string] => ["model", v, v]),
    ...f.flag.map((v): [keyof Filter, string, string] => ["flag", v, s.flags[v]]),
    ...f.weekday.map((v): [keyof Filter, number, string] => ["weekday", v, s.weekdays[v]]),
    ...(f.slot ? [["slot", 0, s.slot(s.weekdays[f.slot[0]], f.slot[1])] as [keyof Filter, number, string]] : []),
  ];
  // Every repository and model of the period, named as the year ranks them.
  const repoItems = [...new Map(cur.map((r) => [r.s.groupKey, r.s.groupName])).entries()];
  for (const k of f.repo) if (!repoItems.some(([x]) => x === k)) repoItems.push([k, ranked.repo.find((x) => x.key === k)?.name ?? k]);
  const modelItems = [...new Set([...cur.map((r) => r.s.model ?? "").filter(Boolean), ...f.model])].map((m): [string, string] => [m, m]);
  return (
    <aside class="st-facets">
      <div class="st-fgroup">
        <h4>{s.period}</h4>
        <Seg value={st.period.value} options={PERIODS.map((p) => [p, s.periodName(p)])} onPick={(p) => { st.period.value = p; }} />
        <div class="range">{s.range(`${from.getFullYear()}/${day(from)}`, day(last), st.period.value)}</div>
      </div>
      <div class="st-chips">
        {chips.length === 0 ? <span class="none">{s.noFilter}</span> : (
          <>
            {chips.map(([kind, v, label]) => (
              <span class="chip" key={`${kind}${v}`}>
                <i>{s.facets[kind]}</i>{label}
                <button onClick={() => (kind === "slot" ? st.clear("slot") : st.toggle(kind as "repo", v as string))}>×</button>
              </span>
            ))}
            <button class="chip clear" onClick={() => st.clear()}>{s.clearAll}</button>
          </>
        )}
      </div>
      {group("repo", repoItems, (r, v) => r.s.groupKey === v, true)}
      {group("model", modelItems, (r, v) => r.s.model === v, true)}
      {group("flag", FLAGS.map((x) => [x, s.flags[x]]), (r, v) => r.flags.includes(v))}
      {group("weekday", [1, 2, 3, 4, 5, 6, 0].map((d) => [d, s.weekdays[d]]), (r, v) => r.start.getDay() === v)}
    </aside>
  );
}

function Spark({ values, color }: { values: number[]; color: string }) {
  const max = Math.max(1e-9, ...values);
  const n = Math.max(1, values.length - 1);
  return (
    <svg viewBox="0 0 100 22" preserveAspectRatio="none" class="spark">
      <polyline points={values.map((v, i) => `${((i / n) * 100).toFixed(1)},${(21 - (v / max) * 19).toFixed(1)}`).join(" ")}
        fill="none" stroke={color} stroke-width="1.5" vector-effect="non-scaling-stroke" />
    </svg>
  );
}

function Delta({ cur, prev, m }: { cur: number; prev: number; m: Metric }) {
  const c = change(cur, prev, m);
  if (!c) return null;
  // Up is worse for the cost and the failure rate, and only says so in color.
  const worse = c.up && (m === "cost" || m === "failRate");
  return <span class={worse ? "worse" : c.up ? "up" : ""}>{c.up ? "▲" : "▼"} {c.text}</span>;
}

function Kpis({ cur, prev, now }: { cur: Row[]; prev: Row[]; now: Date }) {
  const bs = buckets(cur, prev, st.period.value, st.grain.value, now, "sessions", "none", seriesOf("none", { repo: [], model: [] }, s.works, s.other));
  return (
    <div class="st-panel st-kpis">
      {METRICS.map((m) => {
        const series = bs.map((b) => total(b.rows, m));
        return (
          <button key={m} class={`st-kpi ${st.metric.value === m ? "on" : ""}`} onClick={() => { st.metric.value = m; }}>
            <div class="l">{s.metrics[m]}</div>
            <div class="v">{fmt(m, total(cur, m))}</div>
            <div class="d"><Delta cur={total(cur, m)} prev={total(prev, m)} m={m} /> {s.before} {fmt(m, total(prev, m))}</div>
            <Spark values={series} color={m === "failRate" ? "var(--warn)" : "var(--accent)"} />
          </button>
        );
      })}
    </div>
  );
}

function MainChart({ cur, prev, ranked, now }: { cur: Row[]; prev: Row[]; ranked: ReturnType<typeof ranks>; now: Date }) {
  const [ref, width] = useWidth();
  const m = st.metric.value, g = st.group.value;
  const rate = m === "failRate";
  const series: Series[] = rate ? [{ key: "", name: s.metrics.failRate, color: "var(--warn)" }] : seriesOf(g, ranked, s.works, s.other);
  const bs = buckets(cur, prev, st.period.value, st.grain.value, now, m, rate ? "none" : g, series);
  const W = Math.max(200, width), H = 220, L = 50, R = 6, T = 8, B = 22;
  const axis = niceAxis(Math.max(0, ...bs.map((b) => b.parts.reduce((a, v) => a + v, 0)), ...(st.showPrev.value ? bs.map((b) => b.prev) : [])));
  const max = axis.top;
  const band = (W - L - R) / bs.length, bw = Math.max(3, Math.min(24, band - 2));
  const y = (v: number) => T + (1 - v / max) * (H - T - B);
  // A date label every so many bars, about one per 60 pixels.
  const every = Math.ceil(bs.length / Math.max(2, Math.floor((W - L) / 60)));
  const clickable = !rate && (g === "repo" || g === "model");
  const pick = (key: string) => {
    if (!clickable || key === "\0other") return;
    st.toggle(g as "repo", key);
  };
  return (
    <Panel
      title={s.trend(s.metrics[m])}
      right={<label class="ctl"><input type="checkbox" checked={st.showPrev.value} onChange={(e) => { st.showPrev.value = (e.target as HTMLInputElement).checked; }} /> {s.comparePrev}</label>}
      hint={undefined}>
      <div class="st-controls">
        <span class="ctl">{s.groupBy} <Seg value={g} options={(["none", "repo", "model", "work"] as const).map((x) => [x, s.groups[x]])} onPick={(x) => { st.group.value = x; }} /></span>
        <span class="ctl">{s.grainBy} <Seg value={st.grain.value} options={(["day", "week"] as const).map((x) => [x, s.grains[x]])} onPick={(x) => { st.grain.value = x; }} /></span>
      </div>
      {series.length > 1 || st.showPrev.value ? (
        <div class="st-legend">
          {series.length > 1 && series.map((x) => (
            <span key={x.key} class={clickable && x.key !== "\0other" ? "pick" : ""} onClick={() => pick(x.key)}><i style={{ background: x.color }} />{x.name}</span>
          ))}
          {st.showPrev.value && <span><i class="ln" />{s.prevTotal}</span>}
        </div>
      ) : null}
      <div ref={ref} class="st-chart">
        {width > 0 && (
          <svg viewBox={`0 0 ${W} ${H}`} height={H} width="100%"
            onMouseMove={(e) => {
              const box = (e.currentTarget as SVGSVGElement).getBoundingClientRect();
              const i = Math.floor(((e.clientX - box.left) * (W / box.width) - L) / band);
              const b = bs[i];
              if (!b) return hideTip();
              const rows: [string, string, string][] = series.map((x, j): [string, string, string] => [x.name || s.metrics[m], fmt(m, b.parts[j] ?? 0), x.color]).filter((_, j) => (b.parts[j] ?? 0) > 0);
              if (st.showPrev.value) rows.push([s.before, fmt(m, b.prev), "var(--dim)"]);
              const target = e.target as SVGElement;
              showTip(e, { title: `${day(b.from)}${st.grain.value === "week" ? `–${day(new Date(b.to.getTime() - 1))}` : ""} · ${s.nSessions(b.rows.length)}`, rows, hint: target.dataset.key ? s.clickToFilter : undefined });
            }}
            onMouseLeave={hideTip}>
            {Array.from({ length: Math.round(max / axis.step) + 1 }, (_, k) => {
              const v = axis.step * k;
              return <g key={k}><line class="grid" x1={L} x2={W - R} y1={y(v)} y2={y(v)} /><text x={L - 6} y={y(v) + 3} text-anchor="end">{fmt(m, v)}</text></g>;
            })}
            {bs.map((b, i) => {
              const x = L + band * i + (band - bw) / 2;
              let acc = 0;
              const lastIdx = b.parts.reduce((li, v, j) => (v > 0 ? j : li), -1);
              return (
                <g key={i}>
                  {b.parts.map((v, j) => {
                    if (v <= 0) return null;
                    const y1 = y(acc + v), h = y(acc) - y1 - (acc > 0 ? 2 : 0);
                    acc += v;
                    const key = series[j]?.key ?? "";
                    return <rect key={j} x={x} y={y1} width={bw} height={Math.max(0.5, h)} rx={j === lastIdx ? 3 : 0} fill={series[j]?.color ?? OTHER}
                      class={clickable && key !== "\0other" ? "pick" : ""} data-key={clickable && key !== "\0other" ? key : undefined}
                      onClick={() => pick(key)} />;
                  })}
                  {i % every === 0 && <text x={x + bw / 2} y={H - 6} text-anchor="middle">{day(b.from)}</text>}
                </g>
              );
            })}
            {st.showPrev.value && (
              <path class="prev" d={bs.map((b, i) => `${i ? "L" : "M"}${(L + band * i + band / 2).toFixed(1)},${y(b.prev).toFixed(1)}`).join("")} />
            )}
          </svg>
        )}
      </div>
    </Panel>
  );
}

function Hours({ rows }: { rows: Row[] }) {
  const grid = hours(rows);
  const nz = grid.flat().filter((v) => v > 0).sort((a, b) => a - b);
  const q = (p: number) => nz[Math.min(nz.length - 1, Math.floor(nz.length * p))] ?? 1;
  const level = (v: number) => (v <= 0 ? 0 : v <= q(0.25) ? 1 : v <= q(0.5) ? 2 : v <= q(0.75) ? 3 : 4);
  const slot = st.filter.value.slot;
  return (
    <div class="st-hours">
      <span />
      {Array.from({ length: 24 }, (_, h) => <span key={h} class="hl">{h % 3 ? "" : h}</span>)}
      {[1, 2, 3, 4, 5, 6, 0].map((d) => [
        <span key={`d${d}`} class="dl">{s.weekdays[d]}</span>,
        ...grid[d].map((v, h) => (
          <span key={`${d}-${h}`} class={`cell ${slot && slot[0] === d && slot[1] === h ? "sel" : ""}`} data-q={level(v)}
            onClick={() => st.setSlot(d, h)}
            onMouseMove={(e) => showTip(e, { title: s.slot(s.weekdays[d], h), rows: [[s.nSessions(v), "", "transparent"]], hint: s.clickToFilter })}
            onMouseLeave={hideTip} />
        )),
      ])}
    </div>
  );
}

function Lengths({ rows }: { rows: Row[] }) {
  const [ref, width] = useWidth();
  const ns = lengths(rows);
  const W = Math.max(160, width), H = 128, L = 26, R = 4, T = 4, B = 20;
  const max = Math.max(1, ...ns), band = (W - L - R) / ns.length, bw = Math.min(24, band - 4);
  const y = (v: number) => T + (1 - v / max) * (H - T - B);
  return (
    <div ref={ref}>
      {width > 0 && (
        <svg viewBox={`0 0 ${W} ${H}`} height={H} width="100%">
          <line class="grid" x1={L} x2={W - R} y1={y(0)} y2={y(0)} />
          <text x={L - 4} y={y(max) + 4} text-anchor="end">{max}</text>
          {ns.map((n, i) => {
            const x = L + band * i + (band - bw) / 2;
            return (
              <g key={i}>
                <rect x={x} y={y(n)} width={bw} height={y(0) - y(n)} rx={3} fill="var(--accent)"
                  onMouseMove={(e) => showTip(e, { title: s.lengthBars[i], rows: [[s.nSessions(n), "", "transparent"]] })} onMouseLeave={hideTip} />
                <text x={x + bw / 2} y={H - 5} text-anchor="middle">{s.lengthBars[i]}</text>
              </g>
            );
          })}
        </svg>
      )}
      <div class="st-sub">{s.median(minutes(median(rows.map((r) => r.s.activeMinutes))), rows.filter((r) => r.s.activeMinutes >= LENGTHS[4][0]).length)}</div>
    </div>
  );
}

const WORK_COLORS = ["var(--s1)", "var(--s2)", "var(--s3)", "var(--s4)", "var(--s5)", OTHER];

function WorkPanel({ cur, prev }: { cur: Row[]; prev: Row[] }) {
  const now = workShares(cur), before = workShares(prev);
  return (
    <>
      <div class="st-wbar">
        {WORKS.map((w, i) => now[w] > 0 && (
          <span key={w} style={{ width: `${now[w] * 100}%`, background: WORK_COLORS[i] }}
            onMouseMove={(e) => showTip(e, { title: s.works[w], rows: [[s.workTools[w], `${Math.round(now[w] * 100)}%`, WORK_COLORS[i]]] })} onMouseLeave={hideTip} />
        ))}
      </div>
      <div class="st-wlist">
        {WORKS.map((w, i) => {
          const d = Math.round(now[w] * 100) - Math.round(before[w] * 100);
          return [
            <i key={`${w}i`} style={{ background: WORK_COLORS[i] }} />,
            <span key={`${w}n`}>{s.works[w]}<span class="tn">{s.workTools[w]}</span></span>,
            <span key={`${w}v`} class="n">{Math.round(now[w] * 100)}%</span>,
            <span key={`${w}d`} class="dd">{d === 0 ? "±0" : `${d > 0 ? "▲" : "▼"}${Math.abs(d)}`}pt</span>,
          ];
        })}
      </div>
    </>
  );
}

const TABLES: Breakdown[] = ["repo", "tool", "program", "file", "model"];

function BreakdownTable({ cur, prev, ranked }: { cur: Row[]; prev: Row[]; ranked: ReturnType<typeof ranks> }) {
  const kind = st.table.value;
  const lines = breakdown(kind, cur, prev).slice(0, 15);
  const f = st.filter.value;
  const cols: [string, (v: number) => string, ((l: Line) => string)?][] =
    kind === "repo" || kind === "model"
      ? [[s.cols.hours, (v) => fmt("hours", v)], [s.cols.sessions, num], [s.cols.tokens, compact], [s.cols.cost, money], [s.cols.failRate, (v) => fmt("failRate", v), (l) => (l.values[4] > 0.04 ? "worse" : "")]]
      : kind === "tool"
        ? [[s.cols.calls, compact], [s.cols.failed, num], [s.cols.failRate, (v) => fmt("failRate", v), (l) => (l.values[2] > 0.04 ? "worse" : "")], [s.cols.prevRate, (v) => fmt("failRate", v), () => "muted"]]
        : kind === "program" ? [[s.cols.runs, num]] : [[s.cols.total, num], [s.cols.read, num], [s.cols.edited, num]];
  const first = lines[0]?.values[0] ?? 1;
  const colorOf = (l: Line) => (kind === "repo" ? ranked.repo : kind === "model" ? ranked.model : []).find((x) => x.key === l.key)?.color;
  const on = (l: Line) => (l.filter?.repo ? f.repo.includes(l.filter.repo) : l.filter?.model ? f.model.includes(l.filter.model) : false);
  return (
    <div class="st-panel">
      <div class="st-ph">
        <div class="tabs">{TABLES.map((k) => <button key={k} class={`tab ${k === kind ? "on" : ""}`} onClick={() => { st.table.value = k; }}>{s.tables[k]}</button>)}</div>
        <span class="right hint">{kind === "repo" || kind === "model" ? s.tableHint : kind === "tool" ? s.toolHint : ""}</span>
      </div>
      <div class="st-scroll">
        <table class="st-t">
          <thead><tr><th>{s.tables[kind]}</th>{cols.map(([c], i) => <th key={c} class={`n ${i ? "hs" : ""}`}>{c}</th>)}<th class="n">{s.cols.change}</th></tr></thead>
          <tbody>
            {lines.map((l) => (
              <tr key={l.key} class={`${l.filter ? "pick" : ""} ${on(l) ? "on" : ""}`}
                onClick={() => { if (l.filter?.repo) st.toggle("repo", l.filter.repo); else if (l.filter?.model) st.toggle("model", l.filter.model); }}>
                <td class={`k ${kind === "repo" ? "" : "mono"}`}>{colorOf(l) && <span class="sw" style={{ background: colorOf(l) }} />}{l.name}</td>
                {cols.map(([c, show, cls], i) => (
                  <td key={c} class={`n ${i ? "hs" : ""} ${cls?.(l) ?? ""}`}>
                    {show(l.values[i])}{i === 0 && <span class="shr" style={{ width: `${Math.max(2, (l.values[0] / first) * 60)}px` }} />}
                  </td>
                ))}
                <td class="n"><Delta cur={l.values[0]} prev={l.prev} m="calls" /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

const SORTS: SessionSort[] = ["cost", "failed", "length", "recent"];

function Sessions({ rows, ranked, onOpen, onOpenFolder }: { rows: Row[]; ranked: ReturnType<typeof ranks>; onOpen: (id: string) => void; onOpenFolder: (key: string) => void }) {
  const list = sortSessions(rows, st.sessionSort.value);
  const shown = list.slice(0, st.limit.value);
  const f = st.filter.value;
  // The list can show the same sessions only when they are narrowed by one repository.
  const onlyRepo = f.repo.length === 1 && filtered(f) && f.model.length + f.flag.length + f.weekday.length === 0 && !f.slot;
  return (
    <Panel title={s.sessionsTitle} hint={s.matching(list.length)}
      right={<span class="ctl">{s.sortBy} <Seg value={st.sessionSort.value} options={SORTS.map((x) => [x, s.sorts[x]])} onPick={(x) => { st.sessionSort.value = x; }} /></span>}>
      <div class="st-scroll">
        <table class="st-t">
          <thead><tr>
            <th>{s.sessCols.session}</th><th class="hs">{s.sessCols.repo}</th><th class="hs">{s.sessCols.marks}</th>
            <th class="n">{s.sessCols.length}</th><th class="n hs">{s.sessCols.failed}</th><th class="n hs">{s.sessCols.tokens}</th>
            <th class="n">{s.sessCols.cost}</th><th class="n hs">{s.sessCols.date}</th>
          </tr></thead>
          <tbody>
            {shown.map((r) => (
              <tr key={r.s.id} class="pick" onClick={() => onOpen(r.s.id)}>
                <td class="title">{r.s.title} <span class="id mono">{r.s.id.slice(0, 8)}</span></td>
                <td class="k hs"><span class="sw" style={{ background: ranked.repo.find((x) => x.key === r.s.groupKey)?.color ?? OTHER }} />{r.s.groupName}</td>
                <td class="hs">{r.flags.map((x) => <span key={x} class={`tag ${x === "failing" ? "bad" : ""}`}>{s.tags[x]}</span>)}</td>
                <td class="n">{minutes(r.s.activeMinutes)}</td>
                <td class={`n hs ${r.failed >= 10 ? "worse" : ""}`}>{r.failed}</td>
                <td class="n hs">{compact(r.tokens)}</td>
                <td class="n" title={r.s.costPartial ? s.partial : undefined}>{money(r.s.cost)}{r.s.costPartial ? "*" : ""}</td>
                <td class="n hs muted">{day(r.start)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div class="st-more">
        {list.length > shown.length && <>{s.more(list.length - shown.length)} · <button onClick={() => { st.limit.value += 20; }}>{s.showMore}</button></>}
        {onlyRepo && <>{list.length > shown.length && " · "}<button onClick={() => onOpenFolder(f.repo[0])}>{s.openInList}</button></>}
      </div>
    </Panel>
  );
}
