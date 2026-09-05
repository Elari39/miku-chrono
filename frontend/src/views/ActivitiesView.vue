<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ActivityService, CategoryService, type Activity, type Category } from "../lib/api";
import { useToast } from "../composables/useToast";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import Modal from "../components/Modal.vue";

const { success, error } = useToast();

const activities = ref<Activity[]>([]);
const categories = ref<Category[]>([]);
const loading = ref(true);
const showArchived = ref(false);

const editorOpen = ref(false);
const editing = ref<Activity | null>(null);
const form = ref({ name: "", color: "#cc785c", icon: "", categoryId: null as number | null, dailyGoalMinutes: 0 });
const formError = ref("");

const deleteTarget = ref<Activity | null>(null);

// ── 类别管理 ──────────────────────────────────────────────
const catManagerOpen = ref(false);
const catView = ref<"list" | "form">("list");
const catEditing = ref<Category | null>(null);
const catForm = ref({ name: "", icon: "", color: "#cc785c" });
const catFormError = ref("");
const deleteCatTarget = ref<Category | null>(null);

const visible = computed(() =>
  showArchived.value ? activities.value : activities.value.filter((a) => !a.archived),
);
const archivedCount = computed(() => activities.value.filter((a) => a.archived).length);

const categoryById = computed(() => new Map(categories.value.map((c) => [c.id, c])));
const activityCountByCategory = computed(() => {
  const counts = new Map<number, number>();
  for (const a of activities.value) {
    if (a.categoryId == null) continue;
    counts.set(a.categoryId, (counts.get(a.categoryId) ?? 0) + 1);
  }
  return counts;
});

const palette = ["#cc785c", "#a9583e", "#5db8a6", "#e8a55a", "#5db872", "#d4a017", "#c64545", "#141413", "#6c6a64"];

async function load() {
  loading.value = true;
  try {
    const [acts, cats] = await Promise.all([ActivityService.List(true), CategoryService.List()]);
    activities.value = acts ?? [];
    categories.value = cats ?? [];
  } catch (err) {
    console.error(err);
    error("加载失败");
  } finally {
    loading.value = false;
  }
}

// ── 活动增删改 ────────────────────────────────────────────

function openCreate() {
  editing.value = null;
  form.value = { name: "", color: palette[0], icon: "", categoryId: null, dailyGoalMinutes: 0 };
  formError.value = "";
  editorOpen.value = true;
}

function openEdit(a: Activity) {
  editing.value = a;
  form.value = {
    name: a.name,
    color: a.color,
    icon: a.icon,
    categoryId: a.categoryId ?? null,
    dailyGoalMinutes: a.dailyGoalMinutes,
  };
  formError.value = "";
  editorOpen.value = true;
}

async function save() {
  formError.value = "";
  if (!form.value.name.trim()) {
    formError.value = "活动名称不能为空";
    return;
  }
  const payload: Activity = editing.value
    ? { ...editing.value, ...form.value }
    : ({ ...form.value } as Activity);
  try {
    if (editing.value) {
      await ActivityService.Update(payload);
      success("活动已更新");
    } else {
      await ActivityService.Create(payload);
      success("活动已创建");
    }
    editorOpen.value = false;
    await load();
  } catch (err) {
    formError.value = String((err as Error).message ?? err).replace(/^\w+:\s*/, "");
  }
}

async function toggleArchive(a: Activity) {
  try {
    await ActivityService.SetArchived(a.id, !a.archived);
    success(a.archived ? "已取消归档" : "已归档");
    await load();
  } catch (err) {
    error(String((err as Error).message ?? err).replace(/^\w+:\s*/, ""));
  }
}

async function doDelete() {
  if (!deleteTarget.value) return;
  try {
    await ActivityService.Delete(deleteTarget.value.id);
    success("活动及其记录已删除");
    deleteTarget.value = null;
    await load();
  } catch (err) {
    error(String((err as Error).message ?? err).replace(/^\w+:\s*/, ""));
  }
}

// ── 类别增删改 ────────────────────────────────────────────

function openCatManager() {
  catView.value = "list";
  catFormError.value = "";
  catManagerOpen.value = true;
}

function openCatCreate() {
  catEditing.value = null;
  catForm.value = { name: "", icon: "", color: palette[0] };
  catFormError.value = "";
  catView.value = "form";
}

function openCatEdit(c: Category) {
  catEditing.value = c;
  catForm.value = { name: c.name, icon: c.icon, color: c.color };
  catFormError.value = "";
  catView.value = "form";
}

async function saveCategory() {
  catFormError.value = "";
  if (!catForm.value.name.trim()) {
    catFormError.value = "类别名称不能为空";
    return;
  }
  try {
    if (catEditing.value) {
      await CategoryService.Update({ ...catEditing.value, ...catForm.value });
      success("类别已更新");
    } else {
      await CategoryService.Create({ ...catForm.value } as Category);
      success("类别已创建");
    }
    catView.value = "list";
    await load();
  } catch (err) {
    catFormError.value = String((err as Error).message ?? err).replace(/^\w+:\s*/, "");
  }
}

async function doDeleteCategory() {
  if (!deleteCatTarget.value) return;
  try {
    await CategoryService.Delete(deleteCatTarget.value.id);
    success("类别已删除,其中活动已变为未分类");
    deleteCatTarget.value = null;
    await load();
  } catch (err) {
    error(String((err as Error).message ?? err).replace(/^\w+:\s*/, ""));
  }
}

onMounted(load);
</script>

<template>
  <div class="mx-auto max-w-3xl px-8 py-8">
    <div class="mb-6 flex flex-wrap items-end justify-between gap-3">
      <h1 class="font-display text-2xl font-semibold text-ink">活动管理</h1>
      <div class="flex items-center gap-3">
        <label class="flex cursor-pointer items-center gap-1.5 text-sm text-muted">
          <input v-model="showArchived" type="checkbox" class="accent-primary" />
          显示已归档（{{ archivedCount }}）
        </label>
        <button class="mc-btn-ghost" @click="openCatManager">类别管理</button>
        <button class="mc-btn-primary" @click="openCreate">＋ 新建活动</button>
      </div>
    </div>

    <div v-if="loading" class="py-10 text-center text-sm text-muted">加载中…</div>
    <div v-else-if="visible.length === 0" class="mc-card p-10 text-center text-sm text-muted">暂无活动</div>

    <div v-else class="flex flex-col gap-3">
      <div
        v-for="a in visible"
        :key="a.id"
        class="mc-card flex items-center gap-4 p-4"
        :class="!a.archived ? 'opacity-60' : ''"
      >
        <span class="h-10 w-1.5 rounded-full" :style="{ backgroundColor: a.color }" />
        <span v-if="a.icon" class="text-xl">{{ a.icon }}</span>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-sm font-semibold text-ink">{{ a.name }}</span>
            <span
              v-if="a.categoryId != null && categoryById.get(a.categoryId)"
              class="inline-flex items-center gap-1.5 rounded-full bg-surface-card px-2 py-0.5 text-[11px] text-body"
            >
              <span class="h-1.5 w-1.5 rounded-full" :style="{ backgroundColor: categoryById.get(a.categoryId)!.color }" />
              {{ categoryById.get(a.categoryId)!.name }}
            </span>
            <span v-if="a.archived" class="rounded-full bg-surface-card px-2 py-0.5 text-[10px] text-muted">已归档</span>
          </div>
          <p class="mt-0.5 text-xs text-muted">
            {{ a.dailyGoalMinutes > 0 ? `每日目标 ${a.dailyGoalMinutes} 分钟` : "未设目标" }}
          </p>
        </div>
        <div class="flex gap-1">
          <button class="mc-btn-ghost px-3 py-1.5 text-xs" @click="openEdit(a)">编辑</button>
          <button class="mc-btn-ghost px-3 py-1.5 text-xs" @click="toggleArchive(a)">
            {{ !a.archived ? "恢复" : "归档" }}
          </button>
          <button class="mc-btn-ghost px-3 py-1.5 text-xs text-error hover:bg-error/10" @click="deleteTarget = a">删除</button>
        </div>
      </div>
    </div>

    <!-- 活动编辑 -->
    <Modal :open="editorOpen" :title="editing ? '编辑活动' : '新建活动'" @close="editorOpen = false">
      <div class="flex flex-col gap-4">
        <div>
          <label class="mc-label">名称</label>
          <input v-model="form.name" type="text" maxlength="50" placeholder="例如：阅读、健身…" class="mc-input" />
        </div>
        <div>
          <label class="mc-label">类别（可选）</label>
          <select v-model="form.categoryId" class="mc-input">
            <option :value="null">未分类</option>
            <option v-for="c in categories" :key="c.id" :value="c.id">
              {{ c.icon ? `${c.icon} ` : "" }}{{ c.name }}
            </option>
          </select>
          <p class="mt-1 text-xs text-muted-soft">类别可在「类别管理」中自定义。</p>
        </div>
        <div>
          <label class="mc-label">颜色</label>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="c in palette"
              :key="c"
              class="h-7 w-7 cursor-pointer rounded-full border-2 transition-transform"
              :class="form.color === c ? 'scale-110 border-ink' : 'border-transparent'"
              :style="{ backgroundColor: c }"
              @click="form.color = c"
            />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mc-label">图标（可选，一个 emoji）</label>
            <input v-model="form.icon" type="text" maxlength="4" placeholder="📚" class="mc-input" />
          </div>
          <div>
            <label class="mc-label">每日目标（分钟，0 为不设）</label>
            <input v-model.number="form.dailyGoalMinutes" type="number" min="0" max="1440" class="mc-input" />
          </div>
        </div>
        <p v-if="formError" class="text-xs text-error">{{ formError }}</p>
      </div>
      <template #footer>
        <button class="mc-btn-ghost" @click="editorOpen = false">取消</button>
        <button class="mc-btn-primary" @click="save">保存</button>
      </template>
    </Modal>

    <!-- 类别管理（列表 / 表单双视图，避免弹窗叠弹窗） -->
    <Modal :open="catManagerOpen" :title="catView === 'list' ? '类别管理' : catEditing ? '编辑类别' : '新建类别'" wide @close="catManagerOpen = false">
      <div v-if="catView === 'list'">
        <div class="mb-3 flex items-center justify-between">
          <p class="text-xs text-muted">按类别整理活动，打卡页会按类别分组展示。</p>
          <button class="mc-btn-primary px-3 py-1.5 text-xs" @click="openCatCreate">＋ 新建类别</button>
        </div>

        <div v-if="categories.length === 0" class="rounded-xl bg-surface-soft p-8 text-center">
          <p class="text-sm font-medium text-body">还没有类别</p>
          <p class="mt-1 text-xs text-muted">创建一个类别，把活动分类整理起来。</p>
        </div>

        <div v-else class="flex flex-col gap-2">
          <div
            v-for="c in categories"
            :key="c.id"
            class="flex items-center gap-3 rounded-xl border border-hairline p-3"
          >
            <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-base" :style="{ backgroundColor: `${c.color}1a` }">
              {{ c.icon || "●" }}
            </span>
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-semibold text-ink">{{ c.name }}</p>
              <p class="text-xs text-muted">{{ activityCountByCategory.get(c.id) ?? 0 }} 个活动</p>
            </div>
            <div class="flex gap-1">
              <button class="mc-btn-ghost px-3 py-1.5 text-xs" @click="openCatEdit(c)">编辑</button>
              <button class="mc-btn-ghost px-3 py-1.5 text-xs text-error hover:bg-error/10" @click="deleteCatTarget = c">删除</button>
            </div>
          </div>
        </div>
      </div>

      <div v-else class="flex flex-col gap-4">
        <div>
          <label class="mc-label">名称</label>
          <input v-model="catForm.name" type="text" maxlength="20" placeholder="例如：学习、健康、娱乐…" class="mc-input" />
        </div>
        <div>
          <label class="mc-label">图标（可选，一个 emoji）</label>
          <input v-model="catForm.icon" type="text" maxlength="4" placeholder="📚" class="mc-input" />
        </div>
        <div>
          <label class="mc-label">颜色</label>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="c in palette"
              :key="c"
              class="h-7 w-7 cursor-pointer rounded-full border-2 transition-transform"
              :class="catForm.color === c ? 'scale-110 border-ink' : 'border-transparent'"
              :style="{ backgroundColor: c }"
              @click="catForm.color = c"
            />
          </div>
        </div>
        <p v-if="catFormError" class="text-xs text-error">{{ catFormError }}</p>
      </div>

      <template #footer>
        <template v-if="catView === 'list'">
          <button class="mc-btn-ghost" @click="catManagerOpen = false">关闭</button>
        </template>
        <template v-else>
          <button class="mc-btn-ghost" @click="catView = 'list'">返回</button>
          <button class="mc-btn-primary" @click="saveCategory">保存</button>
        </template>
      </template>
    </Modal>

    <ConfirmDialog
      :open="deleteTarget !== null"
      title="删除活动"
      :danger="true"
      confirm-text="全部删除"
      :message="deleteTarget ? `删除「${deleteTarget.name}」会同时删除它的所有计时记录，且无法恢复。确定继续吗？` : ''"
      @confirm="doDelete"
      @cancel="deleteTarget = null"
    />

    <ConfirmDialog
      :open="deleteCatTarget !== null"
      title="删除类别"
      :danger="true"
      confirm-text="删除"
      :message="
        deleteCatTarget
          ? activityCountByCategory.get(deleteCatTarget.id) ?? 0 > 0
            ? `删除类别「${deleteCatTarget.name}」后，该类别下的 ${activityCountByCategory.get(deleteCatTarget.id)} 个活动将变为未分类，所有记录都会保留。确定继续吗？`
            : `删除类别「${deleteCatTarget.name}」？该类别下没有活动，所有记录不受影响。`
          : ''
      "
      @confirm="doDeleteCategory"
      @cancel="deleteCatTarget = null"
    />
  </div>
</template>