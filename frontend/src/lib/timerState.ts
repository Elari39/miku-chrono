// Pure state-transition helpers for the shared timer state, extracted from
// useTimer.ts so the logic is unit-testable without touching the Wails
// runtime. The composable keeps the ticker/window concerns.
import type { TimerState } from "./api";

export const DEFAULT_ACTIVITY_COLOR = "#cc785c";

/** The reactive slice useTimer exposes to views/components. */
export interface TimerStateView {
  loaded: boolean;
  running: boolean;
  activityId: number;
  activityName: string;
  activityColor: string;
  startedAt: string;
  elapsed: number;
  sessionElapsed: number;
  lastActivityId: number;
  lastActivityName: string;
  lastActivityColor: string;
  lastElapsed: number;
}

/** Default values for the reactive state object. */
export function createTimerStateView(): TimerStateView {
  return {
    loaded: false,
    running: false,
    activityId: 0,
    activityName: "",
    activityColor: DEFAULT_ACTIVITY_COLOR,
    startedAt: "",
    elapsed: 0,
    sessionElapsed: 0,
    lastActivityId: 0,
    lastActivityName: "",
    lastActivityColor: DEFAULT_ACTIVITY_COLOR,
    lastElapsed: 0,
  };
}

/** Copy an authoritative backend TimerState onto the reactive view. */
export function applyTimerState(state: TimerStateView, st: TimerState): void {
  state.loaded = true;
  state.running = st.running;
  state.activityId = st.activityId;
  state.activityName = st.activityName;
  state.activityColor = st.activityColor || DEFAULT_ACTIVITY_COLOR;
  state.startedAt = st.startedAt;
  state.elapsed = st.elapsedSeconds;
  state.sessionElapsed = st.sessionElapsedSeconds;
  state.lastActivityId = st.lastActivityId;
  state.lastActivityName = st.lastActivityName;
  state.lastActivityColor = st.lastActivityColor || DEFAULT_ACTIVITY_COLOR;
  state.lastElapsed = st.lastElapsedSeconds;
}

/**
 * Seconds elapsed projected from a state snapshot: the snapshot's value plus
 * the whole seconds passed since it was taken. Ticks project from the latest
 * backend snapshot instead of incrementing a counter, so paused schedulers,
 * system sleep or missed callbacks catch up on the next tick instead of
 * leaving the display behind (MC-007). A rollback of the system clock (now
 * before the snapshot) keeps the base: the next backend snapshot re-anchors
 * the projection.
 */
export function projectElapsed(baseSeconds: number, snapshotAtMs: number, nowMs: number): number {
  if (snapshotAtMs <= 0 || nowMs < snapshotAtMs) return baseSeconds;
  return baseSeconds + Math.floor((nowMs - snapshotAtMs) / 1000);
}
