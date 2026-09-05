import { describe, expect, it } from "vitest";
import {
  buildHeatmapGrid,
  heatmapLevel,
  HEATMAP_CELL,
  HEATMAP_GAP,
  HEATMAP_PAD_L,
  HEATMAP_PAD_T,
} from "./heatmapGrid";

describe("heatmapLevel", () => {
  it("returns 0 for no data or empty max", () => {
    expect(heatmapLevel(0, 100)).toBe(0);
    expect(heatmapLevel(50, 0)).toBe(0);
  });

  it("buckets by ratio of the max day", () => {
    expect(heatmapLevel(1, 100)).toBe(1); // <= 0.1
    expect(heatmapLevel(10, 100)).toBe(1); // exact 0.1 boundary is low
    expect(heatmapLevel(11, 100)).toBe(2); // > 0.1
    expect(heatmapLevel(33, 100)).toBe(2); // exact 0.33 boundary is low
    expect(heatmapLevel(34, 100)).toBe(3); // > 0.33
    expect(heatmapLevel(66, 100)).toBe(3); // 0.66 is not above 0.66
    expect(heatmapLevel(67, 100)).toBe(4);
    expect(heatmapLevel(100, 100)).toBe(4);
  });
});

describe("buildHeatmapGrid", () => {
  it("ends the last column in the week containing today, rows on Mondays", () => {
    // 2025-09-05 is a Friday; its Monday is 2025-09-01.
    const grid = buildHeatmapGrid({}, 2, "2025-09-05");
    expect(grid.cells).toHaveLength(14);
    expect(grid.cells[0].date).toBe("2025-08-25"); // Monday, one column back
    expect(grid.cells[13].date).toBe("2025-09-07"); // Sunday of today's week
    // Column x positions step by CELL + GAP.
    expect(grid.cells[7].x).toBe(HEATMAP_PAD_L + (HEATMAP_CELL + HEATMAP_GAP));
    // Row y positions step by CELL + GAP.
    expect(grid.cells[1].y).toBe(HEATMAP_PAD_T + (HEATMAP_CELL + HEATMAP_GAP));
  });

  it("includes today and marks later days of the week as future", () => {
    const grid = buildHeatmapGrid({ "2025-09-05": 3600 }, 1, "2025-09-05");
    const today = grid.cells.find((c) => c.date === "2025-09-05");
    expect(today).toBeDefined();
    expect(today!.future).toBe(false);
    expect(today!.level).toBe(4); // only day with data → max

    const future = grid.cells.find((c) => c.date === "2025-09-06");
    expect(future).toBeDefined();
    expect(future!.future).toBe(true);
    expect(future!.level).toBe(0);
  });

  it("annotates the first Monday of each new month", () => {
    // Back window 2025-08-25 (Mon) … 2025-09-07 (Sun) spans two months.
    const grid = buildHeatmapGrid({}, 2, "2025-09-05");
    expect(grid.monthMarks).toEqual([
      { x: HEATMAP_PAD_L, label: "8月" },
      { x: HEATMAP_PAD_L + HEATMAP_CELL + HEATMAP_GAP, label: "9月" },
    ]);
  });

  it("computes sane dimensions", () => {
    const grid = buildHeatmapGrid({}, 17, "2025-09-05");
    expect(grid.width).toBe(HEATMAP_PAD_L + 17 * (HEATMAP_CELL + HEATMAP_GAP));
    expect(grid.height).toBe(HEATMAP_PAD_T + 7 * (HEATMAP_CELL + HEATMAP_GAP) + 18);
  });

  it("handles an empty data map without crashing", () => {
    const grid = buildHeatmapGrid({}, 17);
    expect(grid.cells.length).toBe(119);
    expect(grid.cells.every((c) => c.secs === 0)).toBe(true);
  });
});
