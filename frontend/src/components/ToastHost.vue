<script setup lang="ts">
import { useToast } from "../composables/useToast";

const { toasts } = useToast();

const styles: Record<string, string> = {
  info: "border-hairline bg-white text-body",
  success: "border-success/40 bg-white text-body",
  error: "border-error/40 bg-white text-body",
};
const dots: Record<string, string> = {
  info: "bg-muted",
  success: "bg-success",
  error: "bg-error",
};
</script>

<template>
  <Teleport to="body">
    <div class="fixed right-5 bottom-5 z-[60] flex flex-col gap-2">
      <TransitionGroup
        enter-active-class="transition duration-200 ease-out"
        enter-from-class="translate-y-2 opacity-0"
        leave-active-class="transition duration-150 ease-in"
        leave-to-class="opacity-0"
      >
        <div
          v-for="t in toasts"
          :key="t.id"
          class="flex items-center gap-2.5 rounded-xl border px-4 py-2.5 text-sm shadow-lg"
          :class="styles[t.kind]"
        >
          <span class="h-2 w-2 shrink-0 rounded-full" :class="dots[t.kind]" />
          <span class="max-w-xs break-all">{{ t.text }}</span>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>
