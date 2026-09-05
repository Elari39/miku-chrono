<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { Events, Screens, Window } from "@wailsio/runtime";
import { useTimer } from "../composables/useTimer";
import { formatClock } from "../lib/format";
import { BallService } from "../lib/api";

// Must match the window size in main.go — the ball is never resized at runtime.
const BALL_W = 280;
const EDGE_MARGIN = 16;

const { state, refresh } = useTimer();

const running = computed(() => state.running);

// Transient hint shown when the context-menu action cannot be performed.
const toast = ref("");
let toastTimer: number | undefined;
function showToast(message: string) {
  toast.value = message;
  if (toastTimer !== undefined) window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => (toast.value = ""), 1800);
}

// Poll the authoritative state once per second so the ball always mirrors the
// main window (start/stop from anywhere is reflected within ~1s).
let pollTimer: number | undefined;

function startPolling() {
  stopPolling();
  pollTimer = window.setInterval(async () => {
    try {
      await refresh();
    } catch (err) {
      console.error("ball refresh failed", err);
    }
  }, 1000);
}

function stopPolling() {
  if (pollTimer !== undefined) {
    window.clearInterval(pollTimer);
    pollTimer = undefined;
  }
}

// `useTimer` already starts a local 1s ticker while running, so the clock
// stays smooth between the authoritative refreshes above.

/** True when the rectangle is at least partially on a connected screen. */
async function positionOnScreen(x: number, y: number): Promise<boolean> {
  const screens = await Screens.GetAll();
  const slack = 48; // tolerate partially offscreen positions
  return screens.some(
    (s) =>
      x >= s.WorkArea.X - slack &&
      x <= s.WorkArea.X + s.WorkArea.Width + slack &&
      y >= s.WorkArea.Y - slack &&
      y <= s.WorkArea.Y + s.WorkArea.Height + slack,
  );
}

/** Restore the saved position, or snap to the top-right of the primary screen. */
async function placeBall() {
  try {
    const pos = await BallService.GetBallPosition();
    if (pos.set && (await positionOnScreen(pos.x, pos.y))) {
      await Window.SetPosition(pos.x, pos.y);
      return;
    }
    const primary = await Screens.GetPrimary();
    const wa = primary.WorkArea;
    await Window.SetPosition(wa.X + wa.Width - BALL_W - EDGE_MARGIN, wa.Y + EDGE_MARGIN);
  } catch (err) {
    console.error("place ball failed", err);
  }
}

async function toggleMainWindow() {
  try {
    await BallService.ToggleMainWindow();
  } catch (err) {
    console.error(err);
  }
}

function onContextMenu(e: MouseEvent) {
  // The native mini menu is bound via --custom-contextmenu; make sure the
  // webview's own menu never appears.
  e.preventDefault();
}

function makePageTransparent() {
  document.documentElement.style.background = "transparent";
  document.body.style.background = "transparent";
}

function restorePageBackground() {
  document.documentElement.style.background = "";
  document.body.style.background = "";
}

onMounted(() => {
  makePageTransparent();
  void placeBall();
  startPolling();
  Events.On("ball:toast", (ev) => showToast(String(ev.data ?? "")));
});

onBeforeUnmount(() => {
  stopPolling();
  if (toastTimer !== undefined) window.clearTimeout(toastTimer);
  restorePageBackground();
});
</script>

<template>
  <div class="relative h-full w-full select-none" @contextmenu="onContextMenu">
    <!-- The pill: whole surface is a drag handle + native context-menu binding. -->
    <div
      class="absolute inset-x-1.5 top-1.5 bottom-1.5 flex cursor-grab items-center gap-3 rounded-full bg-surface-dark px-4 text-white shadow-md"
      style="
        --wails-draggable: drag;
        --custom-contextmenu: ball-menu;
        user-select: none;
        -webkit-user-select: none;
      "
      title="左键双击：显示/隐藏主窗口 · 右键：开始/停止计时"
      @dblclick="toggleMainWindow"
    >
      <span class="relative flex h-3 w-3 shrink-0">
        <span
          v-if="running"
          class="absolute inline-flex h-full w-full animate-ping rounded-full opacity-60"
          :style="{ backgroundColor: state.activityColor }"
        />
        <span
          class="relative inline-flex h-3 w-3 rounded-full"
          :style="{ backgroundColor: running ? state.activityColor : '#6c6a64' }"
        />
      </span>

      <span class="min-w-0 flex-1 truncate text-sm font-medium">
        {{ running ? state.activityName || "计时中" : state.lastActivityName || "未在计时" }}
      </span>

      <span
        class="font-mono text-xl leading-none font-semibold tabular-nums"
        :class="running ? 'text-white' : 'text-white/45'"
      >
        {{ formatClock(running ? state.elapsed : state.lastElapsed) }}
      </span>
    </div>

    <!-- Transient hint for failed menu actions (covers the pill briefly). -->
    <Transition
      enter-active-class="transition-opacity duration-150"
      leave-active-class="transition-opacity duration-300"
      enter-from-class="opacity-0"
      leave-to-class="opacity-0"
    >
      <div
        v-if="toast"
        class="pointer-events-none absolute inset-x-1.5 top-1.5 bottom-1.5 z-10 flex items-center justify-center rounded-full bg-dark-elevated/95 px-4 text-xs text-white shadow-lg"
      >
        {{ toast }}
      </div>
    </Transition>
  </div>
</template>