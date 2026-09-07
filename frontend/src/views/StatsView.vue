<script setup lang="ts">
import { computed, ref } from "vue";
import {
  EntryService,
  StatsService,
  type ActivityTotal,
  type Entry,
  type EntryFilter,
  type StreakInfo,
} from "../lib/api";
import { useToast } from "../composables/useToast";
import { useVersionedLoad } from "../composables/useVersionedLoad";
import { dateStr, formatDay, formatDurationLong, todayStr } from "../lib/format";
import {
  buildTimelineSegments,
  dayPortionSeconds,
  elapsedDaysInPeriod,
  fillDailyBuckets,
  fillMonthBuckets,
  GRANULARITY_LABELS,
  GRANULARITY_ORDER,
  peakBucket,
  periodEnd,
  periodLabel,
  periodStart,
  shiftPeriod,
  type Granularity,
  type StatBucket,
} from "../lib/statsPeriods";
import BarChart from "../components/BarChart.vue";
import DurationText from "../components/DurationText.vue";
import EmptyState from "../components/EmptyState.vue";
import EntryRow from "../components/EntryRow.vue";
import Timeline24h from "../components/Timeline24h.vue";

const { error } = useToast();

// ---- period state: granularity + an anchor date inside the period ----
const gran = ref<Granularity>("day");
const anchor = ref(todayStr());

const range = computed(() => ({
  start: periodStart(gran.value, anchor.value),
  end: periodEnd(gran.value, anchor.value),
}));
const label = computed(() => periodLabel(gran.value, anchor.value));
const isYear = computed(() => gran.value === "year");
const chartTitle = computed(() => (isYear.value ? "每月专注时长" : "每日专注时长"));

// ---- data ----
const buckets = ref<StatBucket[]>([]);
const totals = ref<ActivityTotal[]>([]);
const dayEntries = ref<Entry[]>([]);
const dayTotal = ref(0);
const streak = ref<StreakInfo>({ current: 0, longest: 0 });

const colors = computed(() => {
  const map: Record<string, string> = {};
  for (const t of totals.value) map[String(t.activity.id)] = t.activity.color;
  return map;
});
const names = computed(() => {
  const map: Record<string, string> = {};
  for (const t of totals.value) map[String(t.activity.id)] = t.activity.name;
  return map;
});

const grandTotal = computed(() => totals.value.reduce((s, t) => s + t.seconds, 0));
const peak = computed(() => peakBucket(buckets.value));
// Day view shows the overlap-filtered list, so cross-midnight entries are
// clipped to the displayed day's own slice on the timeline.
const timelineSegments = computed(() => buildTimelineSegments(dayEntries.value, range.value.start));

// Cross-midnight badge: either endpoint falls outside the displayed day.
// Both endpoints are parsed offset-aware via Date — slicing the stored
// string would read the UTC calendar date since the v3 storage migration.
function isCrossDay(e: Entry): boolean {
  return (
    dateStr(new Date(e.startedAt)) !== range.value.start ||
    dateStr(new Date(e.endedAt)) !== range.value.start
  );
}
// Entry point for the full list when the day view truncates at 200 items.
const recordsLink = computed(() => ({
  path: "/records",
  query: { from: range.value.start, to: range.value.end },
}));

async function loadPeriod(isCurrent: () => boolean) {
  try {
    const { start, end } = range.value;
    const g = gran.value;
    if (g === "day") {
      const [totalsRes, list] = await Promise.all([
        StatsService.ActivityTotals(start, end),
        EntryService.List({
          activityId: null,
          fromDate: start,
          toDate: end,
          // Overlap: yesterday-started entries that cross midnight show up
          // on today's timeline and detail list, clipped to today's part.
          overlap: true,
          page: 1,
          pageSize: 200,
        } satisfies EntryFilter),
      ]);
      if (!isCurrent()) return;
      totals.value = totalsRes ?? [];
      dayEntries.value = list?.items ?? [];
      dayTotal.value = list?.total ?? 0;
      buckets.value = [];
    } else if (g === "year") {
      const year = Number(start.slice(0, 4));
      const [monthRes, totalsRes] = await Promise.all([
        StatsService.MonthlyStacked(year),
        StatsService.ActivityTotals(start, end),
      ]);
      if (!isCurrent()) return;
      buckets.value = fillMonthBuckets(year, monthRes ?? []);
      totals.value = totalsRes ?? [];
      dayEntries.value = [];
      dayTotal.value = 0;
    } else {
      const [bucketRes, totalsRes] = await Promise.all([
        StatsService.DailyStacked(start, end),
        StatsService.ActivityTotals(start, end),
      ]);
      if (!isCurrent()) return;
      buckets.value = fillDailyBuckets(start, end, bucketRes ?? []);
      totals.value = totalsRes ?? [];
      dayEntries.value = [];
      dayTotal.value = 0;
    }
  } catch (err) {
    console.error(err);
    error("加载统计数据失败");
  }
}

// Period data reloads on granularity/anchor changes and on timer activity;
// streaks are range-independent.
const { loading } = useVersionedLoad(loadPeriod, {
  triggers: [gran, anchor],
});
useVersionedLoad(async (isCurrent) => {
  try {
    const s = await StatsService.Streaks({ activityId: null });
    if (!isCurrent()) return;
    streak.value = s ?? { current: 0, longest: 0 };
  } catch (err) {
    console.error(err);
  }
});

// ---- navigation ----
const QUICK_LABELS: Record<Granularity, string> = {
  day: "今天",
  week: "本周",
  month: "本月",
  year: "今年",
};

function switchGran(g: Granularity) {
  if (gran.value === g) return;
  gran.value = g;
  anchor.value = todayStr();
}
const prevPeriod = () => (anchor.value = shiftPeriod(gran.value, anchor.value, -1));
const nextPeriod = () => (anchor.value = shiftPeriod(gran.value, anchor.value, 1));
const goCurrent = () => (anchor.value = todayStr());
function onDayPick(e: Event) {
  const v = (e.target as HTMLInputElement).value;
  if (v) anchor.value = v;
}

// ---- summary cards ----
const metric3 = computed(() => {
  if (gran.value === "month" || gran.value === "year") {
    const days = elapsedDaysInPeriod(gran.value, anchor.value, todayStr());
    const avg = days > 0 ? Math.round(grandTotal.value / days) : 0;
    return {
      label: "日均时长",
      value: formatDurationLong(avg),
      sub: days > 0 ? `按已过 ${days} 天折算` : "尚未开始",
    };
  }
  if (gran.value === "week") {
    const p = peak.value;
    return {
      label: "峰值日",
      value: p ? formatDurationLong(p.total) : "0 分钟",
      sub: p ? formatDay(p.date, true) : "本周暂无记录",
    };
  }
  return {
    label: "记录条数",
    value: `${dayTotal.value} 条`,
    sub: dayTotal.value > 200 ? "时间轴与列表仅展示前 200 条" : "含计时与补录",
  };
});
const participantsSub = computed(() =>
  totals.value.length ? totals.value.map((t) => t.activity.name).join("、") : "暂无活动记录",
);
</script>

<template>
  <div class="mx-auto max-w-5xl px-8 py-8">
    <div class="mb-6 flex flex-wrap items-center gap-x-4 gap-y-3">
      <h1 class="shrink-0 font-display text-2xl font-semibold text-ink">统计</h1>
      <!-- granularity switcher: left-anchored next to the title, so its
           position never depends on the right-side controls -->
      <div class="flex rounded-lg border border-hairline bg-white p-0.5">
        <button
          v-for="g in GRANULARITY_ORDER"
          :key="g"
          class="cursor-pointer rounded-md px-3 py-1.5 text-sm transition-colors"
          :class="gran === g ? 'bg-primary font-medium text-white' : 'text-muted hover:text-body'"
          @click="switchGran(g)"
        >
          {{ GRANULARITY_LABELS[g] }}
        </button>
      </div>
      <!-- period navigation + day picker: right-anchored; width changes here
           never displace the switcher -->
      <div class="ml-auto flex flex-wrap items-center justify-end gap-1">
        <div class="flex items-center gap-1">
          <button class="mc-btn-ghost px-2 py-1.5" title="上一段" @click="prevPeriod">←</button>
          <span class="whitespace-nowrap text-sm font-medium text-ink">{{ label }}</span>
          <button class="mc-btn-ghost px-2 py-1.5" title="下一段" @click="nextPeriod">→</button>
          <button class="mc-btn-ghost py-1.5" @click="goCurrent">{{ QUICK_LABELS[gran] }}</button>
        </div>
        <input
          v-if="gran === 'day'"
          type="date"
          :value="anchor"
          class="mc-input w-36"
          @change="onDayPick"
        />
      </div>
    </div>

    <!-- summary cards -->
    <div class="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
      <div class="mc-card p-4">
        <div class="text-xs text-muted">时段总时长</div>
        <div class="mt-1.5 font-display text-2xl font-semibold text-ink">
          {{ formatDurationLong(grandTotal) }}
        </div>
        <div class="mt-1 truncate text-[11px] text-muted">{{ range.start }} 至 {{ range.end }}</div>
      </div>

      <div class="mc-card p-4">
        <div class="text-xs text-muted">连续打卡</div>
        <div class="mt-1.5 flex items-baseline gap-2">
          <span>
            <span class="font-display text-2xl font-semibold text-primary">{{
              streak.current
            }}</span>
            <span class="ml-0.5 text-xs text-muted">天</span>
          </span>
          <span class="text-hairline">/</span>
          <span>
            <span class="font-display text-2xl font-semibold text-ink">{{ streak.longest }}</span>
            <span class="ml-0.5 text-xs text-muted">天最长</span>
          </span>
        </div>
        <div class="mt-1 text-[11px] text-muted">全部活动口径</div>
      </div>

      <div class="mc-card p-4">
        <div class="text-xs text-muted">{{ metric3.label }}</div>
        <div class="mt-1.5 font-display text-2xl font-semibold text-ink">{{ metric3.value }}</div>
        <div class="mt-1 truncate text-[11px] text-muted">{{ metric3.sub }}</div>
      </div>

      <div class="mc-card p-4">
        <div class="text-xs text-muted">参与活动</div>
        <div class="mt-1.5 font-display text-2xl font-semibold text-ink">{{ totals.length }}</div>
        <div class="mt-1 truncate text-[11px] text-muted">{{ participantsSub }}</div>
      </div>
    </div>

    <div v-if="loading" class="py-10 text-center text-sm text-muted">加载中…</div>

    <div v-else class="grid gap-4 lg:grid-cols-3">
      <!-- main chart card -->
      <div class="mc-card p-5 lg:col-span-2">
        <div class="mb-3 text-sm font-medium text-ink">{{ chartTitle }}</div>
        <BarChart
          v-if="gran !== 'day'"
          :buckets="buckets"
          :colors="colors"
          :names="names"
          :label-mode="isYear ? 'month' : 'day'"
        />
        <template v-else>
          <Timeline24h v-if="dayEntries.length" :segments="timelineSegments" />
          <EmptyState v-else title="这一天还没有记录" hint="去计时或到记录页补录，再回来看看。" />
        </template>
      </div>

      <!-- activity share card -->
      <div class="mc-card p-5">
        <div class="text-sm font-medium text-ink">活动占比</div>
        <div class="mt-2 font-display text-2xl font-semibold text-ink">
          {{ formatDurationLong(grandTotal) }}
        </div>
        <div class="mt-4 flex flex-col gap-3">
          <div v-for="t in totals" :key="t.activity.id">
            <div class="mb-1 flex items-center justify-between text-xs">
              <span class="flex min-w-0 items-center gap-1.5">
                <span
                  class="h-2 w-2 shrink-0 rounded-full"
                  :style="{ backgroundColor: t.activity.color }"
                />
                <span class="truncate text-body">{{ t.activity.name }}</span>
              </span>
              <DurationText :seconds="t.seconds" class="shrink-0 text-muted" />
            </div>
            <div class="h-1.5 overflow-hidden rounded-full bg-black/5">
              <div
                class="h-full rounded-full"
                :style="{
                  backgroundColor: t.activity.color,
                  width: `${grandTotal > 0 ? (t.seconds / grandTotal) * 100 : 0}%`,
                }"
              />
            </div>
          </div>
          <div v-if="!totals.length" class="text-xs text-muted">该时段暂无活动记录</div>
        </div>
      </div>
    </div>

    <!-- day-mode detail list -->
    <template v-if="gran === 'day' && !loading">
      <div class="mb-3 mt-6 flex items-center justify-between">
        <div class="text-sm font-medium text-ink">当日明细</div>
        <div v-if="dayTotal > 200" class="text-xs text-muted">
          时间轴与明细仅展示前 200 条，共 {{ dayTotal }} 条 ·
          <router-link :to="recordsLink" class="text-primary hover:underline">
            在记录页查看完整列表</router-link
          >
        </div>
      </div>
      <div v-if="dayEntries.length" class="mc-card divide-y divide-hairline overflow-hidden">
        <EntryRow
          v-for="e in dayEntries"
          :key="e.id"
          :entry="e"
          :duration-seconds="dayPortionSeconds(e, range.start)"
          :cross-day="isCrossDay(e)"
        />
      </div>
    </template>
  </div>
</template>
