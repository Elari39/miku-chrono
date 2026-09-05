// Pure SVG bar-chart layout math for the statistics views. Extracted from
// components/BarChart.vue so it stays unit-testable.
import { formatDay, formatDuration } from "./format";

export const BAR_CHART_LAYOUT = {
  H: 200,
  /** Left/right padding keep centered axis labels inside the viewBox —
   *  a single-day chart used to clip the leading "9" of "9月5日". */
  PAD_L: 24,
  PAD_R: 34,
  /** Top padding leaves room for the always-visible value labels. */
  PAD_T: 20,
  PAD_B: 22,
  /** Columns fill the measured container width but never squeeze below
   *  this — a 31-day month keeps ~22px columns and scrolls horizontally
   *  instead of turning into unreadable slivers. */
  MIN_COL: 22,
  /** Bar width as a fraction of the column (26→14 in the old fixed grid),
   *  capped so the 7-bar week doesn't turn into blobs. */
  BAR_RATIO: 0.56,
  MAX_BAR_W: 40,
} as const;

/** Bars at or below this count get a formatted total printed above them. */
export const VALUE_LABEL_MAX_BARS = 16;

/** Drawing widths below this fall back to it (0px during mount, jsdom). */
const MIN_DRAW_W = 240;

export interface BarSegment {
  secs: number;
  color: string;
  name: string;
  y: number;
  h: number;
}

export interface Bar {
  date: string;
  /** Left edge of the whole column (hit area + label anchor). */
  colX: number;
  /** Left edge of the colored bar rect (column-centered). */
  x: number;
  total: number;
  /** y of the topmost segment (= value-label anchor); baseline when empty. */
  top: number;
  /** Short axis label: "9月5日" / "9月". */
  label: string;
  /** Longer tooltip heading: "9月5日 周五" / "2025年9月". */
  tooltipTitle: string;
  segments: BarSegment[];
}

export function labelStep(n: number): number {
  if (n <= 14) return 1;
  if (n <= 31) return 2;
  return 5;
}

/** Candidate gridline steps in seconds, roughly one per power-of-two-ish. */
const TICK_STEPS = [
  60, 300, 900, 1800, 3600, 7200, 14400, 28800, 86400, 172800, 432000, 864000, 1728000, 2592000,
];

/** 2-3 "nice" gridline values (0, step, 2·step, …) below maxTotal. */
export function niceTicks(maxTotal: number): number[] {
  if (maxTotal <= 0) return [];
  const raw = maxTotal / 3;
  let step = TICK_STEPS.at(-1) ?? 2592000;
  for (const s of TICK_STEPS) {
    if (s >= raw) {
      step = s;
      break;
    }
  }
  const ticks: number[] = [];
  for (let v = step; v <= maxTotal; v += step) ticks.push(v);
  return ticks;
}

/** Compact tick caption: 30分 / 2时 / 5天. */
export function formatTick(secs: number): string {
  if (secs >= 86400) {
    const d = secs / 86400;
    return `${d % 1 === 0 ? d : d.toFixed(1)}天`;
  }
  return formatDuration(secs);
}

export interface BucketInput {
  date: string;
  total: number;
  /** Wails-generated models type bucket values as `number | undefined`;
   *  undefined entries are treated as absent. */
  byActivity?: { [key: string]: number | undefined } | null;
}

export function buildBars(
  buckets: BucketInput[],
  colors: Record<string, string>,
  names: Record<string, string>,
  opts: { labelMode?: "day" | "month"; width: number },
) {
  const { H, PAD_L, PAD_R, PAD_T, PAD_B, MIN_COL, MAX_BAR_W, BAR_RATIO } = BAR_CHART_LAYOUT;
  const labelMode = opts.labelMode ?? "day";
  const drawW = Math.max(opts.width, MIN_DRAW_W);

  if (buckets.length === 0) {
    return { bars: [], width: drawW, maxTotal: 1, labelEvery: 1, colW: 0, barW: 0 };
  }

  // Columns stretch to fill the measured width; only a clamp at MIN_COL can
  // push the content wider than the container (→ horizontal scroll there).
  // Round so float noise (537/7·7) never leaks into the SVG width attribute.
  const n = buckets.length;
  const colW = Math.max(MIN_COL, (drawW - PAD_L - PAD_R) / n);
  const barW = Math.min(MAX_BAR_W, colW * BAR_RATIO);
  const width = Math.max(drawW, Math.round(PAD_L + colW * n + PAD_R));
  const maxTotal = Math.max(1, ...buckets.map((b) => b.total));
  const usable = H - PAD_T - PAD_B;
  const labelEvery = labelStep(n);

  const bars: Bar[] = buckets.map((b, i) => {
    const colX = PAD_L + i * colW;
    const x = colX + (colW - barW) / 2;
    let y = H - PAD_B;
    const segments: BarSegment[] = Object.entries(b.byActivity ?? {})
      .map(([activityId, secs]) => ({ activityId, secs: secs ?? 0 }))
      .filter((s) => s.secs > 0)
      .sort((a, b2) => b2.secs - a.secs)
      .map(({ activityId, secs }) => {
        const h = (secs / maxTotal) * usable;
        y -= h;
        return {
          secs,
          color: colors[activityId] ?? "#cc785c",
          name: names[activityId] ?? `活动 ${activityId}`,
          y,
          h,
        };
      });

    let label: string;
    let tooltipTitle: string;
    if (labelMode === "month") {
      const month = Number(b.date.slice(5, 7));
      label = `${month}月`;
      tooltipTitle = `${b.date.slice(0, 4)}年${month}月`;
    } else {
      label = formatDay(b.date);
      tooltipTitle = formatDay(b.date, true);
    }

    return {
      date: b.date,
      colX,
      x,
      total: b.total,
      top: y,
      label,
      tooltipTitle,
      segments,
    };
  });

  return { bars, width, maxTotal, labelEvery, colW, barW };
}
