<script setup lang="ts">
import { computed } from "vue";
import { useTimer } from "../composables/useTimer";
import { useToday } from "../composables/useToday";
import { formatDuration } from "../lib/format";

// Header total for the check-in page. Extracted from CheckInView so the
// per-second `state.sessionElapsed` tick only re-renders this span instead
// of re-running the whole view's template diff.
const props = defineProps<{
  /** Recorded seconds today across all activities (from Overview). */
  todaySeconds: number;
}>();

const { state } = useTimer();
const { today } = useToday();

// Live total: includes the running session's elapsed time (only when it
// started today — a session started before midnight counts towards
// yesterday). Uses sessionElapsed (current session only), NOT elapsed
// (chain total): the chain total already contains previously recorded
// sessions that todaySeconds covers too, so adding it would double-count
// on resume. `today` is the reactive date, so the running session flips
// out of "today" exactly at midnight instead of freezing on the mount-day
// comparison.
const displayTotal = computed(() => {
  if (state.running && state.startedAt.startsWith(today.value)) {
    return props.todaySeconds + state.sessionElapsed;
  }
  return props.todaySeconds;
});
</script>

<template>
  <span class="font-medium text-body">{{ formatDuration(displayTotal) }}</span>
</template>
