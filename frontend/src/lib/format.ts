// Time and duration formatting helpers. All stored timestamps are local
// RFC3339 strings produced by the Go backend.

const pad = (n: number) => String(n).padStart(2, "0");

/** 3661 → "1:01:01" (mono clock display). */
export function formatClock(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${pad(m)}:${pad(sec)}`;
}

/** 3661 → "1时01分" / 45 → "45秒" (compact human form). */
export function formatDuration(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds));
  if (s < 60) return `${s}秒`;
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (h === 0) return `${m}分`;
  if (m === 0) return `${h}时`;
  return `${h}时${pad(m)}分`;
}

/** 3661 → "1 小时 1 分钟" (verbose form for stats labels). */
export function formatDurationLong(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (h > 0 && m > 0) return `${h} 小时 ${m} 分钟`;
  if (h > 0) return `${h} 小时`;
  return `${m} 分钟`;
}

/** Local date string "YYYY-MM-DD" for a Date. */
export function dateStr(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

export function todayStr(): string {
  return dateStr(new Date());
}

/** Date n days before today. */
export function daysAgoStr(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return dateStr(d);
}

/** "2025-09-05" → "9月5日"; optionally with weekday. */
export function formatDay(date: string, withWeekday = false): string {
  const d = new Date(date + "T00:00:00");
  if (Number.isNaN(d.getTime())) return date;
  const base = `${d.getMonth() + 1}月${d.getDate()}日`;
  if (!withWeekday) return base;
  return `${base} ${["周日", "周一", "周二", "周三", "周四", "周五", "周六"][d.getDay()]}`;
}

/** Local offset suffix like "+08:00". */
function tzSuffix(d: Date): string {
  const off = -d.getTimezoneOffset();
  const sign = off >= 0 ? "+" : "-";
  const abs = Math.abs(off);
  return `${sign}${pad(Math.floor(abs / 60))}:${pad(abs % 60)}`;
}

/** Date → local RFC3339 (matches the Go storage format). */
export function localRFC3339(d: Date): string {
  return (
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` +
    `T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}` +
    tzSuffix(d)
  );
}

/** Stored RFC3339 → value for <input type="datetime-local"> ("YYYY-MM-DDTHH:mm"). */
export function toLocalInput(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return `${dateStr(d)}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** datetime-local value → local RFC3339 (empty when invalid). */
export function fromLocalInput(value: string): string {
  if (!value) return "";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "";
  return localRFC3339(d);
}

/** Stored RFC3339 → "今天 14:30" / "9月4日 09:15". */
export function formatWhen(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const hm = `${pad(d.getHours())}:${pad(d.getMinutes())}`;
  if (dateStr(d) === todayStr()) return `今天 ${hm}`;
  return `${formatDay(dateStr(d))} ${hm}`;
}
