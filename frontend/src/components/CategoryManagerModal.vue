<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { CategoryService, type Activity, type Category } from "../lib/api";
import { useToast } from "../composables/useToast";
import ConfirmDialog from "./ConfirmDialog.vue";
import Modal from "./Modal.vue";

const props = defineProps<{
  open: boolean;
  categories: Category[];
  /** All activities (including archived) for per-category counts. */
  activities: Activity[];
}>();

const emit = defineEmits<{ close: []; changed: [] }>();

const { success, error } = useToast();

const palette = [
  "#cc785c",
  "#a9583e",
  "#5db8a6",
  "#e8a55a",
  "#5db872",
  "#d4a017",
  "#c64545",
  "#141413",
  "#6c6a64",
];

const catView = ref<"list" | "form">("list");
const catEditing = ref<Category | null>(null);
const catForm = ref({ name: "", icon: "", color: palette[0] });
const catFormError = ref("");
const deleteCatTarget = ref<Category | null>(null);

const activityCountByCategory = computed(() => {
  const counts = new Map<number, number>();
  for (const a of props.activities) {
    if (a.categoryId == null) continue;
    counts.set(a.categoryId, (counts.get(a.categoryId) ?? 0) + 1);
  }
  return counts;
});

watch(
  () => props.open,
  (open) => {
    if (!open) return;
    catView.value = "list";
    catFormError.value = "";
  },
);

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
    emit("changed");
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
    emit("changed");
  } catch (err) {
    error(String((err as Error).message ?? err).replace(/^\w+:\s*/, ""));
  }
}
</script>

<template>
  <!-- 类别管理（列表 / 表单双视图，避免弹窗叠弹窗） -->
  <Modal
    :open="open"
    :title="catView === 'list' ? '类别管理' : catEditing ? '编辑类别' : '新建类别'"
    wide
    @close="emit('close')"
  >
    <div v-if="catView === 'list'">
      <div class="mb-3 flex items-center justify-between">
        <p class="text-xs text-muted">按类别整理活动，打卡页会按类别分组展示。</p>
        <button class="mc-btn-primary px-3 py-1.5 text-xs" @click="openCatCreate">
          ＋ 新建类别
        </button>
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
          <span
            class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-base"
            :style="{ backgroundColor: `${c.color}1a` }"
          >
            {{ c.icon || "●" }}
          </span>
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-semibold text-ink">{{ c.name }}</p>
            <p class="text-xs text-muted">{{ activityCountByCategory.get(c.id) ?? 0 }} 个活动</p>
          </div>
          <div class="flex gap-1">
            <button class="mc-btn-ghost px-3 py-1.5 text-xs" @click="openCatEdit(c)">编辑</button>
            <button
              class="mc-btn-ghost px-3 py-1.5 text-xs text-error hover:bg-error/10"
              @click="deleteCatTarget = c"
            >
              删除
            </button>
          </div>
        </div>
      </div>
    </div>

    <div v-else class="flex flex-col gap-4">
      <div>
        <label class="mc-label">名称</label>
        <input
          v-model="catForm.name"
          type="text"
          maxlength="20"
          placeholder="例如：学习、健康、娱乐…"
          class="mc-input"
        />
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
        <button class="mc-btn-ghost" @click="emit('close')">关闭</button>
      </template>
      <template v-else>
        <button class="mc-btn-ghost" @click="catView = 'list'">返回</button>
        <button class="mc-btn-primary" @click="saveCategory">保存</button>
      </template>
    </template>

    <ConfirmDialog
      :open="deleteCatTarget !== null"
      title="删除类别"
      :danger="true"
      confirm-text="删除"
      :message="
        deleteCatTarget
          ? (activityCountByCategory.get(deleteCatTarget.id) ?? 0) > 0
            ? `删除类别「${deleteCatTarget.name}」后，该类别下的 ${activityCountByCategory.get(deleteCatTarget.id)} 个活动将变为未分类，所有记录都会保留。确定继续吗？`
            : `删除类别「${deleteCatTarget.name}」？该类别下没有活动，所有记录不受影响。`
          : ''
      "
      @confirm="doDeleteCategory"
      @cancel="deleteCatTarget = null"
    />
  </Modal>
</template>
