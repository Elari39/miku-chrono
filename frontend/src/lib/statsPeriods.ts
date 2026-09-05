// Pure period/aggregation math for the statistics page's four granularities
// (每日/每周/每月/每年). No Vue or Wails imports so it stays trivially
// testable; local "YYYY-MM-DD" date strings go in and out.
import { dateStr } from "./format";

export type Granularity = "day" | "week" | "month" | "year";

/** Tab order for the granularity switcher. */
export const GRANULARITY_ORDER: Granularity[] = ["day", "week", "month", "year"];

/** 每日/每周/每月/每年 — tab captions. */
export const GRANULARITY_LABELS: Record<Granularity, string> = {
  day: "每日",
  week: "每周",
  month: "每月",
  year: "每年",
};

/**
 * Normalized aggregation bucket shared by the week/month views (per-day,
 * from DailyStacked) and the year view (per-month, from MonthlyStacked).
 */
export interface StatBucket {
  /** "YYYY-MM-DD" (day/week/month views) or "YYYY-MM" (year view). */
  date: string;
  total: number;
  /** Wails-generated models type bucket values as `number | undefined`;
   *  undefined entries are treated as absent. */
  byActivity: { [key: string]: number | undefined };
}

const MS_DAY = 86_400_000;
const DAY_MIN = 24 * 60;

function parseDay(s: string): Date {
  return new Date(s + "T00:00:00");
}

function addDays(s: string, n: number): string {
  const d = parseDay(s);
  d.setDate(d.getDate() + n);
  return dateStr(d);
}

/** Monday of the week containing the given local date. */
export function weekStartOf(date: string): string {
  const d = parseDay(date);
  d.setDate(d.getDate() - ((d.getDay() + 6) % 7));
  return dateStr(d);
}

export function monthStartOf(date: string): string {
  return date.slice(0, 8) + "01";
}

export function yearStartOf(date: string): string {
  return date.slice(0, 5) + "01-01";
}

/** Inclusive start date of the period containing `anchor`. */
export function periodStart(gran: Granularity, anchor: string): string {
  if (gran === "day") return anchor;
  if (gran === "week") return weekStartOf(anchor);
  if (gran === "month") return monthStartOf(anchor);
  return yearStartOf(anchor);
}

/** Inclusive end date of the period containing `anchor`. */
export function periodEnd(gran: Granularity, anchor: string): string {
  if (gran === "day") return anchor;
  const start = periodStart(gran, anchor);
  if (gran === "week") return addDays(start, 6);
  const d = parseDay(start);
  if (gran === "month") {
    d.setMonth(d.getMonth() + 1);
    d.setDate(0); // last day of the month
    return dateStr(d);
  }
  d.setMonth(11);
  d.setDate(31);
  return dateStr(d);
}

/** Shift the anchor by ±1 period; the result lands on the new period's start. */
export function shiftPeriod(gran: Granularity, anchor: string, delta: number): string {
  if (gran === "day") return addDays(anchor, delta);
  const start = periodStart(gran, anchor);
  if (gran === "week") return addDays(start, delta * 7);
  if (gran === "month") {
    // start is always day 01, so month arithmetic never rolls over.
    const d = parseDay(start);
    d.setMonth(d.getMonth() + delta);
    return dateStr(d);
  }
  return `${Number(start.slice(0, 4)) + delta}-01-01`;
}

/** ISO-8601 week number of the week containing the given local date. */
export function isoWeekNumber(date: string): number {
  // Move to this week's Thursday; the Thursday's year owns the ISO year and
  // Jan 4 always sits inside ISO week 1.
  const d = parseDay(date);
  d.setDate(d.getDate() + 3 - ((d.getDay() + 6) % 7));
  const week1 = new Date(d.getFullYear(), 0, 4);
  const diffDays = (d.getTime() - week1.getTime()) / MS_DAY;
  return 1 + Math.round((diffDays - 3 + ((week1.getDay() + 6) % 7)) / 7);
}

function isoWeekYear(date: string): number {
  const d = parseDay(date);
  d.setDate(d.getDate() + 3 - ((d.getDay() + 6) % 7));
  return d.getFullYear();
}

/** Human-readable caption for the period: 2025年9月5日 周五 / …第37周（…）/ 2025年9月 / 2025年. */
export function periodLabel(gran: Granularity, anchor: string): string {
  if (gran === "day") {
    const d = parseDay(anchor);
    const wd = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"][d.getDay()];
    return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日 ${wd}`;
  }
  if (gran === "week") {
    const start = periodStart("week", anchor);
    const s = parseDay(start);
    const e = parseDay(periodEnd("week", anchor));
    const week = isoWeekNumber(start);
    return `${isoWeekYear(start)}年第${week}周（${s.getMonth() + 1}/${s.getDate()} – ${e.getMonth() + 1}/${e.getDate()}）`;
  }
  if (gran === "month") {
    const d = parseDay(anchor);
    return `${d.getFullYear()}年${d.getMonth() + 1}月`;
  }
  return `${anchor.slice(0, 4)}年`;
}

/** Enumerate every local date from..to inclusive (from <= to required). */
export function enumerateDays(from: string, to: string): string[] {
  const out: string[] = [];
  for (let d = parseDay(from), end = parseDay(to); d <= end; d.setDate(d.getDate() + 1)) {
    out.push(dateStr(d));
  }
  return out;
}

/**
 * Fill gaps so every day between from..to carries a bucket (zero when the
 * backend returned nothing for that day). Sparse SQL output must not
 * compress the chart's date axis.
 */
export function fillDailyBuckets(
  from: string,
  to: string,
  buckets: {
    date: string;
    total: number;
    byActivity?: { [key: string]: number | undefined } | null;
  }[],
): StatBucket[] {
  const map = new Map(buckets.map((b) => [b.date, b]));
  return enumerateDays(from, to).map((d) => {
    const b = map.get(d);
    return { date: d, total: b?.total ?? 0, byActivity: { ...(b?.byActivity ?? {}) } };
  });
}

/** Fill all 12 months of `year`, zero-padded, preserving backend per-activity maps. */
export function fillMonthBuckets(
  year: number,
  buckets: {
    month: string;
    total: number;
    byActivity?: { [key: string]: number | undefined } | null;
  }[],
): StatBucket[] {
  const map = new Map(buckets.map((b) => [b.month, b]));
  return Array.from({ length: 12 }, (_, i) => {
    const m = `${year}-${String(i + 1).padStart(2, "0")}`;
    const b = map.get(m);
    return { date: m, total: b?.total ?? 0, byActivity: { ...(b?.byActivity ?? {}) } };
  });
}

/** The bucket with the largest total, or null when everything is zero. */
export function peakBucket(buckets: StatBucket[]): StatBucket | null {
  let best: StatBucket | null = null;
  for (const b of buckets) {
    if (b.total > 0 && (!best || b.total > best.total)) best = b;
  }
  return best;
}

/**
 * Days from period start through min(today, period end), inclusive; 0 when
 * the period has not begun yet. Used to折算日均 without counting future days.
 */
export function elapsedDaysInPeriod(gran: Granularity, anchor: string, today: string): number {
  const start = periodStart(gran, anchor);
  const end = periodEnd(gran, anchor);
  if (today < start) return 0;
  return enumerateDays(start, today < end ? today : end).length;
}

export interface TimelineSegment {
  activityId: number;
  color: string;
  name: string;
  /** Minutes from 00:00 of the day. */
  startMin: number;
  /** Minutes from 00:00, clamped to 1440 — cross-midnight entries stop here. */
  endMin: number;
  /** Seconds spent inside this day (startMin..endMin). */
  secs: number;
}

/** Parse an RFC3339 local timestamp into minutes-from-midnight. */
function minutesOfDay(iso: string): number {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return 0;
  return d.getHours() * 60 + d.getMinutes() + d.getSeconds() / 60;
}

/**
 * Convert one day's entries into 0..1440 minute segments sorted by start.
 * Records belong to their start day (backend semantics), so anything running
 * past midnight is clamped to 24:00 of this day's strip.
 */
export function buildTimelineSegments(
  entries: {
    activityId: number;
    activityColor: string;
    activityName: string;
    startedAt: string;
    durationSeconds: number;
  }[],
): TimelineSegment[] {
  return entries
    .map((e) => {
      const startMin = Math.min(DAY_MIN, Math.max(0, minutesOfDay(e.startedAt)));
      const rawEnd = startMin + Math.max(0, e.durationSeconds) / 60;
      const endMin = Math.min(DAY_MIN, rawEnd);
      return {
        activityId: e.activityId,
        color: e.activityColor || "#cc785c",
        name: e.activityName || `活动 ${e.activityId}`,
        startMin,
        endMin,
        secs: Math.max(0, Math.min(e.durationSeconds, Math.round((DAY_MIN - startMin) * 60))),
      };
    })
    .filter((s) => s.endMin > s.startMin)
    .sort((a, b) => a.startMin - b.startMin || a.endMin - b.endMin);
}
