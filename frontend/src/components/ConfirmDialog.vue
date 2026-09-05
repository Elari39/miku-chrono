<script setup lang="ts">
import Modal from "./Modal.vue";

withDefaults(
  defineProps<{
    open: boolean;
    title: string;
    message: string;
    danger?: boolean;
    confirmText?: string;
  }>(),
  { danger: false, confirmText: "确认" },
);
const emit = defineEmits<{ confirm: []; cancel: [] }>();
</script>

<template>
  <Modal :open="open" :title="title" @close="emit('cancel')">
    <p class="text-sm leading-relaxed text-body">{{ message }}</p>
    <template #footer>
      <button class="mc-btn-ghost" @click="emit('cancel')">取消</button>
      <button :class="danger ? 'mc-btn-danger' : 'mc-btn-primary'" @click="emit('confirm')">
        {{ confirmText }}
      </button>
    </template>
  </Modal>
</template>
