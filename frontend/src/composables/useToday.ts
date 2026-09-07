import { ref } from "vue";
import { todayStr } from "../lib/format";

/** Milliseconds from now until the next local midnight (clamped to ≥ 1s so a
 * call placed exactly at 00:00:00.000 never schedules a past-due timeout). */
export function msUntilNextMidnight(now: Date): number {
  const next = new Date(now);
  next.setHours(24, 0, 0, 0);
  return Math.max(1000, next.getTime() - now.getTime());
}

// Module-level singleton: every view shares one reactive local date that
// rolls over at midnight. Views watch it to refresh their "今日" figures —
// without it, an app left open past 00:00 keeps showing yesterday's numbers
// until the next timer event or remount.
export const today = ref(todayStr());

function scheduleRollover() {
  window.setTimeout(() => {
    today.value = todayStr();
    scheduleRollover();
  }, msUntilNextMidnight(new Date()));
}
scheduleRollover();

// A suspended webview (system sleep) can miss the scheduled update entirely;
// re-read the date whenever the page becomes visible again.
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible") today.value = todayStr();
});

/** Reactive local date ("YYYY-MM-DD"), shared by every view; rolls over at
 * midnight and re-checks on visibility. */
export function useToday() {
  return { today };
}
