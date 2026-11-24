<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import {
  PICKABLE_ICONS,
  CATEGORY_COLOR_PRESETS,
  getIconSvg,
} from '@/constants/icons';
import type { CategoryTreeNode, Category } from '@/types';
import CategoryBadge from '@/components/shared/CategoryBadge.vue';

const store = useAppStore();

const catType = ref<'expense' | 'income'>('expense');
const selectedCatId = ref<string>('');
const newSubName = ref('');
const newSubIcon = ref<string>('coffee');
const savingSub = ref(false);

const catModalOpen = ref(false);
const editingParentCat = ref<CategoryTreeNode | null>(null);
const savingParentCat = ref(false);
const parentCatForm = reactive({
  name: '',
  icon: 'tag',
  color: CATEGORY_COLOR_PRESETS[0].color,
  bgColor: CATEGORY_COLOR_PRESETS[0].bgColor,
});

const categoryList = computed<CategoryTreeNode[]>(() =>
  catType.value === 'income' ? store.incomeCategories : store.expenseCategories
);

const activeCategory = computed<CategoryTreeNode | undefined>(() => {
  if (!categoryList.value.length) return undefined;
  return (
    categoryList.value.find((c) => c.id === selectedCatId.value) || categoryList.value[0]
  );
});

watch(
  [catType, categoryList],
  () => {
    if (!categoryList.value.length) {
      selectedCatId.value = '';
      return;
    }
    if (!categoryList.value.some((c) => c.id === selectedCatId.value)) {
      selectedCatId.value = categoryList.value[0].id;
    }
  },
  { immediate: true }
);

function openAddParentCat() {
  editingParentCat.value = null;
  parentCatForm.name = '';
  parentCatForm.icon = PICKABLE_ICONS[0] || 'tag';
  const preset = CATEGORY_COLOR_PRESETS[1] || CATEGORY_COLOR_PRESETS[0];
  parentCatForm.color = preset.color;
  parentCatForm.bgColor = preset.bgColor;
  catModalOpen.value = true;
}

function openEditParentCat(cat: CategoryTreeNode) {
  editingParentCat.value = cat;
  parentCatForm.name = cat.name;
  parentCatForm.icon = cat.icon || 'tag';
  parentCatForm.color = cat.color || CATEGORY_COLOR_PRESETS[0].color;
  parentCatForm.bgColor = cat.bg_color || CATEGORY_COLOR_PRESETS[0].bgColor;
  catModalOpen.value = true;
}

function pickColorPreset(preset: { color: string; bgColor: string }) {
  parentCatForm.color = preset.color;
  parentCatForm.bgColor = preset.bgColor;
}

async function handleSaveParentCat() {
  if (savingParentCat.value) return;
  const name = parentCatForm.name.trim();
  if (!name || !store.activeBookId) return;

  savingParentCat.value = true;
  try {
    if (editingParentCat.value) {
      await api.updateCategory(editingParentCat.value.id, {
        name,
        icon: parentCatForm.icon,
        color: parentCatForm.color,
        bg_color: parentCatForm.bgColor,
      });
      store.showToast(`已更新一级分类「${name}」`, 'success');
    } else {
      const created = await api.createCategory({
        book_id: store.activeBookId,
        type: catType.value,
        name,
        icon: parentCatForm.icon,
        color: parentCatForm.color,
        bg_color: parentCatForm.bgColor,
        parent_id: null,
        is_custom: true,
        sort_order: categoryList.value.length + 1,
      });
      selectedCatId.value = created.id;
      store.showToast(`已创建一级分类「${name}」`, 'success');
    }

    await store.refreshCategories(store.activeBookId);
    catModalOpen.value = false;
  } catch (err: any) {
    store.showToast(err?.message || '保存分类失败', 'error');
  } finally {
    savingParentCat.value = false;
  }
}

async function handleDeleteParentCat(cat: CategoryTreeNode) {
  if (categoryList.value.length <= 1) {
    store.showToast('至少需保留一个一级分类', 'error');
    return;
  }
  try {
    await api.deleteCategory(cat.id);
    await store.refreshCategories(store.activeBookId);
    store.showToast(`已删除分类「${cat.name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '删除分类失败', 'error');
  }
}

async function handleCreateSubCategory() {
  if (!activeCategory.value || !store.activeBookId || savingSub.value) return;
  const name = newSubName.value.trim();
  if (!name) return;

  savingSub.value = true;
  try {
    await api.createCategory({
      book_id: store.activeBookId,
      type: catType.value,
      name,
      icon: newSubIcon.value,
      color: activeCategory.value.color,
      bg_color: activeCategory.value.bg_color,
      parent_id: activeCategory.value.id,
      is_custom: true,
      sort_order: (activeCategory.value.children?.length || 0) + 1,
    });
    newSubName.value = '';
    await store.refreshCategories(store.activeBookId);
    store.showToast(`已添加二级分类「${name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '添加二级分类失败', 'error');
  } finally {
    savingSub.value = false;
  }
}

async function handleDeleteSubCategory(sub: Category) {
  try {
    await api.deleteCategory(sub.id);
    await store.refreshCategories(store.activeBookId);
    store.showToast(`已移除二级分类「${sub.name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '删除二级分类失败', 'error');
  }
}
</script>

<template>
  <div class="grid grid-cols-1 lg:grid-cols-12 gap-5 items-start">
    <!-- 左侧 5 列：一级分类管理 -->
    <section class="pc-card p-5 lg:col-span-5 space-y-4">
      <div class="flex items-center justify-between gap-2">
        <div class="p-1 rounded-xl bg-[#F4F5F8] inline-flex text-xs font-semibold">
          <button
            type="button"
            class="px-3.5 py-1 rounded-lg transition"
            :class="
              catType === 'expense' ? 'bg-white text-[#18181B] shadow-xs' : 'text-[#64748B]'
            "
            @click="catType = 'expense'"
          >
            支出分类
          </button>
          <button
            type="button"
            class="px-3.5 py-1 rounded-lg transition"
            :class="
              catType === 'income' ? 'bg-white text-[#18181B] shadow-xs' : 'text-[#64748B]'
            "
            @click="catType = 'income'"
          >
            收入分类
          </button>
        </div>

        <button
          type="button"
          class="text-xs font-semibold text-[#DE8A1E] hover:underline shrink-0"
          @click="openAddParentCat"
        >
          + 新增一级
        </button>
      </div>

      <div v-if="categoryList.length === 0" class="py-10 text-center text-xs text-[#94A3B8]">
        暂无一级分类，点击右上角新增
      </div>

      <div v-else class="space-y-1.5">
        <div
          v-for="cat in categoryList"
          :key="cat.id"
          class="px-3 py-2.5 rounded-2xl flex items-center justify-between gap-2 transition cursor-pointer"
          :class="
            activeCategory?.id === cat.id
              ? 'bg-[#FFF6E8] ring-1 ring-[#DE8A1E]/40'
              : 'hover:bg-[#FAFBFD]'
          "
          @click="selectedCatId = cat.id"
        >
          <div class="flex items-center gap-3 min-w-0">
            <CategoryBadge
              :icon="cat.icon"
              :color="cat.color"
              :bg-color="cat.bg_color || '#FFF6E8'"
              size="sm"
            />
            <div class="min-w-0">
              <p class="text-xs font-bold text-[#18181B] truncate">{{ cat.name }}</p>
              <p class="text-[10px] text-[#94A3B8]">
                {{ cat.children?.length || 0 }} 个二级分类
              </p>
            </div>
          </div>

          <div class="flex items-center gap-2 shrink-0" @click.stop>
            <button
              type="button"
              class="text-[11px] font-semibold text-[#64748B] hover:text-[#DE8A1E]"
              @click="openEditParentCat(cat)"
            >
              编辑
            </button>
            <button
              type="button"
              class="text-[11px] font-semibold text-[#94A3B8] hover:text-[#D6553E]"
              @click="handleDeleteParentCat(cat)"
            >
              删除
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- 右侧 7 列：二级子分类网格与新增 -->
    <section class="pc-card p-5 lg:col-span-7 space-y-4">
      <template v-if="activeCategory">
        <div class="flex items-center justify-between border-b border-[#F1F3F6] pb-3.5">
          <div class="flex items-center gap-3">
            <CategoryBadge
              :icon="activeCategory.icon"
              :color="activeCategory.color"
              :bg-color="activeCategory.bg_color || '#FFF6E8'"
              size="lg"
            />
            <div>
              <h2 class="text-sm font-bold text-[#18181B]">
                {{ activeCategory.name }} · 二级分类管理
              </h2>
              <p class="text-[11px] text-[#64748B]">
                每个二级分类均可独立配置双色线框图标，记账时精准匹配
              </p>
            </div>
          </div>

          <button
            type="button"
            class="px-3 py-1.5 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] text-xs font-semibold text-[#475569]"
            @click="openEditParentCat(activeCategory)"
          >
            编辑一级外观
          </button>
        </div>

        <div
          v-if="!activeCategory.children || activeCategory.children.length === 0"
          class="py-8 text-center text-xs text-[#94A3B8] bg-[#FAFBFD] rounded-2xl border border-dashed border-[#EAECEF]"
        >
          「{{ activeCategory.name }}」暂无二级子分类，可在下方快速添加
        </div>

        <div v-else class="grid grid-cols-2 sm:grid-cols-3 gap-2.5">
          <div
            v-for="sub in activeCategory.children"
            :key="sub.id"
            class="p-3 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] flex items-center justify-between gap-2"
          >
            <div class="flex items-center gap-2 min-w-0">
              <CategoryBadge
                :icon="sub.icon || activeCategory.icon"
                :color="activeCategory.color"
                :bg-color="activeCategory.bg_color || '#FFF6E8'"
                size="sm"
              />
              <span class="text-xs font-semibold text-[#18181B] truncate">{{ sub.name }}</span>
            </div>
            <button
              type="button"
              title="删除子分类"
              class="text-[#94A3B8] hover:text-[#D6553E] text-sm px-1 shrink-0"
              @click="handleDeleteSubCategory(sub)"
            >
              ×
            </button>
          </div>
        </div>

        <!-- 新增二级分类表单 -->
        <form
          class="p-4 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] space-y-3"
          @submit.prevent="handleCreateSubCategory"
        >
          <p class="text-xs font-bold text-[#18181B]">
            + 新增「{{ activeCategory.name }}」二级分类
          </p>

          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="iconId in PICKABLE_ICONS"
              :key="iconId"
              type="button"
              class="w-8 h-8 rounded-xl flex items-center justify-center transition"
              :class="
                newSubIcon === iconId
                  ? 'ring-2 ring-[#18181B]'
                  : 'opacity-75 hover:opacity-100'
              "
              :style="{
                backgroundColor:
                  newSubIcon === iconId
                    ? activeCategory.color
                    : activeCategory.bg_color || '#FFF6E8',
                color: newSubIcon === iconId ? '#FFFFFF' : activeCategory.color,
              }"
              @click="newSubIcon = iconId"
            >
              <svg
                class="w-4 h-4"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.75"
                stroke-linecap="round"
                stroke-linejoin="round"
                v-html="getIconSvg(iconId)"
              />
            </button>
          </div>

          <div class="flex gap-2">
            <input
              v-model="newSubName"
              type="text"
              required
              maxlength="12"
              placeholder="输入二级分类名称（如：下午茶、买菜）"
              class="flex-1 min-w-0 h-9 text-xs bg-white border border-[#E6E9EF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E]"
            />
            <button
              type="submit"
              :disabled="savingSub"
              class="h-9 px-4 rounded-xl bg-[#DE8A1E] hover:bg-[#C67612] text-white text-xs font-semibold shrink-0 disabled:opacity-60 transition"
            >
              {{ savingSub ? '添加中…' : '添加二级分类' }}
            </button>
          </div>
        </form>
      </template>

      <div v-else class="py-16 text-center text-xs text-[#94A3B8]">
        请在左侧选择一个一级分类进行管理
      </div>
    </section>

    <!-- 一级分类新增/编辑弹窗 -->
    <Teleport to="body">
      <div v-if="catModalOpen" class="fixed inset-0 z-[70] flex items-center justify-center p-4">
        <div
          class="absolute inset-0 bg-[#0F172A]/40 backdrop-blur-[2px]"
          @click="catModalOpen = false"
        />
        <div class="relative w-full max-w-[460px] bg-white rounded-[24px] border border-[#EAECEF] shadow-2xl overflow-hidden">
          <div class="px-5 py-4 border-b border-[#F1F3F6] flex items-center justify-between">
            <h3 class="text-sm font-bold text-[#18181B]">
              {{ editingParentCat ? '编辑一级分类' : `新增${catType === 'income' ? '收入' : '支出'}一级分类` }}
            </h3>
            <button
              type="button"
              class="w-7 h-7 rounded-full hover:bg-[#F4F5F8] flex items-center justify-center text-[#64748B]"
              @click="catModalOpen = false"
            >
              ×
            </button>
          </div>

          <form class="p-5 space-y-4 text-xs" @submit.prevent="handleSaveParentCat">
            <div class="flex items-center gap-3 p-3 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF]">
              <CategoryBadge
                :icon="parentCatForm.icon"
                :color="parentCatForm.color"
                :bg-color="parentCatForm.bgColor"
                size="lg"
              />
              <div class="flex-1 min-w-0">
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">分类名称</label>
                <input
                  v-model="parentCatForm.name"
                  type="text"
                  required
                  maxlength="10"
                  placeholder="如：健身、旅行"
                  class="w-full h-9 bg-white border border-[#EAECEF] rounded-xl px-3 font-semibold outline-none focus:border-[#DE8A1E]"
                />
              </div>
            </div>

            <div>
              <label class="block text-[11px] font-semibold text-[#64748B] mb-1.5">选择配色</label>
              <div class="flex flex-wrap gap-2">
                <button
                  v-for="preset in CATEGORY_COLOR_PRESETS"
                  :key="preset.color"
                  type="button"
                  class="w-7 h-7 rounded-full transition"
                  :class="parentCatForm.color === preset.color ? 'ring-2 ring-offset-2 ring-[#18181B]' : ''"
                  :style="{ backgroundColor: preset.color }"
                  :title="preset.label"
                  @click="pickColorPreset(preset)"
                />
              </div>
            </div>

            <div>
              <label class="block text-[11px] font-semibold text-[#64748B] mb-1.5">选择图标</label>
              <div class="grid grid-cols-8 gap-1.5 max-h-40 overflow-y-auto thin-scroll p-1">
                <button
                  v-for="iconId in PICKABLE_ICONS"
                  :key="iconId"
                  type="button"
                  class="w-9 h-9 rounded-xl flex items-center justify-center transition"
                  :class="parentCatForm.icon === iconId ? 'ring-2 ring-[#18181B]' : 'hover:opacity-90'"
                  :style="{
                    backgroundColor:
                      parentCatForm.icon === iconId ? parentCatForm.color : parentCatForm.bgColor,
                    color: parentCatForm.icon === iconId ? '#FFFFFF' : parentCatForm.color,
                  }"
                  @click="parentCatForm.icon = iconId"
                >
                  <svg
                    class="w-4 h-4"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.75"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    v-html="getIconSvg(iconId)"
                  />
                </button>
              </div>
            </div>

            <div class="pt-2 border-t border-[#F1F3F6] flex justify-end gap-2">
              <button
                type="button"
                class="px-4 py-2 rounded-xl bg-[#F4F5F8] text-[#475569] font-semibold"
                @click="catModalOpen = false"
              >
                取消
              </button>
              <button
                type="submit"
                :disabled="savingParentCat"
                class="btn-amber px-5 py-2 rounded-xl text-white font-semibold disabled:opacity-60"
              >
                {{ savingParentCat ? '保存中…' : '保存分类' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>
  </div>
</template>
