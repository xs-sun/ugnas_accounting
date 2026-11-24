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
  if (!props.tree.length) return undefined;
  return props.tree.find((c) => c.id === props.modelValue) || props.tree[0];
});

const subCategories = computed(() => activeCategory.value?.children || []);

const gridClass = computed(() =>
  props.columns === 4 ? 'grid-cols-4' : 'grid-cols-5'
);

// 当树或类型切换导致当前选中分类无效时，自动归位到首个分类
watch(
  () => [props.type, props.tree] as const,
  () => {
    if (!props.tree.length) return;
    const exists = props.tree.some((c) => c.id === props.modelValue);
    if (!exists) {
      const first = props.tree[0];
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
    <div v-if="tree.length > 0" class="grid gap-y-2.5 gap-x-1" :class="gridClass">
      <button
        v-for="cat in tree"
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

    <div
      v-if="subCategories.length > 0 && activeCategory"
      class="pt-2.5 border-t border-[#F1F3F6] flex items-center gap-1.5 overflow-x-auto phone-scroll"
    >
      <button
        v-for="sub in subCategories"
        :key="sub.id"
        type="button"
        class="shrink-0 inline-flex items-center gap-1.5 text-[11px] pl-1.5 pr-2.5 py-1 rounded-full transition"
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
