<script setup lang="ts">
import { computed, ref } from "vue";
import { formatDuration, formatDurationLong } from "../lib/format";
import type { TimelineSegment } from "../lib/statsPeriods";

const props = defineProps<{ segments: TimelineSegment[] }>();

// Track geometry: 24h mapped onto a 744px strip inside a 776px viewBox.
const TRACK_X = 8;
const TRACK_W = 744;
const WIDTH = 776;
const TRACK_Y = 10;
const TRACK_H = 20;

const hovered = ref<number | null>(null);

const xAt = (min: number) => TRACK_X + (min / 1440) * TRACK_W;

const hourLabels = Array.from({ length: 9 }, (_, i) => i * 3); // 0点..24点

const legend = computed(() => {
  const byActivity = new Map<number, { name: string; color: string; secs: number }>();
  for (const s of props.segments) {
    const cur = byActivity.get(s.activityId);
    if (cur) cur.secs += s.secs;
    else byActivity.set(s.activityId, { name: s.name, color: s.color, secs: s.secs });
  }
  return [...byActivity.values()].sort((a, b) => b.secs - a.secs);
});

const recordedSecs = computed(() => props.segments.reduce((sum, s) => sum + s.secs, 0));

const hm = (min: number) => {
  const m = Math.min(1440, Math.max(0, Math.round(min)));
  return `${String(Math.floor(m / 60)).padStart(2, "0")}:${String(m % 60).padStart(2, "0")}`;
};

const hoveredSeg = computed(() => (hovered.value === null ? null : props.segments[hovered.value]));

const tipStyle = computed(() => {
  const seg = hoveredSeg.value;
  if (!seg) return {};
  const center = xAt((seg.startMin + seg.endMin) / 2);
  const left = Math.min(Math.max(center, 80), WIDTH - 80);
  return { left: `${left}px`, top: "42px", transform: "translate(-50%, 0)" };
});
</script>

<template>
  <div class="relative" @pointerleave="hovered = null">
    <svg
      :width="WIDTH"
      :height="52"
      :viewBox="`0 0 ${WIDTH} 52`"
      class="block w-full"
      preserveAspectRatio="xMidYMid meet"
    >
      <!-- track = unrecorded time -->
      <rect
        :x="TRACK_X"
        :y="TRACK_Y"
        :width="TRACK_W"
        :height="TRACK_H"
        rx="10"
        class="fill-black/5"
      />
      <!-- activity segments, sorted by start; overlaps paint later-on-top -->
      <rect
        v-for="(s, i) in segments"
        :key="i"
        :x="xAt(s.startMin)"
        :y="TRACK_Y"
        :width="Math.max(2, xAt(s.endMin) - xAt(s.startMin))"
        :height="TRACK_H"
        :fill="s.color"
        rx="3"
        :opacity="hovered === null || hovered === i ? 1 : 0.55"
        @mouseenter="hovered = i"
      />
      <!-- hour ticks -->
      <line
        v-for="h in 23"
        :key="`tick-${h}`"
        :x1="xAt(h * 60)"
        :x2="xAt(h * 60)"
        :y1="TRACK_Y + TRACK_H"
        :y2="TRACK_Y + TRACK_H + 3"
        class="stroke-hairline"
      />
      <text
        v-for="h in hourLabels"
        :key="`label-${h}`"
        :x="xAt(h * 60)"
        :y="TRACK_Y + TRACK_H + 16"
        :text-anchor="h === 0 ? 'start' : h === 24 ? 'end' : 'middle'"
        class="fill-muted text-[9px]"
      >
        {{ h }}点
      </text>
    </svg>

    <!-- hover tooltip (HTML overlay; SVG <title> doesn't show in WebView2) -->
    <div
      v-if="hoveredSeg"
      class="pointer-events-none absolute z-10 rounded-lg border border-hairline bg-surface-card px-3 py-2 shadow-lg"
      :style="tipStyle"
    >
      <div class="flex items-center gap-1.5 text-xs font-medium text-ink">
        <span class="h-2 w-2 rounded-full" :style="{ backgroundColor: hoveredSeg.color }" />
        {{ hoveredSeg.name }}
      </div>
      <div class="mt-0.5 text-[11px] text-muted">
        {{ hm(hoveredSeg.startMin) }} – {{ hm(hoveredSeg.endMin) }} ·
        {{ formatDuration(hoveredSeg.secs) }}
      </div>
    </div>

    <!-- legend -->
    <div class="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1.5">
      <div
        v-for="item in legend"
        :key="item.name"
        class="flex items-center gap-1.5 text-xs text-body"
      >
        <span class="h-2.5 w-2.5 rounded-full" :style="{ backgroundColor: item.color }" />
        <span>{{ item.name }}</span>
        <span class="text-muted">{{ formatDuration(item.secs) }}</span>
      </div>
      <div class="ml-auto text-xs text-muted">全天记录 {{ formatDurationLong(recordedSecs) }}</div>
    </div>
  </div>
</template>
