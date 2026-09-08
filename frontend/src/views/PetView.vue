<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Events, Screens, Window } from "@wailsio/runtime";
import { useTimer } from "../composables/useTimer";
import { formatClock } from "../lib/format";
import { PetService } from "../lib/api";
import { petAsset, type PetState } from "../lib/petAssets";

// Must match the window size in main.go — the pet is never resized at runtime.
const PET_W = 180;
const EDGE_MARGIN = 16;

// After this long without a running timer the pet falls asleep; any start or
// interaction wakes her up again.
const SLEEP_AFTER_MS = 5 * 60 * 1000;
// How long the greeting reaction plays before returning to the base state.
const WAVE_MS = 2000;
// The pet:moving events stop while a native drag ends; without fresh events
// within this window the dragging pose is dropped.
const DRAG_IDLE_MS = 300;

const { state, refresh } = useTimer();
const running = computed(() => state.running);

// Live info bubble: the running activity plus its clock, or the last session
// while idle — the same projection the tray tooltip shows, so the running
// time stays glanceable even with the main window hidden. `useTimer` keeps
// the clock ticking once per second while running.
const infoLabel = computed(() =>
  running.value ? state.activityName || "计时中" : state.lastActivityName || "未在计时",
);
const infoClock = computed(() => formatClock(running.value ? state.elapsed : state.lastElapsed));

// ── celebration easter egg (daily goal reached) ────────────────────────────
// The Go goal notifier broadcasts goal:achieved the moment an activity's
// live daily total (running session included) crosses its goal: Miku cheers
// under a confetti rain, the activity is toasted, and she stays extra
// fidgety ("happy mode") for a couple of minutes.
const CELEBRATE_MS = 5000;
const HAPPY_MODE_MS = 120_000;
const MINI_EVERY_HAPPY_MS = [6_000, 14_000] as const;
const celebrating = ref(false);
let celebrateTimer: number | undefined;
let happyUntil = 0;

function celebrate(name: string) {
  clearMini();
  asleep.value = false;
  disarmSleep();
  celebrating.value = true;
  happyUntil = Date.now() + HAPPY_MODE_MS;
  showToast(`🎉 「${name}」达成今日目标！`, 3200);
  if (celebrateTimer !== undefined) window.clearTimeout(celebrateTimer);
  celebrateTimer = window.setTimeout(() => {
    celebrateTimer = undefined;
    celebrating.value = false;
    armSleep();
    scheduleMini(); // celebrate() cleared the fidget chain; re-arm it
  }, CELEBRATE_MS);
}

// Confetti pieces for the celebration — a deterministic pseudo-random layout
// in Miku teal + the app accent palette so the rain covers the window.
const CONFETTI_COLORS = ["#39C5BB", "#cc785c", "#5db8a6", "#f4b942", "#e8798f", "#7aa5ff"];
const CONFETTI = Array.from({ length: 16 }, (_, i) => ({
  left: `${4 + ((i * 61) % 92)}%`,
  animationDelay: `${((i * 37) % 90) / 100}s`,
  animationDuration: `${(1500 + ((i * 29) % 70)) / 1000}s`,
  backgroundColor: CONFETTI_COLORS[i % CONFETTI_COLORS.length],
  width: `${5 + (i % 3) * 2}px`,
  height: `${8 + (i % 4) * 2}px`,
  borderRadius: i % 3 === 0 ? "50%" : "1px",
  "--dx": `${(i % 2 === 0 ? 1 : -1) * (10 + ((i * 13) % 22))}px`,
}));

const dragging = ref(false);
const waving = ref(false);
const asleep = ref(false);

const baseState = computed<PetState>(() => {
  if (dragging.value) return "dragging";
  if (celebrating.value) return "cheer";
  if (waving.value) return "wave";
  if (running.value) return "running";
  if (asleep.value) return "sleeping";
  return "idle";
});
const spriteSrc = computed(() => petAsset(baseState.value));

// --- random mini fidgets ----------------------------------------------------
// While the pet is calmly idling or timing, a random cute action fires every
// so often: a short CSS-only animation played on the current sprite, then
// back to the base state. Higher-priority states (dragging, waving, sleep)
// cancel the fidget outright.

const MINI_ACTIONS = ["jump", "spin", "dance", "stretch", "bounce", "nod", "wiggle", "pulse", "lean"] as const;
type MiniAction = (typeof MINI_ACTIONS)[number];
// [lo, hi] millisecond ranges for the action length and the gap between them.
const MINI_PLAY_MS = [1600, 2600] as const;
const MINI_EVERY_RUNNING_MS = [10_000, 22_000] as const;
const MINI_EVERY_IDLE_MS = [18_000, 40_000] as const;

const mini = ref<MiniAction | "">("");
let miniTimer: number | undefined;
let miniEndTimer: number | undefined;

const randMs = (range: readonly number[]) => range[0] + Math.random() * (range[1] - range[0]);

function clearMini() {
  mini.value = "";
  if (miniTimer !== undefined) {
    window.clearTimeout(miniTimer);
    miniTimer = undefined;
  }
  if (miniEndTimer !== undefined) {
    window.clearTimeout(miniEndTimer);
    miniEndTimer = undefined;
  }
}

function scheduleMini() {
  if (miniTimer !== undefined) return;
  const delay = celebrating.value || Date.now() < happyUntil
    ? randMs(MINI_EVERY_HAPPY_MS)
    : running.value
      ? randMs(MINI_EVERY_RUNNING_MS)
      : randMs(MINI_EVERY_IDLE_MS);
  miniTimer = window.setTimeout(() => {
    miniTimer = undefined;
    if (dragging.value || waving.value || asleep.value || celebrating.value) {
      // Not a calm base state right now; just try again later.
      scheduleMini();
      return;
    }
    const action = MINI_ACTIONS[Math.floor(Math.random() * MINI_ACTIONS.length)];
    if (!action) {
      scheduleMini();
      return;
    }
    playMini(action);
  }, delay);
}

/** Play one fidget action, then fall back to the random schedule. */
function playMini(action: MiniAction, ms = randMs(MINI_PLAY_MS)) {
  mini.value = action;
  if (miniEndTimer !== undefined) window.clearTimeout(miniEndTimer);
  miniEndTimer = window.setTimeout(() => {
    miniEndTimer = undefined;
    mini.value = "";
    scheduleMini();
  }, ms);
}

// ── focus milestone ────────────────────────────────────────────────────────
// Every 25 minutes of one continuous session the pet stretches and suggests
// a break — a soft pomodoro nudge without any timer UI. Fires through the
// per-second session tick, so it lands right after the minute crosses.
const MILESTONE_STEP = 25 * 60;
let lastMilestone = 0;

watch(
  () => state.sessionElapsed,
  () => {
    if (!running.value || celebrating.value || dragging.value || waving.value) return;
    // Don't stomp an in-progress fidget; the next tick retries.
    if (mini.value) return;
    const reached = Math.floor(state.sessionElapsed / MILESTONE_STEP);
    if (reached > lastMilestone && reached > 0) {
      lastMilestone = reached;
      playMini("stretch");
      showToast(`已连续专注 ${reached * 25} 分钟，休息一下吧～`);
    }
  },
);

// The animation class layers on top of the sprite asset: celebrations and
// fidgets override the base animation while keeping the current pose art.
const spriteClass = computed(() => {
  if (dragging.value) return "is-dragging";
  if (celebrating.value) return "is-cheer";
  if (mini.value) return `mini-${mini.value}`;
  return `is-${baseState.value}`;
});

// --- sleep scheduling -------------------------------------------------------

let sleepTimer: number | undefined;

function disarmSleep() {
  if (sleepTimer !== undefined) {
    window.clearTimeout(sleepTimer);
    sleepTimer = undefined;
  }
}

/** (Re)start the idle countdown; the pet dozes off when it fires untimedly. */
function armSleep() {
  disarmSleep();
  sleepTimer = window.setTimeout(() => {
    sleepTimer = undefined;
    if (!running.value && !dragging.value && !waving.value) {
      clearMini();
      asleep.value = true;
    }
  }, SLEEP_AFTER_MS);
}

// --- reactions --------------------------------------------------------------

let waveTimer: number | undefined;

/** Play the greeting reaction unconditionally (timer starts, boot). */
function playWave() {
  clearMini();
  asleep.value = false;
  armSleep();
  waving.value = true;
  if (waveTimer !== undefined) window.clearTimeout(waveTimer);
  waveTimer = window.setTimeout(() => {
    waveTimer = undefined;
    waving.value = false;
    scheduleMini(); // playWave() cleared the fidget chain; re-arm it
  }, WAVE_MS);
}

/** Click reaction: a wave, unless a higher-priority state owns the pet. */
function react() {
  if (dragging.value || running.value) return;
  playWave();
}

// --- dragging animation -----------------------------------------------------

// The Go side emits pet:moving on every WindowDidMove; the native drag loop
// swallows webview mouse events, so the dragging pose decays on a timer
// instead of listening for a pointer-up.
let dragTimer: number | undefined;

function onPetMoving() {
  clearMini();
  dragging.value = true;
  if (dragTimer !== undefined) window.clearTimeout(dragTimer);
  dragTimer = window.setTimeout(() => {
    dragTimer = undefined;
    dragging.value = false;
    armSleep();
    scheduleMini(); // onPetMoving() cleared the fidget chain; re-arm it
  }, DRAG_IDLE_MS);
}

// --- timer sync -------------------------------------------------------------

let offStarted: (() => void) | undefined;
let offStopped: (() => void) | undefined;
let offMoving: (() => void) | undefined;
let offGoal: (() => void) | undefined;

// Transient hint shown when the context-menu action cannot be performed.
const toast = ref("");
let toastTimer: number | undefined;
let offToast: (() => void) | undefined;
function showToast(message: string, ms = 1800) {
  toast.value = message;
  if (toastTimer !== undefined) window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => (toast.value = ""), ms);
}

// Poll as a low-frequency safety net so the pet converges even if an event
// is lost; normal sync is event-driven (`useTimer` listens for the app-wide
// timer:started / timer:stopped events emitted by the Go side, and the local
// 1s ticker keeps the clock smooth while running).
const FALLBACK_POLL_MS = 30_000;
let pollTimer: number | undefined;

function startPolling() {
  stopPolling();
  pollTimer = window.setInterval(async () => {
    try {
      await refresh();
    } catch (err) {
      console.error("pet refresh failed", err);
    }
  }, FALLBACK_POLL_MS);
}

function stopPolling() {
  if (pollTimer !== undefined) {
    window.clearInterval(pollTimer);
    pollTimer = undefined;
  }
}

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
async function placePet() {
  try {
    const pos = await PetService.GetPetPosition();
    if (pos.set && (await positionOnScreen(pos.x, pos.y))) {
      await Window.SetPosition(pos.x, pos.y);
      return;
    }
    const primary = await Screens.GetPrimary();
    const wa = primary.WorkArea;
    await Window.SetPosition(wa.X + wa.Width - PET_W - EDGE_MARGIN, wa.Y + EDGE_MARGIN);
  } catch (err) {
    console.error("place pet failed", err);
  }
}

async function toggleMainWindow() {
  try {
    await PetService.ToggleMainWindow();
  } catch (err) {
    console.error(err);
  }
}

function onContextMenu(e: MouseEvent) {
  // No menus anywhere in the pet; make sure the webview's own menu never
  // appears.
  e.preventDefault();
}

onMounted(async () => {
  void placePet();
  startPolling();
  offToast = Events.On("pet:toast", (ev) => showToast(String(ev.data ?? "")));
  offMoving = Events.On("pet:moving", onPetMoving);
  offStarted = Events.On("timer:started", () => {
    // playWave, not react: running may not have propagated through useTimer's
    // own refresh yet, and the greeting should play regardless.
    lastMilestone = 0; // a fresh session restarts the focus milestones
    playWave();
  });
  offStopped = Events.On("timer:stopped", () => armSleep());
  offGoal = Events.On("goal:achieved", (ev) => {
    // The Go notifier emits the activity name as the payload.
    const data = (ev as { data?: unknown }).data;
    const name = Array.isArray(data) ? String(data[0] ?? "") : String(data ?? "");
    celebrate(name || "今日目标");
  });
  // Boot state: running pets start lively, idle pets start the doze countdown.
  try {
    await refresh();
  } catch (err) {
    console.error("pet initial refresh failed", err);
  }
  if (running.value) playWave();
  else armSleep();
  scheduleMini();
});

onBeforeUnmount(() => {
  stopPolling();
  offToast?.();
  offMoving?.();
  offStarted?.();
  offStopped?.();
  offGoal?.();
  disarmSleep();
  clearMini();
  if (toastTimer !== undefined) window.clearTimeout(toastTimer);
  if (waveTimer !== undefined) window.clearTimeout(waveTimer);
  if (dragTimer !== undefined) window.clearTimeout(dragTimer);
  if (celebrateTimer !== undefined) window.clearTimeout(celebrateTimer);
});
</script>

<template>
  <div class="relative h-full w-full select-none" @contextmenu="onContextMenu">
    <!-- The sprite: the whole surface is a drag handle. Right-click is
         intentionally inert (no menus of any kind); interactions are drag,
         click, double-click and hover. -->
    <div
      class="pet-stage absolute inset-0 flex cursor-grab items-end justify-center active:cursor-grabbing"
      style="
        --wails-draggable: drag;
        user-select: none;
        -webkit-user-select: none;
      "
      role="button"
      tabindex="0"
      aria-label="初音未来桌宠"
      @click="react"
      @dblclick="toggleMainWindow"
      @keydown.enter.prevent="toggleMainWindow"
      @keydown.space.prevent="toggleMainWindow"
    >
      <!-- The wrapper hugs the sprite exactly, so the hover reaction fires on
           the art itself rather than the whole window rect. -->
      <div class="pet-touch">
        <!-- Mirror-stride layer: while running, the sprite flips horizontally
             every step so arms and legs genuinely swap sides (a classic
             two-frame run trick, guaranteed consistent art). -->
        <div class="pet-flip" :class="{ 'is-striding': baseState === 'running' && !mini }">
          <img
            v-if="spriteSrc"
            :src="spriteSrc"
            :alt="`桌宠${baseState}状态`"
            class="pet-sprite w-[140px] origin-bottom"
            :class="spriteClass"
            draggable="false"
          />
        </div>
      </div>
      <!-- Live activity bubble floating above her head (clicks/drag pass
           through it to the stage). -->
      <div
        class="pointer-events-none absolute inset-x-2 top-1.5 z-10 flex items-center justify-center gap-1.5 rounded-full bg-dark-elevated/90 px-2.5 py-1 text-[11px] leading-none text-white"
      >
        <span
          v-if="running"
          class="h-1.5 w-1.5 shrink-0 rounded-full"
          :style="{ backgroundColor: state.activityColor }"
        />
        <span class="min-w-0 truncate">{{ infoLabel }}</span>
        <span
          class="shrink-0 font-mono tabular-nums"
          :class="running ? 'text-white' : 'text-white/55'"
        >
          {{ infoClock }}
        </span>
      </div>

      <!-- Goal celebration: confetti rain while the cheer pose plays. -->
      <div
        v-if="celebrating"
        class="pointer-events-none absolute inset-0 z-20 overflow-hidden"
        aria-hidden="true"
      >
        <span v-for="(p, i) in CONFETTI" :key="i" class="confetti" :style="p" />
      </div>

      <!-- Floating z's while dozing (pure CSS, no extra art needed). -->
      <span v-if="baseState === 'sleeping'" class="pet-z pet-z-1" aria-hidden="true">z</span>
      <span v-if="baseState === 'sleeping'" class="pet-z pet-z-2" aria-hidden="true">z</span>
      <span v-if="baseState === 'sleeping'" class="pet-z pet-z-3" aria-hidden="true">z</span>
    </div>

    <!-- Transient hint for failed menu actions (covers the sprite briefly). -->
    <Transition
      enter-active-class="transition-opacity duration-150"
      leave-active-class="transition-opacity duration-300"
      enter-from-class="opacity-0"
      leave-to-class="opacity-0"
    >
      <div
        v-if="toast"
        class="pointer-events-none absolute inset-x-2 bottom-2 z-10 flex items-center justify-center rounded-full bg-dark-elevated/95 px-3.5 py-1.5 text-xs text-white"
      >
        {{ toast }}
      </div>
    </Transition>
  </div>
</template>

<style scoped>
/*
 * Per-state sprite animations. Everything is transform-only so the
 * transparent window never re-rasterises the page; transforms anchor to the
 * bottom centre so the feet stay planted.
 */
.pet-sprite {
  animation-duration: 3.2s;
  animation-iteration-count: infinite;
  animation-timing-function: ease-in-out;
}

/* Idle: gentle breathing bob. */
.is-idle {
  animation-name: pet-breathe;
}

/* Running (a relaxed jog): a gentle vertical hop synced to the mirror-stride
   flip below — one bounce per step, no horizontal rotation, which read as
   swaying instead. */
.is-running {
  animation-name: pet-run;
  animation-duration: 0.38s;
}

/* Dragging: startled wobble (the pose art has the surprised face). */
.is-dragging {
  animation-name: pet-wobble;
  animation-duration: 0.35s;
}

/* Sleeping: slow, deep dozing sink. */
.is-sleeping {
  animation-name: pet-doze;
  animation-duration: 4s;
}

/* Wave: a few quick bounces that read as waving. */
.is-wave {
  animation-name: pet-wave;
  animation-duration: 0.5s;
  animation-iteration-count: 4;
}

@keyframes pet-breathe {
  0%,
  100% {
    transform: translateY(0) scaleY(1);
  }
  50% {
    transform: translateY(-5px) scaleY(1.015);
  }
}

@keyframes pet-run {
  0%,
  100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-4px);
  }
}

@keyframes pet-wobble {
  0%,
  100% {
    transform: rotate(-3.5deg);
  }
  50% {
    transform: rotate(3.5deg);
  }
}

@keyframes pet-doze {
  0%,
  100% {
    transform: translateY(0) scaleY(1);
  }
  50% {
    transform: translateY(2px) scaleY(0.97);
  }
}

@keyframes pet-wave {
  0%,
  100% {
    transform: rotate(0deg);
  }
  25% {
    transform: rotate(-5deg) translateY(-4px);
  }
  75% {
    transform: rotate(5deg) translateY(-4px);
  }
}

/*
 * Random mini fidgets: short one-shot animations played on the current pose
 * art (idle or running). Declared after the .is-* rules so the shorthand
 * wins the cascade while a fidget is active; rotations stay within the
 * window margins so the sprite's corners never clip.
 */
.mini-jump {
  animation: pet-mini-jump 1.5s ease-in-out 1;
}

.mini-spin {
  animation: pet-mini-spin 2s ease-in-out 1;
}

.mini-dance {
  animation: pet-mini-dance 1.6s ease-in-out 1;
}

.mini-stretch {
  animation: pet-mini-stretch 1.9s ease-in-out 1;
}

.mini-bounce {
  animation: pet-mini-bounce 1.3s ease-in-out 1;
}

.mini-nod {
  animation: pet-mini-nod 1s ease-in-out 1;
}

.mini-wiggle {
  animation: pet-mini-wiggle 1.2s ease-in-out 1;
}

.mini-pulse {
  animation: pet-mini-pulse 1.3s ease-in-out 1;
}

.mini-lean {
  animation: pet-mini-lean 1.8s ease-in-out 1;
}

/* Two happy hops with a squash before each leap. */
@keyframes pet-mini-jump {
  0% {
    transform: translateY(0) scaleY(1);
  }
  12% {
    transform: translateY(0) scaleY(0.92) scaleX(1.04);
  }
  32% {
    transform: translateY(-22px) scaleY(1.05) scaleX(0.98);
  }
  50% {
    transform: translateY(0) scaleY(0.94) scaleX(1.03);
  }
  68% {
    transform: translateY(-22px) scaleY(1.05) scaleX(0.98);
  }
  86% {
    transform: translateY(0) scaleY(0.96);
  }
  100% {
    transform: translateY(0) scaleY(1);
  }
}

/* Curious look left then right (the scaleX flip reads as turning around). */
@keyframes pet-mini-spin {
  0%,
  100% {
    transform: scaleX(1);
  }
  35%,
  55% {
    transform: scaleX(-1) rotate(2deg);
  }
  85% {
    transform: scaleX(1);
  }
}

/* A little hip-sway dance, keeping the feet planted. */
@keyframes pet-mini-dance {
  0%,
  100% {
    transform: rotate(0deg);
  }
  20% {
    transform: rotate(-5deg) translateY(-3px);
  }
  40% {
    transform: rotate(5deg) translateY(-3px);
  }
  60% {
    transform: rotate(-4deg);
  }
  80% {
    transform: rotate(4deg);
  }
}

/* A slow lazy stretch with a little rise. */
@keyframes pet-mini-stretch {
  0%,
  100% {
    transform: scaleY(1) translateY(0);
  }
  35% {
    transform: scaleY(0.93) scaleX(1.03) translateY(2px);
  }
  70% {
    transform: scaleY(1.06) scaleX(0.98) translateY(-5px);
  }
}

/* Quick tiny steps in place. */
@keyframes pet-mini-bounce {
  0%,
  100% {
    transform: translateX(0) translateY(0);
  }
  25% {
    transform: translateX(-4px) translateY(-6px);
  }
  50% {
    transform: translateX(0) translateY(0);
  }
  75% {
    transform: translateX(4px) translateY(-6px);
  }
}

/* Eager nodding, as if keeping the beat. */
@keyframes pet-mini-nod {
  0%,
  100% {
    transform: scaleY(1);
  }
  25% {
    transform: scaleY(0.95) translateY(2px);
  }
  50% {
    transform: scaleY(1);
  }
  75% {
    transform: scaleY(0.95) translateY(2px);
  }
}

/* A little hip wiggle left and right. */
@keyframes pet-mini-wiggle {
  0%,
  100% {
    transform: translateX(0) rotate(0deg);
  }
  20% {
    transform: translateX(-4px) rotate(-3deg);
  }
  45% {
    transform: translateX(4px) rotate(3deg);
  }
  70% {
    transform: translateX(-3px) rotate(-2deg);
  }
}

/* A happy little puff-up. */
@keyframes pet-mini-pulse {
  0%,
  100% {
    transform: scale(1);
  }
  40% {
    transform: scale(1.06);
  }
  70% {
    transform: scale(0.98);
  }
}

/* A curious head tilt, first one way then the other. */
@keyframes pet-mini-lean {
  0%,
  100% {
    transform: rotate(0deg);
  }
  30%,
  55% {
    transform: rotate(-6deg);
  }
  85% {
    transform: rotate(4deg);
  }
}

/*
 * Mirror-stride run cycle: the sprite holds its normal orientation for one
 * step, then flips horizontally for the next, alternating arms and legs.
 * The near-instant jumps between keyframes (0.1% gaps) read as discrete
 * frames; the period is twice the 0.38s jog hop so every step bounces once.
 */
.pet-flip {
  transform-origin: 50% 100%;
}

.pet-flip.is-striding {
  animation: pet-stride 0.76s linear infinite;
}

@keyframes pet-stride {
  0%,
  24.9% {
    transform: scaleX(1);
  }
  25%,
  49.9% {
    transform: scaleX(-1);
  }
  50%,
  74.9% {
    transform: scaleX(1);
  }
  75%,
  100% {
    transform: scaleX(-1);
  }
}

/*
 * Hover reaction: the wrapper hugs the sprite exactly, so resting the cursor
 * on the art plays a little perk-up (pop + tilt) once per hover entry. It
 * animates the wrapper so it composes with the base state animation on the
 * img itself instead of replacing it.
 */
.pet-touch {
  display: block;
  line-height: 0;
  transform-origin: 50% 100%;
}

.pet-touch:hover {
  animation: pet-hover 0.7s ease-in-out 1;
}

@keyframes pet-hover {
  0%,
  100% {
    transform: scale(1);
  }
  35% {
    transform: scale(1.06) rotate(-3deg);
  }
  70% {
    transform: scale(0.98) rotate(1.5deg);
  }
}

/* Goal celebration: big joyful double-hops with a happy squash, played while
   the cheer pose art is up. */
.is-cheer {
  animation-name: pet-cheer;
  animation-duration: 0.9s;
}

@keyframes pet-cheer {
  0%,
  100% {
    transform: translateY(0);
  }
  12% {
    transform: translateY(0) scaleY(0.9) scaleX(1.06);
  }
  38% {
    transform: translateY(-26px) scaleY(1.05) scaleX(0.98) rotate(-3deg);
  }
  52% {
    transform: translateY(0) scaleY(0.92) scaleX(1.04);
  }
  78% {
    transform: translateY(-20px) rotate(3deg);
  }
}

/* Confetti rain (each piece styled inline from the CONFETTI table). */
.confetti {
  position: absolute;
  top: -12px;
  animation: pet-confetti 1.8s linear both;
}

@keyframes pet-confetti {
  0% {
    transform: translate(0, 0) rotate(0deg);
    opacity: 1;
  }
  100% {
    transform: translate(var(--dx, 12px), 260px) rotate(540deg);
    opacity: 0.85;
  }
}

/* Dozing bubbles drifting up beside the head. */
.pet-z {
  position: absolute;
  right: 14px;
  bottom: 130px;
  font-family: var(--font-display), serif;
  font-style: italic;
  font-weight: 600;
  color: var(--color-accent-teal);
  opacity: 0;
  animation: pet-zz 3s ease-out infinite;
}

.pet-z-1 {
  font-size: 12px;
  animation-delay: 0s;
}

.pet-z-2 {
  font-size: 16px;
  right: 26px;
  bottom: 148px;
  animation-delay: 1s;
}

.pet-z-3 {
  font-size: 20px;
  right: 38px;
  bottom: 168px;
  animation-delay: 2s;
}

@keyframes pet-zz {
  0% {
    opacity: 0;
    transform: translate(0, 0) rotate(8deg);
  }
  25% {
    opacity: 0.9;
  }
  100% {
    opacity: 0;
    transform: translate(10px, -26px) rotate(16deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .pet-sprite,
  .pet-touch,
  .pet-flip,
  .pet-z,
  .confetti {
    animation: none !important;
  }
}

/* Reduced motion: confetti pieces would sit frozen on top of the art, so
   hide the layer entirely instead. */
@media (prefers-reduced-motion: reduce) {
  .confetti {
    display: none;
  }
}
</style>
