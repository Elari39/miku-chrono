<script setup lang="ts">
import { computed, ref } from "vue";
import {
  ActivityService,
  CategoryService,
  StatsService,
  type Activity,
  type ActivityStat,
  type Category,
} from "../lib/api";
import { useTimer } from "../composables/useTimer";
import { useVersionedLoad } from "../composables/useVersionedLoad";
import { useToast } from "../composables/useToast";
import { formatDuration, todayStr } from "../lib/format";
import ActivityCard from "../components/ActivityCard.vue";
import ActivityFormModal from "../components/ActivityFormModal.vue";
import CategoryManagerModal from "../components/CategoryManagerModal.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import RunningTimerCard from "../components/RunningTimerCard.vue";
import EmptyState from "../components/EmptyState.vue";

const { state, running, start, stop } = useTimer();
const { success, error } = useToast();

const stats = ref<ActivityStat[]>([]);
const categories = ref<Category[]>([]);
// All activities (including archived) power the archived section and the
// category manager's per-category counts.
const allActivities = ref<Activity[]>([]);

const activeStats = computed(() => stats.value.filter((s) => !s.activity.archived));
const archived = computed(() => allActivities.value.filter((a) => a.archived));
const archivedOpen = ref(false);

// 「⋯」菜单全局单开：同一时刻至多一张活动卡片展开菜单。
const openMenuId = ref<number | null>(null);

const totalToday = computed(() => stats.value.reduce((sum, s) => sum + s.todaySeconds, 0));
// Live total: includes the running session's elapsed time (only when it
// started today — a session started before midnight counts towards yesterday).
// Uses sessionElapsed (current session only), NOT elapsed (chain total):
// the chain total already contains previously recorded sessions that
// todaySeconds covers too, so adding it would double-count on resume.
const displayTotal = computed(() => {
  if (state.running && state.startedAt.startsWith(todayStr())) {
    return totalToday.value + state.sessionElapsed;
  }
  return totalToday.value;
});

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
const showGroupHeaders = computed(() => categories.value.length > 0 || groups.value.length > 1);

// Aggregate refs reload on mount and whenever the timer starts/stops.
const { loading, reload } = useVersionedLoad(async (isCurrent) => {
  try {
    const [overview, cats, all] = await Promise.all([
      StatsService.Overview(),
      CategoryService.List(),
      ActivityService.List(true),
    ]);
    if (!isCurrent()) return;
    stats.value = overview ?? [];
    categories.value = cats ?? [];
    allActivities.value = all ?? [];
  } catch (err) {
    console.error(err);
    error("加载活动列表失败");
  }
});

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
    void reload();
  } catch (err) {
    error(String((err as Error).message ?? err).replace(/^\w+:\s*/, ""));
  }
}

// ── 活动新建 / 编辑 ──────────────────────────────────────
const editorOpen = ref(false);
const editing = ref<Activity | null>(null);

function openCreate() {
  editing.value = null;
  editorOpen.value = true;
}

function openEdit(a: Activity) {
  editing.value = a;
  editorOpen.value = true;
}

function onSaved() {
  editorOpen.value = false;
  void reload();
}

const catManagerOpen = ref(false);

function onCatChanged() {
  void reload();
}

async function setArchived(a: Activity, archived: boolean) {
  try {
    await ActivityService.SetArchived(a.id, archived);
    success(archived ? "已归档" : "已恢复");
    await reload();
  } catch (err) {
    error(String((err as Error).message ?? err).replace(/^\w+:\s*/, ""));
  }
}

// ── 删除活动 ────────────────────────────────────────────
const deleteTarget = ref<Activity | null>(null);

const deleteMessage = computed(() => {
  if (!deleteTarget.value) return "";
  const runningNote =
    state.running && state.activityId === deleteTarget.value.id ? "（进行中的计时会先停止）" : "";
  return `删除「${deleteTarget.value.name}」会同时删除它的所有计时记录，且无法恢复。${runningNote}确定继续吗？`;
});

async function doDelete() {
  const target = deleteTarget.value;
  if (!target) return;
  try {
    // Deleting the activity whose timer is running would orphan the local
    // timer state; stop first (failure aborts the delete).
    if (state.running && state.activityId === target.id) {
      await stop();
    }
    await ActivityService.Delete(target.id);
    success("活动及其记录已删除");
    deleteTarget.value = null;
    await reload();
  } catch (err) {
    error(String((err as Error).message ?? err).replace(/^\w+:\s*/, ""));
  }
}
</script>

<template>
  <div class="mx-auto max-w-3xl px-8 py-8">
    <div class="mb-6 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="font-display text-2xl font-semibold text-ink">打卡</h1>
        <p class="mt-1 text-sm text-muted">
          今日已专注 <span class="font-medium text-body">{{ formatDuration(displayTotal) }}</span>
        </p>
      </div>
      <div class="flex items-center gap-3">
        <button class="mc-btn-ghost" @click="catManagerOpen = true">类别管理</button>
        <button class="mc-btn-primary" @click="openCreate">＋ 新建活动</button>
      </div>
    </div>

    <div class="mb-6">
      <RunningTimerCard />
    </div>

    <div v-if="loading" class="py-10 text-center text-sm text-muted">加载中…</div>
    <template v-else>
      <EmptyState
        v-if="activeStats.length === 0"
        title="还没有活动"
        hint="点击右上角「＋ 新建活动」创建一个活动，然后回来开始计时。"
      >
        <button class="mc-btn-primary" @click="openCreate">＋ 新建活动</button>
      </EmptyState>

      <div v-else class="flex flex-col gap-8">
        <section v-for="g in groups" :key="g.category?.id ?? 'uncategorized'">
          <div v-if="showGroupHeaders" class="mb-3 flex items-center gap-2">
            <span
              v-if="g.category"
              class="h-2.5 w-2.5 rounded-full"
              :style="{ backgroundColor: g.category.color }"
            />
            <span v-if="g.category?.icon" class="text-base leading-none">{{
              g.category.icon
            }}</span>
            <h2 class="text-sm font-semibold text-ink">
              {{ g.category ? g.category.name : "未分类" }}
            </h2>
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
              :menu-open="openMenuId === s.activity.id"
              @toggle="openMenuId = openMenuId === s.activity.id ? null : s.activity.id"
              @close="openMenuId = null"
              @start="onStart(s.activity.id)"
              @stop="onStop"
              @edit="openEdit(s.activity)"
              @archive="setArchived(s.activity, true)"
              @delete="deleteTarget = s.activity"
            />
          </div>
        </section>
      </div>

      <!-- 已归档：折叠区块，可恢复或删除 -->
      <section v-if="archived.length > 0" class="mt-8 border-t border-hairline pt-4">
        <button
          class="flex w-full items-center justify-between py-1 text-left"
          :aria-expanded="archivedOpen"
          @click="archivedOpen = !archivedOpen"
        >
          <span class="text-sm font-semibold text-ink">已归档（{{ archived.length }}）</span>
          <span
            class="text-xs text-muted transition-transform"
            :class="archivedOpen ? 'rotate-180' : ''"
            >▾</span
          >
        </button>
        <div v-if="archivedOpen" class="mt-2 flex flex-col gap-2">
          <div v-for="a in archived" :key="a.id" class="mc-card flex items-center gap-3 p-3">
            <span class="h-8 w-1 shrink-0 rounded-full" :style="{ backgroundColor: a.color }" />
            <span v-if="a.icon" class="text-base leading-none">{{ a.icon }}</span>
            <span class="min-w-0 flex-1 truncate text-sm font-medium text-ink">{{ a.name }}</span>
            <button class="mc-btn-ghost px-3 py-1.5 text-xs" @click="setArchived(a, false)">
              恢复
            </button>
            <button
              class="mc-btn-ghost px-3 py-1.5 text-xs text-error hover:bg-error/10"
              @click="deleteTarget = a"
            >
              删除
            </button>
          </div>
        </div>
      </section>
    </template>

    <ActivityFormModal
      :open="editorOpen"
      :activity="editing"
      :categories="categories"
      @close="editorOpen = false"
      @saved="onSaved"
    />

    <CategoryManagerModal
      :open="catManagerOpen"
      :categories="categories"
      :activities="allActivities"
      @close="catManagerOpen = false"
      @changed="onCatChanged"
    />

    <ConfirmDialog
      :open="deleteTarget !== null"
      title="删除活动"
      :danger="true"
      confirm-text="全部删除"
      :message="deleteMessage"
      @confirm="doDelete"
      @cancel="deleteTarget = null"
    />
  </div>
</template>
