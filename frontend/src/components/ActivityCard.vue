<script setup lang="ts">
import { computed } from "vue";
import type { Activity } from "../lib/api";
import DurationText from "./DurationText.vue";

const props = defineProps<{
  activity: Activity;
  todaySeconds: number;
  currentStreak: number;
  /** Whether this activity's timer is currently running. */
  isRunning: boolean;
}>();

const emit = defineEmits<{ start: []; stop: [] }>();

const goalSeconds = computed(() => props.activity.dailyGoalMinutes * 60);
const progress = computed(() => {
  if (goalSeconds.value <= 0) return 0;
  return Math.min(100, Math.round((props.todaySeconds / goalSeconds.value) * 100));
});
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
      <button
        v-if="isRunning"
        class="mc-btn-ghost border-primary/40 text-primary"
        @click="emit('stop')"
      >
        <span class="mr-1 inline-block h-2 w-2 rounded-full bg-primary" />停止
      </button>
      <button v-else class="mc-btn-primary" @click="emit('start')">开始</button>
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
