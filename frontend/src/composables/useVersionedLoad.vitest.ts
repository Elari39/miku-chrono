import { describe, expect, it, vi } from "vitest";
import { defineComponent, nextTick, ref, type WatchSource } from "vue";
import { mount } from "@vue/test-utils";
import { useVersionedLoad } from "./useVersionedLoad";

// Controllable shared version, standing in for useTimer's bump counter.
const version = ref(0);
vi.mock("./useTimer", () => ({
  useTimer: () => ({ version }),
}));

const flush = async () => {
  await new Promise((r) => setTimeout(r, 0));
  await nextTick();
};

/** Mount a throwaway host component that runs the composable. */
function mountLoader(loader: (isCurrent: () => boolean) => Promise<void>, triggers: WatchSource[] = []) {
  let api!: ReturnType<typeof useVersionedLoad>;
  mount(
    defineComponent({
      setup() {
        api = useVersionedLoad(loader, { triggers });
        return () => null;
      },
    }),
  );
  return api;
}

describe("useVersionedLoad", () => {
  it("runs the loader once on mount", async () => {
    const loader = vi.fn(async () => {});
    const { loading } = mountLoader(loader);
    await flush();
    expect(loader).toHaveBeenCalledTimes(1);
    expect(loading.value).toBe(false);
  });

  it("re-runs when the timer version bumps", async () => {
    const loader = vi.fn(async () => {});
    mountLoader(loader);
    await flush();

    version.value++;
    await flush();
    version.value++;
    await flush();
    expect(loader).toHaveBeenCalledTimes(3); // mount + two bumps
  });

  it("re-runs on extra triggers", async () => {
    const trigger = ref(0);
    const loader = vi.fn(async () => {});
    mountLoader(loader, [trigger]);
    await flush();

    trigger.value++;
    await flush();
    expect(loader).toHaveBeenCalledTimes(2);
  });

  it("drops stale responses via the isCurrent guard", async () => {
    let resolveFirst!: () => void;
    const committed: number[] = [];
    const loader = vi.fn(async (isCurrent: () => boolean) => {
      const run = loader.mock.calls.length;
      if (run === 1) {
        // The first run hangs until the second has overtaken it.
        await new Promise<void>((r) => (resolveFirst = r));
      }
      if (isCurrent()) committed.push(run);
    });

    const { loading } = mountLoader(loader);
    await flush();
    expect(loading.value).toBe(true);

    // The version bump starts a second run that finishes first.
    version.value++;
    await flush();
    resolveFirst();
    await flush();

    expect(loader).toHaveBeenCalledTimes(2);
    // Only the second run's data may commit; the stale first response must
    // leave the state untouched.
    expect(committed).toEqual([2]);
    expect(loading.value).toBe(false);
  });
});
