<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { BallService } from "./lib/api";
import ToastHost from "./components/ToastHost.vue";
import Modal from "./components/Modal.vue";
import TimerPill from "./components/TimerPill.vue";

const route = useRoute();
const navItems = [
  { to: "/", label: "打卡" },
  { to: "/stats", label: "统计" },
  { to: "/records", label: "记录" },
  { to: "/settings", label: "设置" },
];

// The floating ball window renders this same SPA; on /ball it shows only the
// BallView without the app shell.
const isBallPage = computed(() => route.path === "/ball");

// First-run prompt: choose what the main-window × button does.
const closePromptOpen = ref(false);
const closeActionChoice = ref<"hide" | "quit">("hide");

onMounted(async () => {
  // The ball window shares this SPA but has no close button — asking for
  // the stored close action there would be a wasted IPC call.
  if (isBallPage.value) return;
  try {
    const action = await BallService.GetCloseAction();
    if (!action) closePromptOpen.value = true;
  } catch (err) {
    console.error("load close action failed", err);
  }
});

// Dismissing (Esc, overlay, ×) deliberately does NOT persist: the choice is
// only saved on the explicit confirm below, so the prompt reappears next
// launch until the user actually picks one.
function dismissClosePrompt() {
  closePromptOpen.value = false;
}

async function saveCloseAction() {
  try {
    await BallService.SetCloseAction(closeActionChoice.value);
  } catch (err) {
    console.error(err);
  }
  closePromptOpen.value = false;
}
</script>

<template>
  <div v-if="isBallPage" class="h-full">
    <router-view />
  </div>

  <div v-else class="flex h-full flex-col bg-canvas">
    <!-- 64px cream top navigation -->
    <header class="flex h-16 shrink-0 items-center gap-8 border-b border-hairline bg-canvas px-6">
      <router-link to="/" class="font-display text-lg font-semibold tracking-wide text-ink">
        Miku <span class="italic text-primary">Chrono</span>
      </router-link>

      <nav class="flex items-center gap-1">
        <router-link
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="rounded-lg px-3.5 py-2 text-sm font-medium transition-colors"
          :class="
            route.path === item.to
              ? 'bg-surface-card text-ink'
              : 'text-muted hover:bg-surface-card/60 hover:text-body'
          "
          :aria-current="route.path === item.to ? 'page' : undefined"
        >
          {{ item.label }}
        </router-link>
      </nav>

      <div class="ml-auto">
        <TimerPill />
      </div>
    </header>

    <!-- Scrollable page body -->
    <main class="flex-1 overflow-y-auto bg-canvas">
      <router-view />
    </main>

    <ToastHost />

    <!-- One-time choice for the main-window close button (changeable in 设置). -->
    <Modal :open="closePromptOpen" title="点击主窗口 × 时希望怎样？" @close="dismissClosePrompt">
      <p class="mb-4 text-sm text-muted">
        主窗口右上角的关闭按钮可以隐藏到后台（应用与悬浮球继续运行），也可以直接退出应用。之后可在「设置
        → 悬浮球」中随时修改。
      </p>
      <div class="space-y-2">
        <label
          class="flex cursor-pointer items-start gap-3 rounded-xl border border-hairline p-3 transition-colors hover:border-primary/40"
          :class="closeActionChoice === 'hide' ? 'border-primary/60 bg-primary/5' : ''"
        >
          <input
            v-model="closeActionChoice"
            type="radio"
            value="hide"
            class="mt-0.5 accent-[var(--color-primary)]"
          />
          <span>
            <span class="block text-sm font-medium text-ink">隐藏到后台（推荐）</span>
            <span class="block text-xs text-muted"
              >应用与悬浮球继续运行，双击悬浮球即可恢复主窗口。</span
            >
          </span>
        </label>
        <label
          class="flex cursor-pointer items-start gap-3 rounded-xl border border-hairline p-3 transition-colors hover:border-primary/40"
          :class="closeActionChoice === 'quit' ? 'border-primary/60 bg-primary/5' : ''"
        >
          <input
            v-model="closeActionChoice"
            type="radio"
            value="quit"
            class="mt-0.5 accent-[var(--color-primary)]"
          />
          <span>
            <span class="block text-sm font-medium text-ink">直接退出应用</span>
            <span class="block text-xs text-muted">关闭主窗口时连同悬浮球一起退出。</span>
          </span>
        </label>
      </div>
      <template #footer>
        <button class="mc-btn-primary px-5 py-2" @click="saveCloseAction">确定</button>
      </template>
    </Modal>
  </div>
</template>
