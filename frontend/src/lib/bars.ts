// Pure stacked-bar layout math extracted from components/BarChart.vue so the
// segment stacking / label-spacing logic is testable.
import type { DayBucket } from "./api";

export const BAR_CHART_LAYOUT = {
  COL: 26,
  BAR_W: 14,
  H: 200,
  PAD_T: 10,
  PAD_B: 22,
} as const;

export interface BarSegment {
  x: number;
  y: number;
  w: number;
  h: number;
  color: string;
  name: string;
  secs: number;
}

export interface Bar {
  x: number;
  total: number;
  date: string;
  labelEvery: number;
  segments: BarSegment[];
}

/** How often to draw a date label under the bars, based on bucket count. */
export function labelStep(n: number): number {
  if (n <= 14) return 1;
  if (n <= 31) return 2;
  return 5;
}

export interface BarLayout {
  bars: Bar[];
  width: number;
  /** Largest single-day total (at least 1, so zero-data renders safely). */
  maxTotal: number;
}

/** Compute rects for every day's stacked segments, bottom-up per day. */
export function buildBars(
  buckets: DayBucket[],
  colors: Record<string, string>,
  names: Record<string, string>,
): BarLayout {
  const { COL, BAR_W, H, PAD_T, PAD_B } = BAR_CHART_LAYOUT;
  const width = Math.max(buckets.length * COL + 8, 120);
  const maxTotal = Math.max(1, ...buckets.map((b) => b.total));

  const bars = buckets.map((b, i) => {
    const x = 4 + i * COL;
    let y = H - PAD_B;
    const byActivity = b.byActivity ?? {};
    const segments = Object.keys(byActivity).map((actId) => {
      const secs = byActivity[actId] ?? 0;
      const h = (secs / maxTotal) * (H - PAD_T - PAD_B);
      const seg: BarSegment = {
        x,
        y: y - h,
        w: BAR_W,
        h: Math.max(h, 1),
        color: colors[actId] ?? "#cc785c",
        name: names[actId] ?? `活动 ${actId}`,
        secs,
      };
      y -= h;
      return seg;
    });
    return {
      x,
      total: b.total,
      date: b.date,
      labelEvery: labelStep(buckets.length),
      segments,
    };
  });

  return { bars, width, maxTotal };
}
