<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { formatMoney, getWeekdayLabel } from '@/utils/format';
import type { Transaction } from '@/types';
import CategoryBadge from '@/components/shared/CategoryBadge.vue';
import CustomDropdown from '@/components/shared/CustomDropdown.vue';
import type { DropdownOption } from '@/components/shared/CustomDropdown.vue';

const emit = defineEmits<{
  'edit-tx': [tx: Transaction];
  'open-import-export': [];
  'open-record': [opts?: { mode?: 'ai' | 'manual' }];
}>();

const store = useAppStore();

const scopeMode = ref<'month' | 'all'>('month');
const viewStyle = ref<'table' | 'grouped'>('table');
const searchQuery = ref('');
const filterType = ref<'all' | 'expense' | 'income' | 'transfer'>('all');
const filterCatId = ref<string>('all');
const filterWalletId = ref<string>('all');
const onlyReimburse = ref(false);

const loadingList = ref(false);
const rawTransactions = ref<Transaction[]>([]);
const selectedIds = ref<string[]>([]);
const batchDeleting = ref(false);
const confirmBatchDelete = ref(false);

function getMonthDateRange(year: number, month: number) {
  const mm = String(month).padStart(2, '0');
  const lastDay = new Date(year, month, 0).getDate();
  return {
    start: `${year}-${mm}-01`,
    end: `${year}-${mm}-${String(lastDay).padStart(2, '0')}`,
  };
}

async function fetchLedgerTransactions() {
  if (!store.activeBookId) {
    rawTransactions.value = [];
    return;
  }
  loadingList.value = true;
  try {
    const range =
      scopeMode.value === 'month'
        ? getMonthDateRange(store.currentYear, store.currentMonth)
        : null;
    const res = await api.listTransactions({
      book_id: store.activeBookId,
      start_date: range?.start,
      end_date: range?.end,
      page: 1,
      page_size: 500,
    });
    rawTransactions.value = res.items || [];
    const validSet = new Set(rawTransactions.value.map((t) => t.id));
    selectedIds.value = selectedIds.value.filter((id) => validSet.has(id));
  } catch {
    rawTransactions.value = store.transactions;
  } finally {
    loadingList.value = false;
  }
}

watch(
  () => [
    store.activeBookId,
    store.currentYear,
    store.currentMonth,
    scopeMode.value,
    store.transactionsTotal,
    store.transactions,
  ] as const,
  () => {
    fetchLedgerTransactions();
  },
  { immediate: true }
);

const categoryOptions = computed<DropdownOption[]>(() => {
  const expenseOpts = store.expenseCategories.map((c) => ({
    value: c.id,
    label: c.name,
    dot: c.color || '#D6553E',
  }));
  const incomeOpts = store.incomeCategories.map((c) => ({
    value: c.id,
    label: `${c.name}（收）`,
    dot: c.color || '#3E8E63',
  }));
  return [{ value: 'all', label: '全部分类' }, ...expenseOpts, ...incomeOpts];
});

const walletOptions = computed<DropdownOption[]>(() => [
  { value: 'all', label: '全部账户' },
  ...store.wallets.map((w) => ({
    value: w.id,
    label: w.is_archived ? `${w.name}（已归档）` : w.name,
    dot: w.color || '#DE8A1E',
  })),
]);

const filteredRows = computed<Transaction[]>(() => {
  const q = searchQuery.value.trim().toLowerCase();
  return [...rawTransactions.value]
    .filter((tx) => {
      if (filterType.value !== 'all' && tx.type !== filterType.value) return false;
      if (filterCatId.value !== 'all' && tx.category_id !== filterCatId.value) return false;
      if (filterWalletId.value !== 'all' && tx.wallet_id !== filterWalletId.value) return false;
      if (onlyReimburse.value && !tx.reimburse) return false;
      if (!q) return true;
      const haystack = [
        tx.title,
        tx.category_name,
        tx.sub_category,
        tx.note,
        tx.wallet_name,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(q);
    })
    .sort((a, b) => {
      const dateCmp = (b.date || '').localeCompare(a.date || '');
      if (dateCmp !== 0) return dateCmp;
      return (b.tx_time || '').localeCompare(a.tx_time || '');
    });
});

const filteredTotals = computed(() => {
  let expense = 0;
  let income = 0;
  for (const tx of filteredRows.value) {
    if (tx.type === 'expense') expense += tx.amount || 0;
    else if (tx.type === 'income') income += tx.amount || 0;
  }
  return { expense, income, count: filteredRows.value.length };
});

const groupedRows = computed(() => {
  const map = new Map<
    string,
    { date: string; label: string; expense: number; income: number; items: Transaction[] }
  >();
  for (const tx of filteredRows.value) {
    const key = (tx.date || '').slice(0, 10) || '未知日期';
    if (!map.has(key)) {
      map.set(key, {
        date: key,
        label: getWeekdayLabel(key),
        expense: 0,
        income: 0,
        items: [],
      });
    }
    const group = map.get(key)!;
    if (tx.type === 'expense') group.expense += tx.amount || 0;
    else if (tx.type === 'income') group.income += tx.amount || 0;
    group.items.push(tx);
  }
  return Array.from(map.values());
});

const isAllSelected = computed<boolean>(
  () =>
    filteredRows.value.length > 0 &&
    filteredRows.value.every((tx) => selectedIds.value.includes(tx.id))
);

function toggleSelectAll() {
  confirmBatchDelete.value = false;
  if (isAllSelected.value) {
    selectedIds.value = [];
  } else {
    selectedIds.value = filteredRows.value.map((tx) => tx.id);
  }
}

function toggleSelectRow(id: string) {
  confirmBatchDelete.value = false;
  const idx = selectedIds.value.indexOf(id);
  if (idx >= 0) {
    selectedIds.value.splice(idx, 1);
  } else {
    selectedIds.value.push(id);
  }
}

function resetFilters() {
  searchQuery.value = '';
  filterType.value = 'all';
  filterCatId.value = 'all';
  filterWalletId.value = 'all';
  onlyReimburse.value = false;
}

async function handleBatchDelete() {
  if (!selectedIds.value.length || batchDeleting.value) return;
  if (!confirmBatchDelete.value) {
    confirmBatchDelete.value = true;
    return;
  }

  batchDeleting.value = true;
  try {
    const deletedCount = await store.batchDeleteTransactions([...selectedIds.value]);
    selectedIds.value = [];
    confirmBatchDelete.value = false;
    await fetchLedgerTransactions();
    store.showToast(`已批量删除 ${deletedCount} 条流水`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '批量删除失败', 'error');
  } finally {
    batchDeleting.value = false;
  }
}

async function handleDeleteSingle(tx: Transaction) {
  try {
    await store.deleteTransaction(tx.id);
    await fetchLedgerTransactions();
    store.showToast('已删除该笔流水', 'success');
  } catch (err: any) {
    store.showToast(err?.message || '删除失败', 'error');
  }
}
</script>

<template>
  <div class="space-y-4">
    <!-- 顶部多维筛选工具条 -->
    <section class="pc-card p-4 space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2.5">
        <div class="flex flex-wrap items-center gap-2 flex-1 min-w-[260px]">
          <div class="relative w-full sm:w-60">
            <svg
              class="w-3.5 h-3.5 text-[#94A3B8] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <circle cx="11" cy="11" r="7" />
              <path d="M20 20l-3.5-3.5" />
            </svg>
            <input
              v-model="searchQuery"
              type="text"
              placeholder="搜索商户、分类或备注…"
              class="w-full h-9 text-xs bg-[#F4F5F8] border border-[#EAECEF] rounded-xl pl-8 pr-3 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
            />
          </div>

          <CustomDropdown
            v-model="filterCatId"
            :options="categoryOptions"
            width-class="w-36"
          />

          <CustomDropdown
            v-model="filterWalletId"
            :options="walletOptions"
            width-class="w-36"
          />

          <button
            type="button"
            class="h-9 px-3 rounded-xl text-xs font-semibold border transition"
            :class="
              onlyReimburse
                ? 'bg-[#FEF6E7] border-[#DE8A1E]/50 text-[#D9822B]'
                : 'bg-[#F4F5F8] border-[#EAECEF] text-[#64748B] hover:text-[#18181B]'
            "
            @click="onlyReimburse = !onlyReimburse"
          >
            仅看可报销
          </button>

          <button
            v-if="
              searchQuery ||
              filterType !== 'all' ||
              filterCatId !== 'all' ||
              filterWalletId !== 'all' ||
              onlyReimburse
            "
            type="button"
            class="h-9 px-2.5 text-xs text-[#64748B] hover:text-[#18181B]"
            @click="resetFilters"
          >
            重置筛选
          </button>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <div class="p-0.5 rounded-xl bg-[#F4F5F8] border border-[#EAECEF] inline-flex text-xs font-semibold">
            <button
              type="button"
              class="px-3 py-1 rounded-lg transition"
              :class="viewStyle === 'table' ? 'bg-white text-[#18181B] shadow-xs' : 'text-[#64748B]'"
              @click="viewStyle = 'table'"
            >
              表格
            </button>
            <button
              type="button"
              class="px-3 py-1 rounded-lg transition"
              :class="viewStyle === 'grouped' ? 'bg-white text-[#18181B] shadow-xs' : 'text-[#64748B]'"
              @click="viewStyle = 'grouped'"
            >
              按日
            </button>
          </div>

          <button
            type="button"
            class="h-9 px-3.5 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] text-xs font-semibold text-[#18181B] transition"
            @click="emit('open-import-export')"
          >
            导入 / 导出
          </button>
        </div>
      </div>

      <!-- 第二行：范围、类型切换与统计汇总 -->
      <div class="flex flex-wrap items-center justify-between gap-2 pt-2 border-t border-[#F1F3F6] text-xs">
        <div class="flex flex-wrap items-center gap-2">
          <div class="p-0.5 rounded-xl bg-[#F4F5F8] inline-flex font-semibold">
            <button
              type="button"
              class="px-3 py-1 rounded-lg transition"
              :class="scopeMode === 'month' ? 'bg-white text-[#DE8A1E] shadow-xs' : 'text-[#64748B]'"
              @click="scopeMode = 'month'"
            >
              {{ store.currentYear }}年{{ store.currentMonth }}月
            </button>
            <button
              type="button"
              class="px-3 py-1 rounded-lg transition"
              :class="scopeMode === 'all' ? 'bg-white text-[#18181B] shadow-xs' : 'text-[#64748B]'"
              @click="scopeMode = 'all'"
            >
              全部日期
            </button>
          </div>

          <div class="p-0.5 rounded-xl bg-[#F4F5F8] inline-flex font-semibold">
            <button
              v-for="tab in [
                { id: 'all', label: '全部' },
                { id: 'expense', label: '支出' },
                { id: 'income', label: '收入' },
                { id: 'transfer', label: '转账' },
              ]"
              :key="tab.id"
              type="button"
              class="px-3 py-1 rounded-lg transition"
              :class="
                filterType === tab.id
                  ? 'bg-white text-[#18181B] shadow-xs'
                  : 'text-[#64748B] hover:text-[#18181B]'
              "
              @click="filterType = tab.id as any"
            >
              {{ tab.label }}
            </button>
          </div>
        </div>

        <div class="flex items-center gap-3 text-[11px] text-[#64748B] tabular-nums">
          <span>共 {{ filteredTotals.count }} 笔</span>
          <span class="text-[#D6553E] font-semibold">
            支 {{ formatMoney(filteredTotals.expense, { privacy: store.privacyHidden }) }}
          </span>
          <span class="text-[#3E8E63] font-semibold">
            收 {{ formatMoney(filteredTotals.income, { privacy: store.privacyHidden }) }}
          </span>
        </div>
      </div>

      <!-- 批量删除工具栏 -->
      <div
        v-if="selectedIds.length > 0"
        class="px-3.5 py-2.5 rounded-xl bg-[#FFF6E8] border border-[#F2DFBE] flex flex-wrap items-center justify-between gap-2 text-xs"
      >
        <div class="flex items-center gap-2.5">
          <span class="font-bold text-[#18181B]">
            已勾选 {{ selectedIds.length }} 条流水
          </span>
          <button
            type="button"
            class="text-[#64748B] hover:text-[#18181B] underline"
            @click="selectedIds = []; confirmBatchDelete = false"
          >
            取消选择
          </button>
        </div>
        <div class="flex items-center gap-2">
          <button
            v-if="confirmBatchDelete"
            type="button"
            class="px-3 py-1.5 rounded-lg bg-white text-[#64748B] font-semibold"
            @click="confirmBatchDelete = false"
          >
            先不删
          </button>
          <button
            type="button"
            :disabled="batchDeleting"
            class="px-3.5 py-1.5 rounded-lg font-semibold transition"
            :class="
              confirmBatchDelete
                ? 'bg-[#D6553E] text-white'
                : 'bg-[#FDF0EE] text-[#D6553E] hover:bg-[#FADCD7]'
            "
            @click="handleBatchDelete"
          >
            {{
              batchDeleting
                ? '删除中…'
                : confirmBatchDelete
                  ? `确认永久删除 (${selectedIds.length})`
                  : `批量删除 (${selectedIds.length})`
            }}
          </button>
        </div>
      </div>
    </section>

    <!-- 空状态 -->
    <section
      v-if="!loadingList && filteredRows.length === 0"
      class="pc-card py-14 text-center space-y-3"
    >
      <p class="text-sm font-semibold text-[#475569]">没有匹配的流水记录</p>
      <p class="text-xs text-[#94A3B8]">可尝试调整筛选条件，或点击下方按钮快速记一笔</p>
      <button
        type="button"
        class="btn-amber px-4 py-2 rounded-xl text-white text-xs font-semibold"
        @click="emit('open-record', { mode: 'manual' })"
      >
        + 手动记一笔
      </button>
    </section>

    <!-- 视图一：专业表格清单 -->
    <section v-else-if="viewStyle === 'table'" class="pc-card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="bg-[#FAFBFD] text-[#94A3B8] border-b border-[#F1F3F6]">
              <th class="py-3 px-4 w-10">
                <input
                  type="checkbox"
                  :checked="isAllSelected"
                  class="accent-[#DE8A1E] cursor-pointer"
                  aria-label="全选"
                  @change="toggleSelectAll"
                />
              </th>
              <th class="py-3 px-3 font-semibold">日期时间</th>
              <th class="py-3 px-3 font-semibold">名称 / 备注</th>
              <th class="py-3 px-3 font-semibold">一级 · 二级分类</th>
              <th class="py-3 px-3 font-semibold">账户</th>
              <th class="py-3 px-3 font-semibold text-right">金额</th>
              <th class="py-3 px-4 font-semibold text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[#F1F3F6]">
            <tr
              v-for="tx in filteredRows"
              :key="tx.id"
              class="hover:bg-[#FAFBFD] transition"
            >
              <td class="py-3 px-4">
                <input
                  type="checkbox"
                  :checked="selectedIds.includes(tx.id)"
                  class="accent-[#DE8A1E] cursor-pointer"
                  @change="toggleSelectRow(tx.id)"
                />
              </td>
              <td class="py-3 px-3 text-[#64748B] tabular-nums whitespace-nowrap">
                <p>{{ tx.date }}</p>
                <p v-if="tx.tx_time" class="text-[10px] text-[#94A3B8]">{{ tx.tx_time }}</p>
              </td>
              <td class="py-3 px-3">
                <div class="flex items-center gap-2.5 min-w-0">
                  <CategoryBadge
                    :icon="tx.category_icon || (tx.type === 'transfer' ? 'refund' : 'tag')"
                    :color="tx.category_color || (tx.type === 'income' ? '#3E8E63' : '#64748B')"
                    :bg-color="tx.category_bg_color || (tx.type === 'income' ? '#ECF6F0' : '#F1F5F9')"
                    size="sm"
                  />
                  <div class="min-w-0">
                    <div class="flex items-center gap-1.5">
                      <p class="font-semibold text-[#18181B] truncate max-w-[200px]">
                        {{ tx.title || tx.category_name || '未命名账单' }}
                      </p>
                      <span
                        v-if="tx.reimburse"
                        class="text-[9px] px-1.5 py-0.5 rounded bg-[#FEF6E7] text-[#D9822B] font-semibold shrink-0"
                      >
                        可报销
                      </span>
                    </div>
                    <p v-if="tx.note" class="text-[10px] text-[#94A3B8] truncate max-w-[240px]">
                      {{ tx.note }}
                    </p>
                  </div>
                </div>
              </td>
              <td class="py-3 px-3 whitespace-nowrap">
                <span
                  class="px-2 py-0.5 rounded-full text-[11px] font-medium"
                  :style="{
                    backgroundColor: tx.category_bg_color || '#F4F5F8',
                    color: tx.category_color || '#475569',
                  }"
                >
                  {{ tx.type === 'transfer' ? '账户互转' : tx.category_name || '未分类'
                  }}{{ tx.sub_category ? ` · ${tx.sub_category}` : '' }}
                </span>
              </td>
              <td class="py-3 px-3 text-[#64748B] whitespace-nowrap">
                {{ tx.wallet_name || '默认账户' }}
                <span v-if="tx.to_wallet_name"> → {{ tx.to_wallet_name }}</span>
              </td>
              <td
                class="py-3 px-3 text-right font-bold tabular-nums whitespace-nowrap"
                :class="
                  tx.type === 'income'
                    ? 'text-[#3E8E63]'
                    : tx.type === 'transfer'
                      ? 'text-[#475569]'
                      : 'text-[#D6553E]'
                "
              >
                {{
                  tx.type === 'transfer'
                    ? formatMoney(tx.amount, { privacy: store.privacyHidden })
                    : formatMoney(tx.amount, {
                        privacy: store.privacyHidden,
                        showSign: true,
                        type: tx.type,
                      })
                }}
              </td>
              <td class="py-3 px-4 text-right whitespace-nowrap space-x-2">
                <button
                  type="button"
                  class="text-[11px] font-semibold text-[#DE8A1E] hover:underline"
                  @click="emit('edit-tx', tx)"
                >
                  编辑
                </button>
                <button
                  type="button"
                  class="text-[11px] font-semibold text-[#94A3B8] hover:text-[#D6553E]"
                  @click="handleDeleteSingle(tx)"
                >
                  删除
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- 视图二：按日分组清单 -->
    <section v-else class="space-y-3.5">
      <div
        v-for="group in groupedRows"
        :key="group.date"
        class="pc-card overflow-hidden"
      >
        <div class="px-5 py-2.5 bg-[#FAFBFD] border-b border-[#F1F3F6] flex items-center justify-between text-xs">
          <span class="font-bold text-[#18181B]">{{ group.label }}</span>
          <div class="flex items-center gap-3 tabular-nums text-[11px]">
            <span v-if="group.expense > 0" class="text-[#D6553E] font-semibold">
              支 {{ formatMoney(group.expense, { privacy: store.privacyHidden }) }}
            </span>
            <span v-if="group.income > 0" class="text-[#3E8E63] font-semibold">
              收 {{ formatMoney(group.income, { privacy: store.privacyHidden }) }}
            </span>
          </div>
        </div>

        <div class="divide-y divide-[#F1F3F6]">
          <div
            v-for="tx in group.items"
            :key="tx.id"
            class="px-4 py-3 flex items-center justify-between gap-3 hover:bg-[#FAFBFD] transition"
          >
            <div class="flex items-center gap-3 min-w-0">
              <input
                type="checkbox"
                :checked="selectedIds.includes(tx.id)"
                class="accent-[#DE8A1E] cursor-pointer shrink-0"
                @change="toggleSelectRow(tx.id)"
              />
              <CategoryBadge
                :icon="tx.category_icon || (tx.type === 'transfer' ? 'refund' : 'tag')"
                :color="tx.category_color || (tx.type === 'income' ? '#3E8E63' : '#64748B')"
                :bg-color="tx.category_bg_color || (tx.type === 'income' ? '#ECF6F0' : '#F1F5F9')"
                size="sm"
              />
              <div class="min-w-0">
                <div class="flex items-center gap-1.5">
                  <p class="text-xs font-bold text-[#18181B] truncate">
                    {{ tx.title || tx.category_name || '未命名账单' }}
                  </p>
                  <span
                    v-if="tx.reimburse"
                    class="text-[9px] px-1.5 py-0.5 rounded bg-[#FEF6E7] text-[#D9822B] font-semibold shrink-0"
                  >
                    可报销
                  </span>
                </div>
                <p class="text-[11px] text-[#94A3B8] truncate mt-0.5">
                  {{ tx.category_name || '未分类'
                  }}{{ tx.sub_category ? ` · ${tx.sub_category}` : '' }}
                  · {{ tx.wallet_name || '默认账户' }}
                  {{ tx.tx_time ? ` · ${tx.tx_time}` : '' }}
                  {{ tx.note ? ` · ${tx.note}` : '' }}
                </p>
              </div>
            </div>

            <div class="flex items-center gap-3 shrink-0">
              <span
                class="text-sm font-bold tabular-nums"
                :class="
                  tx.type === 'income'
                    ? 'text-[#3E8E63]'
                    : tx.type === 'transfer'
                      ? 'text-[#475569]'
                      : 'text-[#D6553E]'
                "
              >
                {{
                  tx.type === 'transfer'
                    ? formatMoney(tx.amount, { privacy: store.privacyHidden })
                    : formatMoney(tx.amount, {
                        privacy: store.privacyHidden,
                        showSign: true,
                        type: tx.type,
                      })
                }}
              </span>
              <button
                type="button"
                class="text-[11px] font-semibold text-[#DE8A1E] hover:underline"
                @click="emit('edit-tx', tx)"
              >
                编辑
              </button>
              <button
                type="button"
                class="text-[11px] font-semibold text-[#94A3B8] hover:text-[#D6553E]"
                @click="handleDeleteSingle(tx)"
              >
                删除
              </button>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>
