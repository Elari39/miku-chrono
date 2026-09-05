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

/** Typical measured width of the stats main card at the default window. */
const W = 595;

function bucket(date: string, total: number, byActivity?: Record<string, number>) {
  return { date, total, byActivity: byActivity ?? null };
}

function days(n: number) {
  return Array.from({ length: n }, (_, i) =>
    bucket(`2025-09-${String(i + 1).padStart(2, "0")}`, 60),
  );
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
    const { bars } = buildBars([bucket("2025-09-01", 300, { "1": 100, "2": 200 })], colors, names, {
      width: W,
    });
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
    const { bars } = buildBars([bucket("2025-09-01", 10, { "99": 10 })], colors, names, {
      width: W,
    });
    expect(bars[0].segments[0].color).toBe("#cc785c");
    expect(bars[0].segments[0].name).toBe("活动 99");
  });

  it("survives empty buckets and zero totals without division by zero", () => {
    const { bars, width, maxTotal } = buildBars([], colors, names, { width: 100 });
    expect(bars).toHaveLength(0);
    expect(width).toBe(240); // minimum drawing width
    expect(maxTotal).toBe(1);

    const zero = buildBars([bucket("2025-09-01", 0, {})], colors, names, { width: W });
    expect(zero.bars[0].segments).toHaveLength(0);
    expect(zero.bars[0].top).toBe(BAR_CHART_LAYOUT.H - BAR_CHART_LAYOUT.PAD_B);
  });

  it("skips zero-second segments instead of drawing 1px slivers", () => {
    const { bars } = buildBars([bucket("2025-09-01", 10, { "1": 10, "2": 0 })], colors, names, {
      width: W,
    });
    expect(bars[0].segments).toHaveLength(1);
    expect(bars[0].segments[0].secs).toBe(10);
  });

  it("centers every column inside the padded viewBox", () => {
    // Regression: a single-day chart used to clip the leading digit of its
    // axis label because the text centered on an unpadded bar edge.
    const { bars, width, colW, barW } = buildBars(
      [bucket("2025-09-05", 60, { "1": 60 })],
      colors,
      names,
      { width: W },
    );
    expect(bars[0].colX).toBe(BAR_CHART_LAYOUT.PAD_L);
    const center = bars[0].colX + colW / 2;
    expect(center - 20).toBeGreaterThanOrEqual(0); // room for a ~40px label
    expect(width).toBeGreaterThanOrEqual(center + 20);
    expect(bars[0].x).toBeCloseTo(bars[0].colX + (colW - barW) / 2, 6);
  });

  it("produces day labels by default and month labels in month mode", () => {
    const day = buildBars([bucket("2025-09-05", 60, { "1": 60 })], colors, names, { width: W });
    expect(day.bars[0].label).toBe("9月5日");
    expect(day.bars[0].tooltipTitle).toBe("9月5日 周五");

    const month = buildBars([bucket("2025-09", 60, { "1": 60 })], colors, names, {
      labelMode: "month",
      width: W,
    });
    expect(month.bars[0].label).toBe("9月");
    expect(month.bars[0].tooltipTitle).toBe("2025年9月");
  });

  it("flags when value labels should render", () => {
    // Threshold check mirrors the component's showValues computed.
    const one = buildBars([bucket("2025-09-01", 60, { "1": 60 })], colors, names, { width: W });
    const many = buildBars(
      Array.from({ length: VALUE_LABEL_MAX_BARS + 1 }, (_, i) =>
        bucket(`2025-09-${String(i + 1).padStart(2, "0")}`, 60),
      ),
      colors,
      names,
      { width: W },
    );
    expect(one.bars.length).toBeLessThanOrEqual(VALUE_LABEL_MAX_BARS);
    expect(many.bars.length).toBeGreaterThan(VALUE_LABEL_MAX_BARS);
  });
});

describe("buildBars fluid layout", () => {
  it("spreads a 7-day week across the full measured width", () => {
    const { bars, width, colW, barW, labelEvery } = buildBars(days(7), colors, names, { width: W });
    const avail = W - BAR_CHART_LAYOUT.PAD_L - BAR_CHART_LAYOUT.PAD_R;
    expect(colW).toBeCloseTo(avail / 7, 5);
    expect(barW).toBe(BAR_CHART_LAYOUT.MAX_BAR_W); // capped, not a blob
    expect(width).toBe(W); // exactly fills, nothing to scroll
    expect(labelEvery).toBe(1);
    // Last column ends right at the right padding edge.
    expect(bars.at(-1)!.colX + colW).toBeCloseTo(width - BAR_CHART_LAYOUT.PAD_R, 5);
  });

  it("spreads 12 months across the full measured width", () => {
    const { bars, width, colW, labelEvery } = buildBars(
      Array.from({ length: 12 }, (_, i) => bucket(`2025-${String(i + 1).padStart(2, "0")}`, 60)),
      colors,
      names,
      { labelMode: "month", width: W },
    );
    expect(colW).toBeCloseTo((W - BAR_CHART_LAYOUT.PAD_L - BAR_CHART_LAYOUT.PAD_R) / 12, 5);
    expect(width).toBe(W);
    expect(labelEvery).toBe(1);
    expect(bars).toHaveLength(12);
  });

  it("clamps a 31-day month to MIN_COL and reports the wider content width", () => {
    const { bars, width, colW, barW, labelEvery } = buildBars(days(31), colors, names, {
      width: W,
    });
    expect(colW).toBe(BAR_CHART_LAYOUT.MIN_COL);
    expect(barW).toBeCloseTo(BAR_CHART_LAYOUT.MIN_COL * BAR_CHART_LAYOUT.BAR_RATIO, 5);
    // Content exceeds the container → the component scrolls horizontally.
    expect(width).toBe(
      BAR_CHART_LAYOUT.PAD_L + BAR_CHART_LAYOUT.MIN_COL * 31 + BAR_CHART_LAYOUT.PAD_R,
    );
    expect(width).toBeGreaterThan(W);
    expect(bars[1].colX - bars[0].colX).toBe(BAR_CHART_LAYOUT.MIN_COL);
    expect(labelEvery).toBe(2); // 44px spacing still fits the ~35px labels
  });

  it("fits a 31-day month without scrolling when the card is wide enough", () => {
    const wide = 816; // single-column stats layout at the minimum window size
    const { width, colW } = buildBars(days(31), colors, names, { width: wide });
    expect(colW).toBeCloseTo((wide - BAR_CHART_LAYOUT.PAD_L - BAR_CHART_LAYOUT.PAD_R) / 31, 5);
    expect(colW).toBeGreaterThan(BAR_CHART_LAYOUT.MIN_COL);
    expect(width).toBe(wide);
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
