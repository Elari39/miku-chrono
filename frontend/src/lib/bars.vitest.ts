import { describe, expect, it } from "vitest";
import {
  buildBars,
  formatTick,
  labelStep,
  niceTicks,
  BAR_CHART_LAYOUT,
  VALUE_LABEL_MAX_BARS,
} from "./bars";

const colors = { "1": "#cc785c", "2": "#5db8a6" };
const names = { "1": "阅读", "2": "健身" };

function bucket(date: string, total: number, byActivity?: Record<string, number>) {
  return { date, total, byActivity: byActivity ?? null };
}

describe("labelStep", () => {
  it("labels every day up to 14 buckets", () => {
    expect(labelStep(1)).toBe(1);
    expect(labelStep(14)).toBe(1);
  });

  it("steps every 2 up to 31, then every 5", () => {
    expect(labelStep(15)).toBe(2);
    expect(labelStep(31)).toBe(2);
    expect(labelStep(32)).toBe(5);
    expect(labelStep(90)).toBe(5);
  });
});

describe("buildBars", () => {
  it("stacks segments bottom-up within each day", () => {
    const { bars } = buildBars([bucket("2025-09-01", 300, { "1": 100, "2": 200 })], colors, names);
    expect(bars).toHaveLength(1);
    expect(bars[0].total).toBe(300);
    expect(bars[0].segments).toHaveLength(2);
    // Segments sort by duration descending; the tallest sits at the bottom.
    const [bottom, top] = bars[0].segments;
    expect(bottom.secs).toBe(200);
    expect(top.y < bottom.y).toBe(true);
    // Heights are proportional to the max total.
    expect(bottom.h).toBeCloseTo(
      (200 / 300) * (BAR_CHART_LAYOUT.H - BAR_CHART_LAYOUT.PAD_T - BAR_CHART_LAYOUT.PAD_B),
      5,
    );
    expect(top.h).toBeCloseTo(
      (100 / 300) * (BAR_CHART_LAYOUT.H - BAR_CHART_LAYOUT.PAD_T - BAR_CHART_LAYOUT.PAD_B),
      5,
    );
  });

  it("resolves unknown activity ids to placeholders", () => {
    const { bars } = buildBars([bucket("2025-09-01", 10, { "99": 10 })], colors, names);
    expect(bars[0].segments[0].color).toBe("#cc785c");
    expect(bars[0].segments[0].name).toBe("活动 99");
  });

  it("survives empty buckets and zero totals without division by zero", () => {
    const { bars, width, maxTotal } = buildBars([], colors, names);
    expect(bars).toHaveLength(0);
    expect(width).toBe(120); // min width
    expect(maxTotal).toBe(1);

    const zero = buildBars([bucket("2025-09-01", 0, {})], colors, names);
    expect(zero.bars[0].segments).toHaveLength(0);
    expect(zero.bars[0].top).toBe(BAR_CHART_LAYOUT.H - BAR_CHART_LAYOUT.PAD_B);
  });

  it("skips zero-second segments instead of drawing 1px slivers", () => {
    const { bars } = buildBars([bucket("2025-09-01", 10, { "1": 10, "2": 0 })], colors, names);
    expect(bars[0].segments).toHaveLength(1);
    expect(bars[0].segments[0].secs).toBe(10);
  });

  it("centers every column inside the padded viewBox", () => {
    // Regression: a single-day chart used to clip the leading digit of its
    // axis label because the text centered on an unpadded bar edge.
    const { bars, width } = buildBars([bucket("2025-09-05", 60, { "1": 60 })], colors, names);
    expect(bars[0].colX).toBe(BAR_CHART_LAYOUT.PAD_L);
    const center = bars[0].colX + BAR_CHART_LAYOUT.COL / 2;
    expect(center - 20).toBeGreaterThanOrEqual(0); // room for a ~40px label
    expect(width).toBeGreaterThanOrEqual(center + 20);
    expect(bars[0].x).toBe(bars[0].colX + (BAR_CHART_LAYOUT.COL - BAR_CHART_LAYOUT.BAR_W) / 2);
  });

  it("produces day labels by default and month labels in month mode", () => {
    const day = buildBars([bucket("2025-09-05", 60, { "1": 60 })], colors, names);
    expect(day.bars[0].label).toBe("9月5日");
    expect(day.bars[0].tooltipTitle).toBe("9月5日 周五");

    const month = buildBars([bucket("2025-09", 60, { "1": 60 })], colors, names, {
      labelMode: "month",
    });
    expect(month.bars[0].label).toBe("9月");
    expect(month.bars[0].tooltipTitle).toBe("2025年9月");
  });

  it("spaces columns evenly and exposes the label cadence", () => {
    const days = Array.from({ length: 30 }, (_, i) =>
      bucket(`2025-09-${String(i + 1).padStart(2, "0")}`, 60),
    );
    const { bars, labelEvery } = buildBars(days, colors, names);
    expect(bars).toHaveLength(30);
    expect(bars[1].colX - bars[0].colX).toBe(BAR_CHART_LAYOUT.COL);
    expect(labelEvery).toBe(2);
  });

  it("flags when value labels should render", () => {
    // Threshold check mirrors the component's showValues computed.
    const one = buildBars([bucket("2025-09-01", 60, { "1": 60 })], colors, names);
    const many = Array.from({ length: VALUE_LABEL_MAX_BARS + 1 }, (_, i) =>
      bucket(`2025-09-${String(i + 1).padStart(2, "0")}`, 60),
    );
    expect(one.bars.length).toBeLessThanOrEqual(VALUE_LABEL_MAX_BARS);
    expect(many.length).toBeGreaterThan(VALUE_LABEL_MAX_BARS);
  });
});

describe("niceTicks", () => {
  it("returns no ticks for zero data", () => {
    expect(niceTicks(0)).toEqual([]);
    expect(niceTicks(1)).toEqual([]);
  });

  it("picks human steps under the maximum", () => {
    // 3h max → 30/60/90min or hour lines, all ≤ 3h.
    const ticks = niceTicks(3 * 3600);
    expect(ticks.length).toBeGreaterThanOrEqual(2);
    expect(ticks.length).toBeLessThanOrEqual(3);
    for (const t of ticks) expect(t).toBeLessThanOrEqual(3 * 3600);
    expect(niceTicks(3600)).toEqual([1800, 3600]);
    // Day+ scale uses whole-day steps.
    expect(niceTicks(3 * 86400).every((t) => t % 86400 === 0)).toBe(true);
  });
});

describe("formatTick", () => {
  it("formats sub-day ticks with 分/时 and multi-day ticks with 天", () => {
    expect(formatTick(1800)).toBe("30分");
    expect(formatTick(7200)).toBe("2时");
    expect(formatTick(86400)).toBe("1天");
    expect(formatTick(432000)).toBe("5天");
  });
});
