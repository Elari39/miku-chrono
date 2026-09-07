<script setup lang="ts">
import { PALETTE } from "../lib/palette";

// Shared palette swatch row used by the activity and category forms. The
// buttons expose aria-pressed so screen readers announce the selection.
defineProps<{
  /** The current hex color, bound with v-model. */
  modelValue: string;
}>();

const emit = defineEmits<{ "update:modelValue": [value: string] }>();
</script>

<template>
  <div class="flex flex-wrap gap-2" role="group" aria-label="颜色">
    <button
      v-for="c in PALETTE"
      :key="c"
      type="button"
      class="h-7 w-7 cursor-pointer rounded-full border-2 transition-transform"
      :class="modelValue === c ? 'scale-110 border-ink' : 'border-transparent'"
      :style="{ backgroundColor: c }"
      :aria-label="`颜色 ${c}`"
      :aria-pressed="modelValue === c"
      @click="emit('update:modelValue', c)"
    />
  </div>
</template>
