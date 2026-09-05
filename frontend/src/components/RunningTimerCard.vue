<script setup lang="ts">
import { computed } from "vue";
import { useTimer } from "../composables/useTimer";
import { formatWhen } from "../lib/format";
import DurationText from "./DurationText.vue";

const { state, stop } = useTimer();

const visible = computed(() => state.loaded && state.running);

async function onStop() {
  try {
    const entry = await stop();
    // Entry details surface via toast in the view that owns refresh; here we
    // only guard errors.
    void entry;
  } catch (err) {
    console.error(err);
  }
}
</script>

<template>
  <div
    v-if="visible"
    class="flex items-center justify-between rounded-2xl bg-surface-dark px-6 py-5 text-white shadow-lg"
  >
    <div class="flex items-center gap-4">
      <span class="relative flex h-3.5 w-3.5">
        <span class="absolute inline-flex h-full w-full animate-ping rounded-full opacity-60" :style="{ backgroundColor: state.activityColor }" />
        <span class="relative inline-flex h-3.5 w-3.5 rounded-full" :style="{ backgroundColor: state.activityColor }" />
      </span>
      <div>
        <div class="text-lg font-semibold">{{ state.activityName || "计时中" }}</div>
        <div class="text-xs text-white/50">开始于 {{ formatWhen(state.startedAt) }}</div>
      </div>
    </div>
    <div class="flex items-center gap-6">
      <DurationText :seconds="state.elapsed" mode="clock" class="text-4xl font-semibold tracking-wide text-white" />
      <button class="mc-btn-primary px-5 py-2.5" @click="onStop">停止</button>
    </div>
  </div>
</template>
