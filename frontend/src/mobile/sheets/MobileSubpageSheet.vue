<script setup lang="ts">
import { ref, reactive, computed, watch, onBeforeUnmount } from 'vue';
import { api, getAuthHeaders } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import {
  PICKABLE_ICONS,
  CATEGORY_COLOR_PRESETS,
  LEDGER_COLOR_PRESETS,
  getIconSvg,
} from '@/constants/icons';
import { centsToYuan, yuanToCents, formatMoney, todayIso } from '@/utils/format';
import type {
  Category,
  CategoryBudgetItem,
  CategoryTreeNode,
  ImportedItem,
  RecurringRule,
  SuishoujiColumnTarget,
  SuishoujiCommitResult,
  SuishoujiParseResult,
  SuishoujiPreviewRow,
  Wallet,
  WalletType,
} from '@/types';
import { SUISHOUJI_TARGET_OPTIONS } from '@/types';
import type { SubpageType } from '../types';
import BottomSheet from '../components/BottomSheet.vue';
import AccountNetWorthCard from '@/components/shared/AccountNetWorthCard.vue';
import BudgetProgressBar from '@/components/shared/BudgetProgressBar.vue';
import CategoryBadge from '@/components/shared/CategoryBadge.vue';
import CategoryPicker from '@/components/shared/CategoryPicker.vue';
import CustomDropdown, { type DropdownOption } from '@/components/shared/CustomDropdown.vue';
import ToggleSwitch from '@/components/shared/ToggleSwitch.vue';
import VerificationCodeField, { type CodeDeliveryHint } from '@/components/shared/VerificationCodeField.vue';

const props = defineProps<{
  open: boolean;
  page: SubpageType;
}>();

const emit = defineEmits<{
  close: [];
  'update:open': [val: boolean];
}>();

const store = useAppStore();

// ==================== 1. 账本新建 / 编辑 ====================
const ledgerMode = ref<'create' | 'edit'>('create');
const ledgerName = ref('');
const ledgerBudgetYuan = ref('3000');
const ledgerCurrency = ref('CNY');
const ledgerDesc = ref('');
const ledgerColor = ref(LEDGER_COLOR_PRESETS[0]);
const ledgerMemberMode = ref<'personal' | 'shared'>('personal');
const ledgerIsDefault = ref(false);
const savingLedger = ref(false);
const confirmDeleteLedger = ref(false);

const CURRENCY_OPTIONS: DropdownOption[] = [
  { value: 'CNY', label: '人民币 (CNY ¥)' },
  { value: 'USD', label: '美元 (USD $)' },
  { value: 'HKD', label: '港币 (HKD $)' },
  { value: 'JPY', label: '日元 (JPY ¥)' },
  { value: 'EUR', label: '欧元 (EUR €)' },
];

function initLedgerForm(mode: 'create' | 'edit') {
  ledgerMode.value = mode;
  confirmDeleteLedger.value = false;
  if (mode === 'edit' && store.activeBook) {
    const b = store.activeBook;
    ledgerName.value = b.name;
    ledgerBudgetYuan.value = b.budget > 0 ? String(Math.round(centsToYuan(b.budget))) : '3000';
    ledgerCurrency.value = b.currency || 'CNY';
    ledgerDesc.value = b.description || '';
    ledgerColor.value = b.color || LEDGER_COLOR_PRESETS[0];
    ledgerMemberMode.value = (b.members?.length || 1) > 1 ? 'shared' : 'personal';
    ledgerIsDefault.value = Boolean(b.is_default);
  } else {
    ledgerName.value = '';
    ledgerBudgetYuan.value = '3000';
    ledgerCurrency.value = 'CNY';
    ledgerDesc.value = '';
    ledgerColor.value = LEDGER_COLOR_PRESETS[0];
    ledgerMemberMode.value = 'personal';
    ledgerIsDefault.value = false;
  }
}

async function submitLedgerForm() {
  const name = ledgerName.value.trim();
  if (!name) {
    store.showToast('请输入账本名称', 'error');
    return;
  }
  if (savingLedger.value) return;

  const budgetParsed = parseFloat(ledgerBudgetYuan.value);
  const budgetCents =
    Number.isFinite(budgetParsed) && budgetParsed >= 0 ? yuanToCents(budgetParsed) : 0;
  const fallbackDesc =
    ledgerMemberMode.value === 'shared'
      ? '家庭/多人共享 · 支持邀请成员共同记账'
      : '个人日常核算 · 独立收支追踪';
  const desc = ledgerDesc.value.trim() || fallbackDesc;

  savingLedger.value = true;
  try {
    if (ledgerMode.value === 'edit' && store.activeBook) {
      const targetBookId = store.activeBook.id;
      const wasDefault = Boolean(store.activeBook.is_default);
      await api.updateBook(targetBookId, {
        name,
        currency: ledgerCurrency.value,
        description: desc,
        color: ledgerColor.value,
        budget: budgetCents,
      });
      if (ledgerIsDefault.value && !wasDefault) {
        await store.setDefaultBook(targetBookId);
      } else if (!ledgerIsDefault.value && wasDefault) {
        await store.clearDefaultBook();
      }
      await Promise.allSettled([store.refreshBooks(), store.refreshBudgets()]);
      store.showToast(`已保存账本「${name}」`, 'success');
    } else {
      const created = await api.createBook({
        name,
        currency: ledgerCurrency.value,
        description: desc,
        color: ledgerColor.value,
        budget: budgetCents,
      });
      if (ledgerIsDefault.value) {
        await store.setDefaultBook(created.id);
      }
      await store.refreshBooks();
      await store.switchBook(created.id);
      store.showToast(`已创建并切换至「${created.name}」`, 'success');
    }
    closeSheet();
  } catch (err: any) {
    store.showToast(err?.message || '保存账本失败', 'error');
  } finally {
    savingLedger.value = false;
  }
}

async function handleDeleteCurrentLedger() {
  const book = store.activeBook;
  if (!book || savingLedger.value) return;
  if (store.books.length <= 1) {
    store.showToast('至少需要保留一个账本', 'error');
    return;
  }
  if (!confirmDeleteLedger.value) {
    confirmDeleteLedger.value = true;
    return;
  }
  savingLedger.value = true;
  try {
    await api.deleteBook(book.id);
    await store.refreshBooks();
    if (store.books[0]?.id) {
      await store.switchBook(store.books[0].id);
    }
    store.showToast(`已删除账本「${book.name}」`, 'success');
    closeSheet();
  } catch (err: any) {
    store.showToast(err?.message || '删除账本失败', 'error');
  } finally {
    savingLedger.value = false;
  }
}

// ==================== 2. 预算管理 ====================
const totalBudgetInput = ref('3000');
const editingTotalBudget = ref(false);
const savingTotalBudget = ref(false);
const editingCatBudgetId = ref<string | null>(null);
const catBudgetInputYuan = ref('');
const savingCatBudgetId = ref<string | null>(null);

const budgetTotalLimit = computed(
  () => store.budgets?.total_budget ?? store.activeBook?.budget ?? 0
);
const budgetTotalSpent = computed(() => store.budgets?.total_spent ?? 0);
const budgetTotalPct = computed(() => {
  if (typeof store.budgets?.percentage === 'number') return Math.round(store.budgets.percentage);
  if (budgetTotalLimit.value <= 0) return 0;
  return Math.round((budgetTotalSpent.value / budgetTotalLimit.value) * 100);
});

const mergedCategoryBudgets = computed<CategoryBudgetItem[]>(() => {
  const existing = store.budgets?.categories || [];
  const byId = new Map<string, CategoryBudgetItem>();
  for (const item of existing) {
    byId.set(item.category_id, item);
  }

  const rows: CategoryBudgetItem[] = [...existing];
  for (const cat of store.expenseCategories) {
    if (!byId.has(cat.id)) {
      rows.push({
        category_id: cat.id,
        category_name: cat.name,
        category_icon: cat.icon,
        category_color: cat.color,
        bg_color: cat.bg_color,
        limit_amount: 0,
        spent_amount: 0,
        remaining_amount: 0,
        percentage: 0,
      });
    }
  }
  return rows;
});

function initBudgetsState() {
  editingTotalBudget.value = false;
  editingCatBudgetId.value = null;
  totalBudgetInput.value = String(Math.round(centsToYuan(budgetTotalLimit.value)) || 3000);
}

async function saveTotalBudget() {
  const bid = store.activeBookId;
  if (!bid || savingTotalBudget.value) return;
  const val = parseFloat(totalBudgetInput.value);
  if (!Number.isFinite(val) || val < 0) {
    store.showToast('请输入有效的总预算金额', 'error');
    return;
  }
  savingTotalBudget.value = true;
  try {
    await api.updateTotalBudget({ book_id: bid, budget: yuanToCents(val) });
    await Promise.allSettled([store.refreshBudgets(bid), store.refreshBooks()]);
    editingTotalBudget.value = false;
    store.showToast(`总预算已更新为 ¥${Math.round(val)}`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '更新总预算失败', 'error');
  } finally {
    savingTotalBudget.value = false;
  }
}

async function adjustCategoryBudget(row: CategoryBudgetItem, deltaYuan: number) {
  const bid = store.activeBookId;
  if (!bid || savingCatBudgetId.value) return;
  const nextCents = Math.max(0, row.limit_amount + deltaYuan * 100);
  savingCatBudgetId.value = row.category_id;
  try {
    await api.updateCategoryBudget({
      book_id: bid,
      category_id: row.category_id,
      limit_amount: nextCents,
    });
    await store.refreshBudgets(bid);
    store.showToast(`已更新「${row.category_name}」预算`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '更新分类预算失败', 'error');
  } finally {
    savingCatBudgetId.value = null;
  }
}

function startEditCatBudget(row: CategoryBudgetItem) {
  editingCatBudgetId.value = row.category_id;
  catBudgetInputYuan.value = row.limit_amount > 0 ? String(Math.round(centsToYuan(row.limit_amount))) : '';
}

async function saveCatBudgetInput(row: CategoryBudgetItem) {
  const bid = store.activeBookId;
  if (!bid || savingCatBudgetId.value) return;
  const parsed = parseFloat(catBudgetInputYuan.value || '0');
  if (!Number.isFinite(parsed) || parsed < 0) {
    store.showToast('请输入非负预算金额', 'error');
    return;
  }
  savingCatBudgetId.value = row.category_id;
  try {
    await api.updateCategoryBudget({
      book_id: bid,
      category_id: row.category_id,
      limit_amount: yuanToCents(parsed),
    });
    await store.refreshBudgets(bid);
    editingCatBudgetId.value = null;
    store.showToast(`已设置「${row.category_name}」月预算`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '保存分类预算失败', 'error');
  } finally {
    savingCatBudgetId.value = null;
  }
}

// ==================== 3. 资产账户管理 ====================
const editingWalletId = ref<string | null>(null);
const walletForm = reactive<{
  name: string;
  type: WalletType;
  group_name: string;
  color: string;
  balanceYuan: string;
  creditLimitYuan: string;
  cardLastFour: string;
  excludeNetAsset: boolean;
}>({
  name: '',
  type: 'debit',
  group_name: '储蓄卡',
  color: LEDGER_COLOR_PRESETS[0],
  balanceYuan: '0',
  creditLimitYuan: '',
  cardLastFour: '',
  excludeNetAsset: false,
});
const savingWallet = ref(false);
const confirmDeleteWalletId = ref<string | null>(null);

const WALLET_TYPE_OPTIONS: DropdownOption[] = [
  { value: 'wechat', label: '微信钱包' },
  { value: 'alipay', label: '支付宝' },
  { value: 'debit', label: '储蓄卡' },
  { value: 'credit', label: '信用卡' },
  { value: 'cash', label: '现金' },
  { value: 'investment', label: '投资理财' },
  { value: 'custom', label: '自定义账户' },
];

const WALLET_TYPE_GROUP_MAP: Record<WalletType, string> = {
  wechat: '电子钱包',
  alipay: '电子钱包',
  debit: '银行卡',
  credit: '信用账户',
  cash: '现金账户',
  investment: '投资理财',
  custom: '其他账户',
  other: '其他账户',
};

const groupedWallets = computed(() => {
  const map = new Map<string, Wallet[]>();
  for (const w of store.activeWallets) {
    const g = w.group_name || WALLET_TYPE_GROUP_MAP[w.type] || '默认分组';
    if (!map.has(g)) map.set(g, []);
    map.get(g)!.push(w);
  }
  return Array.from(map.entries()).map(([groupName, list]) => ({
    groupName,
    list,
  }));
});

function resetWalletForm() {
  editingWalletId.value = null;
  confirmDeleteWalletId.value = null;
  walletForm.name = '';
  walletForm.type = 'debit';
  walletForm.group_name = '银行卡';
  walletForm.color = LEDGER_COLOR_PRESETS[0];
  walletForm.balanceYuan = '';
  walletForm.creditLimitYuan = '';
  walletForm.cardLastFour = '';
  walletForm.excludeNetAsset = false;
}

function onWalletTypeChange(val: string | number) {
  const t = String(val) as WalletType;
  walletForm.type = t;
  if (!editingWalletId.value) {
    walletForm.group_name = WALLET_TYPE_GROUP_MAP[t] || '其他账户';
  }
}

function startEditWallet(w: Wallet) {
  editingWalletId.value = w.id;
  walletForm.name = w.name;
  walletForm.type = w.type || 'custom';
  walletForm.group_name = w.group_name || WALLET_TYPE_GROUP_MAP[w.type] || '其他账户';
  walletForm.color = w.color || LEDGER_COLOR_PRESETS[0];
  walletForm.balanceYuan = centsToYuan(w.balance).toFixed(2);
  walletForm.creditLimitYuan = w.credit_limit > 0 ? String(Math.round(centsToYuan(w.credit_limit))) : '';
  walletForm.cardLastFour = w.card_last_four || '';
  walletForm.excludeNetAsset = Boolean(w.exclude_net_asset);
}

async function submitWalletForm() {
  const name = walletForm.name.trim();
  if (!name) {
    store.showToast('请输入账户名称', 'error');
    return;
  }
  if (!store.activeBookId || savingWallet.value) return;

  const balVal = parseFloat(walletForm.balanceYuan || '0');
  if (!Number.isFinite(balVal)) {
    store.showToast('请输入有效的账户余额', 'error');
    return;
  }
  if (walletForm.cardLastFour.trim() && !/^\d{4}$/.test(walletForm.cardLastFour.trim())) {
    store.showToast('银行卡尾号需为 4 位数字', 'error');
    return;
  }

  const limitVal = parseFloat(walletForm.creditLimitYuan || '0');
  const payload: Partial<Wallet> = {
    book_id: store.activeBookId,
    name,
    type: walletForm.type,
    group_name: walletForm.group_name.trim() || WALLET_TYPE_GROUP_MAP[walletForm.type] || '默认分组',
    color: walletForm.color,
    balance: yuanToCents(balVal),
    credit_limit: Number.isFinite(limitVal) && limitVal > 0 ? yuanToCents(limitVal) : 0,
    card_last_four: walletForm.cardLastFour.trim(),
    exclude_net_asset: walletForm.excludeNetAsset,
  };

  savingWallet.value = true;
  try {
    if (editingWalletId.value) {
      await api.updateWallet(editingWalletId.value, payload);
      store.showToast(`已更新账户「${name}」`, 'success');
    } else {
      await api.createWallet(payload);
      store.showToast(`已添加账户「${name}」`, 'success');
    }
    await store.refreshWallets();
    resetWalletForm();
  } catch (err: any) {
    store.showToast(err?.message || '保存账户失败', 'error');
  } finally {
    savingWallet.value = false;
  }
}

async function handleDeleteWallet(w: Wallet) {
  if (savingWallet.value) return;
  if (confirmDeleteWalletId.value !== w.id) {
    confirmDeleteWalletId.value = w.id;
    return;
  }
  savingWallet.value = true;
  try {
    await api.deleteWallet(w.id);
    await store.refreshWallets();
    if (editingWalletId.value === w.id) resetWalletForm();
    confirmDeleteWalletId.value = null;
    store.showToast(`已移除账户「${w.name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '删除账户失败', 'error');
  } finally {
    savingWallet.value = false;
  }
}

// ==================== 4. 分类管理（一二级分类） ====================
const catMgmtType = ref<'expense' | 'income'>('expense');
const expandedCatId = ref<string | null>(null);
const newSubCatName = ref('');
const newSubCatIcon = ref('coffee');
const editingCatId = ref<string | null>(null);
const catFormName = ref('');
const catFormIcon = ref('sparkles');
const catFormColor = ref(CATEGORY_COLOR_PRESETS[1].color);
const catFormBgColor = ref(CATEGORY_COLOR_PRESETS[1].bgColor);
const savingCat = ref(false);
const confirmDeleteCatId = ref<string | null>(null);

const currentCategoryTree = computed<CategoryTreeNode[]>(() =>
  catMgmtType.value === 'income' ? store.incomeCategories : store.expenseCategories
);

function initCategoriesState() {
  confirmDeleteCatId.value = null;
  editingCatId.value = null;
  catFormName.value = '';
  newSubCatName.value = '';
  const first = currentCategoryTree.value[0];
  expandedCatId.value = first?.id || null;
}

watch(catMgmtType, () => {
  confirmDeleteCatId.value = null;
  editingCatId.value = null;
  catFormName.value = '';
  expandedCatId.value = currentCategoryTree.value[0]?.id || null;
});

function pickCategoryColorPreset(preset: { color: string; bgColor: string }) {
  catFormColor.value = preset.color;
  catFormBgColor.value = preset.bgColor;
}

function startEditCategory(cat: Category) {
  editingCatId.value = cat.id;
  catFormName.value = cat.name;
  catFormIcon.value = cat.icon || 'tag';
  catFormColor.value = cat.color || CATEGORY_COLOR_PRESETS[0].color;
  catFormBgColor.value = cat.bg_color || CATEGORY_COLOR_PRESETS[0].bgColor;
}

function cancelEditCategory() {
  editingCatId.value = null;
  catFormName.value = '';
  catFormIcon.value = 'sparkles';
  catFormColor.value = CATEGORY_COLOR_PRESETS[1].color;
  catFormBgColor.value = CATEGORY_COLOR_PRESETS[1].bgColor;
}

async function submitPrimaryCategory() {
  const name = catFormName.value.trim();
  if (!name) {
    store.showToast('请输入分类名称', 'error');
    return;
  }
  if (!store.activeBookId || savingCat.value) return;

  savingCat.value = true;
  try {
    if (editingCatId.value) {
      await api.updateCategory(editingCatId.value, {
        name,
        icon: catFormIcon.value,
        color: catFormColor.value,
        bg_color: catFormBgColor.value,
      });
      store.showToast(`已更新分类「${name}」`, 'success');
      cancelEditCategory();
    } else {
      await api.createCategory({
        book_id: store.activeBookId,
        type: catMgmtType.value,
        name,
        icon: catFormIcon.value,
        color: catFormColor.value,
        bg_color: catFormBgColor.value,
        is_custom: true,
        parent_id: null,
      });
      catFormName.value = '';
      store.showToast(`已新增分类「${name}」`, 'success');
    }
    await store.refreshCategories();
  } catch (err: any) {
    store.showToast(err?.message || '保存分类失败', 'error');
  } finally {
    savingCat.value = false;
  }
}

async function submitSubCategory(parent: CategoryTreeNode) {
  const name = newSubCatName.value.trim();
  if (!name) {
    store.showToast('请输入二级分类名称', 'error');
    return;
  }
  if (!store.activeBookId || savingCat.value) return;
  savingCat.value = true;
  try {
    await api.createCategory({
      book_id: store.activeBookId,
      type: catMgmtType.value,
      name,
      icon: newSubCatIcon.value || parent.icon || 'tag',
      color: parent.color,
      bg_color: parent.bg_color,
      parent_id: parent.id,
      is_custom: true,
    });
    newSubCatName.value = '';
    await store.refreshCategories();
    store.showToast(`已添加「${parent.name} · ${name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '添加二级分类失败', 'error');
  } finally {
    savingCat.value = false;
  }
}

async function handleDeleteCategory(cat: Category) {
  if (savingCat.value) return;
  if (confirmDeleteCatId.value !== cat.id) {
    confirmDeleteCatId.value = cat.id;
    return;
  }
  savingCat.value = true;
  try {
    await api.deleteCategory(cat.id);
    await store.refreshCategories();
    if (editingCatId.value === cat.id) cancelEditCategory();
    confirmDeleteCatId.value = null;
    store.showToast(`已删除「${cat.name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '删除分类失败', 'error');
  } finally {
    savingCat.value = false;
  }
}

// ==================== 5. 定时记账管理 ====================
const recurringView = ref<'list' | 'form'>('list');
const editingRecurringId = ref<string | null>(null);
const recurringForm = reactive<{
  type: 'expense' | 'income';
  categoryId: string;
  subCategory: string;
  title: string;
  amountYuan: string;
  cycle: string;
  startDate: string;
  walletId: string;
  excludeStats: boolean;
}>({
  type: 'expense',
  categoryId: '',
  subCategory: '',
  title: '',
  amountYuan: '',
  cycle: '每月',
  startDate: todayIso(),
  walletId: '',
  excludeStats: false,
});
const savingRecurring = ref(false);
const confirmDeleteRecId = ref<string | null>(null);

const RECURRING_CYCLES = ['每天', '每周', '每月', '每年'];

const recurringCategoryTree = computed(() =>
  recurringForm.type === 'income' ? store.incomeCategories : store.expenseCategories
);

const walletDropdownOptions = computed<DropdownOption[]>(() =>
  store.activeWallets.map((w) => ({
    value: w.id,
    label: w.name,
    dot: w.color,
  }))
);

function normalizeCycleLabel(cycle: string): string {
  const map: Record<string, string> = {
    daily: '每天',
    weekly: '每周',
    monthly: '每月',
    yearly: '每年',
  };
  return map[cycle] || cycle || '每月';
}

function initRecurringState() {
  recurringView.value = 'list';
  editingRecurringId.value = null;
  confirmDeleteRecId.value = null;
}

function openCreateRecurring() {
  editingRecurringId.value = null;
  confirmDeleteRecId.value = null;
  recurringForm.type = 'expense';
  const firstCat = store.expenseCategories[0];
  recurringForm.categoryId = firstCat?.id || '';
  recurringForm.subCategory = firstCat?.children?.[0]?.name || '';
  recurringForm.title = '';
  recurringForm.amountYuan = '';
  recurringForm.cycle = '每月';
  recurringForm.startDate = todayIso();
  recurringForm.walletId = store.activeWallets[0]?.id || '';
  recurringForm.excludeStats = false;
  recurringView.value = 'form';
}

function openEditRecurring(rule: RecurringRule) {
  editingRecurringId.value = rule.id;
  confirmDeleteRecId.value = null;
  recurringForm.type = rule.type === 'income' ? 'income' : 'expense';
  recurringForm.categoryId = rule.category_id || '';
  recurringForm.subCategory = rule.sub_category || '';
  recurringForm.title = rule.title || '';
  recurringForm.amountYuan = centsToYuan(rule.amount).toFixed(2);
  recurringForm.cycle = normalizeCycleLabel(rule.cycle);
  recurringForm.startDate = (rule.start_date || todayIso()).slice(0, 10);
  recurringForm.walletId = rule.wallet_id || store.activeWallets[0]?.id || '';
  recurringForm.excludeStats = Boolean(rule.exclude_stats);
  recurringView.value = 'form';
}

function switchRecurringType(next: 'expense' | 'income') {
  if (recurringForm.type === next) return;
  recurringForm.type = next;
  const tree = next === 'income' ? store.incomeCategories : store.expenseCategories;
  recurringForm.categoryId = tree[0]?.id || '';
  recurringForm.subCategory = tree[0]?.children?.[0]?.name || '';
}

async function handleToggleRecurring(rule: RecurringRule) {
  try {
    const updated = await api.toggleRecurring(rule.id);
    await store.refreshRecurring();
    store.showToast(
      updated.is_active ? `已开启「${rule.title}」` : `已暂停「${rule.title}」`,
      'info'
    );
  } catch (err: any) {
    store.showToast(err?.message || '切换规则状态失败', 'error');
  }
}

async function submitRecurringForm() {
  const amt = parseFloat(recurringForm.amountYuan);
  if (!Number.isFinite(amt) || amt <= 0) {
    store.showToast('请输入大于 0 的金额', 'error');
    return;
  }
  const walletId = recurringForm.walletId || store.activeWallets[0]?.id || '';
  if (!walletId) {
    store.showToast('请先添加资产账户', 'error');
    return;
  }
  if (!store.activeBookId || savingRecurring.value) return;

  const activeCat = recurringCategoryTree.value.find((c) => c.id === recurringForm.categoryId);
  const title =
    recurringForm.title.trim() ||
    recurringForm.subCategory ||
    activeCat?.name ||
    '定时记账';

  const payload = {
    book_id: store.activeBookId,
    type: recurringForm.type,
    category_id: recurringForm.categoryId,
    sub_category: recurringForm.subCategory || '',
    title,
    amount: yuanToCents(amt),
    cycle: recurringForm.cycle,
    start_date: recurringForm.startDate || todayIso(),
    end_rule: '不结束',
    wallet_id: walletId,
    exclude_stats: recurringForm.excludeStats,
  };

  savingRecurring.value = true;
  try {
    if (editingRecurringId.value) {
      await api.updateRecurring(editingRecurringId.value, payload);
      store.showToast(`已保存定时记账「${title}」`, 'success');
    } else {
      await api.createRecurring(payload);
      store.showToast(`已添加定时记账「${title}」`, 'success');
    }
    await store.refreshRecurring();
    recurringView.value = 'list';
    editingRecurringId.value = null;
  } catch (err: any) {
    store.showToast(err?.message || '保存定时规则失败', 'error');
  } finally {
    savingRecurring.value = false;
  }
}

async function handleDeleteRecurring(id: string) {
  if (savingRecurring.value) return;
  if (confirmDeleteRecId.value !== id) {
    confirmDeleteRecId.value = id;
    return;
  }
  savingRecurring.value = true;
  try {
    await api.deleteRecurring(id);
    await store.refreshRecurring();
    confirmDeleteRecId.value = null;
    if (editingRecurringId.value === id) {
      editingRecurringId.value = null;
      recurringView.value = 'list';
    }
    store.showToast('已删除定时记账规则', 'success');
  } catch (err: any) {
    store.showToast(err?.message || '删除规则失败', 'error');
  } finally {
    savingRecurring.value = false;
  }
}

// ==================== 6. 绑定 / 更换邮箱 ====================
const bindEmail = ref('');
const bindCode = ref('');
const sendingBindCode = ref(false);
const submittingBind = ref(false);
const bindCooldown = ref(0);
const bindSentHint = ref<CodeDeliveryHint | null>(null);
let bindCooldownTimer: ReturnType<typeof setInterval> | null = null;

function clearBindTimer() {
  if (bindCooldownTimer) {
    clearInterval(bindCooldownTimer);
    bindCooldownTimer = null;
  }
}

function initBindEmailState() {
  bindEmail.value = '';
  bindCode.value = '';
  bindSentHint.value = null;
}

async function handleSendBindCode() {
  if (bindCooldown.value > 0 || sendingBindCode.value) return;
  const email = bindEmail.value.trim();
  if (!email) {
    store.showToast('请先输入邮箱地址', 'error');
    return;
  }
  sendingBindCode.value = true;
  try {
    const res = await api.sendCode({ email, purpose: 'bind' });
    bindSentHint.value = {
      email,
      demoCode: res.demo_code || undefined,
    };
    bindCooldown.value = 60;
    clearBindTimer();
    bindCooldownTimer = setInterval(() => {
      bindCooldown.value = Math.max(0, bindCooldown.value - 1);
      if (bindCooldown.value === 0) clearBindTimer();
    }, 1000);
    if (res.demo_code) {
      store.showToast(`验证码 ${res.demo_code} 已发送（演示模式）`, 'info', 4000);
    } else {
      store.showToast('验证码已发送至邮箱', 'success');
    }
  } catch (err: any) {
    store.showToast(err?.message || '发送验证码失败', 'error');
  } finally {
    sendingBindCode.value = false;
  }
}

async function submitBindEmail() {
  const email = bindEmail.value.trim();
  const code = bindCode.value.trim();
  if (!email || !code) {
    store.showToast('请填写邮箱地址和验证码', 'error');
    return;
  }
  if (submittingBind.value) return;
  submittingBind.value = true;
  try {
    const updated = await api.bindEmail({ email, code });
    store.user = updated;
    bindSentHint.value = null;
    store.showToast(`已成功绑定邮箱 ${email}`, 'success');
    closeSheet();
  } catch (err: any) {
    store.showToast(err?.message || '绑定邮箱失败', 'error');
  } finally {
    submittingBind.value = false;
  }
}

// ==================== 7. 导入与导出 CSV ====================
const importPlatform = ref<'wechat' | 'alipay' | 'suishouji'>('wechat');
const parsingImport = ref(false);
const committingImport = ref(false);
const importedItems = ref<ImportedItem[]>([]);
const importWalletId = ref('');
const exportingCsv = ref(false);

// 随手记导入状态
const suishoujiResult = ref<SuishoujiParseResult | null>(null);
const suishoujiMapping = ref<Record<string, SuishoujiColumnTarget>>({});
const suishoujiPreview = ref<SuishoujiPreviewRow[]>([]);
const suishoujiPreviewLoading = ref(false);
const suishoujiCommitResult = ref<SuishoujiCommitResult | null>(null);

const suishoujiMappingComplete = computed(() => {
  const vals = Object.values(suishoujiMapping.value);
  return vals.includes('date') && vals.includes('amount');
});

const importSummary = computed(() => {
  let expense = 0;
  let income = 0;
  for (const item of importedItems.value) {
    if (item.type === 'income') income += Number(item.amount || 0);
    else expense += Number(item.amount || 0);
  }
  return { expense, income, count: importedItems.value.length };
});

function initImportExportState() {
  importedItems.value = [];
  importWalletId.value = store.activeWallets[0]?.id || '';
  suishoujiResult.value = null;
  suishoujiMapping.value = {};
  suishoujiPreview.value = [];
  suishoujiCommitResult.value = null;
}

async function refreshSuishoujiPreview() {
  if (!suishoujiResult.value || !store.activeBookId) return;
  suishoujiPreviewLoading.value = true;
  try {
    const res = await api.previewSuishouji({
      book_id: store.activeBookId,
      mapping: suishoujiMapping.value,
      default_wallet_id: importWalletId.value || suishoujiResult.value.default_wallet_id,
      headers: suishoujiResult.value.headers,
      rows: suishoujiResult.value.rows.slice(0, 10),
    });
    suishoujiPreview.value = res.preview || [];
  } catch (err: any) {
    store.showToast(err?.message || '预览失败', 'error');
  } finally {
    suishoujiPreviewLoading.value = false;
  }
}

function applySuishoujiAiSuggestions() {
  if (!suishoujiResult.value) return;
  suishoujiMapping.value = { ...suishoujiResult.value.ai_suggestions };
  void refreshSuishoujiPreview();
}

async function handleCommitSuishouji() {
  if (!suishoujiResult.value || committingImport.value) return;
  const bid = store.activeBookId;
  if (!bid) {
    store.showToast('请先选择目标账本', 'error');
    return;
  }
  if (!suishoujiMappingComplete.value) {
    store.showToast('请先完成字段映射：日期和金额列必须映射', 'error');
    return;
  }
  const wid = importWalletId.value || suishoujiResult.value.default_wallet_id || store.activeWallets[0]?.id || '';
  if (!wid) {
    store.showToast('请先选择入账账户', 'error');
    return;
  }
  committingImport.value = true;
  try {
    const res = await api.commitSuishouji({
      book_id: bid,
      mapping: suishoujiMapping.value,
      default_wallet_id: wid,
      headers: suishoujiResult.value.headers,
      rows: suishoujiResult.value.rows,
    });
    suishoujiCommitResult.value = res;
    await store.refreshBookData(bid);
    store.showToast(`导入完成：成功 ${res.inserted} 条，重复跳过 ${res.duplicates} 条，失败 ${res.failed} 条`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '提交导入失败', 'error');
  } finally {
    committingImport.value = false;
  }
}

async function handleExportCsv() {
  if (exportingCsv.value) return;
  exportingCsv.value = true;
  try {
    const url = api.getExportUrl(store.activeBookId);
    const headers = await getAuthHeaders();
    const res = await fetch(url, { headers });
    if (!res.ok) {
      throw new Error(`导出失败 (HTTP ${res.status})`);
    }
    const blob = await res.blob();
    const blobUrl = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = blobUrl;
    const bookName = (store.activeBook?.name || '七猫账本').replace(/[\\/:*?"<>|]/g, '_');
    link.download = `${bookName}_${todayIso()}.csv`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(blobUrl);
    store.showToast('CSV 账单已导出', 'success');
  } catch (err: any) {
    store.showToast(err?.message || '导出账单失败', 'error');
  } finally {
    exportingCsv.value = false;
  }
}

async function handleImportFileChange(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;

  parsingImport.value = true;
  try {
    if (importPlatform.value === 'suishouji') {
      if (!store.activeBookId) {
        store.showToast('请先选择账本', 'error');
        return;
      }
      const res = await api.parseSuishouji(file, store.activeBookId);
      suishoujiResult.value = res;
      suishoujiMapping.value = { ...(res.auto_mapping || {}) };
      suishoujiPreview.value = res.preview || [];
      suishoujiCommitResult.value = null;
      if (res.default_wallet_id) importWalletId.value = res.default_wallet_id;
      if (res.total_rows === 0) {
        store.showToast('文件中未解析到有效数据行', 'info');
      } else {
        store.showToast(`成功解析 ${res.total_rows} 行，请核对字段映射`, 'success');
      }
      return;
    }
    const res =
      importPlatform.value === 'wechat'
        ? await api.importWeChat(file)
        : await api.importAlipay(file);
    importedItems.value = res.items || [];
    if (!importWalletId.value) {
      importWalletId.value = store.activeWallets[0]?.id || '';
    }
    if (importedItems.value.length === 0) {
      store.showToast('未在文件中解析到有效流水记录', 'info');
    } else {
      store.showToast(`成功解析 ${importedItems.value.length} 条流水，请核对后提交`, 'success');
    }
  } catch (err: any) {
    store.showToast(err?.message || '解析账单文件失败', 'error');
  } finally {
    parsingImport.value = false;
    input.value = '';
  }
}

async function handleCommitImport() {
  if (importPlatform.value === 'suishouji') {
    await handleCommitSuishouji();
    return;
  }
  if (!importedItems.value.length || committingImport.value) return;
  const bid = store.activeBookId;
  const wid = importWalletId.value || store.activeWallets[0]?.id || '';
  if (!bid || !wid) {
    store.showToast('请先选择目标账本和入账账户', 'error');
    return;
  }
  committingImport.value = true;
  try {
    const res = await api.commitImport(bid, wid, importedItems.value);
    importedItems.value = [];
    await store.refreshBookData(bid);
    store.showToast(`已成功导入 ${res.inserted} 条账单流水`, 'success');
    closeSheet();
  } catch (err: any) {
    store.showToast(err?.message || '提交导入失败', 'error');
  } finally {
    committingImport.value = false;
  }
}

// ==================== Sheet 元信息与生命周期 ====================
const isMultiDetent = computed(
  () => !['new-ledger', 'bind-email'].includes(props.page)
);

const pageTitle = computed(() => {
  switch (props.page) {
    case 'new-ledger':
      return ledgerMode.value === 'edit' ? '编辑当前账本' : '新建账本';
    case 'budgets':
      return '预算管理';
    case 'wallets':
      return '账户与资产管理';
    case 'categories':
      return '分类管理（一级 / 二级图标）';
    case 'recurring':
      return recurringView.value === 'form'
        ? editingRecurringId.value
          ? '编辑定时记账'
          : '添加定时记账'
        : '定时记账';
    case 'bind-email':
      return store.user?.email_bound ? '更换绑定邮箱' : '绑定安全邮箱';
    case 'import-export':
      return '账单导入与导出';
    default:
      return '账务管理';
  }
});

watch(
  () => [props.open, props.page] as const,
  ([opened, page]) => {
    if (!opened) return;
    if (page === 'new-ledger') initLedgerForm('create');
    else if (page === 'budgets') initBudgetsState();
    else if (page === 'wallets') resetWalletForm();
    else if (page === 'categories') initCategoriesState();
    else if (page === 'recurring') initRecurringState();
    else if (page === 'bind-email') initBindEmailState();
    else if (page === 'import-export') initImportExportState();
  },
  { immediate: true }
);

onBeforeUnmount(() => {
  clearBindTimer();
});

function closeSheet() {
  emit('update:open', false);
  emit('close');
}
</script>

<template>
  <BottomSheet
    :open="open"
    :multi-detent="isMultiDetent"
    max-height-class="max-h-[92dvh]"
    bg-class="bg-[#F5F6F8]"
    z-index-class="z-40"
    @close="emit('close')"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <div class="px-5 py-2 flex items-center justify-between gap-2 shrink-0">
        <div class="flex items-center gap-2 min-w-0">
          <button
            v-if="page === 'recurring' && recurringView === 'form'"
            type="button"
            aria-label="返回定时列表"
            class="w-7 h-7 rounded-full bg-white border border-[#ECEEF2] flex items-center justify-center text-[#475569] hover:text-[#18181B] shrink-0"
            @click="recurringView = 'list'"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
              <path d="M15 18l-6-6 6-6" />
            </svg>
          </button>
          <h3 class="text-[15px] font-bold text-[#18181B] truncate">{{ pageTitle }}</h3>
        </div>

        <button
          type="button"
          aria-label="关闭子页面"
          class="w-8 h-8 rounded-full bg-white border border-[#ECEEF2] flex items-center justify-center text-[#64748B] hover:text-[#18181B] shrink-0 transition"
          @click="closeSheet"
        >
          <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
        </button>
      </div>
    </template>

    <div class="flex-1 min-h-0 overflow-y-auto phone-scroll px-4 pt-1 pb-6">
      <!-- ==================== 1. 新建 / 编辑账本 ==================== -->
      <div v-if="page === 'new-ledger'" class="space-y-3.5">
        <div
          v-if="store.activeBook"
          class="p-1 rounded-2xl bg-[#ECEEF2] grid grid-cols-2 text-xs font-semibold"
        >
          <button
            type="button"
            class="py-1.5 rounded-xl transition"
            :class="
              ledgerMode === 'create'
                ? 'bg-white text-[#18181B] shadow-xs'
                : 'text-[#64748B]'
            "
            @click="initLedgerForm('create')"
          >
            新建账本
          </button>
          <button
            type="button"
            class="py-1.5 rounded-xl transition"
            :class="
              ledgerMode === 'edit'
                ? 'bg-white text-[#18181B] shadow-xs'
                : 'text-[#64748B]'
            "
            @click="initLedgerForm('edit')"
          >
            编辑当前账本
          </button>
        </div>

        <form class="space-y-3.5" @submit.prevent="submitLedgerForm">
          <div class="app-card p-4 space-y-3">
            <div>
              <label class="block text-xs font-bold text-[#18181B] mb-1.5">账本名称</label>
              <input
                v-model="ledgerName"
                type="text"
                required
                maxlength="20"
                placeholder="如：京都旅行账本、家庭日常、副业收支"
                class="w-full text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 py-2.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
              />
            </div>

            <div class="grid grid-cols-2 gap-2.5">
              <div>
                <label class="block text-xs font-bold text-[#18181B] mb-1.5">月度预算（元）</label>
                <input
                  v-model="ledgerBudgetYuan"
                  type="number"
                  min="0"
                  step="100"
                  placeholder="3000"
                  class="w-full h-9 text-xs font-semibold tabular-nums bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
                />
              </div>
              <div>
                <label class="block text-xs font-bold text-[#18181B] mb-1.5">结算币种</label>
                <CustomDropdown
                  v-model="ledgerCurrency"
                  :options="CURRENCY_OPTIONS"
                  size="md"
                />
              </div>
            </div>

            <div>
              <label class="block text-xs font-bold text-[#18181B] mb-1.5">默认成员模式</label>
              <div class="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  class="p-2.5 rounded-xl border text-left transition"
                  :class="
                    ledgerMemberMode === 'personal'
                      ? 'border-[#DE8A1E] bg-[#FFF6E8]'
                      : 'border-[#ECEEF2] bg-white'
                  "
                  @click="ledgerMemberMode = 'personal'"
                >
                  <p class="text-xs font-bold text-[#18181B]">个人独立记账</p>
                  <p class="text-[10px] text-[#64748B] mt-0.5">默认仅本人管理收支</p>
                </button>
                <button
                  type="button"
                  class="p-2.5 rounded-xl border text-left transition"
                  :class="
                    ledgerMemberMode === 'shared'
                      ? 'border-[#DE8A1E] bg-[#FFF6E8]'
                      : 'border-[#ECEEF2] bg-white'
                  "
                  @click="ledgerMemberMode = 'shared'"
                >
                  <p class="text-xs font-bold text-[#18181B]">家庭 / 共享协作</p>
                  <p class="text-[10px] text-[#64748B] mt-0.5">支持生成口令邀请成员</p>
                </button>
              </div>
            </div>

            <div>
              <label class="block text-xs font-bold text-[#18181B] mb-1.5">账本用途说明</label>
              <input
                v-model="ledgerDesc"
                type="text"
                maxlength="36"
                placeholder="如：独立核算 · 可邀请成员共同记录"
                class="w-full text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 py-2.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
              />
            </div>

            <div>
              <label class="block text-xs font-bold text-[#18181B] mb-2">主题标记色</label>
              <div class="flex items-center gap-2.5">
                <button
                  v-for="c in LEDGER_COLOR_PRESETS"
                  :key="c"
                  type="button"
                  class="w-7 h-7 rounded-full transition"
                  :class="ledgerColor === c ? 'ring-2 ring-offset-2 ring-[#18181B] scale-105' : ''"
                  :style="{ backgroundColor: c }"
                  @click="ledgerColor = c"
                />
              </div>
            </div>

            <div class="flex items-center justify-between pt-2.5 border-t border-[#F1F3F6]">
              <div>
                <p class="text-xs font-bold text-[#18181B]">设为默认账本</p>
                <p class="text-[10px] text-[#64748B] mt-0.5">每次打开应用时自动加载该账本</p>
              </div>
              <ToggleSwitch v-model="ledgerIsDefault" />
            </div>
          </div>

          <button
            type="submit"
            :disabled="savingLedger"
            class="w-full py-3 rounded-2xl bg-[#DE8A1E] text-white text-xs font-semibold shadow-sm hover:bg-[#C67612] disabled:opacity-60 transition"
          >
            {{
              savingLedger
                ? '保存中…'
                : ledgerMode === 'edit'
                  ? '保存账本设置'
                  : '创建并切换至该账本'
            }}
          </button>

          <button
            v-if="ledgerMode === 'edit' && store.books.length > 1"
            type="button"
            :disabled="savingLedger"
            class="w-full py-2.5 rounded-2xl text-xs font-semibold transition"
            :class="
              confirmDeleteLedger
                ? 'bg-[#D6553E] text-white'
                : 'bg-[#FDF0EE] text-[#D6553E] hover:bg-[#FADCD7]'
            "
            @click="handleDeleteCurrentLedger"
          >
            {{ confirmDeleteLedger ? '再次点击确认删除此账本' : '删除当前账本' }}
          </button>
        </form>
      </div>

      <!-- ==================== 2. 预算管理 ==================== -->
      <div v-else-if="page === 'budgets'" class="space-y-3">
        <div class="app-card p-4 space-y-2.5">
          <div class="flex items-center justify-between gap-2">
            <div>
              <p class="text-[11px] text-[#64748B]">
                {{ store.activeBook?.name || '当前账本' }} · 月度总预算
              </p>
              <p class="text-2xl font-bold text-[#18181B] tabular-nums mt-0.5">
                {{
                  budgetTotalLimit > 0
                    ? formatMoney(budgetTotalLimit, { privacy: store.privacyHidden })
                    : '未设置'
                }}
              </p>
            </div>
            <button
              type="button"
              class="text-xs font-semibold px-3 py-1.5 rounded-full bg-[#FFF6E8] text-[#DE8A1E] hover:bg-[#FBEBD1] transition shrink-0"
              @click="editingTotalBudget = !editingTotalBudget"
            >
              {{ editingTotalBudget ? '收起' : '调整总预算' }}
            </button>
          </div>

          <div v-if="editingTotalBudget" class="pt-2 space-y-2">
            <div class="flex items-center gap-2">
              <input
                v-model="totalBudgetInput"
                type="number"
                min="0"
                step="100"
                placeholder="输入月总预算（元）"
                class="flex-1 min-w-0 h-9 text-xs font-bold tabular-nums bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
              />
              <button
                type="button"
                :disabled="savingTotalBudget"
                class="px-4 h-9 rounded-xl bg-[#DE8A1E] text-white text-xs font-semibold disabled:opacity-60"
                @click="saveTotalBudget"
              >
                {{ savingTotalBudget ? '保存中' : '保存' }}
              </button>
            </div>
            <div class="flex gap-1.5">
              <button
                v-for="v in [3000, 4500, 6000, 8000]"
                :key="v"
                type="button"
                class="flex-1 py-1 rounded-lg bg-[#F5F6F8] hover:bg-[#FFF6E8] text-[11px] font-semibold text-[#475569] hover:text-[#DE8A1E]"
                @click="totalBudgetInput = String(v)"
              >
                ¥{{ v }}
              </button>
            </div>
          </div>

          <BudgetProgressBar
            :percentage="budgetTotalPct"
            :spent="budgetTotalSpent"
            :limit="budgetTotalLimit"
            :privacy="store.privacyHidden"
            show-summary
            height-class="h-2"
          />
        </div>

        <div>
          <div class="flex items-center justify-between px-1 mb-2">
            <span class="text-xs font-bold text-[#18181B]">一级分类预算控制</span>
            <span class="text-[10px] text-[#94A3B8]">点 + / − 微调 ¥100，或点金额设定</span>
          </div>

          <div class="space-y-2">
            <div
              v-for="row in mergedCategoryBudgets"
              :key="row.category_id"
              class="p-3.5 rounded-2xl bg-white border border-[#ECEEF2] space-y-2"
            >
              <div class="flex items-center justify-between gap-2">
                <div class="flex items-center gap-2.5 min-w-0">
                  <CategoryBadge
                    :icon="row.category_icon || 'tag'"
                    :color="row.category_color || '#DE8A1E'"
                    :bg-color="row.bg_color || '#FFF6E8'"
                    size="sm"
                  />
                  <div class="min-w-0">
                    <p class="text-xs font-bold text-[#18181B] truncate">
                      {{ row.category_name }}
                    </p>
                    <button
                      type="button"
                      class="text-[10px] text-[#64748B] hover:text-[#DE8A1E] tabular-nums text-left"
                      @click="startEditCatBudget(row)"
                    >
                      已花 {{ formatMoney(row.spent_amount, { privacy: store.privacyHidden }) }} /
                      预算
                      {{
                        row.limit_amount > 0
                          ? formatMoney(row.limit_amount, { privacy: store.privacyHidden })
                          : '点击设置'
                      }}
                    </button>
                  </div>
                </div>

                <div class="flex items-center gap-1 shrink-0">
                  <button
                    type="button"
                    :disabled="savingCatBudgetId === row.category_id"
                    class="w-6 h-6 rounded-full bg-[#F5F6F8] text-xs font-bold text-[#475569] hover:bg-[#ECEEF2] disabled:opacity-50"
                    @click="adjustCategoryBudget(row, -100)"
                  >
                    −
                  </button>
                  <button
                    type="button"
                    :disabled="savingCatBudgetId === row.category_id"
                    class="w-6 h-6 rounded-full bg-[#F5F6F8] text-xs font-bold text-[#475569] hover:bg-[#ECEEF2] disabled:opacity-50"
                    @click="adjustCategoryBudget(row, 100)"
                  >
                    +
                  </button>
                </div>
              </div>

              <div
                v-if="editingCatBudgetId === row.category_id"
                class="flex items-center gap-2 pt-1"
              >
                <input
                  v-model="catBudgetInputYuan"
                  type="number"
                  min="0"
                  step="50"
                  placeholder="输入分类预算（元）"
                  class="flex-1 min-w-0 h-8 text-xs tabular-nums bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-2.5 outline-none focus:border-[#DE8A1E]"
                />
                <button
                  type="button"
                  class="px-2.5 h-8 rounded-xl bg-[#F5F6F8] text-[11px] font-semibold text-[#64748B]"
                  @click="editingCatBudgetId = null"
                >
                  取消
                </button>
                <button
                  type="button"
                  :disabled="savingCatBudgetId === row.category_id"
                  class="px-3 h-8 rounded-xl bg-[#DE8A1E] text-white text-[11px] font-semibold"
                  @click="saveCatBudgetInput(row)"
                >
                  确定
                </button>
              </div>

              <BudgetProgressBar
                :percentage="row.percentage"
                :spent="row.spent_amount"
                :limit="row.limit_amount"
                :privacy="store.privacyHidden"
                :accent-color="row.category_color"
                height-class="h-1.5"
              />
            </div>
          </div>
        </div>
      </div>

      <!-- ==================== 3. 资产账户管理 ==================== -->
      <div v-else-if="page === 'wallets'" class="space-y-3">
        <AccountNetWorthCard
          :summary="store.walletSummary"
          :privacy="store.privacyHidden"
          :currency="store.activeBook?.currency || 'CNY'"
        />

        <!-- 分组账户列表 -->
        <div
          v-for="group in groupedWallets"
          :key="group.groupName"
          class="app-card overflow-hidden"
        >
          <div class="px-4 py-2 bg-[#F8F9FB] border-b border-[#ECEEF2] text-[11px] font-semibold text-[#64748B]">
            {{ group.groupName }}
          </div>
          <div class="divide-y divide-[#F1F3F6]">
            <div
              v-for="w in group.list"
              :key="w.id"
              class="px-4 py-3 flex items-center justify-between gap-2"
            >
              <div class="flex items-center gap-3 min-w-0">
                <span
                  class="w-9 h-9 rounded-2xl text-xs font-bold text-white flex items-center justify-center shrink-0"
                  :style="{ backgroundColor: w.color || '#DE8A1E' }"
                >
                  {{ w.name.slice(0, 1) }}
                </span>
                <div class="min-w-0">
                  <div class="flex items-center gap-1.5">
                    <p class="text-xs font-bold text-[#18181B] truncate">{{ w.name }}</p>
                    <span
                      v-if="w.card_last_four"
                      class="text-[10px] text-[#94A3B8] tabular-nums"
                    >
                      ({{ w.card_last_four }})
                    </span>
                    <span
                      v-if="w.exclude_net_asset"
                      class="text-[9px] px-1.5 py-0.5 rounded bg-[#F1F5F9] text-[#64748B]"
                    >
                      不计净资产
                    </span>
                  </div>
                  <p class="text-[10px] text-[#94A3B8]">
                    {{ WALLET_TYPE_OPTIONS.find((o) => o.value === w.type)?.label || '其他账户' }}
                  </p>
                </div>
              </div>

              <div class="flex items-center gap-2 shrink-0">
                <span
                  class="text-sm font-bold tabular-nums"
                  :class="w.balance < 0 ? 'text-[#D6553E]' : 'text-[#18181B]'"
                >
                  {{ formatMoney(w.balance, { privacy: store.privacyHidden }) }}
                </span>
                <button
                  type="button"
                  class="text-[11px] font-semibold text-[#64748B] hover:text-[#DE8A1E] px-1.5 py-1"
                  @click="startEditWallet(w)"
                >
                  编辑
                </button>
                <button
                  type="button"
                  class="text-[11px] font-semibold px-1.5 py-1"
                  :class="
                    confirmDeleteWalletId === w.id
                      ? 'text-[#D6553E] bg-[#FDF0EE] rounded-lg'
                      : 'text-[#94A3B8] hover:text-[#D6553E]'
                  "
                  @click="handleDeleteWallet(w)"
                >
                  {{ confirmDeleteWalletId === w.id ? '确认' : '删除' }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- 新增 / 编辑账户表单 -->
        <div class="app-card p-4 space-y-3">
          <div class="flex items-center justify-between">
            <p class="text-xs font-bold text-[#18181B]">
              {{ editingWalletId ? '编辑资产账户' : '+ 添加新账户' }}
            </p>
            <button
              v-if="editingWalletId"
              type="button"
              class="text-[11px] font-semibold text-[#64748B] hover:text-[#18181B]"
              @click="resetWalletForm"
            >
              取消编辑
            </button>
          </div>

          <form class="space-y-2.5 text-xs" @submit.prevent="submitWalletForm">
            <div class="grid grid-cols-2 gap-2">
              <input
                v-model="walletForm.name"
                type="text"
                required
                maxlength="16"
                placeholder="账户名（如：工行卡）"
                class="h-9 text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
              />
              <input
                v-model="walletForm.balanceYuan"
                type="number"
                step="0.01"
                required
                placeholder="余额（负债填负数）"
                class="h-9 text-xs tabular-nums bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
              />
            </div>

            <div class="grid grid-cols-2 gap-2">
              <CustomDropdown
                :model-value="walletForm.type"
                :options="WALLET_TYPE_OPTIONS"
                size="md"
                @change="onWalletTypeChange"
              />
              <input
                v-model="walletForm.group_name"
                type="text"
                maxlength="12"
                placeholder="账户分组"
                class="h-9 text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
              />
            </div>

            <div v-if="walletForm.type === 'credit' || walletForm.type === 'debit'" class="grid grid-cols-2 gap-2">
              <input
                v-model="walletForm.cardLastFour"
                type="text"
                maxlength="4"
                placeholder="卡尾号（4位数字，选填）"
                class="h-9 text-xs tabular-nums bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
              />
              <input
                v-if="walletForm.type === 'credit'"
                v-model="walletForm.creditLimitYuan"
                type="number"
                min="0"
                step="100"
                placeholder="信用额度（元，选填）"
                class="h-9 text-xs tabular-nums bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
              />
            </div>

            <div class="flex items-center justify-between pt-1">
              <div class="flex items-center gap-2">
                <button
                  v-for="c in LEDGER_COLOR_PRESETS"
                  :key="c"
                  type="button"
                  class="w-6 h-6 rounded-full transition"
                  :class="walletForm.color === c ? 'ring-2 ring-offset-1 ring-[#18181B]' : ''"
                  :style="{ backgroundColor: c }"
                  @click="walletForm.color = c"
                />
              </div>
              <label class="flex items-center gap-2 text-[11px] text-[#64748B]">
                <span>不计入净资产</span>
                <ToggleSwitch v-model="walletForm.excludeNetAsset" aria-label="不计入净资产" />
              </label>
            </div>

            <button
              type="submit"
              :disabled="savingWallet"
              class="w-full py-2.5 rounded-xl bg-[#DE8A1E] text-white text-xs font-semibold hover:bg-[#C67612] disabled:opacity-60 transition"
            >
              {{
                savingWallet
                  ? '保存中…'
                  : editingWalletId
                    ? '保存账户修改'
                    : '添加到资产列表'
              }}
            </button>
          </form>
        </div>
      </div>

      <!-- ==================== 4. 分类管理 ==================== -->
      <div v-else-if="page === 'categories'" class="space-y-3">
        <div class="p-1 rounded-2xl bg-[#ECEEF2] grid grid-cols-2 text-xs font-semibold">
          <button
            type="button"
            class="py-1.5 rounded-xl transition"
            :class="
              catMgmtType === 'expense'
                ? 'bg-white text-[#18181B] shadow-xs'
                : 'text-[#64748B]'
            "
            @click="catMgmtType = 'expense'"
          >
            支出分类 ({{ store.expenseCategories.length }})
          </button>
          <button
            type="button"
            class="py-1.5 rounded-xl transition"
            :class="
              catMgmtType === 'income'
                ? 'bg-white text-[#18181B] shadow-xs'
                : 'text-[#64748B]'
            "
            @click="catMgmtType = 'income'"
          >
            收入分类 ({{ store.incomeCategories.length }})
          </button>
        </div>

        <!-- 一级 & 二级分类手风琴列表 -->
        <div class="app-card divide-y divide-[#F1F3F6] overflow-hidden">
          <div v-for="cat in currentCategoryTree" :key="cat.id" class="px-4 py-3">
            <div class="flex items-center justify-between gap-2">
              <button
                type="button"
                class="flex-1 flex items-center gap-3 text-left min-w-0"
                @click="expandedCatId = expandedCatId === cat.id ? null : cat.id"
              >
                <CategoryBadge
                  :icon="cat.icon"
                  :color="cat.color"
                  :bg-color="cat.bg_color || '#FFF6E8'"
                  size="sm"
                />
                <div class="min-w-0 flex items-center gap-1.5 flex-wrap">
                  <span class="text-xs font-bold text-[#18181B] truncate">{{ cat.name }}</span>
                  <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-[#F5F6F8] text-[#64748B]">
                    {{ cat.children?.length || 0 }} 个二级
                  </span>
                  <span
                    v-if="cat.is_custom"
                    class="text-[10px] px-1.5 py-0.5 rounded-full bg-[#FFF6E8] text-[#DE8A1E]"
                  >
                    自定义
                  </span>
                </div>
              </button>

              <div class="flex items-center gap-1 shrink-0">
                <template v-if="cat.is_custom">
                  <button
                    type="button"
                    class="text-[11px] font-medium text-[#64748B] hover:text-[#DE8A1E] px-1.5 py-1"
                    @click="startEditCategory(cat)"
                  >
                    编辑
                  </button>
                  <button
                    type="button"
                    class="text-[11px] font-medium px-1.5 py-1"
                    :class="
                      confirmDeleteCatId === cat.id
                        ? 'bg-[#D6553E] text-white rounded-lg'
                        : 'text-[#D6553E]'
                    "
                    @click="handleDeleteCategory(cat)"
                  >
                    {{ confirmDeleteCatId === cat.id ? '确认' : '删除' }}
                  </button>
                </template>
                <button
                  type="button"
                  aria-label="展开二级分类"
                  class="w-7 h-7 rounded-full hover:bg-[#F5F6F8] flex items-center justify-center text-[#94A3B8] transition"
                  @click="expandedCatId = expandedCatId === cat.id ? null : cat.id"
                >
                  <svg
                    class="w-4 h-4 transition-transform"
                    :class="{ 'rotate-180': expandedCatId === cat.id }"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path d="M6 9l6 6 6-6" />
                  </svg>
                </button>
              </div>
            </div>

            <div
              v-if="expandedCatId === cat.id"
              class="mt-3 pt-2.5 border-t border-[#F1F3F6] space-y-2.5"
            >
              <div v-if="cat.children?.length" class="grid grid-cols-2 gap-1.5">
                <div
                  v-for="sub in cat.children"
                  :key="sub.id"
                  class="flex items-center justify-between gap-1.5 pl-1.5 pr-2 py-1.5 rounded-xl bg-[#F7F8FA] border border-[#ECEEF2]"
                >
                  <div class="flex items-center gap-2 min-w-0">
                    <span
                      class="w-6 h-6 rounded-lg flex items-center justify-center shrink-0"
                      :style="{
                        backgroundColor: sub.bg_color || cat.bg_color || '#FFF6E8',
                        color: sub.color || cat.color,
                      }"
                    >
                      <svg
                        class="w-3.5 h-3.5"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="1.75"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        v-html="getIconSvg(sub.icon || cat.icon)"
                      />
                    </span>
                    <span class="text-[11px] font-semibold text-[#18181B] truncate">
                      {{ sub.name }}
                    </span>
                  </div>

                  <button
                    v-if="sub.is_custom"
                    type="button"
                    class="text-xs leading-none px-1 shrink-0"
                    :class="
                      confirmDeleteCatId === sub.id
                        ? 'text-white bg-[#D6553E] rounded py-0.5'
                        : 'text-[#94A3B8] hover:text-[#D6553E]'
                    "
                    @click="handleDeleteCategory(sub)"
                  >
                    {{ confirmDeleteCatId === sub.id ? '删' : '×' }}
                  </button>
                </div>
              </div>

              <!-- 添加二级分类表单 -->
              <div class="p-2.5 rounded-xl bg-[#F8F9FB] border border-[#ECEEF2] space-y-2">
                <p class="text-[10px] font-semibold text-[#64748B]">选择二级分类图标并添加</p>
                <div class="flex items-center gap-1.5 overflow-x-auto phone-scroll pb-0.5">
                  <button
                    v-for="ico in PICKABLE_ICONS"
                    :key="ico"
                    type="button"
                    class="w-7 h-7 rounded-lg shrink-0 flex items-center justify-center transition"
                    :class="
                      newSubCatIcon === ico
                        ? 'ring-1.5 ring-[#18181B] scale-105'
                        : 'opacity-75 hover:opacity-100'
                    "
                    :style="{
                      backgroundColor: newSubCatIcon === ico ? cat.color : cat.bg_color || '#FFF6E8',
                      color: newSubCatIcon === ico ? '#FFFFFF' : cat.color,
                    }"
                    @click="newSubCatIcon = ico"
                  >
                    <svg
                      class="w-3.5 h-3.5"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="1.75"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      v-html="getIconSvg(ico)"
                    />
                  </button>
                </div>

                <form class="flex items-center gap-1.5" @submit.prevent="submitSubCategory(cat)">
                  <input
                    v-model="newSubCatName"
                    type="text"
                    maxlength="10"
                    placeholder="输入二级分类名（如：下午茶）"
                    class="flex-1 min-w-0 text-[11px] bg-white border border-[#E6E9EF] rounded-xl px-2.5 py-1.5 outline-none focus:border-[#DE8A1E]"
                  />
                  <button
                    type="submit"
                    :disabled="savingCat"
                    class="px-3 py-1.5 rounded-xl bg-[#18181B] text-white text-[11px] font-semibold shrink-0 disabled:opacity-60"
                  >
                    + 添加
                  </button>
                </form>
              </div>
            </div>
          </div>
        </div>

        <!-- 新增 / 编辑自定义一级分类 -->
        <div class="app-card p-4">
          <div class="flex items-center justify-between mb-2.5">
            <p class="text-xs font-bold text-[#18181B]">
              {{
                editingCatId
                  ? '编辑自定义分类'
                  : `+ 新增一级${catMgmtType === 'expense' ? '支出' : '收入'}分类`
              }}
            </p>
            <div class="flex items-center gap-2">
              <button
                v-if="editingCatId"
                type="button"
                class="text-[11px] font-semibold text-[#64748B]"
                @click="cancelEditCategory"
              >
                取消编辑
              </button>
              <CategoryBadge
                :icon="catFormIcon"
                :color="catFormColor"
                :bg-color="catFormBgColor"
                size="xs"
              />
            </div>
          </div>

          <form class="space-y-3" @submit.prevent="submitPrimaryCategory">
            <input
              v-model="catFormName"
              type="text"
              required
              maxlength="10"
              placeholder="一级分类名称（如：健身、数码）"
              class="w-full text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3 py-2 outline-none focus:border-[#DE8A1E] focus:bg-white"
            />

            <div>
              <p class="text-[10px] font-semibold text-[#94A3B8] mb-1.5">选择精致图标</p>
              <div class="grid grid-cols-6 gap-1.5">
                <button
                  v-for="ico in PICKABLE_ICONS"
                  :key="ico"
                  type="button"
                  class="h-8 rounded-xl flex items-center justify-center transition"
                  :class="
                    catFormIcon === ico
                      ? 'ring-1.5 ring-[#18181B]'
                      : 'bg-[#F5F6F8] hover:bg-[#ECEEF2]'
                  "
                  :style="
                    catFormIcon === ico
                      ? { backgroundColor: catFormColor, color: '#FFFFFF' }
                      : { color: catFormColor }
                  "
                  @click="catFormIcon = ico"
                >
                  <svg
                    class="w-4 h-4"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.75"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    v-html="getIconSvg(ico)"
                  />
                </button>
              </div>
            </div>

            <div>
              <p class="text-[10px] font-semibold text-[#94A3B8] mb-1.5">选择配色方案</p>
              <div class="flex flex-wrap items-center gap-2">
                <button
                  v-for="preset in CATEGORY_COLOR_PRESETS"
                  :key="preset.color"
                  type="button"
                  :title="preset.label"
                  class="w-6 h-6 rounded-full transition"
                  :class="
                    catFormColor === preset.color ? 'ring-2 ring-offset-1 ring-[#18181B] scale-105' : ''
                  "
                  :style="{ backgroundColor: preset.color }"
                  @click="pickCategoryColorPreset(preset)"
                />
              </div>
            </div>

            <button
              type="submit"
              :disabled="savingCat"
              class="w-full py-2.5 rounded-xl bg-[#DE8A1E] text-white text-xs font-semibold hover:bg-[#C67612] disabled:opacity-60 transition"
            >
              {{ savingCat ? '保存中…' : editingCatId ? '保存修改' : '添加分类' }}
            </button>
          </form>
        </div>
      </div>

      <!-- ==================== 5. 定时记账 ==================== -->
      <div v-else-if="page === 'recurring'" class="space-y-3">
        <!-- 列表视图 -->
        <template v-if="recurringView === 'list'">
          <div class="flex items-center justify-between px-1">
            <span class="text-xs text-[#64748B]">
              共 {{ store.recurringRules.length }} 条周期规则 · 点击卡片可编辑
            </span>
          </div>

          <div
            v-if="store.recurringRules.length === 0"
            class="app-card p-6 text-center text-xs text-[#94A3B8]"
          >
            暂无定时记账规则，点击下方按钮添加自动扣费或工资入账周期
          </div>

          <div class="space-y-2.5">
            <div
              v-for="rule in store.recurringRules"
              :key="rule.id"
              class="app-card p-4 cursor-pointer hover:border-[#DE8A1E]/50 transition"
              :class="{ 'opacity-65': !rule.is_active }"
              @click="openEditRecurring(rule)"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="flex items-center gap-3 min-w-0">
                  <CategoryBadge
                    :icon="rule.category_icon || 'tag'"
                    :color="rule.category_color || '#DE8A1E'"
                    :bg-color="rule.bg_color || '#FFF6E8'"
                    size="md"
                  />
                  <div class="min-w-0">
                    <div class="flex items-center gap-1.5">
                      <span class="text-sm font-bold text-[#18181B] truncate">
                        {{ rule.title }}
                      </span>
                      <span class="text-[10px] font-medium px-1.5 py-0.5 rounded bg-[#F5F6F8] text-[#64748B] shrink-0">
                        {{ normalizeCycleLabel(rule.cycle) }}
                      </span>
                    </div>
                    <p
                      class="text-base font-bold tabular-nums mt-0.5"
                      :class="rule.type === 'income' ? 'text-[#3E8E63]' : 'text-[#D6553E]'"
                    >
                      {{
                        formatMoney(rule.amount, {
                          privacy: store.privacyHidden,
                          showSign: true,
                          type: rule.type,
                        })
                      }}
                    </p>
                    <p class="text-[11px] text-[#94A3B8] mt-0.5 truncate">
                      {{ rule.category_name
                      }}{{ rule.sub_category ? ` · ${rule.sub_category}` : '' }} ·
                      {{ rule.wallet_name || '默认账户' }}
                    </p>
                  </div>
                </div>

                <button
                  type="button"
                  class="text-[11px] px-2 py-1 rounded-lg shrink-0 transition"
                  :class="
                    confirmDeleteRecId === rule.id
                      ? 'bg-[#D6553E] text-white font-semibold'
                      : 'text-[#94A3B8] hover:text-[#D6553E]'
                  "
                  @click.stop="handleDeleteRecurring(rule.id)"
                >
                  {{ confirmDeleteRecId === rule.id ? '确认删除' : '删除' }}
                </button>
              </div>

              <div
                class="mt-3 pt-2.5 border-t border-[#F1F3F6] flex items-center justify-between text-[11px]"
                @click.stop
              >
                <span class="text-[#64748B] tabular-nums">
                  已执行 {{ rule.executed_count }} 笔 · 下一笔 {{ rule.next_date }}
                </span>
                <ToggleSwitch
                  :model-value="rule.is_active"
                  :aria-label="`切换${rule.title}定时记账`"
                  @change="handleToggleRecurring(rule)"
                />
              </div>
            </div>
          </div>
        </template>

        <!-- 新增 / 编辑定时规则表单 -->
        <form v-else class="space-y-3" @submit.prevent="submitRecurringForm">
          <div class="app-card p-3.5 space-y-3">
            <div class="flex items-center justify-between">
              <div class="inline-flex p-0.5 rounded-full bg-[#ECEEF2] text-xs font-semibold">
                <button
                  type="button"
                  class="px-3.5 py-1 rounded-full transition"
                  :class="
                    recurringForm.type === 'expense'
                      ? 'bg-white text-[#D6553E] shadow-xs'
                      : 'text-[#64748B]'
                  "
                  @click="switchRecurringType('expense')"
                >
                  支出
                </button>
                <button
                  type="button"
                  class="px-3.5 py-1 rounded-full transition"
                  :class="
                    recurringForm.type === 'income'
                      ? 'bg-white text-[#3E8E63] shadow-xs'
                      : 'text-[#64748B]'
                  "
                  @click="switchRecurringType('income')"
                >
                  收入
                </button>
              </div>
              <span class="text-xs font-semibold text-[#64748B]">
                {{ store.activeBook?.name || '当前账本' }}
              </span>
            </div>

            <CategoryPicker
              v-model="recurringForm.categoryId"
              v-model:sub-category="recurringForm.subCategory"
              :type="recurringForm.type"
              :tree="recurringCategoryTree"
              :columns="5"
            />
          </div>

          <div class="app-card divide-y divide-[#F1F3F6] text-xs">
            <div class="px-4 py-3 flex items-center justify-between">
              <span class="text-[#475569] font-medium">开始时间</span>
              <input
                v-model="recurringForm.startDate"
                type="date"
                class="text-right text-xs font-semibold text-[#18181B] bg-transparent outline-none"
              />
            </div>
            <div class="px-4 py-3 flex items-center justify-between gap-2">
              <span class="text-[#475569] font-medium shrink-0">重复周期</span>
              <div class="flex gap-1">
                <button
                  v-for="cy in RECURRING_CYCLES"
                  :key="cy"
                  type="button"
                  class="text-[11px] px-2.5 py-1 rounded-lg transition"
                  :class="
                    recurringForm.cycle === cy
                      ? 'bg-[#FFF6E8] text-[#DE8A1E] font-semibold'
                      : 'text-[#64748B] bg-[#F5F6F8]'
                  "
                  @click="recurringForm.cycle = cy"
                >
                  {{ cy }}
                </button>
              </div>
            </div>
          </div>

          <div class="app-card divide-y divide-[#F1F3F6] text-xs">
            <div class="px-4 py-3 flex items-center justify-between gap-3">
              <span class="text-[#475569] font-medium shrink-0">金额（元）</span>
              <input
                v-model="recurringForm.amountYuan"
                type="number"
                step="0.01"
                min="0.01"
                required
                placeholder="如 1850"
                class="w-40 text-right text-sm font-bold tabular-nums outline-none bg-transparent placeholder-[#94A3B8]"
                :class="recurringForm.type === 'income' ? 'text-[#3E8E63]' : 'text-[#DE8A1E]'"
              />
            </div>
            <div class="px-4 py-3 flex items-center justify-between gap-3">
              <span class="text-[#475569] font-medium shrink-0">备注名称</span>
              <input
                v-model="recurringForm.title"
                type="text"
                maxlength="20"
                placeholder="如：房租物业 / 会员续费"
                class="flex-1 min-w-0 text-right text-xs text-[#18181B] outline-none bg-transparent placeholder-[#94A3B8]"
              />
            </div>
            <div class="px-4 py-3 flex items-center justify-between gap-3">
              <span class="text-[#475569] font-medium shrink-0">资产账户</span>
              <CustomDropdown
                v-model="recurringForm.walletId"
                :options="walletDropdownOptions"
                placeholder="选择账户"
                width-class="w-44"
                size="sm"
              />
            </div>
          </div>

          <div class="app-card px-4 py-3 flex items-center justify-between text-xs">
            <span class="text-[#475569] font-medium">不计入月度收支统计</span>
            <ToggleSwitch v-model="recurringForm.excludeStats" aria-label="不计入月度收支统计" />
          </div>
        </form>
      </div>

      <!-- ==================== 6. 绑定 / 更换邮箱 ==================== -->
      <div v-else-if="page === 'bind-email'" class="space-y-3">
        <div class="app-card p-4 flex items-center justify-between gap-3">
          <div class="min-w-0">
            <p class="text-[11px] text-[#64748B]">当前绑定状态</p>
            <p class="text-xs font-bold text-[#18181B] mt-0.5 truncate">
              {{ store.user?.email_bound ? store.user.email : '尚未绑定安全邮箱' }}
            </p>
          </div>
          <span
            class="text-[10px] font-semibold px-2.5 py-1 rounded-full shrink-0"
            :class="
              store.user?.email_bound
                ? 'bg-[#ECF6F0] text-[#3E8E63]'
                : 'bg-[#FFF6E8] text-[#DE8A1E]'
            "
          >
            {{ store.user?.email_bound ? '已绑定' : '未绑定' }}
          </span>
        </div>

        <form class="app-card p-4 space-y-3" @submit.prevent="submitBindEmail">
          <div>
            <label class="block text-xs font-bold text-[#18181B] mb-1.5">
              {{ store.user?.email_bound ? '新邮箱地址' : '邮箱地址' }}
            </label>
            <input
              v-model="bindEmail"
              type="email"
              required
              placeholder="请输入常用邮箱，如 you@example.com"
              class="w-full text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 py-2.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
            />
          </div>

          <VerificationCodeField
            v-model="bindCode"
            label="邮箱验证码"
            placeholder="请输入 6 位数字验证码"
            :required="true"
            :cooldown="bindCooldown"
            :sending="sendingBindCode"
            :sent-hint="bindSentHint"
            variant="mobile"
            @send="handleSendBindCode"
          />

          <p class="text-[11px] text-[#94A3B8] leading-relaxed">
            绑定邮箱后可用于找回密码、接收家庭共享账本邀请及同步管理多个设备。
          </p>

          <button
            type="submit"
            :disabled="submittingBind"
            class="w-full py-2.5 rounded-xl bg-[#DE8A1E] text-white text-xs font-semibold shadow-sm hover:bg-[#C67612] disabled:opacity-60 transition"
          >
            {{
              submittingBind
                ? '提交中…'
                : store.user?.email_bound
                  ? '确认更换邮箱'
                  : '确认绑定'
            }}
          </button>
        </form>
      </div>

      <!-- ==================== 7. 导入与导出 CSV ==================== -->
      <div v-else-if="page === 'import-export'" class="space-y-3">
        <!-- 导出 CSV -->
        <div class="app-card p-4 space-y-2.5">
          <div class="flex items-center justify-between gap-2">
            <div>
              <h4 class="text-xs font-bold text-[#18181B]">导出当前账本 CSV</h4>
              <p class="text-[11px] text-[#64748B] mt-0.5">
                导出「{{ store.activeBook?.name || '当前账本' }}」全部流水明细，支持 Excel 打开
              </p>
            </div>
          </div>
          <button
            type="button"
            :disabled="exportingCsv"
            class="w-full py-2.5 rounded-xl bg-[#18181B] text-white text-xs font-semibold hover:bg-[#27272A] disabled:opacity-60 transition"
            @click="handleExportCsv"
          >
            {{ exportingCsv ? '正在生成 CSV…' : '导出账单 CSV 文件' }}
          </button>
        </div>

        <!-- 导入微信 / 支付宝 / 随手记账单 -->
        <div class="app-card p-4 space-y-3">
          <div>
            <h4 class="text-xs font-bold text-[#18181B]">导入微信 / 支付宝 / 随手记账单</h4>
            <p class="text-[11px] text-[#64748B] mt-0.5">
              上传平台导出的明细文件，解析预览后再批量入账
            </p>
          </div>

          <div class="p-1 rounded-2xl bg-[#ECEEF2] grid grid-cols-3 text-xs font-semibold">
            <button
              type="button"
              class="py-1.5 rounded-xl transition"
              :class="
                importPlatform === 'wechat'
                  ? 'bg-white text-[#3E8E63] shadow-xs'
                  : 'text-[#64748B]'
              "
              @click="importPlatform = 'wechat'"
            >
              微信账单 CSV
            </button>
            <button
              type="button"
              class="py-1.5 rounded-xl transition"
              :class="
                importPlatform === 'alipay'
                  ? 'bg-white text-[#2F78A8] shadow-xs'
                  : 'text-[#64748B]'
              "
              @click="importPlatform = 'alipay'"
            >
              支付宝账单 CSV
            </button>
            <button
              type="button"
              class="py-1.5 rounded-xl transition"
              :class="
                importPlatform === 'suishouji'
                  ? 'bg-white text-[#DE8A1E] shadow-xs'
                  : 'text-[#64748B]'
              "
              @click="importPlatform = 'suishouji'"
            >
              随手记 Excel / CSV
            </button>
          </div>

          <label
            class="cursor-pointer w-full py-3 rounded-xl border border-dashed border-[#CBD5E1] bg-[#F8F9FB] hover:border-[#DE8A1E] hover:bg-[#FFF6E8]/40 text-xs font-semibold text-[#475569] flex items-center justify-center gap-2 transition"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="17 8 12 3 7 8" />
              <line x1="12" y1="3" x2="12" y2="15" />
            </svg>
            <span>
              {{
                parsingImport
                  ? '正在解析文件…'
                  : importPlatform === 'suishouji'
                    ? '选择随手记 Excel / CSV 文件'
                    : `选择${importPlatform === 'wechat' ? '微信' : '支付宝'} CSV 文件`
              }}
            </span>
            <input
              type="file"
              :accept="importPlatform === 'suishouji' ? '.xlsx,.xls,.csv,text/csv' : '.csv,text/csv,text/plain'"
              class="hidden"
              :disabled="parsingImport"
              @change="handleImportFileChange"
            />
          </label>

          <!-- 导入解析预览 -->
          <div v-if="importedItems.length > 0" class="space-y-3 pt-2 border-t border-[#F1F3F6]">
            <div class="grid grid-cols-3 gap-2 text-center bg-[#F8F9FB] rounded-xl p-2.5">
              <div>
                <p class="text-[10px] text-[#94A3B8]">识别笔数</p>
                <p class="text-xs font-bold text-[#18181B] tabular-nums mt-0.5">
                  {{ importSummary.count }} 笔
                </p>
              </div>
              <div>
                <p class="text-[10px] text-[#94A3B8]">支出合计</p>
                <p class="text-xs font-bold text-[#D6553E] tabular-nums mt-0.5">
                  {{ formatMoney(importSummary.expense) }}
                </p>
              </div>
              <div>
                <p class="text-[10px] text-[#94A3B8]">收入合计</p>
                <p class="text-xs font-bold text-[#3E8E63] tabular-nums mt-0.5">
                  {{ formatMoney(importSummary.income) }}
                </p>
              </div>
            </div>

            <div class="flex items-center justify-between gap-2 text-xs">
              <span class="text-[#64748B] shrink-0">导入至账户</span>
              <CustomDropdown
                v-model="importWalletId"
                :options="walletDropdownOptions"
                placeholder="选择目标账户"
                width-class="w-48"
                size="sm"
              />
            </div>

            <div class="max-h-52 overflow-y-auto phone-scroll divide-y divide-[#F1F3F6] border border-[#ECEEF2] rounded-xl">
              <div
                v-for="(item, idx) in importedItems.slice(0, 30)"
                :key="idx"
                class="px-3 py-2 flex items-center justify-between gap-2 text-xs"
              >
                <div class="min-w-0">
                  <p class="font-semibold text-[#18181B] truncate">
                    {{ item.counterparty || item.product || '导入账单' }}
                  </p>
                  <p class="text-[10px] text-[#94A3B8] truncate">
                    {{ item.date }} · {{ item.product || item.note || '无摘要' }}
                  </p>
                </div>
                <span
                  class="font-bold tabular-nums shrink-0"
                  :class="item.type === 'income' ? 'text-[#3E8E63]' : 'text-[#D6553E]'"
                >
                  {{ formatMoney(item.amount, { showSign: true, type: item.type }) }}
                </span>
              </div>
            </div>

            <p v-if="importedItems.length > 30" class="text-[10px] text-center text-[#94A3B8]">
              仅预览前 30 条，提交时将导入全部 {{ importedItems.length }} 条记录
            </p>

            <div class="grid grid-cols-2 gap-2">
              <button
                type="button"
                class="py-2.5 rounded-xl bg-[#F5F6F8] text-xs font-semibold text-[#64748B]"
                @click="importedItems = []"
              >
                清空预览
              </button>
              <button
                type="button"
                :disabled="committingImport"
                class="py-2.5 rounded-xl bg-[#DE8A1E] text-white text-xs font-semibold hover:bg-[#C67612] disabled:opacity-60 transition"
                @click="handleCommitImport"
              >
                {{ committingImport ? '导入中…' : `确认导入 ${importedItems.length} 笔` }}
              </button>
            </div>
          </div>

          <!-- 随手记：字段映射 -->
          <div v-if="suishoujiResult" class="space-y-3 pt-2 border-t border-[#F1F3F6]">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-[#18181B]">字段映射（共 {{ suishoujiResult.total_rows }} 行）</span>
              <button
                v-if="suishoujiResult.ai_available"
                type="button"
                class="px-2.5 py-1.5 rounded-lg bg-[#FFF6E8] text-[#DE8A1E] text-[11px] font-semibold"
                @click="applySuishoujiAiSuggestions"
              >
                采用 AI 建议
              </button>
              <span v-else class="text-[10px] text-[#94A3B8]">
                {{ suishoujiResult.ai_reason === 'ai_limit' ? 'AI 今日调用已达上限' : '未配置 AI 模型' }}，请手动映射
              </span>
            </div>
            <div class="space-y-2">
              <div
                v-for="header in suishoujiResult.headers"
                :key="header"
                class="flex items-center gap-2 text-xs"
              >
                <span class="w-24 shrink-0 font-semibold text-[#18181B] truncate" :title="header">{{ header }}</span>
                <span class="text-[#94A3B8] shrink-0">→</span>
                <select
                  v-model="suishoujiMapping[header]"
                  class="flex-1 min-w-0 h-8 rounded-lg border border-[#E2E8F0] bg-white px-2 text-xs font-medium text-[#18181B] outline-none"
                  @change="refreshSuishoujiPreview"
                >
                  <option
                    v-for="opt in SUISHOUJI_TARGET_OPTIONS"
                    :key="opt.value"
                    :value="opt.value"
                  >
                    {{ opt.label }}
                  </option>
                </select>
              </div>
            </div>
            <p v-if="!suishoujiMappingComplete" class="text-[11px] text-[#D6553E]">
              日期和金额列必须映射后才能导入
            </p>

            <div class="flex items-center justify-between gap-2 text-xs">
              <span class="text-[#64748B] shrink-0">导入至账户</span>
              <CustomDropdown
                v-model="importWalletId"
                :options="walletDropdownOptions"
                placeholder="选择目标账户"
                width-class="w-48"
                size="sm"
              />
            </div>

            <!-- 随手记：预览前 10 行 -->
            <div class="max-h-52 overflow-y-auto phone-scroll divide-y divide-[#F1F3F6] border border-[#ECEEF2] rounded-xl">
              <div
                v-for="row in suishoujiPreview"
                :key="row.row_index"
                class="px-3 py-2 flex items-center justify-between gap-2 text-xs"
              >
                <div class="min-w-0">
                  <p class="font-semibold text-[#18181B] truncate">{{ row.title || '—' }}</p>
                  <p class="text-[10px] truncate" :class="row.valid ? 'text-[#94A3B8]' : 'text-[#D6553E]'">
                    {{ row.valid ? `${row.date} · ${row.category_name}${row.sub_category ? ' · ' + row.sub_category : ''} · ${row.wallet_name}` : row.error }}
                  </p>
                </div>
                <span
                  class="font-bold tabular-nums shrink-0"
                  :class="row.type === 'income' ? 'text-[#3E8E63]' : 'text-[#D6553E]'"
                >
                  {{ row.valid ? formatMoney(row.amount_cents, { showSign: true, type: row.type }) : '—' }}
                </span>
              </div>
              <p v-if="suishoujiPreviewLoading" class="py-2 text-center text-[11px] text-[#94A3B8]">刷新预览中…</p>
            </div>

            <!-- 随手记：导入结果 -->
            <div v-if="suishoujiCommitResult" class="rounded-xl bg-[#ECF6F0] px-3 py-2.5 text-xs">
              <p class="font-bold text-[#18181B]">
                导入完成：成功 {{ suishoujiCommitResult.inserted }} 条，重复跳过 {{ suishoujiCommitResult.duplicates }} 条，失败 {{ suishoujiCommitResult.failed }} 条
              </p>
              <p v-for="f in suishoujiCommitResult.failures" :key="f.row" class="text-[#D6553E] mt-1">
                第 {{ f.row }} 行：{{ f.reason }}
              </p>
            </div>

            <div class="grid grid-cols-2 gap-2">
              <button
                type="button"
                class="py-2.5 rounded-xl bg-[#F5F6F8] text-xs font-semibold text-[#64748B]"
                @click="suishoujiResult = null; suishoujiPreview = []; suishoujiCommitResult = null"
              >
                清空重选
              </button>
              <button
                type="button"
                :disabled="committingImport || !suishoujiMappingComplete"
                class="py-2.5 rounded-xl bg-[#DE8A1E] text-white text-xs font-semibold hover:bg-[#C67612] disabled:opacity-60 transition"
                @click="handleCommitImport"
              >
                {{ committingImport ? '导入中…' : `确认导入 ${suishoujiResult.total_rows} 笔` }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 定时记账底部固定操作栏 -->
    <template v-if="page === 'recurring'" #footer>
      <div class="shrink-0 bg-white/95 backdrop-blur-md border-t border-[#ECEEF2] px-4 py-2.5">
        <button
          v-if="recurringView === 'list'"
          type="button"
          class="w-full py-3 rounded-2xl bg-[#DE8A1E] text-white text-xs font-semibold shadow-sm hover:bg-[#C67612] transition"
          @click="openCreateRecurring"
        >
          + 添加定时记账
        </button>

        <div
          v-else
          class="grid gap-2"
          :class="editingRecurringId ? 'grid-cols-3' : 'grid-cols-2'"
        >
          <button
            v-if="editingRecurringId"
            type="button"
            :disabled="savingRecurring"
            class="py-3 rounded-2xl text-xs font-semibold transition"
            :class="
              confirmDeleteRecId === editingRecurringId
                ? 'bg-[#D6553E] text-white'
                : 'bg-[#FDF0EE] text-[#D6553E]'
            "
            @click="handleDeleteRecurring(editingRecurringId)"
          >
            {{ confirmDeleteRecId === editingRecurringId ? '确认删除' : '删除' }}
          </button>
          <button
            type="button"
            class="py-3 rounded-2xl bg-[#F5F6F8] text-xs font-semibold text-[#475569] hover:bg-[#ECEEF2] transition"
            @click="recurringView = 'list'"
          >
            返回列表
          </button>
          <button
            type="button"
            :disabled="savingRecurring"
            class="py-3 rounded-2xl bg-[#DE8A1E] text-white text-xs font-semibold shadow-sm hover:bg-[#C67612] disabled:opacity-60 transition"
            @click="submitRecurringForm"
          >
            {{
              savingRecurring
                ? '保存中…'
                : editingRecurringId
                  ? '保存修改'
                  : '确认添加'
            }}
          </button>
        </div>
      </div>
    </template>
  </BottomSheet>
</template>
