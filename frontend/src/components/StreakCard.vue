<script setup lang="ts">
import type { StreakInfo } from "../lib/api";

defineProps<{ info: StreakInfo; title?: string }>();
</script>

<template>
  <div class="mc-card p-5">
    <h3 class="mb-3 text-sm font-medium text-muted">{{ title ?? "连续打卡" }}</h3>
    <div class="flex items-end gap-8">
      <div>
        <div class="font-display text-4xl font-semibold text-primary">{{ info.current }}</div>
        <div class="mt-1 text-xs text-muted">🔥 当前连续天数</div>
      </div>
      <div class="border-l border-hairline pl-8">
        <div class="font-display text-4xl font-semibold text-ink">{{ info.longest }}</div>
        <div class="mt-1 text-xs text-muted">🏆 最长连续天数</div>
      </div>
    </div>
  </div>
</template>
