import { computed, reactive, ref } from "vue";
import { Events } from "@wailsio/runtime";
import { TimerService, type Entry, type TimerState } from "../lib/api";
import {
  applyTimerState,
  createTimerStateView,
  projectElapsed,
  type TimerStateView,
} from "../lib/timerState";

// Singleton reactive timer state shared by every view. The backend owns the
// authoritative started_at; the frontend projects elapsed time from the last
// backend snapshot every second so the clock stays smooth without hammering
// the API — and still catches up after paused schedulers or system sleep,
// where a naive per-tick counter would fall behind.
const state = reactive<TimerStateView>(createTimerStateView());

/** Bumped on every start/stop so views know when to refresh aggregates. */
const version = ref(0);

// Elapsed-time snapshot: the authoritative values from the backend plus the
// local time they were received. Ticks project from this baseline.
let baseElapsed = 0;
let baseSession = 0;
let snapshotAt = 0;

let ticker: number | undefined;

function takeSnapshot(st: TimerState) {
  baseElapsed = st.elapsedSeconds;
  baseSession = st.sessionElapsedSeconds;
  snapshotAt = Date.now();
}

function startTicker() {
  stopTicker();
  ticker = window.setInterval(() => {
    const now = Date.now();
    state.elapsed = projectElapsed(baseElapsed, snapshotAt, now);
    state.sessionElapsed = projectElapsed(baseSession, snapshotAt, now);
  }, 1000);
}

function stopTicker() {
  if (ticker !== undefined) {
    window.clearInterval(ticker);
    ticker = undefined;
  }
}

function apply(st: TimerState) {
  applyTimerState(state, st);
  takeSnapshot(st);
  if (st.running) startTicker();
  else stopTicker();
}

/** Fetch the authoritative state from the backend. */
async function refresh(): Promise<void> {
  try {
    apply(await TimerService.GetState());
  } catch (err) {
    console.error("GetState failed", err);
    state.loaded = true;
  }
}

/** Start timing an activity (mutually exclusive; auto-closes the previous). */
async function start(activityId: number): Promise<void> {
  apply(await TimerService.Start(activityId));
  // Aggregate refresh is driven by the app-wide timer:started event (the Go
  // service broadcasts after each successful change) so every window reloads
  // exactly once per change.
}

/** Stop the running timer. Returns the recorded entry (null if discarded). */
async function stop(): Promise<Entry | null> {
  // The backend broadcasts timer:stopped after the stop settles; that event
  // refreshes the state below — a local refresh here would fire a second
  // GetState for the same change.
  return TimerService.Stop();
}

const running = computed(() => state.running);

export function useTimer() {
  return { state, running, version, refresh, start, stop };
}

// Kick off the initial fetch as soon as the module is imported.
void refresh();

// Refresh when the timer is stopped/started from the ball's context menu or
// the system tray (the Go side emits these app-wide after each toggle —
// including from the main window's own start/stop and a settings-page clear).
Events.On("timer:stopped", () => {
  void refresh();
  version.value++;
});
Events.On("timer:started", () => {
  void refresh();
  version.value++;
});

// Re-calibrate against the backend when the page becomes visible again —
// schedulers and webview timers may have been paused while hidden/asleep,
// and the projected clock only converges once a fresh snapshot arrives.
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible") void refresh();
});
