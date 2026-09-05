import { describe, expect, it } from "vitest";
import {
  buildTimelineSegments,
  elapsedDaysInPeriod,
  enumerateDays,
  fillDailyBuckets,
  fillMonthBuckets,
  isoWeekNumber,
  peakBucket,
  periodEnd,
  periodLabel,
  periodStart,
  shiftPeriod,
  type StatBucket,
} from "./statsPeriods";

describe("periodStart / periodEnd", () => {
  it("day period is the anchor itself", () => {
    expect(periodStart("day", "2025-09-05")).toBe("2025-09-05");
    expect(periodEnd("day", "2025-09-05")).toBe("2025-09-05");
  });

  it("week starts on Monday and ends on Sunday", () => {
    expect(periodStart("week", "2025-09-10")).toBe("2025-09-08"); // Wednesday → Monday
    expect(periodEnd("week", "2025-09-10")).toBe("2025-09-14"); // → Sunday
    expect(periodStart("week", "2025-09-08")).toBe("2025-09-08"); // already Monday
    expect(periodStart("week", "2025-09-07")).toBe("2025-09-01"); // Sunday belongs to previous week
  });

  it("month handles short and leap months", () => {
    expect(periodStart("month", "2025-09-05")).toBe("2025-09-01");
    expect(periodEnd("month", "2025-09-05")).toBe("2025-09-30");
    expect(periodEnd("month", "2025-02-10")).toBe("2025-02-28");
    expect(periodEnd("month", "2024-02-10")).toBe("2024-02-29"); // leap year
    expect(periodEnd("month", "2025-12-31")).toBe("2025-12-31");
  });

  it("year spans Jan 1 to Dec 31", () => {
    expect(periodStart("year", "2025-09-05")).toBe("2025-01-01");
    expect(periodEnd("year", "2025-09-05")).toBe("2025-12-31");
  });
});

describe("shiftPeriod", () => {
  it("shifts days", () => {
    expect(shiftPeriod("day", "2025-09-05", 1)).toBe("2025-09-06");
    expect(shiftPeriod("day", "2025-09-01", -1)).toBe("2025-08-31");
  });

  it("shifts weeks by whole weeks from Monday", () => {
    expect(shiftPeriod("week", "2025-09-10", 1)).toBe("2025-09-15");
    expect(shiftPeriod("week", "2025-09-10", -1)).toBe("2025-09-01");
    expect(shiftPeriod("week", "2025-09-10", -1 * 52)).toBe("2024-09-09");
  });

  it("shifts months across year boundaries without rollover", () => {
    expect(shiftPeriod("month", "2025-09-15", 1)).toBe("2025-10-01");
    expect(shiftPeriod("month", "2025-09-15", -1)).toBe("2025-08-01");
    expect(shiftPeriod("month", "2025-01-01", -1)).toBe("2024-12-01");
    expect(shiftPeriod("month", "2025-03-31", -1)).toBe("2025-02-01"); // anchor day-31 must not spill into March
  });

  it("shifts years", () => {
    expect(shiftPeriod("year", "2025-09-05", 1)).toBe("2026-01-01");
    expect(shiftPeriod("year", "2025-09-05", -2)).toBe("2023-01-01");
  });
});

describe("isoWeekNumber", () => {
  it("matches known ISO weeks", () => {
    expect(isoWeekNumber("2025-01-01")).toBe(1);
    expect(isoWeekNumber("2025-09-08")).toBe(37); // the example week from the plan
    expect(isoWeekNumber("2024-12-30")).toBe(1); // ISO 2025-W01
    expect(isoWeekNumber("2025-12-29")).toBe(1); // ISO 2026-W01
  });
});

describe("periodLabel", () => {
  it("labels days with weekday", () => {
    expect(periodLabel("day", "2025-09-05")).toBe("2025年9月5日 周五");
  });

  it("labels weeks with ISO week number and range", () => {
    expect(periodLabel("week", "2025-09-10")).toBe("2025年第37周（9/8 – 9/14）");
    expect(periodLabel("week", "2025-12-29")).toBe("2026年第1周（12/29 – 1/4）");
  });

  it("labels months and years", () => {
    expect(periodLabel("month", "2025-09-05")).toBe("2025年9月");
    expect(periodLabel("year", "2025-09-05")).toBe("2025年");
  });
});

describe("enumerateDays / fillDailyBuckets", () => {
  it("enumerates every date inclusively", () => {
    expect(enumerateDays("2025-09-08", "2025-09-10")).toEqual([
      "2025-09-08",
      "2025-09-09",
      "2025-09-10",
    ]);
  });

  it("fills missing days with zero buckets so the axis stays honest", () => {
    const out = fillDailyBuckets(
      "2025-09-01",
      "2025-09-03",
      [{ date: "2025-09-02", total: 120, byActivity: { "1": 120 } }],
    );
    expect(out.map((b) => b.total)).toEqual([0, 120, 0]);
    expect(out[0].byActivity).toEqual({});
    expect(out[1].byActivity).toEqual({ "1": 120 });
  });

  it("normalizes null byActivity maps from the binding layer", () => {
    const out = fillDailyBuckets("2025-09-01", "2025-09-01", [
      { date: "2025-09-01", total: 5, byActivity: null },
    ]);
    expect(out[0].byActivity).toEqual({});
  });
});

describe("fillMonthBuckets", () => {
  it("fills all 12 months in order, zero-padded", () => {
    const out = fillMonthBuckets(2025, [
      { month: "2025-01", total: 100, byActivity: { "1": 100 } },
      { month: "2025-11", total: 30, byActivity: null },
    ]);
    expect(out).toHaveLength(12);
    expect(out[0].date).toBe("2025-01");
    expect(out[0].total).toBe(100);
    expect(out[8].total).toBe(0);
    expect(out[10].date).toBe("2025-11");
    expect(out[10].byActivity).toEqual({});
  });
});

describe("peakBucket", () => {
  it("returns the largest non-zero bucket and null when empty", () => {
    const buckets: StatBucket[] = [
      { date: "2025-09-01", total: 60, byActivity: {} },
      { date: "2025-09-02", total: 600, byActivity: {} },
      { date: "2025-09-03", total: 0, byActivity: {} },
    ];
    expect(peakBucket(buckets)?.date).toBe("2025-09-02");
    expect(peakBucket([{ date: "2025-09-01", total: 0, byActivity: {} }])).toBeNull();
  });
});

describe("elapsedDaysInPeriod", () => {
  it("counts elapsed days inside the current period", () => {
    expect(elapsedDaysInPeriod("month", "2025-09-05", "2025-09-10")).toBe(10);
    expect(elapsedDaysInPeriod("year", "2025-09-05", "2025-09-10")).toBe(253);
    expect(elapsedDaysInPeriod("week", "2025-09-10", "2025-09-10")).toBe(3); // Mon..Wed
  });

  it("clamps past periods to their full length and future periods to zero", () => {
    expect(elapsedDaysInPeriod("month", "2025-02-01", "2025-09-10")).toBe(28);
    expect(elapsedDaysInPeriod("month", "2025-12-01", "2025-09-10")).toBe(0);
  });
});

describe("buildTimelineSegments", () => {
  const base = {
    activityId: 1,
    activityColor: "#cc785c",
    activityName: "阅读",
    durationSeconds: 3600,
  };

  it("maps entries to minute segments sorted by start", () => {
    const segs = buildTimelineSegments([
      { ...base, startedAt: "2025-09-05T10:00:00+08:00", durationSeconds: 1800 },
      { ...base, startedAt: "2025-09-05T08:30:00+08:00", durationSeconds: 3600 },
    ]);
    expect(segs).toHaveLength(2);
    expect(segs[0].startMin).toBe(8 * 60 + 30);
    expect(segs[0].endMin).toBe(9 * 60 + 30);
    expect(segs[0].secs).toBe(3600);
    expect(segs[1].startMin).toBe(600);
  });

  it("clamps cross-midnight entries at 24:00", () => {
    const segs = buildTimelineSegments([
      { ...base, startedAt: "2025-09-05T23:00:00+08:00", durationSeconds: 7200 },
    ]);
    expect(segs[0].endMin).toBe(1440);
    expect(segs[0].secs).toBe(3600); // only the part inside this day
  });

  it("drops zero-length segments and survives null activity colors", () => {
    const segs = buildTimelineSegments([
      { ...base, startedAt: "2025-09-05T09:00:00+08:00", durationSeconds: 0 },
      { ...base, startedAt: "2025-09-05T09:00:00+08:00", activityColor: "", durationSeconds: 60 },
    ]);
    expect(segs).toHaveLength(1);
    expect(segs[0].color).toBe("#cc785c"); // fallback color
  });
});
