<script setup lang="ts">
import { computed, watch } from 'vue';
import type { CategoryTreeNode } from '@/types';
import { getIconSvg } from '@/constants/icons';
import CategoryBadge from './CategoryBadge.vue';

const props = withDefaults(
  defineProps<{
    type: 'expense' | 'income';
    modelValue: string;
    subCategory?: string;
    tree: CategoryTreeNode[];
    columns?: 4 | 5;
  }>(),
  {
    subCategory: '',
    columns: 5,
  }
);

const emit = defineEmits<{
  'update:modelValue': [categoryId: string];
  'update:subCategory': [subName: string];
  change: [payload: { categoryId: string; subCategory: string; category?: CategoryTreeNode }];
}>();

const activeCategory = computed<CategoryTreeNode | undefined>(() => {
  if (!sortedTree.value.length) return undefined;
  return sortedTree.value.find((c) => c.id === props.modelValue) || sortedTree.value[0];
});

// 动态排序：近 90 天常用在前（usage_count 大的置顶），没用过的按原 sort_order 沉底；
// 只影响记账选分类的展示顺序，分类管理里的手动顺序不受影响。
// 同一父级下的二级分类也按各自的使用次数排序。
const sortedTree = computed<CategoryTreeNode[]>(() => {
  const byUsage = (
    a: { usage_count?: number; sort_order?: number },
    b: { usage_count?: number; sort_order?: number }
  ) => (b.usage_count ?? 0) - (a.usage_count ?? 0) || (a.sort_order ?? 0) - (b.sort_order ?? 0);
  return [...props.tree]
    .sort(byUsage)
    .map((n) => ({ ...n, children: [...(n.children || [])].sort(byUsage) }));
});

const subCategories = computed(() => activeCategory.value?.children || []);

const gridClass = computed(() =>
  props.columns === 4 ? 'grid-cols-4' : 'grid-cols-5'
);

// 当树或类型切换导致当前选中分类无效时，自动归位到首个分类（常用在前的第一个）
watch(
  () => [props.type, props.tree] as const,
  () => {
    if (!sortedTree.value.length) return;
    const exists = sortedTree.value.some((c) => c.id === props.modelValue);
    if (!exists) {
      const first = sortedTree.value[0];
      const firstSub = first.children?.[0]?.name || '';
      emit('update:modelValue', first.id);
      emit('update:subCategory', firstSub);
      emit('change', { categoryId: first.id, subCategory: firstSub, category: first });
    }
  },
  { immediate: true }
);

function selectCategory(cat: CategoryTreeNode) {
  const subs = cat.children || [];
  const nextSub = subs.some((s) => s.name === props.subCategory)
    ? props.subCategory
    : subs[0]?.name || '';
  emit('update:modelValue', cat.id);
  emit('update:subCategory', nextSub);
  emit('change', { categoryId: cat.id, subCategory: nextSub, category: cat });
}

function selectSubCategory(subName: string) {
  if (!activeCategory.value) return;
  emit('update:subCategory', subName);
  emit('change', {
    categoryId: activeCategory.value.id,
    subCategory: subName,
    category: activeCategory.value,
  });
}
</script>

<template>
  <div class="space-y-2.5">
    <div v-if="sortedTree.length > 0" class="grid gap-y-2.5 gap-x-1" :class="gridClass">
      <button
        v-for="cat in sortedTree"
        :key="cat.id"
        type="button"
        class="flex flex-col items-center gap-1 py-1.5 px-0.5 rounded-2xl transition"
        :class="
          activeCategory?.id === cat.id
            ? 'bg-white shadow-xs ring-1 ring-[#DE8A1E]/45'
            : 'hover:bg-[#F8F9FB]'
        "
        @click="selectCategory(cat)"
      >
        <CategoryBadge
          :icon="cat.icon"
          :color="cat.color"
          :bg-color="cat.bg_color || '#FFF6E8'"
          size="md"
          :selected="activeCategory?.id === cat.id"
        />
        <span
          class="text-[11px] truncate max-w-full px-0.5"
          :class="
            activeCategory?.id === cat.id
              ? 'font-bold text-[#18181B]'
              : 'font-medium text-[#64748B]'
          "
        >
          {{ cat.name }}
        </span>
      </button>
    </div>

    <!-- 二级分类：自动换行，最多显示两行，超出可上下滚动 -->
    <div
      v-if="subCategories.length > 0 && activeCategory"
      class="pt-2.5 border-t border-[#F1F3F6] flex flex-wrap items-center gap-1.5 overflow-y-auto phone-scroll max-h-[62px]"
    >
      <button
        v-for="sub in subCategories"
        :key="sub.id"
        type="button"
        class="inline-flex items-center gap-1.5 text-[11px] pl-1.5 pr-2.5 py-1 rounded-full transition"
        :class="
          subCategory === sub.name
            ? 'bg-[#18181B] text-white font-semibold shadow-xs'
            : 'bg-white border border-[#E6E9EF] text-[#475569] hover:border-[#DE8A1E]'
        "
        @click="selectSubCategory(sub.name)"
      >
        <span
          class="w-5 h-5 rounded-full flex items-center justify-center shrink-0"
          :style="{
            backgroundColor:
              subCategory === sub.name
                ? 'rgba(255,255,255,0.16)'
                : activeCategory.bg_color || '#FFF6E8',
            color: subCategory === sub.name ? '#FFFFFF' : activeCategory.color,
          }"
        >
          <svg
            class="w-3 h-3"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            stroke-linecap="round"
            stroke-linejoin="round"
            v-html="getIconSvg(sub.icon || activeCategory.icon)"
          />
        </span>
        <span>{{ sub.name }}</span>
      </button>
    </div>
  </div>
</template>
