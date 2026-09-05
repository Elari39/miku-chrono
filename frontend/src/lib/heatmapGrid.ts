// Pure heatmap-grid math extracted from components/Heatmap.vue so the
// weekday alignment / month marking / level bucketing logic is testable.
import { dateStr, todayStr } from "./format";

export const HEATMAP_CELL = 13;
export const HEATMAP_GAP = 3;
export const HEATMAP_PAD_L = 24;
export const HEATMAP_PAD_T = 16;

export const HEATMAP_LEVEL_COLORS = ["#efe9de", "#e3b49f", "#d99a7c", "#d08658", "#cc785c"];
export const HEATMAP_WEEKDAY_LABELS = ["一", "", "三", "", "五", "", "日"];

export interface HeatmapCell {
  date: string;
  secs: number;
  level: 0 | 1 | 2 | 3 | 4;
  x: number;
  y: number;
  future: boolean;
}

export interface HeatmapGrid {
  cells: HeatmapCell[];
  monthMarks: { x: number; label: string }[];
  width: number;
  height: number;
}

/** 0 = none, 1-4 = intensity buckets relative to the max day. */
export function heatmapLevel(secs: number, maxSecs: number): HeatmapCell["level"] {
  if (secs <= 0 || maxSecs <= 0) return 0;
  const ratio = secs / maxSecs;
  if (ratio > 0.66) return 4;
  if (ratio > 0.33) return 3;
  if (ratio > 0.1) return 2;
  return 1;
}

/**
 * Build the cell grid for `weeks` columns whose LAST column is the week
 * containing `today` (rows run Monday→Sunday, GitHub style). Months are
 * marked at the first Monday of each new month.
 */
export function buildHeatmapGrid(
  days: Record<string, number>,
  weeks: number,
  today: string = todayStr(),
): HeatmapGrid {
  const end = new Date(today + "T00:00:00");
  // Monday of the current week (row 0), then walk back (weeks-1) columns.
  const monday = new Date(end);
  monday.setDate(monday.getDate() - ((monday.getDay() + 6) % 7));
  const start = new Date(monday);
  start.setDate(start.getDate() - (weeks - 1) * 7);

  const maxSecs = Math.max(60, ...Object.values(days));
  const cells: HeatmapCell[] = [];
  const monthMarks: { x: number; label: string }[] = [];
  let lastMonth = -1;

  const cur = new Date(start);
  for (let w = 0; w < weeks; w++) {
    for (let d = 0; d < 7; d++) {
      const ds = dateStr(cur);
      const future = ds > today;
      const secs = days[ds] ?? 0;
      cells.push({
        date: ds,
        secs,
        level: future ? 0 : heatmapLevel(secs, maxSecs),
        x: HEATMAP_PAD_L + w * (HEATMAP_CELL + HEATMAP_GAP),
        y: HEATMAP_PAD_T + d * (HEATMAP_CELL + HEATMAP_GAP),
        future,
      });
      if (d === 0 && cur.getMonth() !== lastMonth && !future) {
        lastMonth = cur.getMonth();
        monthMarks.push({
          x: HEATMAP_PAD_L + w * (HEATMAP_CELL + HEATMAP_GAP),
          label: `${lastMonth + 1}月`,
        });
      }
      cur.setDate(cur.getDate() + 1);
    }
  }
  return {
    cells,
    monthMarks,
    width: HEATMAP_PAD_L + weeks * (HEATMAP_CELL + HEATMAP_GAP),
    height: HEATMAP_PAD_T + 7 * (HEATMAP_CELL + HEATMAP_GAP) + 18,
  };
}
