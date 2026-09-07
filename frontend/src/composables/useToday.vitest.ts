import { describe, expect, it } from "vitest";
import { msUntilNextMidnight } from "./useToday";

describe("msUntilNextMidnight", () => {
  it("measures the distance to the next local midnight", () => {
    expect(msUntilNextMidnight(new Date(2025, 8, 5, 12, 0, 0, 0))).toBe(12 * 3600_000);
    expect(msUntilNextMidnight(new Date(2025, 8, 5, 23, 59, 59, 999))).toBe(1000);
    expect(msUntilNextMidnight(new Date(2025, 8, 5, 0, 0, 0, 0))).toBe(24 * 3600_000);
  });

  it("rolls across month boundaries", () => {
    // Sep 30 → Oct 1 is still exactly one day: setHours(24) normalizes.
    expect(msUntilNextMidnight(new Date(2025, 8, 30, 13, 30, 0, 0))).toBe(10.5 * 3600_000);
  });

  it("never schedules a past-due timeout", () => {
    expect(msUntilNextMidnight(new Date(2025, 8, 5, 23, 59, 59, 999999))).toBeGreaterThanOrEqual(1000);
  });
});
