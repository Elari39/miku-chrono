<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { CategoryService, StatsService, type ActivityStat, type Category } from "../lib/api";
import { useTimer } from "../composables/useTimer";
import { useToast } from "../composables/useToast";
import { formatDuration } from "../lib/format";
import ActivityCard from "../components/ActivityCard.vue";
import RunningTimerCard from "../components/RunningTimerCard.vue";
import EmptyState from "../components/EmptyState.vue";

const { state, running, version, start, stop } = useTimer();
const { success, error } = useToast();

const stats = ref<ActivityStat[]>([]);
const categories = ref<Category[]>([]);
const loading = ref(true);

const activeStats = computed(() => stats.value.filter((s) => !s.activity.archived));
const totalToday = computed(() => stats.value.reduce((sum, s) => sum + s.todaySeconds, 0));

// Group active activities by category (未分类 last). Only groups that
// actually contain activities render.
const groups = computed(() => {
  const byCategory = new Map<number | null, ActivityStat[]>();
  for (const s of activeStats.value) {
    const key = s.activity.categoryId ?? null;
    const bucket = byCategory.get(key);
    if (bucket) bucket.push(s);
    else byCategory.set(key, [s]);
  }
  const out: { category: Category | null; stats: ActivityStat[] }[] = [];
  for (const c of categories.value) {
    const bucket = byCategory.get(c.id);
    if (bucket?.length) out.push({ category: c, stats: bucket });
  }
  const uncategorized = byCategory.get(null);
  if (uncategorized?.length) out.push({ category: null, stats: uncategorized });
  return out;
});

// Headers only add meaning once there is more than one group or at least
// one real category exists; a single flat list renders exactly like before.
const showGroupHeaders = computed(
  () => categories.value.length > 0 || groups.value.length > 1,
);

async function load() {
  try {
    const [overview, cats] = await Promise.all([StatsService.Overview(), CategoryService.List()]);
    stats.value = overview ?? [];
    categories.value = cats ?? [];
  } catch (err) {
    console.error(err);
    error("加载活动列表失败");
  } finally {
    loading.value = false;
  }
}

async function onStart(id: number) {
  try {
    await start(id);
  } catch (err) {
    error(String((err as Error).message ?? err).replace(/^\w+:\s*/, ""));
  }
}

async function onStop() {
  try {
    const entry = await stop();
    if (entry) {
      success(`已记录 ${formatDuration(entry.durationSeconds)} · ${entry.activityName}`);
    }
    await load();
  } catch (err) {
    error(String((err as Error).message ?? err).replace(/^\w+:\s*/, ""));
  }
}

// Refresh aggregates whenever a timer starts/stops elsewhere (e.g. nav pill).
watch(version, () => void load());
onMounted(load);
</script>

<template>
  <div class="mx-auto max-w-3xl px-8 py-8">
    <div class="mb-6 flex items-end justify-between">
      <div>
        <h1 class="font-display text-2xl font-semibold text-ink">打卡</h1>
        <p class="mt-1 text-sm text-muted">
          今日已专注 <span class="font-medium text-body">{{ formatDuration(totalToday) }}</span>
        </p>
      </div>
    </div>

    <div class="mb-6">
      <RunningTimerCard />
    </div>

    <div v-if="loading" class="py-10 text-center text-sm text-muted">加载中…</div>
    <EmptyState
      v-else-if="activeStats.length === 0"
      title="还没有活动"
      hint="先到「活动」页创建一个活动,再回来开始计时打卡。"
    />
    <div v-else class="flex flex-col gap-8">
      <section v-for="g in groups" :key="g.category?.id ?? 'uncategorized'">
        <div v-if="showGroupHeaders" class="mb-3 flex items-center gap-2">
          <span
            v-if="g.category"
            class="h-2.5 w-2.5 rounded-full"
            :style="{ backgroundColor: g.category.color }"
          />
          <span v-if="g.category?.icon" class="text-base leading-none">{{ g.category.icon }}</span>
          <h2 class="text-sm font-semibold text-ink">{{ g.category ? g.category.name : "未分类" }}</h2>
          <span class="text-xs text-muted-soft">{{ g.stats.length }} 项</span>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <ActivityCard
            v-for="s in g.stats"
            :key="s.activity.id"
            :activity="s.activity"
            :today-seconds="s.todaySeconds"
            :current-streak="s.currentStreak"
            :is-running="running && state.activityId === s.activity.id"
            @start="onStart(s.activity.id)"
            @stop="onStop"
          />
        </div>
      </section>
    </div>
  </div>
</template>