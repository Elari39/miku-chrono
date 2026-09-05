<script setup lang="ts">
import { computed } from "vue";
import { formatClock, formatDuration } from "../lib/format";

const props = withDefaults(
  defineProps<{
    seconds: number;
    /** clock = "1:01:01" mono digits; human = "1时01分" */
    mode?: "clock" | "human";
    mono?: boolean;
  }>(),
  { mode: "human", mono: false },
);

const text = computed(() =>
  props.mode === "clock" ? formatClock(props.seconds) : formatDuration(props.seconds),
);
</script>

<template>
  <span :class="props.mono || props.mode === 'clock' ? 'font-mono tabular-nums' : ''">{{
    text
  }}</span>
</template>
