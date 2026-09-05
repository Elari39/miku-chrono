<script setup lang="ts">
import { computed } from "vue";
import type { DayBucket } from "../lib/api";
import { formatDay, formatDuration } from "../lib/format";
import { buildBars, BAR_CHART_LAYOUT } from "../lib/bars";

const props = defineProps<{
  buckets: DayBucket[];
  /** activityId (as string) → color, for stacking. */
  colors: Record<string, string>;
  /** activityId (as string) → name, for tooltips. */
  names: Record<string, string>;
}>();

const { H, PAD_B, BAR_W } = BAR_CHART_LAYOUT;

const layout = computed(() => buildBars(props.buckets, props.colors, props.names));
const bars = computed(() => layout.value.bars);
const width = computed(() => layout.value.width);
</script>

<template>
  <div class="overflow-x-auto">
    <svg :viewBox="`0 0 ${width} ${H}`" :width="width" :height="H" class="block">
      <!-- baseline -->
      <line
        x1="0"
        :y1="H - PAD_B + 0.5"
        :x2="width"
        :y2="H - PAD_B + 0.5"
        class="stroke-hairline"
      />
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
          <title>
            {{ formatDay(bar.date, true) }} · {{ seg.name }} {{ formatDuration(seg.secs) }}
          </title>
        </rect>
        <text
          v-if="i % bar.labelEvery === 0"
          :x="bar.x + BAR_W / 2"
          :y="H - PAD_B + 14"
          text-anchor="middle"
          class="fill-muted text-[9px]"
        >
          {{ formatDay(bar.date) }}
        </text>
      </g>
      <!-- empty hint -->
      <text
        v-if="buckets.every((b) => b.total === 0)"
        :x="width / 2"
        :y="H / 2"
        text-anchor="middle"
        class="fill-muted text-xs"
      >
        所选时段暂无记录
      </text>
    </svg>
  </div>
</template>
