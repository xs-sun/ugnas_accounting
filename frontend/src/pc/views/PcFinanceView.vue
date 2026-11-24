<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { CATEGORY_COLOR_PRESETS } from '@/constants/icons';
import { centsToYuan, yuanToCents, formatMoney, todayIso } from '@/utils/format';
import type { Wallet, WalletType, RecurringRule, CategoryBudgetItem } from '@/types';
import AccountNetWorthCard from '@/components/shared/AccountNetWorthCard.vue';
import BudgetProgressBar from '@/components/shared/BudgetProgressBar.vue';
import CategoryBadge from '@/components/shared/CategoryBadge.vue';
import CustomDropdown from '@/components/shared/CustomDropdown.vue';
import type { DropdownOption } from '@/components/shared/CustomDropdown.vue';
import ToggleSwitch from '@/components/shared/ToggleSwitch.vue';

const store = useAppStore();

const WALLET_TYPE_OPTIONS: Array<{ value: WalletType; label: string; group: string; color: string }> = [
  { value: 'wechat', label: '微信支付', group: '电子钱包', color: '#3E8E63' },
  { value: 'alipay', label: '支付宝', group: '电子钱包', color: '#2F78A8' },
  { value: 'debit', label: '储蓄卡', group: '银行卡', color: '#DE8A1E' },
  { value: 'credit', label: '信用卡 / 花呗', group: '信用负债', color: '#D6553E' },
  { value: 'cash', label: '现金钱包', group: '现金', color: '#64748B' },
  { value: 'investment', label: '投资理财', group: '投资账户', color: '#7E57A6' },
  { value: 'custom', label: '自定义账户', group: '其他账户', color: '#456799' },
];

const CYCLE_OPTIONS: DropdownOption[] = [
  { value: '每天', label: '每天' },
  { value: '工作日', label: '工作日（周一至五）' },
  { value: '每周', label: '每周' },
  { value: '每月', label: '每月' },
  { value: '每年', label: '每年' },
];

// ==================== 总预算与分类预算 ====================
const totalBudgetInput = ref('');
const savingTotalBudget = ref(false);
const editingCatBudgetId = ref<string | null>(null);
const editingCatBudgetYuan = ref('');
const savingCatBudgetId = ref<string | null>(null);

watch(
  () => store.budgets?.total_budget ?? store.activeBook?.budget ?? 0,
  (cents) => {
    totalBudgetInput.value = centsToYuan(cents).toFixed(0);
  },
  { immediate: true }
);

const budgetPercentage = computed(() => Math.round(store.budgets?.percentage ?? 0));
const budgetSpent = computed(() => store.budgets?.total_spent ?? 0);
const budgetLimit = computed(
  () => store.budgets?.total_budget ?? store.activeBook?.budget ?? 0
);

interface MergedCategoryBudget {
  category_id: string;
  category_name: string;
  category_icon: string;
  category_color: string;
  bg_color: string;
  limit_amount: number;
  spent_amount: number;
  percentage: number;
}

const mergedCategoryBudgets = computed<MergedCategoryBudget[]>(() => {
  const map = new Map<string, CategoryBudgetItem>();
  for (const item of store.budgets?.categories || []) {
    map.set(item.category_id, item);
  }
  const result: MergedCategoryBudget[] = [];
  for (const cat of store.expenseCategories) {
    const existing = map.get(cat.id);
    if (existing) {
      result.push({
        category_id: existing.category_id,
        category_name: existing.category_name || cat.name,
        category_icon: existing.category_icon || cat.icon,
        category_color: existing.category_color || cat.color,
        bg_color: existing.bg_color || cat.bg_color,
        limit_amount: existing.limit_amount,
        spent_amount: existing.spent_amount,
        percentage: existing.percentage,
      });
    } else {
      result.push({
        category_id: cat.id,
        category_name: cat.name,
        category_icon: cat.icon,
        category_color: cat.color,
        bg_color: cat.bg_color,
        limit_amount: 0,
        spent_amount: 0,
        percentage: 0,
      });
    }
  }
  return result;
});

async function saveTotalBudget() {
  if (!store.activeBookId || savingTotalBudget.value) return;
  const cents = Math.max(0, yuanToCents(totalBudgetInput.value));
  savingTotalBudget.value = true;
  try {
    await api.updateTotalBudget({ book_id: store.activeBookId, budget: cents });
    await Promise.all([
      store.refreshBudgets(store.activeBookId, store.currentYear, store.currentMonth),
      store.refreshBooks(),
    ]);
    store.showToast('月度总预算已更新', 'success');
  } catch (err: any) {
    store.showToast(err?.message || '更新总预算失败', 'error');
  } finally {
    savingTotalBudget.value = false;
  }
}

async function adjustCategoryBudget(item: MergedCategoryBudget, deltaYuan: number) {
  if (!store.activeBookId || savingCatBudgetId.value) return;
  const nextCents = Math.max(0, item.limit_amount + deltaYuan * 100);
  await commitCategoryBudget(item.category_id, nextCents);
}

function startEditCategoryBudget(item: MergedCategoryBudget) {
  editingCatBudgetId.value = item.category_id;
  editingCatBudgetYuan.value = centsToYuan(item.limit_amount).toFixed(0);
}

async function saveInlineCategoryBudget(catId: string) {
  const cents = Math.max(0, yuanToCents(editingCatBudgetYuan.value));
  editingCatBudgetId.value = null;
  await commitCategoryBudget(catId, cents);
}

async function commitCategoryBudget(categoryId: string, limitCents: number) {
  if (!store.activeBookId) return;
  savingCatBudgetId.value = categoryId;
  try {
    await api.updateCategoryBudget({
      book_id: store.activeBookId,
      category_id: categoryId,
      limit_amount: limitCents,
    });
    await store.refreshBudgets(store.activeBookId, store.currentYear, store.currentMonth);
    store.showToast('分类预算已更新', 'success');
  } catch (err: any) {
    store.showToast(err?.message || '更新分类预算失败', 'error');
  } finally {
    savingCatBudgetId.value = null;
  }
}

// ==================== 资产账户管理 ====================
const groupedWallets = computed(() => {
  const groups = new Map<string, Wallet[]>();
  for (const w of store.activeWallets) {
    const key = w.group_name || WALLET_TYPE_OPTIONS.find((t) => t.value === w.type)?.group || '其他账户';
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)!.push(w);
  }
  return Array.from(groups.entries()).map(([groupName, items]) => ({
    groupName,
    items,
  }));
});

const walletModalOpen = ref(false);
const editingWallet = ref<Wallet | null>(null);
const savingWallet = ref(false);
const walletError = ref('');

const walletForm = reactive({
  name: '',
  type: 'debit' as WalletType,
  groupName: '银行卡',
  color: '#DE8A1E',
  balanceYuan: '0.00',
  creditLimitYuan: '10000',
  billingDay: 1,
  repaymentDay: 20,
  cardLastFour: '',
  excludeNetAsset: false,
});

const walletTypeDropdownOpts = computed<DropdownOption[]>(() =>
  WALLET_TYPE_OPTIONS.map((t) => ({
    value: t.value,
    label: t.label,
    dot: t.color,
  }))
);

function openAddWallet() {
  editingWallet.value = null;
  walletError.value = '';
  walletForm.name = '';
  walletForm.type = 'debit';
  walletForm.groupName = '银行卡';
  walletForm.color = '#DE8A1E';
  walletForm.balanceYuan = '0.00';
  walletForm.creditLimitYuan = '10000';
  walletForm.billingDay = 1;
  walletForm.repaymentDay = 20;
  walletForm.cardLastFour = '';
  walletForm.excludeNetAsset = false;
  walletModalOpen.value = true;
}

function openEditWallet(w: Wallet) {
  editingWallet.value = w;
  walletError.value = '';
  walletForm.name = w.name;
  walletForm.type = w.type || 'custom';
  walletForm.groupName = w.group_name || '其他账户';
  walletForm.color = w.color || '#DE8A1E';
  walletForm.balanceYuan = centsToYuan(w.balance).toFixed(2);
  walletForm.creditLimitYuan = centsToYuan(w.credit_limit).toFixed(0);
  walletForm.billingDay = w.billing_day || 1;
  walletForm.repaymentDay = w.repayment_day || 20;
  walletForm.cardLastFour = w.card_last_four || '';
  walletForm.excludeNetAsset = Boolean(w.exclude_net_asset);
  walletModalOpen.value = true;
}

function onWalletTypeChange(val: string | number) {
  const nextType = String(val) as WalletType;
  walletForm.type = nextType;
  const preset = WALLET_TYPE_OPTIONS.find((t) => t.value === nextType);
  if (preset && !editingWallet.value) {
    walletForm.groupName = preset.group;
    walletForm.color = preset.color;
  }
}

async function handleSaveWallet() {
  if (savingWallet.value) return;
  walletError.value = '';
  const name = walletForm.name.trim();
  if (!name) {
    walletError.value = '请输入账户名称';
    return;
  }
  const lastFour = walletForm.cardLastFour.trim();
  if (lastFour && !/^\d{4}$/.test(lastFour)) {
    walletError.value = '卡号尾号须为 4 位数字或留空';
    return;
  }
  const billingDay = Math.min(31, Math.max(1, Number(walletForm.billingDay) || 1));
  const repaymentDay = Math.min(31, Math.max(1, Number(walletForm.repaymentDay) || 20));

  savingWallet.value = true;
  try {
    const payload: Partial<Wallet> = {
      book_id: store.activeBookId,
      name,
      type: walletForm.type,
      group_name: walletForm.groupName.trim() || '其他账户',
      color: walletForm.color,
      balance: yuanToCents(walletForm.balanceYuan),
      credit_limit: walletForm.type === 'credit' ? yuanToCents(walletForm.creditLimitYuan) : 0,
      billing_day: billingDay,
      repayment_day: repaymentDay,
      card_last_four: lastFour,
      exclude_net_asset: walletForm.excludeNetAsset,
    };

    if (editingWallet.value) {
      await api.updateWallet(editingWallet.value.id, payload);
      store.showToast(`已更新账户「${name}」`, 'success');
    } else {
      await api.createWallet(payload);
      store.showToast(`已创建账户「${name}」`, 'success');
    }

    await store.refreshWallets(store.activeBookId);
    walletModalOpen.value = false;
  } catch (err: any) {
    walletError.value = err?.message || '保存账户失败';
    store.showToast(walletError.value, 'error');
  } finally {
    savingWallet.value = false;
  }
}

async function handleDeleteWallet(w: Wallet) {
  if (store.activeWallets.length <= 1) {
    store.showToast('至少保留一个可用资产账户', 'error');
    return;
  }
  try {
    await api.deleteWallet(w.id);
    await store.refreshWallets(store.activeBookId);
    store.showToast(`已归档账户「${w.name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '归档账户失败', 'error');
  }
}

// ==================== 定时自动记账规则 ====================
const recModalOpen = ref(false);
const editingRule = ref<RecurringRule | null>(null);
const savingRule = ref(false);
const recError = ref('');

const recForm = reactive({
  type: 'expense' as 'expense' | 'income',
  title: '',
  amountYuan: '',
  cycle: '每月',
  startDate: todayIso(),
  categoryId: '',
  subCategory: '',
  walletId: '',
  excludeStats: false,
});

const recCategoryOptions = computed<DropdownOption[]>(() => {
  const list = recForm.type === 'income' ? store.incomeCategories : store.expenseCategories;
  return list.map((c) => ({
    value: c.id,
    label: c.name,
    dot: c.color || '#DE8A1E',
  }));
});

const recSubOptions = computed<DropdownOption[]>(() => {
  const list = recForm.type === 'income' ? store.incomeCategories : store.expenseCategories;
  const active = list.find((c) => c.id === recForm.categoryId);
  const subs = active?.children || [];
  return [{ value: '', label: '不指定二级分类' }, ...subs.map((s) => ({ value: s.name, label: s.name }))];
});

const recWalletOptions = computed<DropdownOption[]>(() =>
  store.activeWallets.map((w) => ({
    value: w.id,
    label: w.name,
    dot: w.color || '#DE8A1E',
  }))
);

function openAddRule() {
  editingRule.value = null;
  recError.value = '';
  recForm.type = 'expense';
  recForm.title = '';
  recForm.amountYuan = '';
  recForm.cycle = '每月';
  recForm.startDate = todayIso();
  const firstCat = store.expenseCategories[0];
  recForm.categoryId = firstCat?.id || '';
  recForm.subCategory = firstCat?.children?.[0]?.name || '';
  recForm.walletId = store.activeWallets[0]?.id || '';
  recForm.excludeStats = false;
  recModalOpen.value = true;
}

function openEditRule(rule: RecurringRule) {
  editingRule.value = rule;
  recError.value = '';
  recForm.type = rule.type === 'income' ? 'income' : 'expense';
  recForm.title = rule.title;
  recForm.amountYuan = centsToYuan(rule.amount).toFixed(2);
  recForm.cycle = rule.cycle || '每月';
  recForm.startDate = rule.start_date || todayIso();
  recForm.categoryId = rule.category_id || '';
  recForm.subCategory = rule.sub_category || '';
  recForm.walletId = rule.wallet_id || store.activeWallets[0]?.id || '';
  recForm.excludeStats = Boolean(rule.exclude_stats);
  recModalOpen.value = true;
}

function switchRecType(nextType: 'expense' | 'income') {
  if (recForm.type === nextType) return;
  recForm.type = nextType;
  const list = nextType === 'income' ? store.incomeCategories : store.expenseCategories;
  recForm.categoryId = list[0]?.id || '';
  recForm.subCategory = list[0]?.children?.[0]?.name || '';
}

function onRecCategoryChange(val: string | number) {
  const catId = String(val);
  recForm.categoryId = catId;
  const list = recForm.type === 'income' ? store.incomeCategories : store.expenseCategories;
  const found = list.find((c) => c.id === catId);
  recForm.subCategory = found?.children?.[0]?.name || '';
}

async function handleSaveRule() {
  if (savingRule.value) return;
  recError.value = '';
  if (!store.activeBookId) {
    recError.value = '请先选择账本';
    return;
  }
  const title = recForm.title.trim();
  const cents = yuanToCents(recForm.amountYuan);
  if (!title || cents <= 0) {
    recError.value = '请填写规则名称与大于 0 的金额';
    return;
  }
  const walletId = recForm.walletId || store.activeWallets[0]?.id || '';
  if (!walletId) {
    recError.value = '请先创建可用资产账户';
    return;
  }

  savingRule.value = true;
  try {
    if (editingRule.value) {
      await api.updateRecurring(editingRule.value.id, {
        type: recForm.type,
        category_id: recForm.categoryId,
        sub_category: recForm.subCategory,
        title,
        amount: cents,
        cycle: recForm.cycle,
        start_date: recForm.startDate,
        wallet_id: walletId,
        exclude_stats: recForm.excludeStats,
      });
      store.showToast(`已更新定时规则「${title}」`, 'success');
    } else {
      await api.createRecurring({
        book_id: store.activeBookId,
        type: recForm.type,
        category_id: recForm.categoryId,
        sub_category: recForm.subCategory,
        title,
        amount: cents,
        cycle: recForm.cycle,
        start_date: recForm.startDate,
        wallet_id: walletId,
        exclude_stats: recForm.excludeStats,
      });
      store.showToast(`已添加定时规则「${title}」`, 'success');
    }

    await store.refreshRecurring(store.activeBookId);
    recModalOpen.value = false;
  } catch (err: any) {
    recError.value = err?.message || '保存定时规则失败';
    store.showToast(recError.value, 'error');
  } finally {
    savingRule.value = false;
  }
}

async function handleToggleRule(rule: RecurringRule) {
  try {
    await api.toggleRecurring(rule.id);
    await store.refreshRecurring(store.activeBookId);
    store.showToast(
      rule.is_active ? `已暂停「${rule.title}」` : `已启用「${rule.title}」`,
      'info'
    );
  } catch (err: any) {
    store.showToast(err?.message || '切换规则状态失败', 'error');
  }
}

async function handleDeleteRule(rule: RecurringRule) {
  try {
    await api.deleteRecurring(rule.id);
    await store.refreshRecurring(store.activeBookId);
    store.showToast(`已删除规则「${rule.title}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '删除规则失败', 'error');
  }
}
</script>

<template>
  <div class="grid grid-cols-1 lg:grid-cols-12 gap-5 items-start">
    <!-- 左栏：净资产总览与账户网格 -->
    <section class="lg:col-span-4 space-y-4">
      <div class="pc-card p-5 space-y-4">
        <div class="flex items-center justify-between">
          <div>
            <h2 class="text-sm font-bold text-[#18181B]">资产与负债账户</h2>
            <p class="text-[11px] text-[#94A3B8] mt-0.5">
              共 {{ store.activeWallets.length }} 个活跃账户
            </p>
          </div>
          <button
            type="button"
            class="text-xs font-semibold text-[#DE8A1E] hover:underline"
            @click="openAddWallet"
          >
            + 添加账户
          </button>
        </div>

        <AccountNetWorthCard
          :summary="store.walletSummary"
          :privacy="store.privacyHidden"
          :currency="store.activeBook?.currency || 'CNY'"
        />

        <div v-if="groupedWallets.length === 0" class="py-8 text-center text-xs text-[#94A3B8]">
          暂无可用资产账户，点击右上角添加
        </div>

        <div v-else class="space-y-4">
          <div v-for="group in groupedWallets" :key="group.groupName" class="space-y-2">
            <p class="text-[10px] font-semibold uppercase tracking-wider text-[#94A3B8] px-1">
              {{ group.groupName }}
            </p>
            <div class="space-y-2">
              <div
                v-for="w in group.items"
                :key="w.id"
                class="p-3.5 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] hover:border-[#E2D5C0] transition space-y-2"
              >
                <div class="flex items-center justify-between gap-2">
                  <div class="flex items-center gap-2.5 min-w-0">
                    <span
                      class="w-8 h-8 rounded-xl text-xs font-bold text-white flex items-center justify-center shrink-0"
                      :style="{ backgroundColor: w.color || '#DE8A1E' }"
                    >
                      {{ w.name.slice(0, 1) }}
                    </span>
                    <div class="min-w-0">
                      <div class="flex items-center gap-1.5">
                        <p class="text-xs font-bold text-[#18181B] truncate">{{ w.name }}</p>
                        <span
                          v-if="w.card_last_four"
                          class="text-[10px] font-mono text-[#94A3B8] shrink-0"
                        >
                          *{{ w.card_last_four }}
                        </span>
                      </div>
                      <p class="text-[10px] text-[#94A3B8]">
                        {{ w.exclude_net_asset ? '不计入净资产' : '计入净资产' }}
                      </p>
                    </div>
                  </div>
                  <span
                    class="text-sm font-bold tabular-nums shrink-0"
                    :class="w.balance < 0 ? 'text-[#D6553E]' : 'text-[#18181B]'"
                  >
                    {{ formatMoney(w.balance, { privacy: store.privacyHidden }) }}
                  </span>
                </div>

                <div
                  v-if="w.type === 'credit'"
                  class="text-[10px] text-[#64748B] flex items-center justify-between pt-1.5 border-t border-[#EAECEF] tabular-nums"
                >
                  <span>账单日 {{ w.billing_day }}号 · 还款日 {{ w.repayment_day }}号</span>
                  <span>额度 {{ formatMoney(w.credit_limit, { privacy: store.privacyHidden, compact: true, decimals: 0 }) }}</span>
                </div>

                <div class="flex items-center justify-end gap-3 pt-1 text-[11px]">
                  <button
                    type="button"
                    class="font-semibold text-[#DE8A1E] hover:underline"
                    @click="openEditWallet(w)"
                  >
                    编辑
                  </button>
                  <button
                    type="button"
                    class="font-semibold text-[#94A3B8] hover:text-[#D6553E]"
                    @click="handleDeleteWallet(w)"
                  >
                    归档
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- 中栏：月度总预算与分分类预算 -->
    <section class="lg:col-span-4 space-y-4">
      <div class="pc-card p-5 space-y-4">
        <div>
          <h2 class="text-sm font-bold text-[#18181B]">月度总预算</h2>
          <p class="text-[11px] text-[#94A3B8] mt-0.5">
            {{ store.currentYear }}年{{ store.currentMonth }}月 · 在线修改即时生效
          </p>
        </div>

        <BudgetProgressBar
          :percentage="budgetPercentage"
          :spent="budgetSpent"
          :limit="budgetLimit"
          :privacy="store.privacyHidden"
          show-summary
          height-class="h-2.5"
        />

        <form class="flex items-center gap-2 pt-2 border-t border-[#F1F3F6]" @submit.prevent="saveTotalBudget">
          <div class="relative flex-1 min-w-0">
            <span class="absolute left-3 top-1/2 -translate-y-1/2 text-xs font-bold text-[#94A3B8]">¥</span>
            <input
              v-model="totalBudgetInput"
              type="number"
              min="0"
              step="100"
              class="w-full h-9 text-xs font-bold tabular-nums bg-[#F4F5F8] border border-[#EAECEF] rounded-xl pl-7 pr-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
            />
          </div>
          <button
            type="submit"
            :disabled="savingTotalBudget"
            class="h-9 px-3.5 rounded-xl bg-[#DE8A1E] hover:bg-[#C67612] text-white text-xs font-semibold shrink-0 disabled:opacity-60 transition"
          >
            {{ savingTotalBudget ? '保存中…' : '更新总预算' }}
          </button>
        </form>
      </div>

      <div class="pc-card p-5 space-y-3.5">
        <div class="flex items-center justify-between">
          <h2 class="text-sm font-bold text-[#18181B]">分类月预算</h2>
          <span class="text-[11px] text-[#94A3B8]">点 +/- 或金额直接编辑</span>
        </div>

        <div v-if="mergedCategoryBudgets.length === 0" class="py-8 text-center text-xs text-[#94A3B8]">
          暂无支出分类
        </div>
        <div v-else class="space-y-3 max-h-[540px] overflow-y-auto thin-scroll pr-1">
          <div
            v-for="item in mergedCategoryBudgets"
            :key="item.category_id"
            class="p-3 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] space-y-2"
          >
            <div class="flex items-center justify-between gap-2">
              <div class="flex items-center gap-2 min-w-0">
                <CategoryBadge
                  :icon="item.category_icon"
                  :color="item.category_color"
                  :bg-color="item.bg_color || '#FFF6E8'"
                  size="xs"
                />
                <div class="min-w-0">
                  <p class="text-xs font-bold text-[#18181B] truncate">{{ item.category_name }}</p>
                  <p class="text-[10px] text-[#64748B] tabular-nums">
                    已用 {{ formatMoney(item.spent_amount, { privacy: store.privacyHidden }) }} /
                    <button
                      v-if="editingCatBudgetId !== item.category_id"
                      type="button"
                      class="font-semibold text-[#DE8A1E] hover:underline"
                      @click="startEditCategoryBudget(item)"
                    >
                      {{
                        item.limit_amount > 0
                          ? formatMoney(item.limit_amount, { privacy: store.privacyHidden })
                          : '未设限额'
                      }}
                    </button>
                  </p>
                </div>
              </div>

              <div v-if="editingCatBudgetId === item.category_id" class="flex items-center gap-1 shrink-0">
                <input
                  v-model="editingCatBudgetYuan"
                  type="number"
                  min="0"
                  step="50"
                  class="w-20 h-7 text-xs font-bold tabular-nums bg-white border border-[#DE8A1E] rounded-lg px-2 outline-none"
                />
                <button
                  type="button"
                  class="h-7 px-2 rounded-lg bg-[#DE8A1E] text-white text-[11px] font-semibold"
                  @click="saveInlineCategoryBudget(item.category_id)"
                >
                  存
                </button>
              </div>
              <div v-else class="flex items-center gap-1 shrink-0">
                <button
                  type="button"
                  :disabled="savingCatBudgetId === item.category_id"
                  class="w-6 h-6 rounded-full bg-white border border-[#EAECEF] hover:border-[#DE8A1E] text-xs font-bold text-[#475569] disabled:opacity-50"
                  title="减少 ¥100"
                  @click="adjustCategoryBudget(item, -100)"
                >
                  −
                </button>
                <button
                  type="button"
                  :disabled="savingCatBudgetId === item.category_id"
                  class="w-6 h-6 rounded-full bg-white border border-[#EAECEF] hover:border-[#DE8A1E] text-xs font-bold text-[#475569] disabled:opacity-50"
                  title="增加 ¥100"
                  @click="adjustCategoryBudget(item, 100)"
                >
                  +
                </button>
              </div>
            </div>

            <BudgetProgressBar
              :percentage="item.limit_amount > 0 ? item.percentage : 0"
              :spent="item.spent_amount"
              :limit="item.limit_amount"
              :privacy="store.privacyHidden"
              :accent-color="item.category_color || '#DE8A1E'"
              height-class="h-1.5"
            />
          </div>
        </div>
      </div>
    </section>

    <!-- 右栏：定时自动记账规则 -->
    <section class="lg:col-span-4 space-y-4">
      <div class="pc-card p-5 space-y-3.5">
        <div class="flex items-center justify-between">
          <div>
            <h2 class="text-sm font-bold text-[#18181B]">定时自动记账</h2>
            <p class="text-[11px] text-[#94A3B8] mt-0.5">房租、薪资、固定订阅自动周期入账</p>
          </div>
          <button
            type="button"
            class="text-xs font-semibold text-[#DE8A1E] hover:underline shrink-0"
            @click="openAddRule"
          >
            + 新增规则
          </button>
        </div>

        <div v-if="store.recurringRules.length === 0" class="py-12 text-center space-y-2.5">
          <p class="text-xs text-[#94A3B8]">暂无定时自动记账规则</p>
          <button
            type="button"
            class="px-4 py-1.5 rounded-full bg-[#FFF6E8] text-[#DE8A1E] text-xs font-semibold hover:bg-[#FBEBD1]"
            @click="openAddRule"
          >
            + 添加第一条规则
          </button>
        </div>

        <div v-else class="space-y-2.5">
          <div
            v-for="r in store.recurringRules"
            :key="r.id"
            class="p-3.5 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] transition"
            :class="{ 'opacity-60': !r.is_active }"
          >
            <div class="flex items-center justify-between gap-2">
              <div class="flex items-center gap-2.5 min-w-0">
                <CategoryBadge
                  :icon="r.category_icon || 'tag'"
                  :color="r.category_color || (r.type === 'income' ? '#3E8E63' : '#DE8A1E')"
                  :bg-color="r.bg_color || '#FFF6E8'"
                  size="sm"
                />
                <div class="min-w-0">
                  <p class="text-xs font-bold text-[#18181B] truncate">{{ r.title }}</p>
                  <p class="text-[10px] text-[#94A3B8] truncate">
                    {{ r.cycle }} · {{ r.category_name || '未分类'
                    }}{{ r.sub_category ? ` · ${r.sub_category}` : '' }} · {{ r.wallet_name }}
                  </p>
                </div>
              </div>
              <span
                class="text-xs font-bold tabular-nums shrink-0"
                :class="r.type === 'income' ? 'text-[#3E8E63]' : 'text-[#D6553E]'"
              >
                {{ formatMoney(r.amount, { privacy: store.privacyHidden, showSign: true, type: r.type }) }}
              </span>
            </div>

            <div class="mt-2.5 pt-2 border-t border-[#EAECEF] flex items-center justify-between gap-2 text-[10px]">
              <span class="text-[#64748B] tabular-nums">
                下次 {{ r.next_date }} · 已执行 {{ r.executed_count }} 次
              </span>
              <div class="flex items-center gap-2.5 shrink-0">
                <button
                  type="button"
                  class="text-[11px] font-semibold text-[#DE8A1E] hover:underline"
                  @click="openEditRule(r)"
                >
                  编辑
                </button>
                <button
                  type="button"
                  class="text-[11px] font-semibold text-[#94A3B8] hover:text-[#D6553E]"
                  @click="handleDeleteRule(r)"
                >
                  删除
                </button>
                <ToggleSwitch
                  :model-value="r.is_active"
                  aria-label="启停定时记账规则"
                  @change="handleToggleRule(r)"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- 账户新增/编辑弹窗 -->
    <Teleport to="body">
      <div v-if="walletModalOpen" class="fixed inset-0 z-[70] flex items-center justify-center p-4">
        <div
          class="absolute inset-0 bg-[#0F172A]/40 backdrop-blur-[2px]"
          @click="walletModalOpen = false"
        />
        <div class="relative w-full max-w-[460px] bg-white rounded-[24px] border border-[#EAECEF] shadow-2xl overflow-hidden">
          <div class="px-5 py-4 border-b border-[#F1F3F6] flex items-center justify-between">
            <h3 class="text-sm font-bold text-[#18181B]">
              {{ editingWallet ? '编辑资产账户' : '添加资产账户' }}
            </h3>
            <button
              type="button"
              class="w-7 h-7 rounded-full hover:bg-[#F4F5F8] flex items-center justify-center text-[#64748B]"
              @click="walletModalOpen = false"
            >
              ×
            </button>
          </div>

          <form class="p-5 space-y-3.5 text-xs" @submit.prevent="handleSaveWallet">
            <div
              v-if="walletError"
              class="px-3 py-2 rounded-xl bg-[#FDF0EE] border border-[#F5C4BC] text-[#D6553E] font-medium"
            >
              {{ walletError }}
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">账户名称</label>
                <input
                  v-model="walletForm.name"
                  type="text"
                  required
                  maxlength="20"
                  placeholder="如：招商银行卡"
                  class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
                />
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">账户类型</label>
                <CustomDropdown
                  :model-value="walletForm.type"
                  :options="walletTypeDropdownOpts"
                  @change="onWalletTypeChange"
                />
              </div>
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">所属分组</label>
                <input
                  v-model="walletForm.groupName"
                  type="text"
                  maxlength="12"
                  placeholder="如：电子钱包 / 银行卡"
                  class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
                />
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">
                  {{ walletForm.type === 'credit' ? '当前余额 / 欠款填负（元）' : '当前余额（元）' }}
                </label>
                <input
                  v-model="walletForm.balanceYuan"
                  type="number"
                  step="0.01"
                  required
                  class="w-full h-9 font-bold tabular-nums bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
                />
              </div>
            </div>

            <div v-if="walletForm.type === 'credit'" class="grid grid-cols-2 gap-3 p-3 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF]">
              <div>
                <label class="block text-[10px] text-[#64748B] mb-1">授信额度（元）</label>
                <input
                  v-model="walletForm.creditLimitYuan"
                  type="number"
                  step="100"
                  min="0"
                  class="w-full h-8 bg-white border border-[#EAECEF] rounded-xl px-2.5 outline-none"
                />
              </div>
              <div>
                <label class="block text-[10px] text-[#64748B] mb-1">卡号尾号（4位）</label>
                <input
                  v-model="walletForm.cardLastFour"
                  type="text"
                  maxlength="4"
                  placeholder="如 8848"
                  class="w-full h-8 font-mono bg-white border border-[#EAECEF] rounded-xl px-2.5 outline-none"
                />
              </div>
              <div>
                <label class="block text-[10px] text-[#64748B] mb-1">账单日（1-31）</label>
                <input
                  v-model.number="walletForm.billingDay"
                  type="number"
                  min="1"
                  max="31"
                  class="w-full h-8 bg-white border border-[#EAECEF] rounded-xl px-2.5 outline-none"
                />
              </div>
              <div>
                <label class="block text-[10px] text-[#64748B] mb-1">还款日（1-31）</label>
                <input
                  v-model.number="walletForm.repaymentDay"
                  type="number"
                  min="1"
                  max="31"
                  class="w-full h-8 bg-white border border-[#EAECEF] rounded-xl px-2.5 outline-none"
                />
              </div>
            </div>

            <div>
              <label class="block text-[11px] font-semibold text-[#64748B] mb-1.5">账户主题色</label>
              <div class="flex flex-wrap gap-2">
                <button
                  v-for="preset in CATEGORY_COLOR_PRESETS"
                  :key="preset.color"
                  type="button"
                  class="w-7 h-7 rounded-full transition"
                  :class="walletForm.color === preset.color ? 'ring-2 ring-offset-2 ring-[#18181B]' : ''"
                  :style="{ backgroundColor: preset.color }"
                  :title="preset.label"
                  @click="walletForm.color = preset.color"
                />
              </div>
            </div>

            <div class="flex items-center justify-between px-3 py-2.5 rounded-xl bg-[#FAFBFD] border border-[#EAECEF]">
              <div>
                <p class="font-semibold text-[#18181B]">不计入净资产统计</p>
                <p class="text-[10px] text-[#94A3B8]">开启后该账户余额不参与总资产与净资产计算</p>
              </div>
              <ToggleSwitch v-model="walletForm.excludeNetAsset" aria-label="不计入净资产统计" />
            </div>

            <div class="pt-2 border-t border-[#F1F3F6] flex justify-end gap-2">
              <button
                type="button"
                class="px-4 py-2 rounded-xl bg-[#F4F5F8] text-[#475569] font-semibold"
                @click="walletModalOpen = false"
              >
                取消
              </button>
              <button
                type="submit"
                :disabled="savingWallet"
                class="btn-amber px-5 py-2 rounded-xl text-white font-semibold disabled:opacity-60"
              >
                {{ savingWallet ? '保存中…' : '保存账户' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>

    <!-- 定时记账新增/编辑弹窗 -->
    <Teleport to="body">
      <div v-if="recModalOpen" class="fixed inset-0 z-[70] flex items-center justify-center p-4">
        <div
          class="absolute inset-0 bg-[#0F172A]/40 backdrop-blur-[2px]"
          @click="recModalOpen = false"
        />
        <div class="relative w-full max-w-[460px] bg-white rounded-[24px] border border-[#EAECEF] shadow-2xl overflow-hidden">
          <div class="px-5 py-4 border-b border-[#F1F3F6] flex items-center justify-between">
            <h3 class="text-sm font-bold text-[#18181B]">
              {{ editingRule ? '编辑定时记账规则' : '新增定时记账规则' }}
            </h3>
            <button
              type="button"
              class="w-7 h-7 rounded-full hover:bg-[#F4F5F8] flex items-center justify-center text-[#64748B]"
              @click="recModalOpen = false"
            >
              ×
            </button>
          </div>

          <form class="p-5 space-y-3.5 text-xs" @submit.prevent="handleSaveRule">
            <div
              v-if="recError"
              class="px-3 py-2 rounded-xl bg-[#FDF0EE] border border-[#F5C4BC] text-[#D6553E] font-medium"
            >
              {{ recError }}
            </div>

            <div class="p-0.5 rounded-full bg-[#F4F5F8] inline-flex font-semibold">
              <button
                type="button"
                class="px-4 py-1 rounded-full transition"
                :class="
                  recForm.type === 'expense' ? 'bg-white text-[#D6553E] shadow-xs' : 'text-[#64748B]'
                "
                @click="switchRecType('expense')"
              >
                定时支出
              </button>
              <button
                type="button"
                class="px-4 py-1 rounded-full transition"
                :class="
                  recForm.type === 'income' ? 'bg-white text-[#3E8E63] shadow-xs' : 'text-[#64748B]'
                "
                @click="switchRecType('income')"
              >
                定时收入
              </button>
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">规则名称</label>
                <input
                  v-model="recForm.title"
                  type="text"
                  required
                  maxlength="24"
                  placeholder="如：房租与物业"
                  class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
                />
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">金额（元）</label>
                <input
                  v-model="recForm.amountYuan"
                  type="number"
                  step="0.01"
                  min="0.01"
                  required
                  placeholder="0.00"
                  class="w-full h-9 font-bold tabular-nums bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
                />
              </div>
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">执行周期</label>
                <CustomDropdown v-model="recForm.cycle" :options="CYCLE_OPTIONS" />
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">起始日期</label>
                <input
                  v-model="recForm.startDate"
                  type="date"
                  required
                  class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
                />
              </div>
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">一级分类</label>
                <CustomDropdown
                  :model-value="recForm.categoryId"
                  :options="recCategoryOptions"
                  @change="onRecCategoryChange"
                />
              </div>
              <div>
                <label class="block text-[11px] font-semibold text-[#64748B] mb-1">二级分类</label>
                <CustomDropdown v-model="recForm.subCategory" :options="recSubOptions" />
              </div>
            </div>

            <div>
              <label class="block text-[11px] font-semibold text-[#64748B] mb-1">关联账户</label>
              <CustomDropdown v-model="recForm.walletId" :options="recWalletOptions" />
            </div>

            <div class="flex items-center justify-between px-3 py-2 rounded-xl bg-[#FAFBFD] border border-[#EAECEF]">
              <div>
                <p class="font-semibold text-[#18181B]">不计入预算统计</p>
                <p class="text-[10px] text-[#94A3B8]">适合周期性内部资金划转</p>
              </div>
              <ToggleSwitch v-model="recForm.excludeStats" aria-label="不计入预算统计" />
            </div>

            <div class="pt-2 border-t border-[#F1F3F6] flex justify-end gap-2">
              <button
                type="button"
                class="px-4 py-2 rounded-xl bg-[#F4F5F8] text-[#475569] font-semibold"
                @click="recModalOpen = false"
              >
                取消
              </button>
              <button
                type="submit"
                :disabled="savingRule"
                class="btn-amber px-5 py-2 rounded-xl text-white font-semibold disabled:opacity-60"
              >
                {{ savingRule ? '保存中…' : '保存规则' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>
  </div>
</template>
