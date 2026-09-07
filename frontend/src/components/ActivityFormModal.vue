<script setup lang="ts">
import { ref, watch } from "vue";
import { ActivityService, type Activity, type Category } from "../lib/api";
import { useToast } from "../composables/useToast";
import { errorMessage } from "../lib/errors";
import { PALETTE } from "../lib/palette";
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
        <label class="mc-label">名称</label>
        <input
          v-model="form.name"
          type="text"
          maxlength="50"
          placeholder="例如：阅读、健身…"
          class="mc-input"
        />
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
          <input
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
