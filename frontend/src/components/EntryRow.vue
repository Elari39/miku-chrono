<script setup lang="ts">
import type { Entry } from "../lib/api";
import { formatWhen } from "../lib/format";
import DurationText from "./DurationText.vue";

defineProps<{
  entry: Entry;
  /** Seconds to display — the whole entry, or only the part inside a day. */
  durationSeconds: number;
  /** Shows the 跨天 badge (the stats day view clips cross-midnight entries). */
  crossDay?: boolean;
}>();
</script>

<template>
  <div class="flex items-center gap-4 px-5 py-3.5 hover:bg-surface-card/50">
    <span class="h-8 w-1 rounded-full" :style="{ backgroundColor: entry.activityColor }" />
    <div class="min-w-0 flex-1">
      <div class="flex items-center gap-2">
        <span class="text-sm font-medium text-ink">{{ entry.activityName }}</span>
        <span
          class="rounded-full px-1.5 py-0.5 text-[10px]"
          :class="
            entry.source === 'timer'
              ? 'bg-primary/10 text-primary'
              : 'bg-accent-teal/15 text-accent-teal'
          "
          >{{ entry.source === "timer" ? "计时" : "补录" }}</span
        >
        <span
          v-if="crossDay"
          class="rounded-full bg-black/5 px-1.5 py-0.5 text-[10px] text-muted"
          >跨天</span
        >
      </div>
      <div class="mt-0.5 truncate text-xs text-muted">
        {{ formatWhen(entry.startedAt) }} → {{ formatWhen(entry.endedAt) }}
        <span v-if="entry.note" class="ml-1">· {{ entry.note }}</span>
      </div>
    </div>
    <DurationText :seconds="durationSeconds" class="text-sm font-medium text-body" />
    <slot />
  </div>
</template>
