import { afterEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { nextTick } from "vue";
import Timeline24h from "./Timeline24h.vue";
import type { TimelineSegment } from "../lib/statsPeriods";

const TRACK_X = 8;

/** Measured card width reported by the fake ResizeObserver. */
const CONTAINER_W = 700;

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

// One 10:00–11:00 segment.
const SEGMENTS: TimelineSegment[] = [
  { activityId: 1, color: "#cc785c", name: "阅读", startMin: 600, endMin: 660, secs: 3600 },
];

async function mountTimeline(segments: TimelineSegment[]) {
  stubResizeObserver(CONTAINER_W);
  const wrapper = mount(Timeline24h, { props: { segments } });
  await nextTick();
  return wrapper;
}

describe("Timeline24h fluid width", () => {
  it("renders the track 1:1 at the measured width", async () => {
    const wrapper = await mountTimeline(SEGMENTS);
    expect(wrapper.find("svg").attributes("width")).toBe(String(CONTAINER_W));
    const rects = wrapper.findAll("rect");
    const track = rects[0];
    expect(track.attributes("x")).toBe(String(TRACK_X));
    expect(parseFloat(track.attributes("width") ?? "0")).toBeCloseTo(CONTAINER_W - TRACK_X * 2, 5);
  });

  it("positions segments proportionally on the fluid track", async () => {
    const wrapper = await mountTimeline(SEGMENTS);
    const seg = wrapper.findAll("rect")[1];
    const trackW = CONTAINER_W - TRACK_X * 2;
    expect(parseFloat(seg.attributes("x") ?? "0")).toBeCloseTo(TRACK_X + (600 / 1440) * trackW, 5);
    expect(parseFloat(seg.attributes("width") ?? "0")).toBeCloseTo((60 / 1440) * trackW, 5);
  });

  it("keeps hour labels and the day-total legend", async () => {
    const wrapper = await mountTimeline(SEGMENTS);
    const hourTexts = wrapper.findAll("svg text").map((t) => t.text());
    expect(hourTexts).toContain("0点");
    expect(hourTexts).toContain("24点");
    expect(wrapper.text()).toContain("全天记录 1 小时");
  });
});
