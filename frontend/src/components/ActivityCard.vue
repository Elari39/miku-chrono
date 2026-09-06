<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import type { Activity } from "../lib/api";
import DurationText from "./DurationText.vue";

const props = defineProps<{
  activity: Activity;
  todaySeconds: number;
  currentStreak: number;
  /** Whether this activity's timer is currently running. */
  isRunning: boolean;
}>();

const emit = defineEmits<{ start: []; stop: []; edit: []; archive: []; delete: [] }>();

const goalSeconds = computed(() => props.activity.dailyGoalMinutes * 60);
const progress = computed(() => {
  if (goalSeconds.value <= 0) return 0;
  return Math.min(100, Math.round((props.todaySeconds / goalSeconds.value) * 100));
});

// ── 操作菜单（编辑 / 归档 / 删除） ───────────────────────
const menuOpen = ref(false);
const menuButton = ref<HTMLElement | null>(null);
const menuPanel = ref<HTMLElement | null>(null);

function onClickOutside(e: MouseEvent) {
  if (!menuOpen.value) return;
  const target = e.target as Node;
  if (menuButton.value?.contains(target) || menuPanel.value?.contains(target)) return;
  menuOpen.value = false;
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") menuOpen.value = false;
}

onMounted(() => {
  document.addEventListener("click", onClickOutside);
  document.addEventListener("keydown", onKeydown);
});

onBeforeUnmount(() => {
  document.removeEventListener("click", onClickOutside);
  document.removeEventListener("keydown", onKeydown);
});

function runAction(action: () => void) {
  menuOpen.value = false;
  action();
}
</script>

<template>
  <div
    class="mc-card relative overflow-hidden p-5 transition-shadow hover:shadow-md"
    :class="isRunning ? 'ring-2 ring-primary/40' : ''"
  >
    <span class="absolute inset-y-0 left-0 w-1" :style="{ backgroundColor: activity.color }" />
    <div class="mb-3 flex items-start justify-between">
      <div>
        <div class="flex items-center gap-2">
          <span v-if="activity.icon" class="text-lg leading-none">{{ activity.icon }}</span>
          <h3 class="text-base font-semibold text-ink">{{ activity.name }}</h3>
          <span
            v-if="currentStreak > 0"
            class="rounded-full bg-accent-amber/15 px-2 py-0.5 text-xs font-medium text-accent-amber"
            >🔥 连续 {{ currentStreak }} 天</span
          >
        </div>
        <p class="mt-1 text-xs text-muted">
          今日
          <DurationText :seconds="todaySeconds" />
          <template v-if="goalSeconds > 0"> / 目标 {{ activity.dailyGoalMinutes }} 分钟</template>
        </p>
      </div>
      <div class="flex items-center gap-1.5">
        <button
          v-if="isRunning"
          class="mc-btn-ghost border-primary/40 text-primary"
          @click="emit('stop')"
        >
          <span class="mr-1 inline-block h-2 w-2 rounded-full bg-primary" />停止
        </button>
        <button v-else class="mc-btn-primary" @click="emit('start')">开始</button>

        <div class="relative">
          <button
            ref="menuButton"
            class="flex h-8 w-8 items-center justify-center rounded-lg text-lg leading-none text-muted transition-colors hover:bg-surface-card hover:text-body"
            :aria-label="`${activity.name} 的更多操作`"
            :aria-expanded="menuOpen"
            @click.stop="menuOpen = !menuOpen"
          >
            ⋯
          </button>
          <div
            v-if="menuOpen"
            ref="menuPanel"
            class="absolute right-0 top-full z-10 mt-1 flex w-28 flex-col overflow-hidden rounded-xl border border-hairline bg-surface-card py-1 shadow-lg"
          >
            <button
              class="px-3 py-1.5 text-left text-sm text-body hover:bg-surface-soft"
              @click="runAction(() => emit('edit'))"
            >
              编辑
            </button>
            <button
              class="px-3 py-1.5 text-left text-sm text-body hover:bg-surface-soft"
              @click="runAction(() => emit('archive'))"
            >
              归档
            </button>
            <button
              class="px-3 py-1.5 text-left text-sm text-error hover:bg-error/10"
              @click="runAction(() => emit('delete'))"
            >
              删除
            </button>
          </div>
        </div>
      </div>
    </div>
    <div v-if="goalSeconds > 0" class="h-1.5 overflow-hidden rounded-full bg-surface-card">
      <div
        class="h-full rounded-full transition-all duration-500"
        :class="progress >= 100 ? 'bg-success' : ''"
        :style="{
          width: progress + '%',
          backgroundColor: progress >= 100 ? undefined : activity.color,
        }"
      />
    </div>
    <div v-if="goalSeconds > 0" class="mt-1 text-right text-[11px] text-muted">{{ progress }}%</div>
  </div>
</template>
