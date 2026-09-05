import { ref } from "vue";

export type ToastKind = "info" | "success" | "error";

export interface Toast {
  id: number;
  text: string;
  kind: ToastKind;
}

const toasts = ref<Toast[]>([]);
let nextId = 1;

function push(text: string, kind: ToastKind) {
  const id = nextId++;
  toasts.value.push({ id, text, kind });
  window.setTimeout(() => {
    toasts.value = toasts.value.filter((t) => t.id !== id);
  }, 3500);
}

export function useToast() {
  return {
    toasts,
    toast: (text: string, kind: ToastKind = "info") => push(text, kind),
    success: (text: string) => push(text, "success"),
    error: (text: string) => push(text, "error"),
  };
}
