import { afterEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { nextTick } from "vue";
import BarChart from "./BarChart.vue";
import { BAR_CHART_LAYOUT, type BucketInput } from "../lib/bars";

/** Measured card width reported by the fake ResizeObserver. */
const CONTAINER_W = 595;

function stubResizeObserver(width: number) {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      constructor(private readonly cb: ResizeObserverCallback) {}
      observe() {
        this.cb(
          [{ contentRect: { width } } as ResizeObserverEntry],
          this as unknown as ResizeObserver,
        );
      }
      disconnect() {}
      unobserve() {}
    },
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

function days(n: number, total = 60): BucketInput[] {
  return Array.from({ length: n }, (_, i) => ({
    date: `2025-09-${String(i + 1).padStart(2, "0")}`,
    total,
    byActivity: total > 0 ? { "1": total } : null,
  }));
}

const baseProps = { colors: { "1": "#cc785c" }, names: { "1": "阅读" } };

async function mountChart(buckets: BucketInput[]) {
  stubResizeObserver(CONTAINER_W);
  const wrapper = mount(BarChart, { props: { buckets, ...baseProps } });
  await nextTick();
  return wrapper;
}

describe("BarChart fluid width", () => {
  it("spreads a 7-day week across the measured width without scrolling", async () => {
    const wrapper = await mountChart(days(7));
    expect(wrapper.find("svg").attributes("width")).toBe(String(CONTAINER_W));
    // Scroll wrapper is always present; only wider content engages it.
    expect(wrapper.find(".overflow-x-auto").exists()).toBe(true);
    // Every day of the week gets an axis label.
    expect(wrapper.text()).toContain("9月1日");
    expect(wrapper.text()).toContain("9月7日");
  });

  it("renders a 31-day month at MIN_COL inside a horizontal-scroll wrapper", async () => {
    const wrapper = await mountChart(days(31));
    const contentW =
      BAR_CHART_LAYOUT.PAD_L + BAR_CHART_LAYOUT.MIN_COL * 31 + BAR_CHART_LAYOUT.PAD_R;
    expect(contentW).toBeGreaterThan(CONTAINER_W); // scrolling is required
    expect(wrapper.find("svg").attributes("width")).toBe(String(contentW));
    // Axis labels thin out to every other day.
    expect(wrapper.text()).toContain("9月2日");
    expect(wrapper.text()).not.toContain("9月1日");
  });

  it("shows the empty-state overlay when the period has no records", async () => {
    const wrapper = await mountChart(days(3, 0));
    expect(wrapper.text()).toContain("这个时段还没有记录");
  });
});
