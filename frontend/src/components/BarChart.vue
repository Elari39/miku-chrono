<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useElementWidth } from "../composables/useElementWidth";
import { formatDuration } from "../lib/format";
import {
  BAR_CHART_LAYOUT,
  buildBars,
  formatTick,
  niceTicks,
  VALUE_LABEL_MAX_BARS,
  type Bar,
  type BucketInput,
} from "../lib/bars";

const props = defineProps<{
  buckets: BucketInput[];
  colors: Record<string, string>;
  names: Record<string, string>;
  /** Axis label style: per-day buckets ("9月5日") or per-month ("9月"). */
  labelMode?: "day" | "month";
}>();

const hovered = ref<number | null>(null);

// The chart is laid out at the card's measured width: week/year views fill
// the whole card, while a 31-day month clamps columns to MIN_COL and the
// inner wrapper scrolls horizontally instead of overflowing the card.
const { width: containerW } = useElementWidth("chartEl");
const scrollEl = ref<HTMLDivElement | null>(null);
const scrollLeft = ref(0);

function onScroll(e: Event) {
  scrollLeft.value = (e.target as HTMLElement).scrollLeft;
}

// Navigating to another period starts over at the left edge.
watch(
  () => props.buckets,
  () => {
    if (scrollEl.value) scrollEl.value.scrollLeft = 0;
    scrollLeft.value = 0;
  },
);

const layout = computed(() =>
  buildBars(props.buckets, props.colors, props.names, {
    labelMode: props.labelMode ?? "day",
    width: containerW.value,
  }),
);

const ticks = computed(() => niceTicks(layout.value.maxTotal));
const allEmpty = computed(() => layout.value.bars.every((b) => b.total === 0));
const showValues = computed(
  () => layout.value.bars.length > 0 && layout.value.bars.length <= VALUE_LABEL_MAX_BARS,
);

const tickY = (secs: number) =>
  BAR_CHART_LAYOUT.H -
  BAR_CHART_LAYOUT.PAD_B -
  (secs / layout.value.maxTotal) *
    (BAR_CHART_LAYOUT.H - BAR_CHART_LAYOUT.PAD_T - BAR_CHART_LAYOUT.PAD_B);

// HTML overlay tooltip — SVG <title> hover hints never showed in WebView2.
// Lives outside the scroll wrapper so it is never clipped; the anchor is
// shifted by scrollLeft and clamped to the visible viewport.
const tipStyle = computed(() => {
  if (hovered.value === null) return {};
  const bar: Bar | undefined = layout.value.bars[hovered.value];
  if (!bar) return {};
  const center = bar.colX + layout.value.colW / 2 - scrollLeft.value;
  const left = Math.min(Math.max(center, 90), Math.max(containerW.value - 90, 90));
  const top = Math.max(bar.top - 8, 108);
  return { left: `${left}px`, top: `${top}px`, transform: "translate(-50%, -100%)" };
});
</script>

<template>
  <div ref="chartEl" class="relative" @pointerleave="hovered = null">
    <div ref="scrollEl" class="w-full overflow-x-auto" @scroll="onScroll">
      <svg
        :width="layout.width"
        :height="BAR_CHART_LAYOUT.H"
        :viewBox="`0 0 ${layout.width} ${BAR_CHART_LAYOUT.H}`"
        class="block"
      >
        <!-- horizontal gridlines + right-hand duration scale -->
        <g v-for="t in ticks" :key="`tick-${t}`">
          <line
            :x1="BAR_CHART_LAYOUT.PAD_L"
            :x2="layout.width - BAR_CHART_LAYOUT.PAD_R"
            :y1="tickY(t)"
            :y2="tickY(t)"
            class="stroke-hairline"
            stroke-dasharray="2 4"
          />
          <text
            :x="layout.width - BAR_CHART_LAYOUT.PAD_R + 6"
            :y="tickY(t) + 3"
            class="fill-muted text-[9px]"
          >
            {{ formatTick(t) }}
          </text>
        </g>

        <!-- bars -->
        <g v-for="(bar, i) in layout.bars" :key="bar.date">
          <rect
            :x="bar.colX"
            y="0"
            :width="layout.colW"
            :height="BAR_CHART_LAYOUT.H - BAR_CHART_LAYOUT.PAD_B"
            class="fill-transparent"
            @mouseenter="hovered = i"
          />
          <rect
            v-for="(s, si) in bar.segments"
            :key="si"
            :x="bar.x"
            :y="s.y"
            :width="layout.barW"
            :height="s.h"
            :fill="s.color"
            rx="2"
          />
          <text
            v-if="showValues && bar.total > 0"
            :x="bar.colX + layout.colW / 2"
            :y="bar.top - 5"
            text-anchor="middle"
            class="fill-muted text-[9px]"
          >
            {{ formatDuration(bar.total) }}
          </text>
          <text
            v-if="(i + 1) % layout.labelEvery === 0"
            :x="bar.colX + layout.colW / 2"
            :y="BAR_CHART_LAYOUT.H - 6"
            text-anchor="middle"
            class="fill-muted text-[10px]"
          >
            {{ bar.label }}
          </text>
        </g>

        <!-- baseline -->
        <line
          :x1="BAR_CHART_LAYOUT.PAD_L"
          :x2="layout.width - BAR_CHART_LAYOUT.PAD_R"
          :y1="BAR_CHART_LAYOUT.H - BAR_CHART_LAYOUT.PAD_B"
          :y2="BAR_CHART_LAYOUT.H - BAR_CHART_LAYOUT.PAD_B"
          class="stroke-hairline"
        />
      </svg>
    </div>

    <div
      v-if="allEmpty"
      class="pointer-events-none absolute inset-0 flex items-center justify-center text-sm text-muted"
    >
      这个时段还没有记录
    </div>

    <!-- hover tooltip -->
    <div
      v-if="hovered !== null && layout.bars[hovered]"
      class="pointer-events-none absolute z-10 max-h-36 w-40 overflow-y-auto rounded-lg border border-hairline bg-surface-card px-3 py-2 shadow-lg"
      :style="tipStyle"
    >
      <div class="text-xs font-medium text-ink">{{ layout.bars[hovered].tooltipTitle }}</div>
      <div class="mt-0.5 text-[11px] text-muted">
        合计 {{ formatDuration(layout.bars[hovered].total) }}
      </div>
      <div
        v-for="s in layout.bars[hovered].segments"
        :key="s.name"
        class="mt-1 flex items-center gap-1.5 text-[11px] text-body"
      >
        <span class="h-2 w-2 shrink-0 rounded-full" :style="{ backgroundColor: s.color }" />
        <span class="min-w-0 flex-1 truncate">{{ s.name }}</span>
        <span class="text-muted">{{ formatDuration(s.secs) }}</span>
      </div>
    </div>
  </div>
</template>
