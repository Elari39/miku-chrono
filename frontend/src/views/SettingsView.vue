<script setup lang="ts">
import { onMounted, ref } from "vue";
import { DataService, PetService } from "../lib/api";
import { useToast } from "../composables/useToast";
import { errorMessage } from "../lib/errors";
import ConfirmDialog from "../components/ConfirmDialog.vue";
// Single source of truth for the displayed version: package.json also feeds
// build/config.yml and the Windows version info at release time.
import { version as appVersion } from "../../package.json";

const { success, error } = useToast();

const dataDir = ref("");
const confirmClear1 = ref(false);
const confirmClear2 = ref(false);

// Desktop pet section.
const petVisible = ref(true);
const closeAction = ref<"hide" | "quit">("hide");

// Startup & reminders section.
const autostart = ref(false);
const goalNotify = ref(true);

async function loadDataDir() {
  try {
    dataDir.value = (await DataService.DataDir()) ?? "";
  } catch (err) {
    console.error(err);
  }
}

async function openDir() {
  try {
    await DataService.OpenDataDir();
  } catch (err) {
    error(errorMessage(err));
  }
}

async function exportJSON() {
  try {
    const path = await DataService.ExportJSON();
    if (path) success(`已导出：${path}`);
  } catch (err) {
    error(errorMessage(err));
  }
}

async function exportCSV() {
  try {
    const path = await DataService.ExportCSV();
    if (path) success(`已导出：${path}`);
  } catch (err) {
    error(errorMessage(err));
  }
}

async function doClear() {
  confirmClear1.value = false;
  confirmClear2.value = false;
  try {
    await DataService.ClearEntries();
    success("所有计时记录已清空");
  } catch (err) {
    error(errorMessage(err));
  }
}

async function loadPetState() {
  try {
    petVisible.value = await PetService.IsPetVisible();
    const action = await PetService.GetCloseAction();
    closeAction.value = action === "quit" ? "quit" : "hide";
  } catch (err) {
    console.error(err);
  }
}

async function togglePet() {
  try {
    if (petVisible.value) {
      await PetService.HidePet();
    } else {
      await PetService.ShowPet();
    }
    petVisible.value = !petVisible.value;
  } catch (err) {
    error(errorMessage(err));
  }
}

async function setCloseAction(action: "hide" | "quit") {
  try {
    await PetService.SetCloseAction(action);
    closeAction.value = action;
    success(action === "quit" ? "已保存：关闭主窗口时直接退出" : "已保存：关闭主窗口时隐藏到后台");
  } catch (err) {
    error(errorMessage(err));
  }
}

async function loadStartupState() {
  try {
    autostart.value = await PetService.GetAutostart();
    goalNotify.value = await PetService.GetGoalNotifyEnabled();
  } catch (err) {
    console.error(err);
  }
}

async function toggleAutostart() {
  try {
    await PetService.SetAutostart(!autostart.value);
    autostart.value = !autostart.value;
    success(autostart.value ? "已开启开机自启动" : "已关闭开机自启动");
  } catch (err) {
    error(errorMessage(err));
  }
}

async function toggleGoalNotify() {
  try {
    await PetService.SetGoalNotifyEnabled(!goalNotify.value);
    goalNotify.value = !goalNotify.value;
    success(goalNotify.value ? "已开启每日目标提醒" : "已关闭每日目标提醒");
  } catch (err) {
    error(errorMessage(err));
  }
}

onMounted(() => {
  void loadDataDir();
  void loadPetState();
  void loadStartupState();
});
</script>

<template>
  <div class="mx-auto max-w-2xl px-8 py-8">
    <h1 class="mb-6 font-display text-2xl font-semibold text-ink">设置</h1>

    <div class="mc-card mb-4 p-5">
      <h3 class="mb-1 text-sm font-semibold text-ink">桌宠</h3>
      <p class="mb-4 text-xs text-muted">
        桌面上的初音未来小桌宠：用动作和表情反映计时状态，可拖拽移动，左键双击显示/隐藏主窗口。
      </p>

      <div class="flex items-center justify-between gap-4">
        <div>
          <div class="text-sm text-body">显示桌宠</div>
          <div class="text-xs text-muted">隐藏后可通过此开关或重启应用恢复。</div>
        </div>
        <button class="mc-btn-ghost" @click="togglePet">
          {{ petVisible ? "隐藏桌宠" : "显示桌宠" }}
        </button>
      </div>

      <div class="mt-4 mb-3 border-t border-hairline" />

      <div class="mb-2 text-sm text-body">点击主窗口 × 时</div>
      <label class="flex cursor-pointer items-center gap-2.5 py-1 text-sm text-body">
        <input
          type="radio"
          class="accent-[var(--color-primary)]"
          :checked="closeAction === 'hide'"
          @change="setCloseAction('hide')"
        />
        隐藏到后台（推荐）
        <span class="text-xs text-muted">应用与桌宠继续运行</span>
      </label>
      <label class="flex cursor-pointer items-center gap-2.5 py-1 text-sm text-body">
        <input
          type="radio"
          class="accent-[var(--color-primary)]"
          :checked="closeAction === 'quit'"
          @change="setCloseAction('quit')"
        />
        直接退出应用
        <span class="text-xs text-muted">连同桌宠一起退出</span>
      </label>
    </div>

    <div class="mc-card mb-4 p-5">
      <h3 class="mb-1 text-sm font-semibold text-ink">启动与提醒</h3>
      <p class="mb-4 text-xs text-muted">开机常驻与每日目标通知：让 Miku 安静地等你回来。</p>

      <div class="flex items-center justify-between gap-4">
        <div>
          <div class="text-sm text-body">开机时自动启动</div>
          <div class="text-xs text-muted">
            启动后以静默方式运行：仅显示桌宠与系统托盘，点击托盘图标即可打开主窗口。
          </div>
        </div>
        <button class="mc-btn-ghost" @click="toggleAutostart">
          {{ autostart ? "关闭自启动" : "开启自启动" }}
        </button>
      </div>

      <div class="mt-4 mb-3 border-t border-hairline" />

      <div class="flex items-center justify-between gap-4">
        <div>
          <div class="text-sm text-body">每日目标达成提醒</div>
          <div class="text-xs text-muted">
            活动当天累计达到「每日目标分钟数」时弹出系统通知，每日每活动提醒一次。
          </div>
        </div>
        <button class="mc-btn-ghost" @click="toggleGoalNotify">
          {{ goalNotify ? "关闭提醒" : "开启提醒" }}
        </button>
      </div>
    </div>

    <div class="mc-card mb-4 p-5">
      <h3 class="mb-1 text-sm font-semibold text-ink">数据导出</h3>
      <p class="mb-4 text-xs text-muted">把全部记录备份为文件，保存在你选择的位置。</p>
      <div class="flex gap-2">
        <button class="mc-btn-ghost" @click="exportJSON">导出 JSON（完整备份）</button>
        <button class="mc-btn-ghost" @click="exportCSV">导出 CSV（表格）</button>
      </div>
    </div>

    <div class="mc-card mb-4 p-5">
      <h3 class="mb-1 text-sm font-semibold text-ink">数据位置</h3>
      <p class="mb-3 text-xs text-muted">SQLite 数据库与备份默认保存在：</p>
      <code class="mb-3 block rounded-lg bg-surface-card px-3 py-2 text-xs break-all text-body">{{
        dataDir || "…"
      }}</code>
      <button class="mc-btn-ghost" @click="openDir">打开数据文件夹</button>
    </div>

    <div class="rounded-xl border border-error/30 bg-error/5 p-5">
      <h3 class="mb-1 text-sm font-semibold text-error">危险区域</h3>
      <p class="mb-4 text-xs text-muted">
        清空所有计时记录（活动本身会保留）。此操作无法撤销，建议先导出备份。
      </p>
      <button class="mc-btn-danger" @click="confirmClear1 = true">清空所有记录</button>
    </div>

    <p class="mt-8 text-center text-xs text-muted">
      Miku Chrono v{{ appVersion }} · Wails v3 · {{ new Date().getFullYear() }} · MIT License
    </p>

    <ConfirmDialog
      :open="confirmClear1"
      title="清空所有记录"
      :danger="true"
      confirm-text="下一步"
      message="即将删除全部计时记录（包括计时与补录）。活动、名称与目标会保留。确定要继续吗？"
      @confirm="
        confirmClear1 = false;
        confirmClear2 = true;
      "
      @cancel="confirmClear1 = false"
    />
    <ConfirmDialog
      :open="confirmClear2"
      title="最终确认"
      :danger="true"
      confirm-text="确认清空"
      message="再次确认：所有计时记录将被永久删除，无法恢复。真的要继续吗？"
      @confirm="doClear"
      @cancel="confirmClear2 = false"
    />
  </div>
</template>
