<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { centsToYuan, yuanToCents, todayIso, nowTimeHm } from '@/utils/format';
import type { CategoryTreeNode, Transaction } from '@/types';
import BottomSheet from '../components/BottomSheet.vue';
import CategoryBadge from '@/components/shared/CategoryBadge.vue';
import CategoryPicker from '@/components/shared/CategoryPicker.vue';
import CustomDropdown, { type DropdownOption } from '@/components/shared/CustomDropdown.vue';
import ToggleSwitch from '@/components/shared/ToggleSwitch.vue';

const props = defineProps<{
  open: boolean;
  mode: 'transaction' | 'budget';
  transaction: Transaction | null;
}>();

const emit = defineEmits<{
  close: [];
  'update:open': [val: boolean];
}>();

const store = useAppStore();

// ==================== 流水详情编辑状态 ====================
const txType = ref<'expense' | 'income'>('expense');
const amountYuan = ref('');
const title = ref('');
const categoryId = ref('');
const subCategory = ref('');
const walletId = ref('');
const date = ref(todayIso());
const txTime = ref(nowTimeHm());
const note = ref('');
const includeBudget = ref(true);
const reimburse = ref(false);

const savingTx = ref(false);
const deletingTx = ref(false);
const confirmDelete = ref(false);

// ==================== 快捷月预算编辑状态 ====================
const budgetYuan = ref('3000');
const savingBudget = ref(false);
const BUDGET_PRESETS = [3000, 4500, 6000, 8000];

const categoryTree = computed<CategoryTreeNode[]>(() =>
  txType.value === 'income' ? store.incomeCategories : store.expenseCategories
);

const activeCategory = computed(() =>
  categoryTree.value.find((c) => c.id === categoryId.value) || categoryTree.value[0]
);

const walletOptions = computed<DropdownOption[]>(() =>
  store.activeWallets.map((w) => ({
    value: w.id,
    label: w.name,
    dot: w.color,
  }))
);

function hydrateTransactionForm(tx: Transaction | null) {
  confirmDelete.value = false;
  if (!tx) return;
  txType.value = tx.type === 'income' ? 'income' : 'expense';
  amountYuan.value = centsToYuan(tx.amount).toFixed(2);
  title.value = tx.title || tx.category_name || '';
  categoryId.value = tx.category_id || '';
  subCategory.value = tx.sub_category || '';
  walletId.value = tx.wallet_id || store.activeWallets[0]?.id || '';
  date.value = (tx.date || todayIso()).slice(0, 10);
  txTime.value = tx.tx_time || nowTimeHm();
  note.value = tx.note || '';
  includeBudget.value = tx.include_budget ?? true;
  reimburse.value = Boolean(tx.reimburse);
}

function hydrateBudgetForm() {
  const currentCents = store.budgets?.total_budget ?? store.activeBook?.budget ?? 0;
  const yuan = centsToYuan(currentCents);
  budgetYuan.value = yuan > 0 ? String(Math.round(yuan)) : '3000';
}

watch(
  () => [props.open, props.mode, props.transaction] as const,
  ([opened, mode, tx]) => {
    if (!opened) {
      confirmDelete.value = false;
      return;
    }
    if (mode === 'transaction') {
      hydrateTransactionForm(tx);
    } else {
      hydrateBudgetForm();
    }
  },
  { immediate: true }
);

function switchTxType(next: 'expense' | 'income') {
  if (txType.value === next) return;
  txType.value = next;
  const tree = next === 'income' ? store.incomeCategories : store.expenseCategories;
  if (tree.length > 0) {
    categoryId.value = tree[0].id;
    subCategory.value = tree[0].children?.[0]?.name || '';
  }
}

function closeSheet() {
  confirmDelete.value = false;
  emit('update:open', false);
  emit('close');
}

async function handleSaveTransaction() {
  if (!props.transaction || savingTx.value) return;

  const parsed = parseFloat(amountYuan.value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    store.showToast('请输入大于 0 的有效金额', 'error');
    return;
  }

  const trimmedTitle = title.value.trim() || subCategory.value || activeCategory.value?.name || '未命名账单';
  const targetWalletId = walletId.value || store.activeWallets[0]?.id || '';
  if (!targetWalletId) {
    store.showToast('请选择支付账户', 'error');
    return;
  }

  savingTx.value = true;
  try {
    await store.updateTransaction(props.transaction.id, {
      type: txType.value,
      amount: yuanToCents(parsed),
      title: trimmedTitle,
      category_id: categoryId.value || undefined,
      sub_category: subCategory.value || '',
      wallet_id: targetWalletId,
      date: date.value || todayIso(),
      tx_time: txTime.value || nowTimeHm(),
      note: note.value.trim(),
      include_budget: txType.value === 'expense' ? includeBudget.value : false,
      reimburse: txType.value === 'expense' ? reimburse.value : false,
    });
    store.showToast(`已更新「${trimmedTitle}」`, 'success');
    closeSheet();
  } catch (err: any) {
    store.showToast(err?.message || '更新流水失败', 'error');
  } finally {
    savingTx.value = false;
  }
}

async function handleDeleteTransaction() {
  if (!props.transaction || deletingTx.value) return;
  if (!confirmDelete.value) {
    confirmDelete.value = true;
    return;
  }

  deletingTx.value = true;
  try {
    await store.deleteTransaction(props.transaction.id);
    store.showToast('已删除该笔记录', 'success');
    closeSheet();
  } catch (err: any) {
    store.showToast(err?.message || '删除失败', 'error');
  } finally {
    deletingTx.value = false;
  }
}

async function handleSaveBudget() {
  if (savingBudget.value) return;
  const bid = store.activeBookId;
  if (!bid) {
    store.showToast('请先选择账本', 'error');
    return;
  }

  const parsed = parseFloat(budgetYuan.value);
  if (!Number.isFinite(parsed) || parsed < 0) {
    store.showToast('请输入有效的预算金额', 'error');
    return;
  }

  savingBudget.value = true;
  try {
    await api.updateTotalBudget({
      book_id: bid,
      budget: yuanToCents(parsed),
    });
    await Promise.allSettled([store.refreshBudgets(bid), store.refreshBooks()]);
    store.showToast(`月度预算已更新为 ¥${Math.round(parsed)}`, 'success');
    closeSheet();
  } catch (err: any) {
    store.showToast(err?.message || '更新月预算失败', 'error');
  } finally {
    savingBudget.value = false;
  }
}
</script>

<template>
  <BottomSheet
    :open="open"
    :multi-detent="mode === 'transaction'"
    max-height-class="max-h-[90dvh]"
    bg-class="bg-[#F8F9FB]"
    z-index-class="z-40"
    @close="emit('close')"
    @update:open="emit('update:open', $event)"
  >
    <!-- Header -->
    <template #header>
      <div
        v-if="mode === 'transaction'"
        class="px-4 pb-2.5 flex items-center justify-between gap-2 shrink-0"
      >
        <div class="flex items-center gap-2.5 min-w-0">
          <CategoryBadge
            :icon="activeCategory?.icon || transaction?.category_icon || 'tag'"
            :color="activeCategory?.color || transaction?.category_color || '#DE8A1E'"
            :bg-color="activeCategory?.bg_color || transaction?.category_bg_color || '#FFF6E8'"
            size="sm"
          />
          <div class="min-w-0">
            <h3 class="text-sm font-bold text-[#18181B]">编辑流水明细</h3>
            <p class="text-[11px] text-[#64748B] truncate">
              {{ activeCategory?.name || transaction?.category_name || '未分类'
              }}{{ subCategory ? ` · ${subCategory}` : ''
              }}{{ transaction?.created_by_name ? ` · 记录人 ${transaction.created_by_name}` : '' }}
            </p>
          </div>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <div class="inline-flex p-0.5 rounded-full bg-[#ECEEF2] text-[11px] font-semibold">
            <button
              type="button"
              class="px-3 py-1 rounded-full transition"
              :class="
                txType === 'expense'
                  ? 'bg-white text-[#D6553E] shadow-xs'
                  : 'text-[#64748B]'
              "
              @click="switchTxType('expense')"
            >
              支出
            </button>
            <button
              type="button"
              class="px-3 py-1 rounded-full transition"
              :class="
                txType === 'income'
                  ? 'bg-white text-[#3E8E63] shadow-xs'
                  : 'text-[#64748B]'
              "
              @click="switchTxType('income')"
            >
              收入
            </button>
          </div>
          <button
            type="button"
            aria-label="关闭详情抽屉"
            class="w-8 h-8 rounded-full bg-white border border-[#E6E9EF] flex items-center justify-center text-[#64748B] hover:text-[#18181B]"
            @click="closeSheet"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
              <path d="M18 6L6 18M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      <div v-else class="px-5 pb-2 flex items-center justify-between shrink-0">
        <div>
          <h3 class="text-base font-bold text-[#18181B]">调整月度预算</h3>
          <p class="text-xs text-[#64748B]">
            当前账本：{{ store.activeBook?.name || '默认账本' }}
          </p>
        </div>
        <button
          type="button"
          aria-label="关闭预算调整"
          class="w-8 h-8 rounded-full bg-white border border-[#E6E9EF] flex items-center justify-center text-[#64748B] hover:text-[#18181B]"
          @click="closeSheet"
        >
          <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
        </button>
      </div>
    </template>

    <!-- Body: 编辑交易流水 -->
    <div
      v-if="mode === 'transaction' && transaction"
      class="flex-1 min-h-0 overflow-y-auto phone-scroll px-4 pb-4 space-y-3"
    >
      <!-- 金额与名称 -->
      <div class="app-card p-3.5 space-y-2.5">
        <div class="flex items-center justify-between border-b border-[#F1F3F6] pb-2">
          <span class="text-xs font-medium text-[#64748B]">金额（元）</span>
          <input
            v-model="amountYuan"
            type="number"
            step="0.01"
            min="0.01"
            required
            class="w-36 text-right text-lg font-bold tabular-nums outline-none bg-transparent"
            :class="txType === 'income' ? 'text-[#3E8E63]' : 'text-[#D6553E]'"
          />
        </div>
        <div class="flex items-center justify-between gap-2">
          <span class="text-xs font-medium text-[#64748B] shrink-0">名称 / 商户</span>
          <input
            v-model="title"
            type="text"
            maxlength="32"
            placeholder="输入名称或商户"
            class="flex-1 min-w-0 text-right text-xs font-semibold text-[#18181B] outline-none bg-transparent"
          />
        </div>
      </div>

      <!-- 一二级分类选择 -->
      <div class="app-card p-3">
        <CategoryPicker
          v-model="categoryId"
          v-model:sub-category="subCategory"
          :type="txType"
          :tree="categoryTree"
          :columns="5"
        />
      </div>

      <!-- 日期、时间、账户与备注 -->
      <div class="app-card divide-y divide-[#F1F3F6] text-xs">
        <div class="px-3.5 py-2.5 flex items-center justify-between gap-2">
          <span class="text-[#64748B]">日期</span>
          <input
            v-model="date"
            type="date"
            class="text-right font-semibold text-[#18181B] bg-transparent outline-none"
          />
        </div>
        <div class="px-3.5 py-2.5 flex items-center justify-between gap-2">
          <span class="text-[#64748B]">时间</span>
          <input
            v-model="txTime"
            type="time"
            class="text-right font-semibold text-[#18181B] bg-transparent outline-none"
          />
        </div>
        <div class="px-3.5 py-2.5 flex items-center justify-between gap-3">
          <span class="text-[#64748B] shrink-0">支付账户</span>
          <CustomDropdown
            v-model="walletId"
            :options="walletOptions"
            placeholder="选择支付账户"
            width-class="w-44"
            size="sm"
          />
        </div>
        <div class="px-3.5 py-2.5 flex items-center justify-between gap-2">
          <span class="text-[#64748B] shrink-0">备注</span>
          <input
            v-model="note"
            type="text"
            maxlength="50"
            placeholder="添加备注信息"
            class="flex-1 min-w-0 text-right text-[#18181B] bg-transparent outline-none placeholder-[#94A3B8]"
          />
        </div>
      </div>

      <!-- 开关设置 -->
      <div
        v-if="txType === 'expense'"
        class="app-card divide-y divide-[#F1F3F6] text-xs"
      >
        <div class="px-3.5 py-3 flex items-center justify-between">
          <div>
            <p class="font-medium text-[#18181B]">计入本月预算</p>
            <p class="text-[10px] text-[#94A3B8]">关闭后不计入月度预算进度条统计</p>
          </div>
          <ToggleSwitch v-model="includeBudget" aria-label="计入本月预算" />
        </div>
        <div class="px-3.5 py-3 flex items-center justify-between">
          <div>
            <p class="font-medium text-[#18181B]">可报销账单</p>
            <p class="text-[10px] text-[#94A3B8]">在流水列表中展示「待报销」角标</p>
          </div>
          <ToggleSwitch v-model="reimburse" aria-label="可报销账单" />
        </div>
      </div>
    </div>

    <!-- Body: 快捷修改月预算 -->
    <div v-else class="px-4 py-3 space-y-3">
      <div class="flex items-center gap-2 bg-white border border-[#DFE3EA] rounded-2xl px-3.5 py-2.5">
        <span class="text-base font-bold text-[#DE8A1E]">¥</span>
        <input
          v-model="budgetYuan"
          type="number"
          min="0"
          step="100"
          placeholder="输入月度预算金额"
          class="w-full text-lg font-bold tabular-nums outline-none text-[#18181B] bg-transparent"
        />
      </div>

      <div class="flex gap-2">
        <button
          v-for="val in BUDGET_PRESETS"
          :key="val"
          type="button"
          class="flex-1 py-2 rounded-xl border text-xs font-semibold transition"
          :class="
            Number(budgetYuan) === val
              ? 'bg-[#FFF6E8] border-[#DE8A1E] text-[#DE8A1E]'
              : 'bg-white border-[#E6E9EF] text-[#27272A] hover:border-[#DE8A1E]'
          "
          @click="budgetYuan = String(val)"
        >
          ¥{{ val }}
        </button>
      </div>
    </div>

    <!-- Footer -->
    <template #footer>
      <div
        v-if="mode === 'transaction' && transaction"
        class="shrink-0 px-4 py-2.5 border-t border-[#ECEEF2] bg-[#F8F9FB] grid grid-cols-3 gap-2"
      >
        <button
          type="button"
          :disabled="deletingTx || savingTx"
          class="py-2.5 rounded-2xl text-xs font-semibold transition disabled:opacity-60"
          :class="
            confirmDelete
              ? 'bg-[#D6553E] text-white'
              : 'bg-[#FDF0EE] text-[#D6553E] hover:bg-[#FADCD7]'
          "
          @click="handleDeleteTransaction"
        >
          {{ deletingTx ? '删除中…' : confirmDelete ? '确认删除？' : '删除' }}
        </button>
        <button
          type="button"
          :disabled="savingTx || deletingTx"
          class="col-span-2 py-2.5 rounded-2xl bg-[#DE8A1E] text-white text-xs font-semibold shadow-sm hover:bg-[#C67612] disabled:opacity-60 transition"
          @click="handleSaveTransaction"
        >
          {{ savingTx ? '保存中…' : '保存修改' }}
        </button>
      </div>

      <div
        v-else
        class="shrink-0 px-4 pt-2 pb-5 bg-[#F8F9FB] grid grid-cols-2 gap-2.5"
      >
        <button
          type="button"
          class="py-2.5 rounded-2xl bg-[#ECEEF2] text-[#475569] text-xs font-semibold hover:bg-[#E2E6EC] transition"
          @click="closeSheet"
        >
          取消
        </button>
        <button
          type="button"
          :disabled="savingBudget"
          class="py-2.5 rounded-2xl bg-[#DE8A1E] text-white text-xs font-semibold hover:bg-[#C67612] disabled:opacity-60 transition"
          @click="handleSaveBudget"
        >
          {{ savingBudget ? '保存中…' : '保存预算' }}
        </button>
      </div>
    </template>
  </BottomSheet>
</template>
