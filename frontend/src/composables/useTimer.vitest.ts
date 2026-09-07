import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TimerState } from "../lib/api";

// The composable is a module singleton with import-time side effects
// (initial fetch, event subscriptions), so every test loads a fresh module
// instance against these mocks.
const timerMocks = vi.hoisted(() => ({
  GetState: vi.fn(),
  Start: vi.fn(),
  Stop: vi.fn(),
}));

const eventHandlers = vi.hoisted(() => ({
  registered: {} as Record<string, (data?: unknown) => void>,
}));

vi.mock("../lib/api", () => ({
  TimerService: timerMocks,
}));

vi.mock("@wailsio/runtime", () => ({
  Events: {
    On: (name: string, cb: (data?: unknown) => void) => {
      eventHandlers.registered[name] = cb;
    },
  },
}));

const runningState: TimerState = {
  running: true,
  activityId: 7,
  activityName: "学习",
  activityColor: "#cc785c",
  startedAt: "2025-09-05T01:00:00Z",
  elapsedSeconds: 100,
  sessionElapsedSeconds: 40,
  lastActivityId: 0,
  lastActivityName: "",
  lastActivityColor: "",
  lastElapsedSeconds: 0,
};

const idleState: TimerState = {
  running: false,
  activityId: 0,
  activityName: "",
  activityColor: "",
  startedAt: "",
  elapsedSeconds: 0,
  sessionElapsedSeconds: 0,
  lastActivityId: 7,
  lastActivityName: "学习",
  lastActivityColor: "#cc785c",
  lastElapsedSeconds: 55,
};

const flush = async () => {
  await new Promise((r) => setTimeout(r, 0));
};

async function loadModule() {
  vi.resetModules();
  return await import("./useTimer");
}

beforeEach(() => {
  vi.clearAllMocks();
  eventHandlers.registered = {};
  // Record but never attach visibilitychange listeners so re-imported
  // module instances do not accumulate real document listeners.
  vi.spyOn(document, "addEventListener").mockImplementation(() => {});
});

describe("useTimer", () => {
  it("fetches the authoritative state on load and starts the clock", async () => {
    timerMocks.GetState.mockResolvedValue(runningState);
    const { useTimer } = await loadModule();
    await flush();
    const { state, running } = useTimer();
    expect(timerMocks.GetState).toHaveBeenCalledTimes(1);
    expect(state.loaded).toBe(true);
    expect(state.running).toBe(true);
    expect(state.activityId).toBe(7);
    expect(state.sessionElapsed).toBe(40);
    expect(running.value).toBe(true);
  });

  it("start() applies the backend state without a separate GetState", async () => {
    timerMocks.Start.mockResolvedValue(runningState);
    const { useTimer } = await loadModule();
    await flush();
    timerMocks.GetState.mockClear();

    const { start, state } = useTimer();
    await start(7);
    expect(timerMocks.Start).toHaveBeenCalledWith(7);
    expect(state.running).toBe(true);
    expect(timerMocks.GetState).not.toHaveBeenCalled();
  });

  it("stop() returns the entry and does not fetch state itself", async () => {
    timerMocks.GetState.mockResolvedValue(idleState);
    const { useTimer } = await loadModule();
    await flush();
    timerMocks.GetState.mockClear();

    const entry = { id: 1, durationSeconds: 40 };
    timerMocks.Stop.mockResolvedValue(entry);
    const { stop } = useTimer();
    await expect(stop()).resolves.toBe(entry);
    // The timer:stopped event owns the refresh; a local fetch here would be
    // a second identical GetState per stop.
    expect(timerMocks.Stop).toHaveBeenCalledTimes(1);
    expect(timerMocks.GetState).not.toHaveBeenCalled();
  });

  it("timer:stopped events refresh once and bump the version", async () => {
    timerMocks.GetState.mockResolvedValue(idleState);
    const { useTimer } = await loadModule();
    await flush();
    timerMocks.GetState.mockClear();

    const { version } = useTimer();
    const before = version.value;
    eventHandlers.registered["timer:stopped"]();
    await flush();
    expect(timerMocks.GetState).toHaveBeenCalledTimes(1);
    expect(version.value).toBe(before + 1);
  });

  it("timer:started events refresh and bump the version too", async () => {
    timerMocks.GetState.mockResolvedValue(runningState);
    const { useTimer } = await loadModule();
    await flush();
    timerMocks.GetState.mockClear();

    const { version } = useTimer();
    eventHandlers.registered["timer:started"]();
    await flush();
    expect(timerMocks.GetState).toHaveBeenCalledTimes(1);
    expect(version.value).toBe(1);
  });

  it("re-calibrates when the page becomes visible again", async () => {
    timerMocks.GetState.mockResolvedValue(idleState);
    await loadModule();
    await flush();
    timerMocks.GetState.mockClear();

    const listener = vi.mocked(document.addEventListener).mock.calls.find(
      ([name]) => name === "visibilitychange",
    )?.[1] as EventListener;
    expect(listener).toBeTypeOf("function");
    listener(new Event("visibilitychange"));
    await flush();
    expect(timerMocks.GetState).toHaveBeenCalledTimes(1);
  });
});
