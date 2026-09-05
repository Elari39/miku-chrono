<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import {
  StatsService,
  type ActivityTotal,
  type DayBucket,
  type StreakInfo,
} from "../lib/api";
import { useToast } from "../composables/useToast";
import { useTimer } from "../composables/useTimer";
import { daysAgoStr, formatDurationLong, todayStr } from "../lib/format";
import BarChart from "../components/BarChart.vue";
import Heatmap from "../components/Heatmap.vue";
import StreakCard from "../components/StreakCard.vue";

const { version } = useTimer();
const { error } = useToast();
type RangePreset = 7 | 30 | 90;
const preset = ref<RangePreset>(7);
const fromDate = ref(daysAgoStr(6));
const toDate = ref(todayStr());

const buckets = ref<DayBucket[]>([]);
const totals = ref<ActivityTotal[]>([]);
const heatmapDays = ref<Record<string, number>>({});
const overallStreak = ref<StreakInfo>({ current: 0, longest: 0 });

const colors = computed<Record<string, string>>(() => {
  const map: Record<string, string> = {};
  for (const t of totals.value) map[String(t.activity.id)] = t.activity.color;
  return map;
});
const names = computed<Record<string, string>>(() => {
  const map: Record<string, string> = {};
  for (const t of totals.value) map[String(t.activity.id)] = t.activity.name;
  return map;
});
const grandTotal = computed(() => totals.value.reduce((s, t) => s + t.seconds, 0));

function applyPreset(days: RangePreset) {
  preset.value = days;
  fromDate.value = daysAgoStr(days - 1);
  toDate.value = todayStr();
}

async function load() {
  try {
    const [b, t, h, s] = await Promise.all([
      StatsService.DailyStacked(fromDate.value, toDate.value),
      StatsService.ActivityTotals(fromDate.value, toDate.value),
      StatsService.Heatmap(119),
      StatsService.Streaks({ activityId: null }),
    ]);
    buckets.value = b ?? [];
    totals.value = t ?? [];
    // Normalize: generated map type is Record<string, number | undefined>.
    heatmapDays.value = Object.fromEntries(
      Object.entries(h ?? {}).map(([k, v]) => [k, v ?? 0]),
    );
    overallStreak.value = s ?? { current: 0, longest: 0 };
  } catch (err) {
    console.error(err);
    error("加载统计数据失败");
  }
}

watch(version, () => void load());
onMounted(load);
</script>

<template>
  <div class="mx-auto max-w-4xl px-8 py-8">
    <div class="mb-6 flex flex-wrap items-end justify-between gap-3">
      <h1 class="font-display text-2xl font-semibold text-ink">统计</h1>
      <div class="flex items-center gap-2">
        <div class="flex rounded-lg border border-hairline bg-white p-0.5">
          <button
            v-for="d in [7, 30, 90] as RangePreset[]"
            :key="d"
            class="cursor-pointer rounded-md px-3 py-1.5 text-sm transition-colors"
            :class="preset === d ? 'bg-primary text-white' : 'text-body hover:bg-surface-card'"
            @click="applyPreset(d)"
          >{{ d }}天</button>
        </div>
        <input v-model="fromDate" type="date" class="mc-input w-36" @change="preset = 0 as unknown as RangePreset" />
        <span class="text-muted">–</span>
        <input v-model="toDate" type="date" class="mc-input w-36" @change="preset = 0 as unknown as RangePreset" />
        <button class="mc-btn-ghost" @click="load">应用</button>
      </div>
    </div>

    <div class="grid gap-4 lg:grid-cols-3">
      <div class="mc-card p-5 lg:col-span-2">
        <div class="mb-3 flex items-center justify-between">
          <h3 class="text-sm font-medium text-muted">每日专注时长</h3>
          <div class="flex items-center gap-3 text-xs text-muted">
            <span v-for="t in totals" :key="t.activity.id" class="flex items-center gap-1">
              <span class="h-2.5 w-2.5 rounded-[3px]" :style="{ backgroundColor: t.activity.color }" />
              {{ t.activity.name }}
            </span>
          </div>
        </div>
        <BarChart :buckets="buckets" :colors="colors" :names="names" />
      </div>

      <div class="flex flex-col gap-4">
        <StreakCard :info="overallStreak" title="连续打卡（全部活动）" />
        <div class="mc-card p-5">
          <h3 class="mb-3 text-sm font-medium text-muted">时段合计</h3>
          <p class="font-display text-3xl font-semibold text-ink">{{ formatDurationLong(grandTotal) }}</p>
          <div class="mt-4 flex flex-col gap-2.5">
            <div v-for="t in totals" :key="t.activity.id">
              <div class="mb-1 flex justify-between text-xs">
                <span class="text-body">{{ t.activity.name }}</span>
                <span class="text-muted">{{ formatDurationLong(t.seconds) }}</span>
              </div>
              <div class="h-1.5 overflow-hidden rounded-full bg-surface-card">
                <div
                  class="h-full rounded-full"
                  :style="{ width: grandTotal > 0 ? (t.seconds / grandTotal) * 100 + '%' : '0%', backgroundColor: t.activity.color }"
                />
              </div>
            </div>
            <p v-if="totals.length === 0" class="text-xs text-muted">所选时段暂无记录</p>
          </div>
        </div>
      </div>
    </div>

    <div class="mc-card mt-4 p-5">
      <h3 class="mb-3 text-sm font-medium text-muted">近 17 周热力图</h3>
      <Heatmap :days="heatmapDays" :weeks="17" />
    </div>
  </div>
</template>
