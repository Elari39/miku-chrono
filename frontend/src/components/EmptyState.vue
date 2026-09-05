<script setup lang="ts">
defineProps<{ title: string; hint?: string }>();
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-2 py-16 text-center">
    <div class="text-4xl opacity-40">◍</div>
    <p class="text-sm font-medium text-body">{{ title }}</p>
    <p v-if="hint" class="max-w-sm text-xs text-muted">{{ hint }}</p>
    <div class="mt-2"><slot /></div>
  </div>
</template>
