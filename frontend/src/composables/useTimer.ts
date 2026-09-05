import { computed, reactive, ref } from "vue";
import { Events } from "@wailsio/runtime";
import { TimerService, type Entry, type TimerState } from "../lib/api";

// Singleton reactive timer state shared by every view. The backend owns the
// authoritative started_at; the frontend ticks locally every second so the
// clock stays smooth without hammering the API.
const state = reactive({
  loaded: false,
  running: false,
  activityId: 0,
  activityName: "",
  activityColor: "#cc785c",
  startedAt: "",
  elapsed: 0,
  // Last (paused) timer chain, shown by the ball while idle.
  lastActivityId: 0,
  lastActivityName: "",
  lastActivityColor: "#cc785c",
  lastElapsed: 0,
});

/** Bumped on every start/stop so views know when to refresh aggregates. */
const version = ref(0);

let ticker: number | undefined;

function startTicker() {
  stopTicker();
  ticker = window.setInterval(() => {
    state.elapsed++;
  }, 1000);
}

function stopTicker() {
  if (ticker !== undefined) {
    window.clearInterval(ticker);
    ticker = undefined;
  }
}

function apply(st: TimerState) {
  state.loaded = true;
  state.running = st.running;
  state.activityId = st.activityId;
  state.activityName = st.activityName;
  state.activityColor = st.activityColor || "#cc785c";
  state.startedAt = st.startedAt;
  state.elapsed = st.elapsedSeconds;
  state.lastActivityId = st.lastActivityId;
  state.lastActivityName = st.lastActivityName;
  state.lastActivityColor = st.lastActivityColor || "#cc785c";
  state.lastElapsed = st.lastElapsedSeconds;
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
  version.value++;
}

/** Stop the running timer. Returns the recorded entry (null if discarded). */
async function stop(): Promise<Entry | null> {
  const entry = await TimerService.Stop();
  await refresh();
  version.value++;
  return entry;
}

const running = computed(() => state.running);

export function useTimer() {
  return { state, running, version, refresh, start, stop };
}

// Kick off the initial fetch as soon as the module is imported.
void refresh();

// Refresh when the timer is stopped/started from the ball's context menu or
// the system tray (the Go side emits these app-wide after each toggle).
Events.On("timer:stopped", () => {
  void refresh();
  version.value++;
});
Events.On("timer:started", () => {
  void refresh();
  version.value++;
});
