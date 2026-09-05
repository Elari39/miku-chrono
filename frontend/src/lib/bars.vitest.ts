import { describe, expect, it } from "vitest";
import { buildBars, labelStep, BAR_CHART_LAYOUT } from "./bars";
import type { DayBucket } from "./api";

const colors = { "1": "#cc785c", "2": "#5db8a6" };
const names = { "1": "阅读", "2": "健身" };

function bucket(date: string, total: number, byActivity?: Record<string, number>): DayBucket {
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
    // Taller segment sits directly above the shorter one.
    const [first, second] = bars[0].segments;
    expect(second.y < first.y).toBe(true);
    // Heights are proportional to the max total.
    expect(first.h).toBeCloseTo(
      (100 / 300) * (BAR_CHART_LAYOUT.H - BAR_CHART_LAYOUT.PAD_T - BAR_CHART_LAYOUT.PAD_B),
      5,
    );
    expect(second.h).toBeCloseTo(
      (200 / 300) * (BAR_CHART_LAYOUT.H - BAR_CHART_LAYOUT.PAD_T - BAR_CHART_LAYOUT.PAD_B),
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
  });

  it("spaces columns evenly and sets label cadence", () => {
    const days = Array.from({ length: 30 }, (_, i) =>
      bucket(`2025-09-${String(i + 1).padStart(2, "0")}`, 60),
    );
    const { bars } = buildBars(days, colors, names);
    expect(bars.length).toBe(30);
    expect(bars[1].x - bars[0].x).toBe(BAR_CHART_LAYOUT.COL);
    expect(bars[0].labelEvery).toBe(2);
  });
});
