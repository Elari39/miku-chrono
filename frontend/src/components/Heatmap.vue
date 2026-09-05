<script setup lang="ts">
import { computed } from "vue";
import { formatDay, formatDuration, todayStr } from "../lib/format";
import {
  buildHeatmapGrid,
  HEATMAP_CELL,
  HEATMAP_GAP,
  HEATMAP_LEVEL_COLORS,
  HEATMAP_PAD_T,
  HEATMAP_WEEKDAY_LABELS,
} from "../lib/heatmapGrid";

const props = defineProps<{
  /** date "YYYY-MM-DD" → seconds. */
  days: Record<string, number>;
  weeks?: number;
}>();

const weeksCount = computed(() => props.weeks ?? 17);

const grid = computed(() => buildHeatmapGrid(props.days, weeksCount.value));

const today = todayStr();
</script>

<template>
  <div class="overflow-x-auto">
    <svg
      :viewBox="`0 0 ${grid.width} ${grid.height}`"
      :width="grid.width"
      :height="grid.height"
      class="block"
    >
      <text
        v-for="(w, i) in HEATMAP_WEEKDAY_LABELS"
        :key="'w' + i"
        :x="2"
        :y="HEATMAP_PAD_T + i * (HEATMAP_CELL + HEATMAP_GAP) + 10"
        class="fill-muted text-[9px]"
      >
        {{ w }}
      </text>
      <text
        v-for="m in grid.monthMarks"
        :key="'m' + m.x"
        :x="m.x"
        :y="10"
        class="fill-muted text-[9px]"
      >
        {{ m.label }}
      </text>
      <rect
        v-for="c in grid.cells"
        :key="c.date"
        :x="c.x"
        :y="c.y"
        :width="HEATMAP_CELL"
        :height="HEATMAP_CELL"
        rx="2.5"
        :fill="c.future ? 'transparent' : HEATMAP_LEVEL_COLORS[c.level]"
        :stroke="c.date === today ? '#141413' : undefined"
        stroke-width="1"
        class="cursor-default"
      >
        <title v-if="!c.future">{{ formatDay(c.date, true) }} · {{ formatDuration(c.secs) }}</title>
      </rect>
    </svg>
    <div class="mt-1 flex items-center gap-1 text-[10px] text-muted">
      少
      <span
        v-for="(c, i) in HEATMAP_LEVEL_COLORS"
        :key="i"
        class="inline-block h-2.5 w-2.5 rounded-[2px]"
        :style="{ backgroundColor: c }"
      />
      多
    </div>
  </div>
</template>
