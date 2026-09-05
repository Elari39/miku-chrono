<script setup lang="ts">
import { computed } from "vue";
import type { DayBucket } from "../lib/api";
import { formatDay, formatDuration } from "../lib/format";

const props = defineProps<{
  buckets: DayBucket[];
  /** activityId (as string) → color, for stacking. */
  colors: Record<string, string>;
  /** activityId (as string) → name, for tooltips. */
  names: Record<string, string>;
}>();

const COL = 26;
const BAR_W = 14;
const H = 200;
const PAD_T = 10;
const PAD_B = 22;

const width = computed(() => Math.max(props.buckets.length * COL + 8, 120));
const maxTotal = computed(() => Math.max(1, ...props.buckets.map((b) => b.total)));

const bars = computed(() =>
  props.buckets.map((b, i) => {
    const x = 4 + i * COL;
    let y = H - PAD_B;
    const segments = Object.entries(b.byActivity ?? {}).map(([actId, secsRaw]) => {
      const secs = secsRaw ?? 0;
      const h = (secs / maxTotal.value) * (H - PAD_T - PAD_B);
      const seg = {
        x,
        y: y - h,
        w: BAR_W,
        h: Math.max(h, 1),
        color: props.colors[actId] ?? "#cc785c",
        name: props.names[actId] ?? `活动 ${actId}`,
        secs,
      };
      y -= h;
      return seg;
    });
    return {
      x,
      total: b.total,
      date: b.date,
      labelEvery: labelStep(props.buckets.length),
      segments,
    };
  }),
);

function labelStep(n: number): number {
  if (n <= 14) return 1;
  if (n <= 31) return 2;
  return 5;
}
</script>

<template>
  <div class="overflow-x-auto">
    <svg :viewBox="`0 0 ${width} ${H}`" :width="width" :height="H" class="block">
      <!-- baseline -->
      <line x1="0" :y1="H - PAD_B + 0.5" :x2="width" :y2="H - PAD_B + 0.5" class="stroke-hairline" />
      <g v-for="(bar, i) in bars" :key="bar.date">
        <title>{{ formatDay(bar.date, true) }} · {{ formatDuration(bar.total) }}</title>
        <rect
          v-for="(seg, j) in bar.segments"
          :key="j"
          :x="seg.x"
          :y="seg.y"
          :width="seg.w"
          :height="seg.h"
          :fill="seg.color"
          rx="2"
        >
          <title>{{ formatDay(bar.date, true) }} · {{ seg.name }} {{ formatDuration(seg.secs) }}</title>
        </rect>
        <text
          v-if="i % bar.labelEvery === 0"
          :x="bar.x + BAR_W / 2"
          :y="H - PAD_B + 14"
          text-anchor="middle"
          class="fill-muted text-[9px]"
        >{{ formatDay(bar.date) }}</text>
      </g>
      <!-- empty hint -->
      <text v-if="buckets.every((b) => b.total === 0)" :x="width / 2" :y="H / 2" text-anchor="middle" class="fill-muted text-xs">
        所选时段暂无记录
      </text>
    </svg>
  </div>
</template>
