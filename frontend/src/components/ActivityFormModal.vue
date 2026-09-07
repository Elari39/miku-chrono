<script setup lang="ts">
import { ref, watch } from "vue";
import { ActivityService, type Activity, type Category } from "../lib/api";
import { useToast } from "../composables/useToast";
import { errorMessage } from "../lib/errors";
import { normalizeDailyGoal } from "../lib/goal";
import { PALETTE } from "../lib/palette";
import ColorSwatchPicker from "./ColorSwatchPicker.vue";
import Modal from "./Modal.vue";

const props = defineProps<{
  open: boolean;
  /** Non-null when editing an existing activity; null creates a new one. */
  activity: Activity | null;
  categories: Category[];
}>();

const emit = defineEmits<{ close: []; saved: [] }>();

const { success } = useToast();

const palette = PALETTE;

const emptyForm = () => ({
  name: "",
  color: palette[0],
  icon: "",
  categoryId: null as number | null,
  dailyGoalMinutes: 0,
});
const form = ref(emptyForm());
const formError = ref("");

watch(
  () => props.open,
  (open) => {
    if (!open) return;
    formError.value = "";
    if (props.activity) {
      form.value = {
        name: props.activity.name,
        color: props.activity.color,
        icon: props.activity.icon,
        categoryId: props.activity.categoryId ?? null,
        dailyGoalMinutes: props.activity.dailyGoalMinutes,
      };
    } else {
      form.value = emptyForm();
    }
  },
);

async function save() {
  formError.value = "";
  if (!form.value.name.trim()) {
    formError.value = "活动名称不能为空";
    return;
  }
  // Normalize before sending: an emptied number input would otherwise carry
  // a raw string into the Go int field and fail as a JSON decode error.
  form.value.dailyGoalMinutes = normalizeDailyGoal(form.value.dailyGoalMinutes);
  const payload: Activity = props.activity
    ? { ...props.activity, ...form.value }
    : ({ ...form.value } as Activity);
  try {
    if (props.activity) {
      await ActivityService.Update(payload);
      success("活动已更新");
    } else {
      await ActivityService.Create(payload);
      success("活动已创建");
    }
    emit("saved");
  } catch (err) {
    formError.value = errorMessage(err);
  }
}
</script>

<template>
  <Modal :open="open" :title="activity ? '编辑活动' : '新建活动'" @close="emit('close')">
    <div class="flex flex-col gap-4">
      <div>
        <label class="mc-label" for="activity-name">名称</label>
        <input
          id="activity-name"
          v-model="form.name"
          type="text"
          maxlength="50"
          placeholder="例如：阅读、健身…"
          class="mc-input"
        />
      </div>
      <div>
        <label class="mc-label" for="activity-category">类别（可选）</label>
        <select id="activity-category" v-model="form.categoryId" class="mc-input">
          <option :value="null">未分类</option>
          <option v-for="c in categories" :key="c.id" :value="c.id">
            {{ c.icon ? `${c.icon} ` : "" }}{{ c.name }}
          </option>
        </select>
        <p class="mt-1 text-xs text-muted-soft">类别可在「类别管理」中自定义。</p>
      </div>
      <div>
        <span class="mc-label">颜色</span>
        <ColorSwatchPicker v-model="form.color" />
      </div>
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="mc-label" for="activity-icon">图标（可选，一个 emoji）</label>
          <input
            id="activity-icon"
            v-model="form.icon"
            type="text"
            maxlength="4"
            placeholder="📚"
            class="mc-input"
          />
        </div>
        <div>
          <label class="mc-label" for="activity-goal">每日目标（分钟，0 为不设）</label>
          <input
            id="activity-goal"
            v-model.number="form.dailyGoalMinutes"
            type="number"
            min="0"
            max="1440"
            class="mc-input"
          />
        </div>
      </div>
      <p v-if="formError" class="text-xs text-error">{{ formError }}</p>
    </div>
    <template #footer>
      <button class="mc-btn-ghost" @click="emit('close')">取消</button>
      <button class="mc-btn-primary" @click="save">保存</button>
    </template>
  </Modal>
</template>
