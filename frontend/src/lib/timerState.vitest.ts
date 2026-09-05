import { describe, expect, it } from "vitest";
import {
  applyTimerState,
  createTimerStateView,
  DEFAULT_ACTIVITY_COLOR,
  type TimerStateView,
} from "./timerState";
import type { TimerState } from "./api";

function backendState(partial: Partial<TimerState> = {}): TimerState {
  return {
    running: false,
    activityId: 0,
    activityName: "",
    activityColor: "",
    startedAt: "",
    elapsedSeconds: 0,
    lastActivityId: 0,
    lastActivityName: "",
    lastActivityColor: "",
    lastElapsedSeconds: 0,
    ...partial,
  };
}

describe("createTimerStateView", () => {
  it("starts unloaded with safe defaults", () => {
    const s = createTimerStateView();
    expect(s.loaded).toBe(false);
    expect(s.running).toBe(false);
    expect(s.activityColor).toBe(DEFAULT_ACTIVITY_COLOR);
    expect(s.lastActivityColor).toBe(DEFAULT_ACTIVITY_COLOR);
  });
});

describe("applyTimerState", () => {
  it("copies every field from the backend state", () => {
    const s: TimerStateView = createTimerStateView();
    applyTimerState(
      s,
      backendState({
        running: true,
        activityId: 3,
        activityName: "阅读",
        activityColor: "#5db8a6",
        startedAt: "2025-09-05T09:00:00+08:00",
        elapsedSeconds: 120,
        lastActivityId: 9,
        lastActivityName: "健身",
        lastActivityColor: "#e8a55a",
        lastElapsedSeconds: 3600,
      }),
    );
    expect(s).toEqual({
      loaded: true,
      running: true,
      activityId: 3,
      activityName: "阅读",
      activityColor: "#5db8a6",
      startedAt: "2025-09-05T09:00:00+08:00",
      elapsed: 120,
      lastActivityId: 9,
      lastActivityName: "健身",
      lastActivityColor: "#e8a55a",
      lastElapsed: 3600,
    });
  });

  it("falls back to the default color when the backend omits one", () => {
    const s: TimerStateView = createTimerStateView();
    applyTimerState(s, backendState({ running: true, activityId: 1 }));
    expect(s.activityColor).toBe(DEFAULT_ACTIVITY_COLOR);
    expect(s.lastActivityColor).toBe(DEFAULT_ACTIVITY_COLOR);
  });

  it("is a full snapshot: a later apply overwrites every field", () => {
    const s: TimerStateView = createTimerStateView();
    applyTimerState(
      s,
      backendState({ running: true, activityId: 1, activityName: "A", elapsedSeconds: 5 }),
    );
    applyTimerState(s, backendState({}));
    expect(s.running).toBe(false);
    expect(s.activityName).toBe("");
    expect(s.activityId).toBe(0);
    expect(s.elapsed).toBe(0);
    expect(s.loaded).toBe(true);
  });
});
