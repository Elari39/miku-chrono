<script setup lang="ts">
defineProps<{ open: boolean; title: string; wide?: boolean }>();
const emit = defineEmits<{ close: [] }>();
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-50 flex items-center justify-center bg-ink/40 p-6 backdrop-blur-[2px]"
      @click.self="emit('close')"
    >
      <div
        class="max-h-[85vh] w-full overflow-y-auto rounded-2xl bg-white p-6 shadow-2xl"
        :class="wide ? 'max-w-xl' : 'max-w-md'"
      >
        <div class="mb-4 flex items-center justify-between">
          <h3 class="text-base font-semibold text-ink">{{ title }}</h3>
          <button class="cursor-pointer rounded-md px-2 py-0.5 text-muted hover:bg-surface-card" @click="emit('close')">✕</button>
        </div>
        <slot />
        <div v-if="$slots.footer" class="mt-5 flex justify-end gap-2">
          <slot name="footer" />
        </div>
      </div>
    </div>
  </Teleport>
</template>
