<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import {
  ActivityService,
  EntryService,
  type Activity,
  type Entry,
  type EntryFilter,
} from "../lib/api";
import { useToast } from "../composables/useToast";
import { useVersionedLoad } from "../composables/useVersionedLoad";
import { codePointLength, fromLocalInput, toLocalInput } from "../lib/format";
import { errorMessage } from "../lib/errors";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import EmptyState from "../components/EmptyState.vue";
import EntryRow from "../components/EntryRow.vue";
import Modal from "../components/Modal.vue";

const { success, error } = useToast();
const route = useRoute();

const activities = ref<Activity[]>([]);
const entries = ref<Entry[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;

// Deep link from the stats page truncation hint: /records?from=…&to=… prefills
// the date range so the first load lands pre-filtered. Malformed values are
// ignored rather than surfacing a backend date-parse error.
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;
const queryFrom = typeof route.query.from === "string" && DATE_RE.test(route.query.from) ? route.query.from : "";
const queryTo = typeof route.query.to === "string" && DATE_RE.test(route.query.to) ? route.query.to : "";

const filterActivityId = ref<number | "">("");
const filterFrom = ref(queryFrom);
const filterTo = ref(queryTo);

const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)));
const activityById = computed(() => new Map(activities.value.map((a) => [a.id, a])));

const editorOpen = ref(false);
const editing = ref<Entry | null>(null);
const form = ref({ activityId: 0, start: "", end: "", note: "" });
const formError = ref("");

// Shared counting unit with the backend's 500-rune note limit (code points,
// not UTF-16 units like a native maxlength).
const noteCount = computed(() => codePointLength(form.value.note));

const deleteTarget = ref<Entry | null>(null);

async function loadActivities() {
  try {
    activities.value = (await ActivityService.List(false)) ?? [];
  } catch (err) {
    console.error(err);
  }
}

// Entry list reloads on mount and on timer start/stop; the sequence guard
// drops stale responses when filters/pages are flipped quickly.
const { loading, reload } = useVersionedLoad(async (isCurrent) => {
  try {
    const filter: EntryFilter = {
      activityId: filterActivityId.value === "" ? null : filterActivityId.value,
      fromDate: filterFrom.value,
      toDate: filterTo.value,
      page: page.value,
      pageSize,
    };
    const list = await EntryService.List(filter);
    if (!isCurrent()) return;
    entries.value = list?.items ?? [];
    total.value = list?.total ?? 0;
  } catch (err) {
    console.error(err);
    error("加载记录失败");
  }
});

function applyFilter() {
  page.value = 1;
  void reload();
}

function prevPage() {
  if (page.value <= 1) return;
  page.value--;
  void reload();
}

function nextPage() {
  if (page.value >= pages.value) return;
  page.value++;
  void reload();
}

function openCreate() {
  editing.value = null;
  const now = new Date();
  const hourAgo = new Date(now.getTime() - 3600_000);
  form.value = {
    activityId: activities.value[0]?.id ?? 0,
    start: toLocalInput(hourAgo.toISOString()),
    end: toLocalInput(now.toISOString()),
    note: "",
  };
  formError.value = "";
  editorOpen.value = true;
}

function openEdit(e: Entry) {
  editing.value = e;
  form.value = {
    activityId: e.activityId,
    start: toLocalInput(e.startedAt),
    end: toLocalInput(e.endedAt),
    note: e.note,
  };
  formError.value = "";
  editorOpen.value = true;
}

async function save() {
  formError.value = "";
  const start = fromLocalInput(form.value.start);
  const end = fromLocalInput(form.value.end);
  if (!form.value.activityId) {
    formError.value = "请选择活动";
    return;
  }
  if (!start || !end) {
    formError.value = "请填写开始与结束时间";
    return;
  }
  if (noteCount.value > 500) {
    formError.value = "备注最多 500 字";
    return;
  }
  try {
    if (editing.value) {
      await EntryService.Update({
        ...editing.value,
        activityId: form.value.activityId,
        startedAt: start,
        endedAt: end,
        note: form.value.note,
      });
      success("记录已更新");
    } else {
      await EntryService.CreateManual(form.value.activityId, start, end, form.value.note);
      success("已添加记录");
    }
    editorOpen.value = false;
    void reload();
  } catch (err) {
    formError.value = errorMessage(err);
  }
}

async function doDelete() {
  if (!deleteTarget.value) return;
  try {
    await EntryService.Delete(deleteTarget.value.id);
    success("记录已删除");
    deleteTarget.value = null;
    void reload();
  } catch (err) {
    error(errorMessage(err));
  }
}

onMounted(() => {
  void loadActivities();
});
</script>

<template>
  <div class="mx-auto max-w-4xl px-8 py-8">
    <div class="mb-6 flex flex-wrap items-end justify-between gap-3">
      <h1 class="font-display text-2xl font-semibold text-ink">记录</h1>
      <div class="flex flex-wrap items-center gap-2">
        <select v-model="filterActivityId" class="mc-input w-32" @change="applyFilter">
          <option value="">全部活动</option>
          <option v-for="a in activities" :key="a.id" :value="a.id">{{ a.name }}</option>
        </select>
        <input v-model="filterFrom" type="date" class="mc-input w-36" @change="applyFilter" />
        <span class="text-muted">–</span>
        <input v-model="filterTo" type="date" class="mc-input w-36" @change="applyFilter" />
        <button class="mc-btn-primary" @click="openCreate">＋ 补录</button>
      </div>
    </div>

    <div v-if="loading" class="py-10 text-center text-sm text-muted">加载中…</div>
    <EmptyState
      v-else-if="entries.length === 0"
      title="没有符合条件的记录"
      hint="试试调整筛选条件，或用「补录」添加过去的记录。"
    />

    <div v-else class="mc-card divide-y divide-hairline overflow-hidden">
      <EntryRow
        v-for="e in entries"
        :key="e.id"
        :entry="e"
        :duration-seconds="e.durationSeconds"
      >
        <div class="flex gap-1">
          <button
            class="cursor-pointer rounded-md px-2 py-1 text-xs text-muted hover:bg-surface-card hover:text-body"
            @click="openEdit(e)"
          >
            编辑
          </button>
          <button
            class="cursor-pointer rounded-md px-2 py-1 text-xs text-muted hover:bg-error/10 hover:text-error"
            @click="deleteTarget = e"
          >
            删除
          </button>
        </div>
      </EntryRow>
    </div>

    <div
      v-if="total > pageSize"
      class="mt-4 flex items-center justify-center gap-3 text-sm text-muted"
    >
      <button class="mc-btn-ghost py-1.5" :disabled="page <= 1" @click="prevPage">上一页</button>
      <span>{{ page }} / {{ pages }}（共 {{ total }} 条）</span>
      <button class="mc-btn-ghost py-1.5" :disabled="page >= pages" @click="nextPage">
        下一页
      </button>
    </div>

    <Modal
      :open="editorOpen"
      :title="editing ? '编辑记录' : '补录记录'"
      @close="editorOpen = false"
    >
      <div class="flex flex-col gap-4">
        <div>
          <label class="mc-label" for="manual-activity">活动</label>
          <select id="manual-activity" v-model="form.activityId" class="mc-input">
            <option v-for="a in activities" :key="a.id" :value="a.id">{{ a.name }}</option>
          </select>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mc-label" for="manual-start">开始时间</label>
            <input
              id="manual-start"
              v-model="form.start"
              type="datetime-local"
              step="1"
              class="mc-input"
            />
          </div>
          <div>
            <label class="mc-label" for="manual-end">结束时间</label>
            <input
              id="manual-end"
              v-model="form.end"
              type="datetime-local"
              step="1"
              class="mc-input"
            />
          </div>
        </div>
        <div>
          <label class="mc-label" for="manual-note">备注（可选）</label>
          <input
            id="manual-note"
            v-model="form.note"
            type="text"
            placeholder="这次专注做了什么？"
            class="mc-input"
          />
          <p class="mt-1 text-right text-xs" :class="noteCount > 500 ? 'text-error' : 'text-muted'">
            {{ noteCount }}/500
          </p>
        </div>
        <p v-if="formError" class="text-xs text-error">{{ formError }}</p>
      </div>
      <template #footer>
        <button class="mc-btn-ghost" @click="editorOpen = false">取消</button>
        <button class="mc-btn-primary" @click="save">保存</button>
      </template>
    </Modal>

    <ConfirmDialog
      :open="deleteTarget !== null"
      title="删除记录"
      :danger="true"
      confirm-text="删除"
      :message="
        deleteTarget
          ? `确定删除 ${activityById.get(deleteTarget.activityId)?.name ?? ''} 的这条记录吗？该操作不可撤销。`
          : ''
      "
      @confirm="doDelete"
      @cancel="deleteTarget = null"
    />
  </div>
</template>
