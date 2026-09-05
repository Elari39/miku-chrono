import { describe, expect, it } from "vitest";
import {
  dateStr,
  daysAgoStr,
  formatClock,
  formatDay,
  formatDuration,
  formatDurationLong,
  formatWhen,
  fromLocalInput,
  localRFC3339,
  toLocalInput,
  todayStr,
} from "./format";

describe("formatClock", () => {
  it("formats under an hour as m:ss", () => {
    expect(formatClock(0)).toBe("00:00");
    expect(formatClock(45)).toBe("00:45");
    expect(formatClock(59)).toBe("00:59");
  });

  it("formats minutes and hours with padding", () => {
    expect(formatClock(60)).toBe("01:00");
    expect(formatClock(3661)).toBe("1:01:01");
    expect(formatClock(3600 * 5 + 61)).toBe("5:01:01");
  });

  it("clamps negatives to zero and floors fractional seconds", () => {
    expect(formatClock(-5)).toBe("00:00");
    expect(formatClock(1.9)).toBe("00:01");
  });
});

describe("formatDuration", () => {
  it("uses seconds under a minute", () => {
    expect(formatDuration(0)).toBe("0秒");
    expect(formatDuration(59)).toBe("59秒");
  });

  it("uses minutes under an hour", () => {
    expect(formatDuration(3540)).toBe("59分");
  });

  it("pads minutes in hour form", () => {
    expect(formatDuration(3600)).toBe("1时");
    expect(formatDuration(3661)).toBe("1时01分");
  });

  it("clamps negatives", () => {
    expect(formatDuration(-10)).toBe("0秒");
  });
});

describe("formatDurationLong", () => {
  it("combines hours and minutes", () => {
    expect(formatDurationLong(3661)).toBe("1 小时 1 分钟");
  });

  it("drops the zero part", () => {
    expect(formatDurationLong(3600)).toBe("1 小时");
    expect(formatDurationLong(600)).toBe("10 分钟");
  });
});

describe("date helpers", () => {
  it("pads month and day", () => {
    expect(dateStr(new Date(2025, 0, 5))).toBe("2025-01-05");
  });

  it("daysAgoStr returns a valid date string", () => {
    const today = todayStr();
    const ago = daysAgoStr(1);
    const back = new Date();
    back.setDate(back.getDate() - 1);
    expect(ago).toBe(dateStr(back));
    expect(ago < today).toBe(true);
  });
});

describe("formatDay", () => {
  it("renders month/day with optional weekday", () => {
    // 2025-09-05 is a Friday.
    expect(formatDay("2025-09-05")).toBe("9月5日");
    expect(formatDay("2025-09-05", true)).toBe("9月5日 周五");
  });

  it("falls back to the raw input for invalid dates", () => {
    expect(formatDay("not-a-date")).toBe("not-a-date");
  });
});

describe("datetime-local conversions", () => {
  it("round-trips a timestamp through datetime-local values", () => {
    const iso = localRFC3339(new Date(2025, 8, 5, 14, 30, 45));
    const input = toLocalInput(iso);
    expect(input).toBe("2025-09-05T14:30");
    // Seconds are dropped by toLocalInput, so the round-trip is stable at
    // minute precision only.
    expect(toLocalInput(fromLocalInput(input))).toBe(input);
  });

  it("returns empty strings for invalid input", () => {
    expect(toLocalInput("garbage")).toBe("");
    expect(fromLocalInput("")).toBe("");
  });

  it("honors the local timezone offset", () => {
    const d = new Date(2025, 0, 2, 0, 0, 0);
    const iso = localRFC3339(d);
    expect(iso.startsWith("2025-01-02T00:00:00")).toBe(true);
    const offset = -d.getTimezoneOffset();
    const sign = offset >= 0 ? "+" : "-";
    expect(
      iso.endsWith(
        `${sign}${String(Math.floor(Math.abs(offset) / 60)).padStart(2, "0")}:${String(Math.abs(offset) % 60).padStart(2, "0")}`,
      ),
    ).toBe(true);
  });
});

describe("formatWhen", () => {
  it("labels today's timestamps", () => {
    const d = new Date();
    d.setHours(9, 5, 0, 0);
    expect(formatWhen(d.toISOString())).toBe(`今天 09:05`);
  });

  it("falls back to the raw input for invalid dates", () => {
    expect(formatWhen("nope")).toBe("nope");
  });
});
