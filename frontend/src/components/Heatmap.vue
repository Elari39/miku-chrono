<script setup lang="ts">
import { computed } from "vue";
import { dateStr, formatDay, formatDuration, todayStr } from "../lib/format";

const props = defineProps<{
  /** date "YYYY-MM-DD" → seconds. */
  days: Record<string, number>;
  weeks?: number;
}>();

const CELL = 13;
const GAP = 3;
const PAD_L = 24;
const PAD_T = 16;

const weeksCount = computed(() => props.weeks ?? 17);

interface Cell {
  date: string;
  secs: number;
  level: 0 | 1 | 2 | 3 | 4;
  x: number;
  y: number;
  future: boolean;
}

const grid = computed(() => {
  const today = todayStr();
  const end = new Date(today + "T00:00:00");
  // Walk back so the last column ends today; start on a Monday for tidy rows.
  const total = weeksCount.value * 7;
  const start = new Date(end);
  start.setDate(start.getDate() - (total - 1));
  start.setDate(start.getDate() - ((start.getDay() + 6) % 7)); // align to Monday

  const maxSecs = Math.max(60, ...Object.values(props.days));
  const cells: Cell[] = [];
  const monthMarks: { x: number; label: string }[] = [];
  let lastMonth = -1;

  const cur = new Date(start);
  for (let w = 0; w < weeksCount.value; w++) {
    for (let d = 0; d < 7; d++) {
      const ds = dateStr(cur);
      const future = ds > today;
      const secs = props.days[ds] ?? 0;
      let level: Cell["level"] = 0;
      if (!future && secs > 0) {
        const ratio = secs / maxSecs;
        level = ratio > 0.66 ? 4 : ratio > 0.33 ? 3 : ratio > 0.1 ? 2 : 1;
      }
      cells.push({
        date: ds,
        secs,
        level,
        x: PAD_L + w * (CELL + GAP),
        y: PAD_T + d * (CELL + GAP),
        future,
      });
      if (d === 0 && cur.getMonth() !== lastMonth && !future) {
        lastMonth = cur.getMonth();
        monthMarks.push({ x: PAD_L + w * (CELL + GAP), label: `${lastMonth + 1}月` });
      }
      cur.setDate(cur.getDate() + 1);
    }
  }
  return { cells, monthMarks, width: PAD_L + weeksCount.value * (CELL + GAP), height: PAD_T + 7 * (CELL + GAP) + 18 };
});

const levelColors = ["#efe9de", "#e3b49f", "#d99a7c", "#d08658", "#cc785c"];

const weekdayLabels = ["一", "", "三", "", "五", "", "日"];
</script>

<template>
  <div class="overflow-x-auto">
    <svg :viewBox="`0 0 ${grid.width} ${grid.height}`" :width="grid.width" :height="grid.height" class="block">
      <text v-for="(w, i) in weekdayLabels" :key="'w' + i" :x="2" :y="PAD_T + i * (CELL + GAP) + 10" class="fill-muted text-[9px]">{{ w }}</text>
      <text v-for="m in grid.monthMarks" :key="'m' + m.x" :x="m.x" :y="10" class="fill-muted text-[9px]">{{ m.label }}</text>
      <rect
        v-for="c in grid.cells"
        :key="c.date"
        :x="c.x"
        :y="c.y"
        :width="CELL"
        :height="CELL"
        rx="2.5"
        :fill="c.future ? 'transparent' : levelColors[c.level]"
        :stroke="c.date === todayStr() ? '#141413' : undefined"
        stroke-width="1"
        class="cursor-default"
      >
        <title v-if="!c.future">{{ formatDay(c.date, true) }} · {{ formatDuration(c.secs) }}</title>
      </rect>
    </svg>
    <div class="mt-1 flex items-center gap-1 text-[10px] text-muted">
      少
      <span v-for="(c, i) in levelColors" :key="i" class="inline-block h-2.5 w-2.5 rounded-[2px]" :style="{ backgroundColor: c }" />
      多
    </div>
  </div>
</template>
