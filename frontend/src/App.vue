<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import type {
  Book,
  Wallet,
  WalletSummary,
  Category,
  Tag,
  Transaction,
  MonthlyReport,
  TrendReport,
  UserProfile,
} from './types';
import { api, resolveSession, setCustomUserSession } from './api/client';
import MorandiIcon from './components/MorandiIcon.vue';
import BreathingDonutChart from './components/BreathingDonutChart.vue';
import TrendBarChart from './components/TrendBarChart.vue';
import TransactionModal from './components/TransactionModal.vue';
import WalletModal from './components/WalletModal.vue';
import BookModal from './components/BookModal.vue';
import ImportExportModal from './components/ImportExportModal.vue';
import logoUrl from './assets/logo.png';

// Navigation state
type NavTab = 'transactions' | 'reports' | 'wallets' | 'profile' | 'settings';
const currentTab = ref<NavTab>('transactions');

// Core data
const userProfile = ref<UserProfile | null>(null);
const books = ref<Book[]>([]);
const currentBookId = ref<string>('');
const wallets = ref<Wallet[]>([]);
const walletSummary = ref<WalletSummary>({ total_assets: 0, total_liabilities: 0, net_assets: 0 });
const categories = ref<Category[]>([]);
const tags = ref<Tag[]>([]);
const transactions = ref<Transaction[]>([]);
const totalTransactions = ref(0);
const monthlyReport = ref<MonthlyReport | null>(null);
const trendReport = ref<TrendReport | null>(null);

// Filters
const filterType = ref<string>('');
const filterStartDate = ref<string>('');
const filterEndDate = ref<string>('');
const reportYear = ref(new Date().getFullYear());
const reportMonth = ref(new Date().getMonth() + 1);

// Modals
const showTxModal = ref(false);
const showWalletModal = ref(false);
const editingWallet = ref<Wallet | null>(null);
const showBookModal = ref(false);
const editingBook = ref<Book | null>(null);
const showImportExportModal = ref(false);
const showUserModal = ref(false);
const editUserId = ref('');
const editUserName = ref('');
const editUserType = ref('admin');

const openUserModal = () => {
  const sess = resolveSession();
  editUserId.value = userProfile.value?.user_id || sess.userId || 'ugos_admin';
  editUserName.value = userProfile.value?.user_name || sess.userName || '绿联管理员';
  editUserType.value = userProfile.value?.user_type || sess.userType || 'admin';
  showUserModal.value = true;
};

const handleSaveUserSession = async () => {
  if (!editUserId.value.trim()) {
    alert('用户 ID 不能为空');
    return;
  }
  setCustomUserSession(editUserId.value.trim(), editUserName.value.trim(), editUserType.value);
  showUserModal.value = false;
  await initApp();
};

// Loading & error
const loading = ref(false);
const backendOnline = ref(false);

const currentBook = computed(() => {
  return books.value.find((b) => b.id === currentBookId.value) || books.value[0] || null;
});

// Books created by current user
const myCreatedBooks = computed(() => {
  if (!userProfile.value?.user_id) {
    return books.value;
  }
  const uid = userProfile.value.user_id;
  return books.value.filter((b) => b.owner_id === uid || b.role === 'owner' || !b.role);
});

// Books joined / shared with current user
const myJoinedBooks = computed(() => {
  if (!userProfile.value?.user_id) {
    return [];
  }
  const uid = userProfile.value.user_id;
  return books.value.filter((b) => b.owner_id !== uid && b.role !== 'owner' && Boolean(b.role));
});

// Group transactions by date
const groupedTransactions = computed(() => {
  const groups: { date: string; items: Transaction[]; totalExpense: number; totalIncome: number }[] = [];
  const map = new Map<string, Transaction[]>();

  for (const tx of transactions.value) {
    if (!map.has(tx.date)) {
      map.set(tx.date, []);
    }
    map.get(tx.date)!.push(tx);
  }

  const sortedDates = Array.from(map.keys()).sort((a, b) => (a < b ? 1 : -1));

  for (const d of sortedDates) {
    const items = map.get(d)!;
    let expense = 0;
    let income = 0;
    for (const it of items) {
      if (it.type === 'expense') expense += it.amount;
      else if (it.type === 'income') income += it.amount;
    }
    groups.push({
      date: d,
      items,
      totalExpense: expense,
      totalIncome: income,
    });
  }

  return groups;
});

// Load all book-dependent data
const loadLedgerData = async () => {
  if (!currentBookId.value) return;
  loading.value = true;
  try {
    const [wList, wSum, cList, tList] = await Promise.all([
      api.listWallets(currentBookId.value),
      api.getWalletSummary(currentBookId.value),
      api.listCategories(currentBookId.value),
      api.listTags(currentBookId.value),
    ]);

    wallets.value = wList;
    walletSummary.value = wSum;
    categories.value = cList;
    tags.value = tList;

    await Promise.all([loadTransactions(), loadReports()]);
  } catch (err) {
    console.error('Failed to load ledger data', err);
  } finally {
    loading.value = false;
  }
};

const loadUserProfile = async () => {
  try {
    userProfile.value = await api.getProfile();
  } catch (err) {
    console.error('Failed to load user profile', err);
  }
};

const loadTransactions = async () => {
  if (!currentBookId.value) return;
  try {
    const res = await api.listTransactions({
      book_id: currentBookId.value,
      type: filterType.value || undefined,
      start_date: filterStartDate.value || undefined,
      end_date: filterEndDate.value || undefined,
      page: 1,
      page_size: 100,
    });
    transactions.value = res.items || [];
    totalTransactions.value = res.total || 0;
  } catch (err) {
    console.error('Failed to load transactions', err);
  }
};

const loadReports = async () => {
  if (!currentBookId.value) return;
  try {
    const [mRep, tRep] = await Promise.all([
      api.getMonthlyReport(currentBookId.value, reportYear.value, reportMonth.value),
      api.getTrendReport(currentBookId.value, 6),
    ]);
    monthlyReport.value = mRep;
    trendReport.value = tRep;
  } catch (err) {
    console.error('Failed to load reports', err);
  }
};

const initApp = async () => {
  try {
    const health = await api.getHealth();
    backendOnline.value = health.status === 'ok';

    await loadUserProfile();

    const bList = await api.listBooks();
    books.value = bList;

    if (bList.length > 0) {
      if (!currentBookId.value || !bList.some((b) => b.id === currentBookId.value)) {
        currentBookId.value = bList[0].id;
      }
      await loadLedgerData();
    } else {
      // Create initial default ledger
      const defaultBook = await api.createBook({
        name: '日常记账本',
        currency: 'CNY',
        description: '默认日常收支账本',
      });
      books.value = [defaultBook];
      currentBookId.value = defaultBook.id;
      await loadLedgerData();
    }
  } catch (err) {
    console.error('Init failed', err);
    backendOnline.value = false;
  }
};

const handleSelectBook = (bookId: string) => {
  currentBookId.value = bookId;
  loadLedgerData();
};

const handleDeleteBook = async (bookId: string) => {
  if (books.value.length <= 1) {
    alert('系统至少需保留一个账本，无法删除最后一个账本。');
    return;
  }
  const target = books.value.find((b) => b.id === bookId);
  const name = target ? target.name : '该账本';
  if (!confirm(`确定要删除「${name}」吗？\n删除后该账本下的所有账户、分类和交易明细将被永久清除，无法恢复！`)) {
    return;
  }

  try {
    await api.deleteBook(bookId);
    if (currentBookId.value === bookId) {
      const remaining = books.value.filter((b) => b.id !== bookId);
      currentBookId.value = remaining[0]?.id || '';
    }
    await initApp();
  } catch (err: any) {
    alert(err.message || '删除账本失败');
  }
};

const handleDeleteTransaction = async (txId: string) => {
  if (!confirm('确定删除该笔记录？账户余额将自动回退。')) return;
  try {
    await api.deleteTransaction(txId);
    await loadLedgerData();
    await loadUserProfile();
  } catch (err) {
    alert('删除失败');
  }
};

const openAddWallet = () => {
  editingWallet.value = null;
  showWalletModal.value = true;
};

const openEditWallet = (w: Wallet) => {
  editingWallet.value = w;
  showWalletModal.value = true;
};

const handleArchiveWallet = async (walletId: string) => {
  if (!confirm('确定归档此账户？归档后仍保留历史交易。')) return;
  try {
    await api.deleteWallet(walletId);
    await loadLedgerData();
  } catch (err) {
    alert('归档失败');
  }
};

onMounted(() => {
  initApp();
});
</script>

<template>
  <div class="min-h-screen bg-[#F6F7F8] text-[#171A20] font-sans antialiased flex flex-col md:flex-row select-none">
    <!-- DESKTOP SIDEBAR (>= 768px) -->
    <aside class="hidden md:flex md:w-64 bg-[#FFFFFF] border-r border-[#EDEEF1] flex-col justify-between p-5 shrink-0 h-screen sticky top-0">
      <div class="space-y-6">
        <!-- Brand Header -->
        <div class="flex items-center space-x-3 px-2">
          <img :src="logoUrl" alt="七喵记账" class="w-10 h-10 rounded-[11px] object-cover shadow-xs border border-[#EDEEF1]" />
          <div>
            <h1 class="text-base font-semibold text-[#171A20] tracking-tight">七喵记账</h1>
            <p class="text-[11px] text-[#9AA0AB] font-mono leading-none mt-0.5">UGOS Pro 原生版</p>
          </div>
        </div>

        <!-- Ledger Switcher Dropdown -->
        <div class="bg-[#F6F7F8] p-2 rounded-[14px] border border-[#EDEEF1] space-y-1.5">
          <div class="flex items-center justify-between px-1.5">
            <span class="text-[11px] font-medium text-[#5D6470]">当前账本</span>
            <button
              type="button"
              class="text-[11px] text-[#0E8C7F] hover:underline font-medium cursor-pointer"
              @click="editingBook = null; showBookModal = true"
            >
              + 新建
            </button>
          </div>
          <select
            :value="currentBookId"
            class="w-full h-8 px-2 text-xs bg-[#FFFFFF] rounded-[9px] border border-[#EDEEF1] text-[#171A20] font-medium outline-hidden cursor-pointer"
            @change="handleSelectBook(($event.target as HTMLSelectElement).value)"
          >
            <option v-for="b in books" :key="b.id" :value="b.id">
              {{ b.name }} {{ b.role ? `(${b.role})` : '' }}
            </option>
          </select>
        </div>

        <!-- Navigation Menu Items -->
        <nav class="space-y-1.5">
          <button
            type="button"
            class="w-full flex items-center space-x-3 px-3 py-2.5 rounded-[11px] text-xs font-medium transition-all duration-150 cursor-pointer"
            :class="currentTab === 'transactions' ? 'bg-[#E4F3F1] text-[#0E8C7F] font-semibold' : 'text-[#5D6470] hover:bg-[#F6F7F8] hover:text-[#171A20]'"
            @click="currentTab = 'transactions'"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <line x1="8" y1="6" x2="21" y2="6" />
              <line x1="8" y1="12" x2="21" y2="12" />
              <line x1="8" y1="18" x2="21" y2="18" />
              <line x1="3" y1="6" x2="3.01" y2="6" />
              <line x1="3" y1="12" x2="3.01" y2="12" />
              <line x1="3" y1="18" x2="3.01" y2="18" />
            </svg>
            <span>交易明细</span>
          </button>

          <button
            type="button"
            class="w-full flex items-center space-x-3 px-3 py-2.5 rounded-[11px] text-xs font-medium transition-all duration-150 cursor-pointer"
            :class="currentTab === 'reports' ? 'bg-[#E4F3F1] text-[#0E8C7F] font-semibold' : 'text-[#5D6470] hover:bg-[#F6F7F8] hover:text-[#171A20]'"
            @click="currentTab = 'reports'"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <path d="M21.21 15.89A10 10 0 1 1 8 2.83" />
              <path d="M22 12A10 10 0 0 0 12 2v10z" />
            </svg>
            <span>统计分析</span>
          </button>

          <button
            type="button"
            class="w-full flex items-center space-x-3 px-3 py-2.5 rounded-[11px] text-xs font-medium transition-all duration-150 cursor-pointer"
            :class="currentTab === 'wallets' ? 'bg-[#E4F3F1] text-[#0E8C7F] font-semibold' : 'text-[#5D6470] hover:bg-[#F6F7F8] hover:text-[#171A20]'"
            @click="currentTab = 'wallets'"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <rect x="2" y="4" width="20" height="16" rx="2" />
              <path d="M7 15h0M2 10h20" />
            </svg>
            <span>钱包资产</span>
          </button>

          <button
            type="button"
            class="w-full flex items-center space-x-3 px-3 py-2.5 rounded-[11px] text-xs font-medium transition-all duration-150 cursor-pointer"
            :class="currentTab === 'profile' ? 'bg-[#E4F3F1] text-[#0E8C7F] font-semibold' : 'text-[#5D6470] hover:bg-[#F6F7F8] hover:text-[#171A20]'"
            @click="currentTab = 'profile'"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
              <circle cx="12" cy="7" r="4" />
            </svg>
            <span>我的账本与中心</span>
          </button>

          <button
            type="button"
            class="w-full flex items-center space-x-3 px-3 py-2.5 rounded-[11px] text-xs font-medium transition-all duration-150 cursor-pointer"
            :class="currentTab === 'settings' ? 'bg-[#E4F3F1] text-[#0E8C7F] font-semibold' : 'text-[#5D6470] hover:bg-[#F6F7F8] hover:text-[#171A20]'"
            @click="showImportExportModal = true"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="17 8 12 3 7 8" />
              <line x1="12" y1="3" x2="12" y2="15" />
            </svg>
            <span>导入 / 备份快照</span>
          </button>
        </nav>
      </div>

      <!-- Sidebar Footer User / Status Info & Swagger Doc Link -->
      <div class="pt-4 border-t border-[#EDEEF1] space-y-2 text-[11px] text-[#9AA0AB]">
        <div class="flex items-center justify-between">
          <span class="flex items-center">
            <span class="w-1.5 h-1.5 rounded-full mr-1.5" :class="backendOnline ? 'bg-[#0E8C7F]' : 'bg-[#E5654E]'"></span>
            {{ backendOnline ? '服务就绪' : '未连接' }}
          </span>
          <span class="font-mono text-[#5D6470]">v0.1.7</span>
        </div>
        <div class="flex items-center justify-between pt-1">
          <a
            href="/swagger/"
            target="_blank"
            class="text-[#0E8C7F] hover:underline flex items-center gap-1 font-medium"
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
              <polyline points="14 2 14 8 20 8" />
              <line x1="16" y1="13" x2="8" y2="13" />
              <line x1="16" y1="17" x2="8" y2="17" />
              <polyline points="10 9 9 9 8 9" />
            </svg>
            API 文档 (Swagger) &nearr;
          </a>
        </div>
      </div>
    </aside>

    <!-- MAIN VIEWPORT CANVAS -->
    <main class="flex-1 flex flex-col min-w-0 pb-20 md:pb-8">
      <!-- Top Mobile Navigation Header (< 768px) -->
      <header class="md:hidden bg-[#FFFFFF] border-b border-[#EDEEF1] p-4 sticky top-0 z-30 flex items-center justify-between">
        <div class="flex items-center space-x-2.5">
          <img :src="logoUrl" alt="七喵记账" class="w-7 h-7 rounded-[7px] object-cover border border-[#EDEEF1]" />
          <select
            :value="currentBookId"
            class="text-sm font-semibold text-[#171A20] bg-transparent outline-hidden max-w-[150px] truncate"
            @change="handleSelectBook(($event.target as HTMLSelectElement).value)"
          >
            <option v-for="b in books" :key="b.id" :value="b.id">{{ b.name }}</option>
          </select>
        </div>

        <!-- Net Assets Summary Pill -->
        <div class="flex items-center space-x-2">
          <div class="text-right">
            <span class="text-[10px] text-[#9AA0AB] block leading-none">净资产</span>
            <span class="text-xs font-bold text-[#171A20] tabular-nums tracking-tightest">
              ¥{{ (walletSummary.net_assets / 100).toFixed(2) }}
            </span>
          </div>
        </div>
      </header>

      <!-- Content Area Container -->
      <div class="p-4 sm:p-6 lg:p-8 max-w-6xl w-full mx-auto space-y-6">
        <!-- Top Statistics & Quick Action Banner (Desktop / Large screen) -->
        <div v-if="currentTab !== 'profile'" class="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <!-- Net Asset Card -->
          <div class="bg-[#FFFFFF] rounded-[20px] p-5 border border-[#EDEEF1] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)]">
            <div class="flex items-center justify-between text-xs text-[#5D6470] mb-1">
              <span>总净资产</span>
              <span class="text-[11px] text-[#9AA0AB]">实时核算</span>
            </div>
            <div class="text-2xl font-bold text-[#171A20] tabular-nums tracking-tightest">
              ¥{{ (walletSummary.net_assets / 100).toFixed(2) }}
            </div>
            <div class="flex items-center space-x-3 text-[11px] text-[#9AA0AB] mt-2">
              <span>资产: ¥{{ (walletSummary.total_assets / 100).toFixed(2) }}</span>
              <span>负债: ¥{{ (walletSummary.total_liabilities / 100).toFixed(2) }}</span>
            </div>
          </div>

          <!-- Monthly Expense Card -->
          <div class="bg-[#FFFFFF] rounded-[20px] p-5 border border-[#EDEEF1] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)]">
            <div class="flex items-center justify-between text-xs text-[#5D6470] mb-1">
              <span>本月总支出</span>
              <span class="text-[11px] text-[#E5654E] font-medium">陶土橘</span>
            </div>
            <div class="text-2xl font-bold text-[#E5654E] tabular-nums tracking-tightest">
              ¥{{ ((monthlyReport?.total_expense || 0) / 100).toFixed(2) }}
            </div>
            <div class="text-[11px] text-[#9AA0AB] mt-2">
              本月结余: ¥{{ ((monthlyReport?.net_savings || 0) / 100).toFixed(2) }}
            </div>
          </div>

          <!-- Action Button Card: Strictly Deep Turquoise #0E8C7F -->
          <div class="bg-[#FFFFFF] rounded-[20px] p-5 border border-[#EDEEF1] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] flex flex-col justify-between">
            <div class="flex items-center justify-between text-xs text-[#5D6470]">
              <span>本月总收入</span>
              <span class="text-xs font-bold text-[#0E8C7F] tabular-nums tracking-tightest">
                ¥{{ ((monthlyReport?.total_income || 0) / 100).toFixed(2) }}
              </span>
            </div>
            <button
              type="button"
              class="w-full h-11 bg-[#0E8C7F] hover:bg-[#0B6E64] active:scale-[0.98] text-white font-medium text-sm rounded-full transition-all duration-150 flex items-center justify-center space-x-2 shadow-xs cursor-pointer mt-3"
              @click="showTxModal = true"
            >
              <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
                <line x1="12" y1="5" x2="12" y2="19" />
                <line x1="5" y1="12" x2="19" y2="12" />
              </svg>
              <span>记一笔</span>
            </button>
          </div>
        </div>

        <!-- ================= TAB 1: TRANSACTIONS LIST ================= -->
        <div v-if="currentTab === 'transactions'" class="space-y-4">
          <!-- Filters Toolbar -->
          <div class="flex flex-wrap items-center justify-between gap-3 bg-[#FFFFFF] p-3.5 rounded-[14px] border border-[#EDEEF1]">
            <!-- Type Filter Buttons -->
            <div class="flex items-center space-x-1">
              <button
                v-for="btn in [
                  { label: '全部', val: '' },
                  { label: '支出', val: 'expense' },
                  { label: '收入', val: 'income' },
                  { label: '转账', val: 'transfer' },
                ]"
                :key="btn.val"
                type="button"
                class="px-3 py-1 rounded-full text-xs font-medium transition-colors cursor-pointer"
                :class="filterType === btn.val ? 'bg-[#171A20] text-white' : 'text-[#5D6470] hover:text-[#171A20]'"
                @click="filterType = btn.val; loadTransactions()"
              >
                {{ btn.label }}
              </button>
            </div>

            <div class="flex items-center space-x-2 text-xs text-[#9AA0AB]">
              <span>共 {{ totalTransactions }} 笔记录</span>
            </div>
          </div>

          <!-- Empty State -->
          <div
            v-if="groupedTransactions.length === 0"
            class="bg-[#FFFFFF] rounded-[20px] p-12 border border-[#EDEEF1] text-center space-y-3"
          >
            <div class="w-12 h-12 rounded-full bg-[#F6F7F8] mx-auto flex items-center justify-center text-[#9AA0AB]">
              <svg class="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
                <rect x="2" y="5" width="20" height="14" rx="2" />
                <line x1="2" y1="10" x2="22" y2="10" />
              </svg>
            </div>
            <p class="text-sm text-[#5D6470]">当前账本尚无交易明细</p>
            <button
              type="button"
              class="px-4 py-1.5 bg-[#0E8C7F] text-white text-xs font-medium rounded-full cursor-pointer hover:bg-[#0B6E64]"
              @click="showTxModal = true"
            >
              立即记录第一笔收支
            </button>
          </div>

          <!-- Transactions Grouped by Date -->
          <div v-else class="space-y-4">
            <div
              v-for="group in groupedTransactions"
              :key="group.date"
              class="bg-[#FFFFFF] rounded-[20px] p-4 sm:p-5 border border-[#EDEEF1] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] space-y-3"
            >
              <!-- Group Date Header -->
              <div class="flex items-center justify-between pb-2 border-b border-[#EDEEF1] text-xs">
                <span class="font-mono font-medium text-[#171A20]">{{ group.date }}</span>
                <div class="flex items-center space-x-3 text-[#9AA0AB]">
                  <span v-if="group.totalExpense > 0">支: ¥{{ (group.totalExpense / 100).toFixed(2) }}</span>
                  <span v-if="group.totalIncome > 0" class="text-[#0E8C7F]">收: ¥{{ (group.totalIncome / 100).toFixed(2) }}</span>
                </div>
              </div>

              <!-- Transaction Row Items -->
              <div class="divide-y divide-[#EDEEF1]/60">
                <div
                  v-for="tx in group.items"
                  :key="tx.id"
                  class="flex items-center justify-between py-2.5 group"
                >
                  <!-- Left: Icon & Info -->
                  <div class="flex items-center space-x-3 truncate pr-4">
                    <MorandiIcon
                      :icon="tx.type === 'transfer' ? 'transfer' : tx.category_icon || tx.category_name || 'tag'"
                      :size="36"
                    />
                    <div class="truncate">
                      <div class="text-xs font-semibold text-[#171A20] truncate flex items-center space-x-2">
                        <span>{{ tx.type === 'transfer' ? '转账' : tx.category_name || '未分类' }}</span>
                        <span v-if="tx.note" class="text-[11px] font-normal text-[#5D6470] truncate">
                          {{ tx.note }}
                        </span>
                      </div>
                      <div class="flex items-center space-x-2 text-[11px] text-[#9AA0AB] mt-0.5 font-medium">
                        <span>{{ tx.wallet_name || '钱包' }}</span>
                        <span v-if="tx.to_wallet_name">➔ {{ tx.to_wallet_name }}</span>
                        <span
                          v-for="tg in tx.tags"
                          :key="tg.id"
                          class="bg-[#F6F7F8] px-1.5 py-0.2 rounded text-[10px]"
                        >
                          #{{ tg.name }}
                        </span>
                      </div>
                    </div>
                  </div>

                  <!-- Right: Amount & Actions -->
                  <div class="flex items-center space-x-3 shrink-0">
                    <span
                      class="text-sm font-bold tabular-nums tracking-tightest"
                      :class="
                        tx.type === 'income'
                          ? 'text-[#0E8C7F]'
                          : tx.type === 'expense'
                          ? 'text-[#E5654E]'
                          : 'text-[#171A20]'
                      "
                    >
                      {{ tx.type === 'income' ? '+' : tx.type === 'expense' ? '-' : '' }}¥{{ (tx.amount / 100).toFixed(2) }}
                    </span>

                    <!-- Hover Delete Button -->
                    <button
                      type="button"
                      class="opacity-0 group-hover:opacity-100 text-[11px] text-[#9AA0AB] hover:text-[#E5654E] transition-opacity cursor-pointer p-1"
                      title="删除记录"
                      @click="handleDeleteTransaction(tx.id)"
                    >
                      ✕
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- ================= TAB 2: REPORTS & ANALYTICS ================= -->
        <div v-else-if="currentTab === 'reports'" class="space-y-6">
          <!-- Month Selector Toolbar -->
          <div class="flex items-center justify-between bg-white p-4 rounded-[14px] border border-[#EDEEF1]">
            <span class="text-xs font-semibold text-[#171A20]">月度收支报表</span>
            <div class="flex items-center space-x-2">
              <input
                v-model="reportYear"
                type="number"
                class="w-16 h-8 px-2 text-xs bg-[#F6F7F8] rounded-[8px] border border-[#EDEEF1] text-[#171A20] font-mono text-center"
                @change="loadReports"
              />
              <span class="text-xs text-[#5D6470]">年</span>
              <input
                v-model="reportMonth"
                type="number"
                min="1"
                max="12"
                class="w-12 h-8 px-2 text-xs bg-[#F6F7F8] rounded-[8px] border border-[#EDEEF1] text-[#171A20] font-mono text-center"
                @change="loadReports"
              />
              <span class="text-xs text-[#5D6470]">月</span>
            </div>
          </div>

          <!-- Charts Row -->
          <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <!-- Breathing Donut Chart Card -->
            <div class="bg-white rounded-[20px] p-6 border border-[#EDEEF1] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] flex flex-col items-center">
              <h4 class="text-xs font-semibold text-[#171A20] self-start mb-4">支出分类占比 (呼吸环)</h4>
              <BreathingDonutChart
                :categories="monthlyReport?.categories || []"
                :total-expense="monthlyReport?.total_expense || 0"
              />
            </div>

            <!-- Category Ranking Progress Bars -->
            <div class="bg-white rounded-[20px] p-6 border border-[#EDEEF1] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] space-y-4">
              <h4 class="text-xs font-semibold text-[#171A20]">分类排行榜</h4>
              <div v-if="!monthlyReport?.categories || monthlyReport.categories.length === 0" class="h-44 flex items-center justify-center text-xs text-[#9AA0AB]">
                本月暂无支出记录
              </div>
              <div v-else class="space-y-3 max-h-64 overflow-y-auto pr-1">
                <div
                  v-for="cat in monthlyReport.categories"
                  :key="cat.category_id"
                  class="space-y-1.5"
                >
                  <div class="flex items-center justify-between text-xs">
                    <div class="flex items-center space-x-2">
                      <MorandiIcon :icon="cat.category_icon || cat.category_name" :size="24" />
                      <span class="font-medium text-[#171A20]">{{ cat.category_name }}</span>
                    </div>
                    <div class="flex items-center space-x-2">
                      <span class="text-[#9AA0AB]">{{ cat.percentage }}%</span>
                      <span class="font-bold tabular-nums text-[#171A20]">
                        ¥{{ (cat.amount / 100).toFixed(2) }}
                      </span>
                    </div>
                  </div>
                  <!-- Progress Bar with rounded tip -->
                  <div class="w-full h-1.5 bg-[#F6F7F8] rounded-full overflow-hidden">
                    <div
                      class="h-full bg-[#0E8C7F] rounded-full transition-all duration-300"
                      :style="{ width: `${Math.min(100, Math.max(2, cat.percentage))}%` }"
                    ></div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Multi-month Trend Bar Chart Card -->
          <div class="bg-white rounded-[20px] p-6 border border-[#EDEEF1] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] space-y-3">
            <div class="flex items-center justify-between">
              <h4 class="text-xs font-semibold text-[#171A20]">多月收支趋势 (6个月)</h4>
              <span class="text-[11px] text-[#9AA0AB]">实时聚合</span>
            </div>
            <TrendBarChart :months="trendReport?.months || []" />
          </div>
        </div>

        <!-- ================= TAB 3: WALLETS MANAGEMENT ================= -->
        <div v-else-if="currentTab === 'wallets'" class="space-y-6">
          <div class="flex items-center justify-between">
            <h3 class="text-sm font-semibold text-[#171A20]">钱包与资产账户</h3>
            <button
              type="button"
              class="px-4 py-1.5 bg-[#0E8C7F] hover:bg-[#0B6E64] text-white text-xs font-medium rounded-full cursor-pointer transition-colors"
              @click="openAddWallet"
            >
              + 添加账户
            </button>
          </div>

          <!-- Wallets Cards Grid -->
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            <div
              v-for="w in wallets"
              :key="w.id"
              class="bg-white rounded-[20px] p-5 border border-[#EDEEF1] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] flex flex-col justify-between space-y-4 hover:shadow-[0_2px_6px_rgba(23,26,32,0.04),0_14px_36px_-4px_rgba(23,26,32,0.07)] transition-all"
            >
              <div class="flex items-start justify-between">
                <div class="flex items-center space-x-3">
                  <MorandiIcon :icon="w.type" :size="40" />
                  <div>
                    <h4 class="text-sm font-semibold text-[#171A20]">{{ w.name }}</h4>
                    <span v-if="w.card_last_four" class="text-[11px] font-mono text-[#9AA0AB]">
                      尾号: {{ w.card_last_four }}
                    </span>
                  </div>
                </div>
                <span
                  v-if="w.exclude_net_asset"
                  class="text-[10px] text-[#9AA0AB] bg-[#F6F7F8] px-2 py-0.5 rounded-full"
                >
                  不计净资产
                </span>
              </div>

              <!-- Balance Display -->
              <div>
                <span class="text-[11px] text-[#5D6470] block mb-0.5">
                  {{ w.type === 'credit' ? '欠款金额' : '账户余额' }}
                </span>
                <div
                  class="text-2xl font-bold tabular-nums tracking-tightest"
                  :class="w.type === 'credit' && w.balance < 0 ? 'text-[#E5654E]' : 'text-[#171A20]'"
                >
                  ¥{{ (w.balance / 100).toFixed(2) }}
                </div>
              </div>

              <!-- Credit Card Details -->
              <div v-if="w.type === 'credit'" class="text-[11px] text-[#9AA0AB] flex justify-between pt-2 border-t border-[#EDEEF1]">
                <span>账单日: {{ w.billing_day }}号</span>
                <span>还款日: {{ w.repayment_day }}号</span>
                <span>额度: ¥{{ (w.credit_limit / 100).toFixed(0) }}</span>
              </div>

              <!-- Card Actions -->
              <div class="flex items-center justify-end space-x-3 pt-2 border-t border-[#EDEEF1] text-xs">
                <button
                  type="button"
                  class="text-[#0E8C7F] hover:underline cursor-pointer"
                  @click="openEditWallet(w)"
                >
                  编辑
                </button>
                <button
                  type="button"
                  class="text-[#9AA0AB] hover:text-[#E5654E] cursor-pointer"
                  @click="handleArchiveWallet(w.id)"
                >
                  归档
                </button>
              </div>
            </div>
          </div>
        </div>
        <!-- ================= TAB 4: PROFILE & LEDGER MANAGEMENT (我的) ================= -->
        <div v-else-if="currentTab === 'profile'" class="space-y-6">
          <!-- User Profile Banner Card -->
          <div class="bg-white rounded-[20px] p-6 border border-[#EDEEF1] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] space-y-6">
            <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <!-- Left: User Identity Info -->
              <div class="flex items-center space-x-4">
                <div class="w-14 h-14 rounded-full bg-[#E4F3F1] border-2 border-[#0E8C7F]/20 flex items-center justify-center text-[#0E8C7F] font-bold text-xl shadow-xs">
                  {{ (userProfile?.user_name || '喵')[0] }}
                </div>
                <div>
                  <div class="flex items-center space-x-2">
                    <h3 class="text-base font-bold text-[#171A20]">{{ userProfile?.user_name || '绿联用户' }}</h3>
                    <span
                      class="px-2 py-0.5 rounded-full text-[10px] font-medium"
                      :class="userProfile?.user_type === 'admin' ? 'bg-[#E4F3F1] text-[#0E8C7F]' : 'bg-[#F6F7F8] text-[#5D6470]'"
                    >
                      {{ userProfile?.user_type === 'admin' ? 'NAS 主管理员' : 'NAS 本地用户' }}
                    </span>
                  </div>
                  <p class="text-xs text-[#9AA0AB] font-mono mt-0.5">UID: {{ userProfile?.user_id || 'ugos_user' }}</p>
                </div>
              </div>

              <!-- Right: Quick Action Buttons -->
              <div class="flex items-center space-x-2">
                <button
                  type="button"
                  class="px-4 py-2 bg-[#0E8C7F] hover:bg-[#0B6E64] text-white text-xs font-medium rounded-full cursor-pointer transition-all shadow-xs"
                  @click="editingBook = null; showBookModal = true"
                >
                  + 创建新账本
                </button>
                <button
                  type="button"
                  class="px-3.5 py-2 bg-[#F6F7F8] hover:bg-[#EDEEF1] text-[#5D6470] text-xs font-medium rounded-full cursor-pointer transition-all"
                  @click="openUserModal"
                >
                  切换/设置身份
                </button>
                <button
                  type="button"
                  class="px-3.5 py-2 bg-[#F6F7F8] hover:bg-[#EDEEF1] text-[#5D6470] text-xs font-medium rounded-full cursor-pointer transition-all"
                  @click="initApp"
                >
                  刷新资料
                </button>
              </div>
            </div>

            <!-- Stats Overview Row -->
            <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 pt-4 border-t border-[#EDEEF1]">
              <div class="bg-[#F6F7F8] p-3.5 rounded-[14px]">
                <span class="text-[11px] text-[#5D6470] block">我创建的账本</span>
                <span class="text-lg font-bold text-[#171A20] tabular-nums mt-0.5 block">
                  {{ userProfile?.created_books_count ?? myCreatedBooks.length }}
                </span>
              </div>
              <div class="bg-[#F6F7F8] p-3.5 rounded-[14px]">
                <span class="text-[11px] text-[#5D6470] block">我参与的账本</span>
                <span class="text-lg font-bold text-[#0E8C7F] tabular-nums mt-0.5 block">
                  {{ userProfile?.joined_books_count ?? myJoinedBooks.length }}
                </span>
              </div>
              <div class="bg-[#F6F7F8] p-3.5 rounded-[14px]">
                <span class="text-[11px] text-[#5D6470] block">钱包账户总数</span>
                <span class="text-lg font-bold text-[#171A20] tabular-nums mt-0.5 block">
                  {{ userProfile?.total_wallets_count ?? wallets.length }}
                </span>
              </div>
              <div class="bg-[#F6F7F8] p-3.5 rounded-[14px]">
                <span class="text-[11px] text-[#5D6470] block">累计记账笔数</span>
                <span class="text-lg font-bold text-[#171A20] tabular-nums mt-0.5 block">
                  {{ userProfile?.total_transactions_count ?? totalTransactions }}
                </span>
              </div>
            </div>
          </div>

          <!-- Section 1: 我创建的账本 -->
          <div class="space-y-4">
            <div class="flex items-center justify-between">
              <div class="flex items-center space-x-2">
                <h4 class="text-sm font-semibold text-[#171A20]">我创建的账本</h4>
                <span class="px-2 py-0.5 rounded-full bg-[#E4F3F1] text-[#0E8C7F] text-[11px] font-medium">
                  {{ myCreatedBooks.length }}
                </span>
              </div>
              <button
                type="button"
                class="text-xs text-[#0E8C7F] hover:underline font-medium cursor-pointer"
                @click="editingBook = null; showBookModal = true"
              >
                + 新建账本
              </button>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div
                v-for="b in myCreatedBooks"
                :key="b.id"
                class="bg-white rounded-[20px] p-5 border shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] flex flex-col justify-between space-y-4 transition-all"
                :class="b.id === currentBookId ? 'border-[#0E8C7F]/60 ring-2 ring-[#0E8C7F]/10' : 'border-[#EDEEF1] hover:border-[#D1D5DB]'"
              >
                <div>
                  <div class="flex items-start justify-between">
                    <div class="flex items-center space-x-2.5">
                      <div class="w-9 h-9 rounded-[10px] bg-[#E4F3F1] text-[#0E8C7F] flex items-center justify-center font-bold text-sm">
                        {{ b.currency || '¥' }}
                      </div>
                      <div>
                        <h5 class="text-sm font-bold text-[#171A20] flex items-center gap-1.5">
                          <span>{{ b.name }}</span>
                          <span
                            v-if="b.id === currentBookId"
                            class="px-1.5 py-0.2 rounded text-[10px] bg-[#0E8C7F] text-white font-medium"
                          >
                            当前使用中
                          </span>
                        </h5>
                        <p class="text-[11px] text-[#9AA0AB] mt-0.5 line-clamp-1">
                          {{ b.description || '暂无描述' }}
                        </p>
                      </div>
                    </div>
                    <span class="px-2 py-0.5 rounded-full bg-[#F6F7F8] text-[#5D6470] text-[10px] font-medium">
                      所有者 (Owner)
                    </span>
                  </div>

                  <div class="mt-4 pt-3 border-t border-[#EDEEF1]/80 text-[11px] text-[#9AA0AB] flex items-center justify-between">
                    <span>货币: {{ b.currency || 'CNY' }}</span>
                    <span>创建于: {{ b.created_at ? b.created_at.substring(0, 10) : '近期' }}</span>
                  </div>
                </div>

                <!-- Book Action Buttons -->
                <div class="flex items-center justify-between pt-2 border-t border-[#EDEEF1] text-xs">
                  <button
                    v-if="b.id !== currentBookId"
                    type="button"
                    class="px-3 py-1 rounded-[8px] bg-[#E4F3F1] text-[#0E8C7F] font-medium hover:bg-[#D4ECE8] cursor-pointer transition-colors"
                    @click="handleSelectBook(b.id)"
                  >
                    切换至此账本
                  </button>
                  <span v-else class="text-[11px] text-[#0E8C7F] font-medium flex items-center gap-1">
                    <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                      <polyline points="20 6 9 17 4 12" />
                    </svg>
                    活跃状态
                  </span>

                  <div class="flex items-center space-x-2">
                    <button
                      type="button"
                      class="text-[#0E8C7F] hover:underline cursor-pointer"
                      @click="editingBook = b; showBookModal = true"
                    >
                      编辑 / 成员管理
                    </button>
                    <button
                      type="button"
                      class="text-[#9AA0AB] hover:text-[#E5654E] cursor-pointer ml-1"
                      @click="handleDeleteBook(b.id)"
                    >
                      删除
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Section 2: 我参与的账本 -->
          <div class="space-y-4">
            <div class="flex items-center space-x-2">
              <h4 class="text-sm font-semibold text-[#171A20]">我参与的共享账本</h4>
              <span class="px-2 py-0.5 rounded-full bg-[#F6F7F8] text-[#5D6470] text-[11px] font-medium">
                {{ myJoinedBooks.length }}
              </span>
            </div>

            <!-- Empty state for joined books -->
            <div
              v-if="myJoinedBooks.length === 0"
              class="bg-white rounded-[20px] p-6 border border-[#EDEEF1] text-center space-y-2"
            >
              <p class="text-xs text-[#5D6470]">暂无参与的他人共享账本</p>
              <p class="text-[11px] text-[#9AA0AB]">
                当其他绿联 NAS 用户在账本成员中添加您的用户 ID ({{ userProfile?.user_id || '当前用户' }}) 为协作者后，账本将自动在此呈现。
              </p>
            </div>

            <!-- Joined Books Cards Grid -->
            <div v-else class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div
                v-for="b in myJoinedBooks"
                :key="b.id"
                class="bg-white rounded-[20px] p-5 border shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] flex flex-col justify-between space-y-4"
                :class="b.id === currentBookId ? 'border-[#0E8C7F]/60 ring-2 ring-[#0E8C7F]/10' : 'border-[#EDEEF1]'"
              >
                <div>
                  <div class="flex items-start justify-between">
                    <div class="flex items-center space-x-2.5">
                      <div class="w-9 h-9 rounded-[10px] bg-[#F6F7F8] text-[#5D6470] flex items-center justify-center font-bold text-sm">
                        {{ b.currency || '¥' }}
                      </div>
                      <div>
                        <h5 class="text-sm font-bold text-[#171A20] flex items-center gap-1.5">
                          <span>{{ b.name }}</span>
                          <span
                            v-if="b.id === currentBookId"
                            class="px-1.5 py-0.2 rounded text-[10px] bg-[#0E8C7F] text-white font-medium"
                          >
                            当前使用中
                          </span>
                        </h5>
                        <p class="text-[11px] text-[#9AA0AB] mt-0.5">所有者 UID: {{ b.owner_id }}</p>
                      </div>
                    </div>
                    <span
                      class="px-2 py-0.5 rounded-full text-[10px] font-medium"
                      :class="b.role === 'editor' ? 'bg-[#FDF2E9] text-[#B36B39]' : 'bg-[#F6F7F8] text-[#5D6470]'"
                    >
                      {{ b.role === 'editor' ? '协作编辑' : '只读查看' }}
                    </span>
                  </div>

                  <p class="text-xs text-[#5D6470] mt-3 line-clamp-2">
                    {{ b.description || '无账本描述' }}
                  </p>
                </div>

                <div class="flex items-center justify-between pt-2 border-t border-[#EDEEF1] text-xs">
                  <button
                    v-if="b.id !== currentBookId"
                    type="button"
                    class="px-3 py-1 rounded-[8px] bg-[#E4F3F1] text-[#0E8C7F] font-medium hover:bg-[#D4ECE8] cursor-pointer"
                    @click="handleSelectBook(b.id)"
                  >
                    切换至此账本
                  </button>
                  <span v-else class="text-[11px] text-[#0E8C7F] font-medium">活跃状态</span>

                  <button
                    type="button"
                    class="text-[#0E8C7F] hover:underline cursor-pointer"
                    @click="editingBook = b; showBookModal = true"
                  >
                    查看成员
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Section 3: 快捷设置与系统状态 -->
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4 pt-2">
            <!-- Card 1: 导入与备份 -->
            <div class="bg-white rounded-[20px] p-5 border border-[#EDEEF1] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] space-y-3 flex flex-col justify-between">
              <div class="space-y-1.5">
                <div class="w-8 h-8 rounded-[9px] bg-[#E4F3F1] text-[#0E8C7F] flex items-center justify-center">
                  <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                    <polyline points="17 8 12 3 7 8" />
                    <line x1="12" y1="3" x2="12" y2="15" />
                  </svg>
                </div>
                <h5 class="text-xs font-bold text-[#171A20]">数据导入与备份快照</h5>
                <p class="text-[11px] text-[#9AA0AB] leading-relaxed">
                  支持微信/支付宝账单 CSV 智能解析导入，并提供 SQLite 全量热备份快照与一键恢复。
                </p>
              </div>
              <button
                type="button"
                class="w-full h-8 bg-[#F6F7F8] hover:bg-[#EDEEF1] text-[#171A20] text-xs font-medium rounded-[9px] transition-colors cursor-pointer"
                @click="showImportExportModal = true"
              >
                打开备份与导入管理
              </button>
            </div>

            <!-- Card 2: Swagger API 文档 -->
            <div class="bg-white rounded-[20px] p-5 border border-[#EDEEF1] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] space-y-3 flex flex-col justify-between">
              <div class="space-y-1.5">
                <div class="w-8 h-8 rounded-[9px] bg-[#E4F3F1] text-[#0E8C7F] flex items-center justify-center">
                  <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
                    <polyline points="14 2 14 8 20 8" />
                    <line x1="16" y1="13" x2="8" y2="13" />
                    <line x1="16" y1="17" x2="8" y2="17" />
                  </svg>
                </div>
                <h5 class="text-xs font-bold text-[#171A20]">API 接口交互文档 (Swagger)</h5>
                <p class="text-[11px] text-[#9AA0AB] leading-relaxed">
                  遵循 OpenAPI 3.0 标准规范，提供完整的账本、交易、资产、报表与快照接口文档。
                </p>
              </div>
              <a
                href="/swagger/"
                target="_blank"
                class="w-full h-8 bg-[#0E8C7F] hover:bg-[#0B6E64] text-white text-xs font-medium rounded-[9px] flex items-center justify-center transition-colors cursor-pointer"
              >
                浏览交互式 Swagger UI &nearr;
              </a>
            </div>

            <!-- Card 3: 隐私与本地存储安全 -->
            <div class="bg-white rounded-[20px] p-5 border border-[#EDEEF1] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] space-y-3 flex flex-col justify-between">
              <div class="space-y-1.5">
                <div class="w-8 h-8 rounded-[9px] bg-[#E4F3F1] text-[#0E8C7F] flex items-center justify-center">
                  <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                  </svg>
                </div>
                <h5 class="text-xs font-bold text-[#171A20]">本地私有化安全保障</h5>
                <p class="text-[11px] text-[#9AA0AB] leading-relaxed">
                  100% 存储于绿联 NAS 本地 SQLite WAL 数据库，零云端上传，保证全家财务数据绝对私密。
                </p>
              </div>
              <div class="text-[11px] text-[#0E8C7F] font-medium flex items-center space-x-1.5">
                <span class="w-2 h-2 rounded-full bg-[#0E8C7F]"></span>
                <span>本地运行中 · WAL 安全事务</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- MOBILE BOTTOM NAVIGATION TABBAR (< 768px) -->
    <nav class="md:hidden fixed bottom-0 inset-x-0 bg-[#FFFFFF] border-t border-[#EDEEF1] z-40 px-6 py-2 flex items-center justify-between shadow-lg">
      <button
        type="button"
        class="flex flex-col items-center justify-center cursor-pointer"
        :class="currentTab === 'transactions' ? 'text-[#0E8C7F]' : 'text-[#9AA0AB]'"
        @click="currentTab = 'transactions'"
      >
        <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
          <line x1="8" y1="6" x2="21" y2="6" />
          <line x1="8" y1="12" x2="21" y2="12" />
          <line x1="8" y1="18" x2="21" y2="18" />
        </svg>
        <span class="text-[10px] mt-0.5">明细</span>
      </button>

      <button
        type="button"
        class="flex flex-col items-center justify-center cursor-pointer"
        :class="currentTab === 'reports' ? 'text-[#0E8C7F]' : 'text-[#9AA0AB]'"
        @click="currentTab = 'reports'"
      >
        <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
          <path d="M21.21 15.89A10 10 0 1 1 8 2.83" />
        </svg>
        <span class="text-[10px] mt-0.5">统计</span>
      </button>

      <!-- Center Floating Add Button: Strictly #0E8C7F -->
      <button
        type="button"
        class="w-12 h-12 rounded-full bg-[#0E8C7F] text-white flex items-center justify-center shadow-md active:scale-95 transition-transform -mt-5 cursor-pointer"
        @click="showTxModal = true"
      >
        <svg class="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4">
          <line x1="12" y1="5" x2="12" y2="19" />
          <line x1="5" y1="12" x2="19" y2="12" />
        </svg>
      </button>

      <button
        type="button"
        class="flex flex-col items-center justify-center cursor-pointer"
        :class="currentTab === 'wallets' ? 'text-[#0E8C7F]' : 'text-[#9AA0AB]'"
        @click="currentTab = 'wallets'"
      >
        <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
          <rect x="2" y="4" width="20" height="16" rx="2" />
          <path d="M2 10h20" />
        </svg>
        <span class="text-[10px] mt-0.5">资产</span>
      </button>

      <button
        type="button"
        class="flex flex-col items-center justify-center cursor-pointer"
        :class="currentTab === 'profile' ? 'text-[#0E8C7F]' : 'text-[#9AA0AB]'"
        @click="currentTab = 'profile'"
      >
        <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
          <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
          <circle cx="12" cy="7" r="4" />
        </svg>
        <span class="text-[10px] mt-0.5">我的</span>
      </button>
    </nav>

    <!-- Modals -->
    <TransactionModal
      :show="showTxModal"
      :book-id="currentBookId"
      :wallets="wallets"
      :categories="categories"
      :tags="tags"
      @close="showTxModal = false"
      @saved="loadLedgerData(); loadUserProfile();"
    />

    <WalletModal
      :show="showWalletModal"
      :book-id="currentBookId"
      :wallet="editingWallet"
      @close="showWalletModal = false"
      @saved="loadLedgerData(); loadUserProfile();"
    />

    <BookModal
      :show="showBookModal"
      :book="editingBook"
      @close="showBookModal = false"
      @saved="initApp"
    />

    <ImportExportModal
      :show="showImportExportModal"
      :book-id="currentBookId"
      :wallets="wallets"
      @close="showImportExportModal = false"
      @imported="loadLedgerData(); loadUserProfile();"
    />

    <!-- User Identity & Switch Modal -->
    <div
      v-if="showUserModal"
      class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4 bg-black/30 backdrop-blur-[2px] transition-opacity"
    >
      <div
        class="w-full sm:max-w-md bg-[#FFFFFF] rounded-t-[20px] sm:rounded-[20px] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] border border-[#EDEEF1] flex flex-col overflow-hidden"
      >
        <div class="px-6 py-4 flex items-center justify-between border-b border-[#EDEEF1]">
          <h3 class="text-base font-semibold text-[#171A20]">切换 / 设置当前用户身份</h3>
          <button
            type="button"
            class="w-8 h-8 rounded-full flex items-center justify-center text-[#9AA0AB] hover:text-[#171A20] hover:bg-[#F6F7F8] cursor-pointer"
            @click="showUserModal = false"
          >
            ✕
          </button>
        </div>

        <div class="p-6 space-y-4 max-h-[80vh] overflow-y-auto text-xs">
          <div class="bg-[#E4F3F1]/70 p-3 rounded-[12px] text-[#0E8C7F] leading-relaxed">
            七喵记账支持绿联 NAS 网关鉴权自动识别、URL 参数注入（<code>?user_id=...</code>）、JWT 解密及自定义身份测试。
          </div>

          <div class="space-y-1">
            <label class="text-xs font-medium text-[#5D6470]">用户 UID (唯一识别码)</label>
            <input
              v-model="editUserId"
              type="text"
              placeholder="例如: 10001, tao7, alice, admin..."
              class="w-full h-10 px-3 bg-white rounded-[11px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
            />
          </div>

          <div class="space-y-1">
            <label class="text-xs font-medium text-[#5D6470]">用户昵称</label>
            <input
              v-model="editUserName"
              type="text"
              placeholder="例如: 绿联管理员, 小喵, Alice..."
              class="w-full h-10 px-3 bg-white rounded-[11px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
            />
          </div>

          <div class="space-y-1">
            <label class="text-xs font-medium text-[#5D6470]">系统身份权限</label>
            <select
              v-model="editUserType"
              class="w-full h-10 px-3 bg-white rounded-[11px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
            >
              <option value="admin">NAS 管理员 (admin)</option>
              <option value="users">NAS 普通用户 (users)</option>
            </select>
          </div>
        </div>

        <div class="p-6 pt-3 border-t border-[#EDEEF1]">
          <button
            type="button"
            class="w-full h-11 bg-[#0E8C7F] hover:bg-[#0B6E64] active:scale-[0.98] text-white font-medium text-xs rounded-full transition-all duration-150 flex items-center justify-center space-x-2 cursor-pointer shadow-xs"
            @click="handleSaveUserSession"
          >
            保存并立即生效
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
