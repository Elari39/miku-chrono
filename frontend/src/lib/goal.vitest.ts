import { describe, expect, it } from "vitest";
import { normalizeDailyGoal } from "./goal";

describe("normalizeDailyGoal", () => {
  it("treats unusable input as 'no goal'", () => {
    // An emptied number input yields "" through v-model.number.
    expect(normalizeDailyGoal("")).toBe(0);
    expect(normalizeDailyGoal(null)).toBe(0);
    expect(normalizeDailyGoal(undefined)).toBe(0);
    expect(normalizeDailyGoal("abc")).toBe(0);
    expect(normalizeDailyGoal(-5)).toBe(0);
  });

  it("clamps to the backend's 0–1440 range", () => {
    expect(normalizeDailyGoal(5000)).toBe(1440);
    expect(normalizeDailyGoal("1441")).toBe(1440);
    expect(normalizeDailyGoal(45)).toBe(45);
  });

  it("truncates fractional minutes to whole numbers", () => {
    expect(normalizeDailyGoal(45.9)).toBe(45);
    expect(normalizeDailyGoal("30.2")).toBe(30);
  });
});
