<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";

const props = defineProps<{ open: boolean; title: string; wide?: boolean }>();
const emit = defineEmits<{ close: [] }>();

const overlay = ref<HTMLElement | null>(null);
const panel = ref<HTMLElement | null>(null);
const titleId = `mc-modal-title-${Math.random().toString(36).slice(2, 9)}`;

let previousFocus: HTMLElement | null = null;

/** True when this dialog is the last one mounted — stacked dialogs answer only the top one. */
function isTopmost(): boolean {
  const dialogs = document.querySelectorAll('[role="dialog"]');
  return dialogs.length > 0 && dialogs[dialogs.length - 1] === overlay.value;
}

function onWindowKeydown(e: KeyboardEvent) {
  if (e.key !== "Escape" || !props.open) return;
  if (!isTopmost()) return;
  emit("close");
}

// Keep focus inside the panel while tabbing; a stacked dialog traps its own.
function trapFocus(e: KeyboardEvent) {
  if (e.key !== "Tab") return;
  if (!isTopmost()) return;
  const el = panel.value;
  if (!el) return;
  const focusables = el.querySelectorAll<HTMLElement>(
    'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
  );
  if (focusables.length === 0) return;
  const first = focusables[0];
  const last = focusables[focusables.length - 1];
  const active = document.activeElement;
  if (e.shiftKey && (active === first || !el.contains(active))) {
    e.preventDefault();
    last.focus();
  } else if (!e.shiftKey && (active === last || !el.contains(active))) {
    e.preventDefault();
    first.focus();
  }
}

watch(
  () => props.open,
  async (open) => {
    if (open) {
      previousFocus = document.activeElement as HTMLElement | null;
      await nextTick();
      panel.value?.querySelector<HTMLElement>("button, [href], input, select, textarea")?.focus();
    } else if (previousFocus) {
      previousFocus.focus();
      previousFocus = null;
    }
  },
);

onMounted(() => {
  window.addEventListener("keydown", onWindowKeydown);
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", onWindowKeydown);
});
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      ref="overlay"
      class="fixed inset-0 z-50 flex items-center justify-center bg-ink/40 p-6 backdrop-blur-[2px]"
      role="dialog"
      aria-modal="true"
      :aria-labelledby="titleId"
      @click.self="emit('close')"
      @keydown="trapFocus"
    >
      <div
        ref="panel"
        class="max-h-[85vh] w-full overflow-y-auto rounded-2xl bg-white p-6 shadow-2xl"
        :class="wide ? 'max-w-xl' : 'max-w-md'"
      >
        <div class="mb-4 flex items-center justify-between">
          <h3 :id="titleId" class="text-base font-semibold text-ink">{{ title }}</h3>
          <button
            class="cursor-pointer rounded-md px-2 py-0.5 text-muted hover:bg-surface-card"
            aria-label="关闭"
            @click="emit('close')"
          >
            ✕
          </button>
        </div>
        <slot />
        <div v-if="$slots.footer" class="mt-5 flex justify-end gap-2">
          <slot name="footer" />
        </div>
      </div>
    </div>
  </Teleport>
</template>
