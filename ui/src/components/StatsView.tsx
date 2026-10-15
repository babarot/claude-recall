import { useEffect, useState } from "preact/hooks";
import { t } from "../lib/i18n";

interface Stats {
  totalSessions: number;
  totalMessages: number;
  byProject?: { project: string; sessions: number; messages: number }[];
  byMonth?: { month: string; sessions: number; messages: number }[];
}

/** Counts of sessions and messages, by folder and by month, from /api/stats. */
export function StatsView({ onBack }: { onBack: () => void }) {
  const [data, setData] = useState<Stats | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    fetch("/api/stats").then((r) => (r.ok ? r.json() : Promise.reject(new Error(String(r.status)))))
      .then(setData, (e) => setError(String(e.message ?? e)));
  }, []);
  const maxMonth = Math.max(1, ...(data?.byMonth ?? []).map((m) => m.sessions));
  const maxProject = Math.max(1, ...(data?.byProject ?? []).map((p) => p.sessions));
  return (
    <div class="stats-page">
      <div class="stats-head">
        <button class="btn" onClick={onBack}>{t.stats.back}</button>
        <h1>Stats</h1>
      </div>
      {error && <div class="list-empty">{t.stats.failed(error)}</div>}
      {!data && !error && <div class="list-empty">{t.stats.loading}</div>}
      {data && (
        <div class="done-grid">
          <div class="card"><h3>{t.stats.sessions}</h3><div class="big">{data.totalSessions.toLocaleString()}</div></div>
          <div class="card"><h3>{t.stats.messages}</h3><div class="big">{data.totalMessages.toLocaleString()}</div></div>
          <div class="card" style="grid-column:1/-1">
            <h3>{t.stats.byMonth}</h3>
            {(data.byMonth ?? []).map((m) => (
              <div class="bar-row" key={m.month}>
                <span class="k mono">{m.month}</span>
                <div><div class="bar" style={{ width: `${(m.sessions / maxMonth) * 100}%` }} /></div>
                <span class="v">{m.sessions}</span>
              </div>
            ))}
          </div>
          <div class="card" style="grid-column:1/-1">
            <h3>{t.stats.byFolder}</h3>
            {(data.byProject ?? []).map((p) => (
              <div class="bar-row wide" key={p.project}>
                <span class="k" title={p.project}>{p.project}</span>
                <div><div class="bar cmdb" style={{ width: `${(p.sessions / maxProject) * 100}%` }} /></div>
                <span class="v">{p.sessions}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
