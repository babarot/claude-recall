// How times, sizes and counts show. Times are in the viewer's time zone.

import { t } from "./i18n";

const pad = (n: number) => String(n).padStart(2, "0");

export function hhmm(iso: string | null | undefined): string {
  if (!iso) return "";
  const d = new Date(iso);
  return isNaN(d.getTime()) ? "" : `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function dateTime(iso: string | null | undefined): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${hhmm(iso)}`;
}

function dayStart(d: Date): number {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
}

/** When a session ended, for the list: minutes ago, a time today, days ago, then a date. */
export function relative(iso: string | null | undefined, now: Date): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  const min = Math.round((now.getTime() - d.getTime()) / 60000);
  if (min < 1) return t.time.now;
  if (min < 60) return t.time.minsAgo(min);
  const days = Math.round((dayStart(now) - dayStart(d)) / 86400000);
  if (days === 0) return hhmm(iso);
  if (days === 1) return t.time.yesterday;
  if (days < 7) return t.time.daysAgo(days);
  if (d.getFullYear() === now.getFullYear()) return `${d.getMonth() + 1}/${d.getDate()}`;
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()}`;
}

/** The heading a session goes under in a list ordered by when sessions ended. */
export function dayGroup(iso: string | null | undefined, now: Date): string {
  if (!iso) return t.time.earlier;
  const days = Math.round((dayStart(now) - dayStart(new Date(iso))) / 86400000);
  if (days <= 0) return t.time.today;
  if (days === 1) return t.time.yesterdayGroup;
  if (days < 7) return t.time.thisWeek;
  return t.time.earlier;
}

export function size(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

/** How long between two times: "38s", "1m 12s", "2h 5m". */
export function duration(from: string | null | undefined, to: string | null | undefined): string {
  if (!from || !to) return "";
  const s = Math.round((new Date(to).getTime() - new Date(from).getTime()) / 1000);
  if (!(s >= 0)) return "";
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`;
  return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`;
}
