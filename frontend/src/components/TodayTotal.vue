<script setup lang="ts">
import { computed } from "vue";
import { useTimer } from "../composables/useTimer";
import { formatDuration, todayStr } from "../lib/format";

// Header total for the check-in page. Extracted from CheckInView so the
// per-second `state.sessionElapsed` tick only re-renders this span instead
// of re-running the whole view's template diff.
const props = defineProps<{
  /** Recorded seconds today across all activities (from Overview). */
  todaySeconds: number;
}>();

const { state } = useTimer();

// Live total: includes the running session's elapsed time (only when it
// started today — a session started before midnight counts towards
// yesterday). Uses sessionElapsed (current session only), NOT elapsed
// (chain total): the chain total already contains previously recorded
// sessions that todaySeconds covers too, so adding it would double-count
// on resume.
const displayTotal = computed(() => {
  if (state.running && state.startedAt.startsWith(todayStr())) {
    return props.todaySeconds + state.sessionElapsed;
  }
  return props.todaySeconds;
});
</script>

<template>
  <span class="font-medium text-body">{{ formatDuration(displayTotal) }}</span>
</template>
