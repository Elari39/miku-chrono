<script setup lang="ts">
import { computed } from "vue";
import { useTimer } from "../composables/useTimer";
import { formatClock } from "../lib/format";

// Navbar pill showing the running timer. Extracted from App.vue so the
// per-second `state.elapsed` tick only re-renders this small component
// instead of the whole app shell.
const { state } = useTimer();

const active = computed(() => state.loaded && state.running);
</script>

<template>
  <router-link
    v-if="active"
    to="/"
    class="flex items-center gap-2.5 rounded-full border border-primary/30 bg-white py-1.5 pr-4 pl-2.5 shadow-sm transition-shadow hover:shadow"
  >
    <span class="relative flex h-2.5 w-2.5">
      <span
        class="absolute inline-flex h-full w-full animate-ping rounded-full opacity-60"
        :style="{ backgroundColor: state.activityColor }"
      />
      <span
        class="relative inline-flex h-2.5 w-2.5 rounded-full"
        :style="{ backgroundColor: state.activityColor }"
      />
    </span>
    <span class="max-w-28 truncate text-sm text-body">{{ state.activityName }}</span>
    <span class="font-mono text-sm font-semibold tabular-nums text-ink">{{
      formatClock(state.elapsed)
    }}</span>
  </router-link>
  <span v-else class="text-xs text-muted">未在计时</span>
</template>
