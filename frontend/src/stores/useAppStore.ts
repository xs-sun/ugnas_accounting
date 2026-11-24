import { reactive, computed, toRefs } from 'vue';
import {
  api,
  ApiError,
  clearGatewayAuthCache,
  getStoredAuthToken,
  setStoredAuthToken,
  setUnauthorizedHandler,
} from '@/api/client';
import { todayIso } from '@/utils/format';
import type {
  UserProfile,
  AuthStatusResponse,
  Book,
  Wallet,
  WalletSummary,
  Category,
  CategoryTreeNode,
  Tag,
  Transaction,
  TransactionListParams,
  CreateTransactionPayload,
  BudgetOverview,
  RecurringRule,
  StatsPeriod,
  StatsReport,
  DailyReportItem,
  AiChatMessage,
} from '@/types';

const ACTIVE_BOOK_STORAGE_KEY = 'cat7_active_book_id';
const PRIVACY_STORAGE_KEY = 'cat7_privacy_hidden';

export type ToastType = 'info' | 'success' | 'error';

export interface ToastState {
  visible: boolean;
  message: string;
  type: ToastType;
}

export type AvatarModalTarget = 'profile' | 'register' | string;

export interface AvatarModalState {
  open: boolean;
  target: AvatarModalTarget;
  currentAvatar: string;
  onSelect: ((avatar: string) => void) | null;
}

export interface AppState {
  initialized: boolean;
  loading: boolean;
  authRequired: boolean;
  authStatus: AuthStatusResponse | null;
  initError: string;
  user: UserProfile | null;
  books: Book[];
  activeBookId: string;
  wallets: Wallet[];
  walletSummary: WalletSummary;
  categories: Category[];
  tags: Tag[];
  transactions: Transaction[];
  transactionsTotal: number;
  budgets: BudgetOverview | null;
  recurringRules: RecurringRule[];
  stats: StatsReport | null;
  statsPeriod: StatsPeriod;
  statsType: 'expense' | 'income';
  dailyReport: DailyReportItem[];
  aiMessages: AiChatMessage[];
  privacyHidden: boolean;
  currentYear: number;
  currentMonth: number;
  toast: ToastState;
  avatarModal: AvatarModalState;
  adminPreviewAsUser: boolean;
}

function loadInitialPrivacy(): boolean {
  if (typeof localStorage === 'undefined') return false;
  return localStorage.getItem(PRIVACY_STORAGE_KEY) === '1';
}

function loadInitialBookId(): string {
  if (typeof localStorage === 'undefined') return '';
  return localStorage.getItem(ACTIVE_BOOK_STORAGE_KEY) || '';
}

function createDefaultWelcomeMessage(): AiChatMessage[] {
  return [
    {
      id: 'welcome_cat7',
      role: 'assistant',
      text: '你好！我是七猫记账助手小七。可以直接说“瑞幸拿铁 18.5 元，微信支付”或查账，也支持语音和拍小票记账哦～',
    },
  ];
}

const now = new Date();

const state = reactive<AppState>({
  initialized: false,
  loading: false,
  authRequired: false,
  authStatus: null,
  initError: '',
  user: null,
  books: [],
  activeBookId: loadInitialBookId(),
  wallets: [],
  walletSummary: { total_assets: 0, total_liabilities: 0, net_assets: 0 },
  categories: [],
  tags: [],
  transactions: [],
  transactionsTotal: 0,
  budgets: null,
  recurringRules: [],
  stats: null,
  statsPeriod: 'month',
  statsType: 'expense',
  dailyReport: [],
  aiMessages: createDefaultWelcomeMessage(),
  privacyHidden: loadInitialPrivacy(),
  currentYear: now.getFullYear(),
  currentMonth: now.getMonth() + 1,
  toast: {
    visible: false,
    message: '',
    type: 'info',
  },
  avatarModal: {
    open: false,
    target: 'profile',
    currentAvatar: '',
    onSelect: null,
  },
  adminPreviewAsUser: false,
});

let toastTimer: ReturnType<typeof setTimeout> | null = null;

function buildCategoryTree(
  categories: Category[],
  type: 'expense' | 'income'
): CategoryTreeNode[] {
  const scoped = categories.filter((c) => c.type === type);
  const sorted = [...scoped].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0));

  const map = new Map<string, CategoryTreeNode>();
  const roots: CategoryTreeNode[] = [];

  for (const cat of sorted) {
    map.set(cat.id, { ...cat, children: [] });
  }

  for (const cat of sorted) {
    const node = map.get(cat.id)!;
    if (cat.parent_id && map.has(cat.parent_id)) {
      map.get(cat.parent_id)!.children.push(node);
    } else if (!cat.parent_id) {
      roots.push(node);
    }
  }

  // 若存在未匹配到父级的孤立子分类，也作为一级兜底显示，避免丢失数据
  for (const cat of sorted) {
    if (cat.parent_id && !map.has(cat.parent_id)) {
      roots.push(map.get(cat.id)!);
    }
  }

  return roots;
}

const activeBook = computed<Book | null>(() => {
  if (!state.books.length) return null;
  return state.books.find((b) => b.id === state.activeBookId) || state.books[0] || null;
});

const expenseCategories = computed<CategoryTreeNode[]>(() =>
  buildCategoryTree(state.categories, 'expense')
);

const incomeCategories = computed<CategoryTreeNode[]>(() =>
  buildCategoryTree(state.categories, 'income')
);

const activeWallets = computed<Wallet[]>(() =>
  state.wallets.filter((w) => !w.is_archived)
);

const isRealAdmin = computed<boolean>(
  () => state.user?.role === 'admin' || state.user?.user_type === 'admin'
);

const isAdmin = computed<boolean>(
  () => isRealAdmin.value && !state.adminPreviewAsUser
);

const isAuthenticated = computed<boolean>(
  () => Boolean(state.user && !state.authRequired)
);

const firstInstall = computed<boolean>(
  () => Boolean(state.authStatus?.first_install)
);

function resolveBookId(bookId?: string): string {
  return bookId || state.activeBookId || state.books[0]?.id || '';
}

function showToast(
  message: string,
  type: ToastType = 'info',
  durationMs = 2300
): void {
  state.toast.message = message;
  state.toast.type = type;
  state.toast.visible = true;
  if (toastTimer) {
    clearTimeout(toastTimer);
  }
  toastTimer = setTimeout(() => {
    state.toast.visible = false;
    toastTimer = null;
  }, durationMs);
}

function togglePrivacy(): void {
  state.privacyHidden = !state.privacyHidden;
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(PRIVACY_STORAGE_KEY, state.privacyHidden ? '1' : '0');
  }
}

let initSeq = 0;
let activeInitPromise: Promise<void> | null = null;

function resetBookScopedState(): void {
  state.wallets = [];
  state.walletSummary = { total_assets: 0, total_liabilities: 0, net_assets: 0 };
  state.categories = [];
  state.tags = [];
  state.transactions = [];
  state.transactionsTotal = 0;
  state.budgets = null;
  state.recurringRules = [];
  state.stats = null;
  state.dailyReport = [];
}

function resetToUnauthenticated(): void {
  setStoredAuthToken(null);
  clearGatewayAuthCache();
  state.adminPreviewAsUser = false;
  state.user = null;
  state.books = [];
  state.activeBookId = '';
  resetBookScopedState();
  state.authRequired = true;
  state.initError = '';
  state.initialized = true;
  state.loading = false;
}

async function refreshAuthStatus(): Promise<AuthStatusResponse | null> {
  try {
    const status = await api.getAuthStatus();
    state.authStatus = status;
    return status;
  } catch {
    return null;
  }
}

setUnauthorizedHandler(() => {
  initSeq += 1;
  resetToUnauthenticated();
  void refreshAuthStatus();
});

async function refreshProfile(): Promise<UserProfile | null> {
  try {
    const profile = await api.getProfile();
    state.user = profile;
    state.authRequired = false;
    return profile;
  } catch (err: any) {
    if (err instanceof ApiError && err.status === 401) {
      resetToUnauthenticated();
    }
    return null;
  }
}

async function refreshBooks(): Promise<Book[]> {
  const books = await api.listBooks();
  state.books = books || [];
  if (state.books.length > 0) {
    const matched = state.books.some((b) => b.id === state.activeBookId);
    if (!matched) {
      state.activeBookId = state.books[0].id;
    }
    if (typeof localStorage !== 'undefined' && state.activeBookId) {
      localStorage.setItem(ACTIVE_BOOK_STORAGE_KEY, state.activeBookId);
    }
  } else {
    state.activeBookId = '';
  }
  return state.books;
}

async function refreshWallets(bookId?: string): Promise<void> {
  const bid = resolveBookId(bookId);
  if (!bid) return;
  const [walletsRes, summaryRes] = await Promise.allSettled([
    api.listWallets(bid),
    api.getWalletSummary(bid),
  ]);
  if (walletsRes.status === 'fulfilled') {
    state.wallets = walletsRes.value || [];
  }
  if (summaryRes.status === 'fulfilled' && summaryRes.value) {
    state.walletSummary = summaryRes.value;
  }
}

async function refreshCategories(bookId?: string): Promise<Category[]> {
  const bid = resolveBookId(bookId);
  if (!bid) return [];
  const list = await api.listCategories(bid);
  state.categories = list || [];
  return state.categories;
}

async function refreshTags(bookId?: string): Promise<Tag[]> {
  const bid = resolveBookId(bookId);
  if (!bid) return [];
  const list = await api.listTags(bid);
  state.tags = list || [];
  return state.tags;
}

async function refreshRecurring(bookId?: string): Promise<RecurringRule[]> {
  const bid = resolveBookId(bookId);
  if (!bid) return [];
  const list = await api.listRecurring(bid);
  state.recurringRules = list || [];
  return state.recurringRules;
}

async function refreshTransactions(
  params: Partial<TransactionListParams> = {}
): Promise<Transaction[]> {
  const bid = resolveBookId(params.book_id);
  if (!bid) return [];
  const res = await api.listTransactions({
    book_id: bid,
    page: 1,
    page_size: 200,
    ...params,
  });
  state.transactions = res.items || [];
  state.transactionsTotal = res.total ?? state.transactions.length;
  return state.transactions;
}

async function refreshBudgets(
  bookId?: string,
  year: number = state.currentYear,
  month: number = state.currentMonth
): Promise<BudgetOverview | null> {
  const bid = resolveBookId(bookId);
  if (!bid) return null;
  const overview = await api.getBudgets(bid, year, month);
  state.budgets = overview;
  return overview;
}

async function refreshStats(
  period: StatsPeriod = state.statsPeriod,
  type: 'expense' | 'income' = state.statsType,
  bookId?: string
): Promise<StatsReport | null> {
  const bid = resolveBookId(bookId);
  if (!bid) return null;
  state.statsPeriod = period;
  state.statsType = type;
  const report = await api.getStats({
    book_id: bid,
    period,
    type,
    year: state.currentYear,
    month: state.currentMonth,
  });
  state.stats = report;
  return report;
}

async function refreshDailyReport(
  year: number = state.currentYear,
  month: number = state.currentMonth,
  bookId?: string
): Promise<DailyReportItem[]> {
  const bid = resolveBookId(bookId);
  if (!bid) return [];
  const items = await api.getDailyReport(bid, year, month);
  state.dailyReport = items || [];
  return state.dailyReport;
}

async function refreshBookData(bookId?: string): Promise<void> {
  const bid = resolveBookId(bookId);
  if (!bid) return;
  await Promise.allSettled([
    refreshWallets(bid),
    refreshCategories(bid),
    refreshTags(bid),
    refreshTransactions({ book_id: bid }),
    refreshBudgets(bid, state.currentYear, state.currentMonth),
    refreshRecurring(bid),
    refreshStats(state.statsPeriod, state.statsType, bid),
    refreshDailyReport(state.currentYear, state.currentMonth, bid),
  ]);
}

async function initApp(force = false): Promise<void> {
  if (!force && activeInitPromise) {
    return activeInitPromise;
  }
  const currentSeq = ++initSeq;
  const run = (async () => {
    state.loading = true;
    state.initError = '';
    try {
      const token = getStoredAuthToken();
      if (!token) {
        await refreshAuthStatus();
        if (currentSeq !== initSeq) return;
        resetToUnauthenticated();
        return;
      }

      const profile = await api.getProfile();
      if (currentSeq !== initSeq) return;
      state.user = profile;
      state.authRequired = false;
      await refreshBooks();
      if (currentSeq !== initSeq) return;
      if (state.activeBookId) {
        await refreshBookData(state.activeBookId);
      }
      if (currentSeq !== initSeq) return;
      state.initialized = true;
    } catch (err: any) {
      if (currentSeq !== initSeq) return;
      if (err instanceof ApiError && err.status === 401) {
        await refreshAuthStatus();
        if (currentSeq !== initSeq) return;
        resetToUnauthenticated();
        return;
      }
      state.user = null;
      state.initError = err?.message || '连接服务失败，请检查网络或稍后重试';
      state.initialized = true;
    } finally {
      if (currentSeq === initSeq) {
        state.loading = false;
        activeInitPromise = null;
      }
    }
  })();

  activeInitPromise = run;
  return run;
}

async function switchBook(bookId: string): Promise<void> {
  if (!bookId || bookId === state.activeBookId) return;
  state.activeBookId = bookId;
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(ACTIVE_BOOK_STORAGE_KEY, bookId);
  }
  state.loading = true;
  try {
    await refreshBookData(bookId);
  } finally {
    state.loading = false;
  }
}

async function setCurrentMonth(year: number, month: number): Promise<void> {
  state.currentYear = year;
  state.currentMonth = month;
  const bid = resolveBookId();
  if (!bid) return;
  await Promise.allSettled([
    refreshBudgets(bid, year, month),
    refreshDailyReport(year, month, bid),
    refreshStats(state.statsPeriod, state.statsType, bid),
  ]);
}

async function syncAfterTxMutation(bookId?: string): Promise<void> {
  const bid = resolveBookId(bookId);
  if (!bid) return;
  await Promise.allSettled([
    refreshTransactions({ book_id: bid }),
    refreshWallets(bid),
    refreshBudgets(bid, state.currentYear, state.currentMonth),
    refreshStats(state.statsPeriod, state.statsType, bid),
    refreshDailyReport(state.currentYear, state.currentMonth, bid),
    refreshProfile(),
  ]);
}

async function createTransaction(
  payload: CreateTransactionPayload
): Promise<Transaction> {
  const bookId = resolveBookId(payload.book_id);
  const created = await api.createTransaction({
    ...payload,
    book_id: bookId,
    date: payload.date || todayIso(),
  });
  await syncAfterTxMutation(bookId);
  return created;
}

async function updateTransaction(
  id: string,
  payload: Partial<CreateTransactionPayload>
): Promise<Transaction> {
  const updated = await api.updateTransaction(id, payload);
  await syncAfterTxMutation(updated.book_id || state.activeBookId);
  return updated;
}

async function deleteTransaction(id: string): Promise<void> {
  await api.deleteTransaction(id);
  await syncAfterTxMutation(state.activeBookId);
}

async function batchDeleteTransactions(ids: string[]): Promise<number> {
  if (!ids.length) return 0;
  const res = await api.batchDeleteTransactions(ids);
  await syncAfterTxMutation(state.activeBookId);
  return res.deleted;
}

function openAvatarModal(
  target: AvatarModalTarget = 'profile',
  onSelect?: (avatar: string) => void,
  currentAvatar?: string
): void {
  state.avatarModal.target = target;
  state.avatarModal.onSelect = onSelect ?? null;
  state.avatarModal.currentAvatar = currentAvatar ?? state.user?.avatar ?? '';
  state.avatarModal.open = true;
}

function closeAvatarModal(): void {
  state.avatarModal.open = false;
  state.avatarModal.onSelect = null;
}

async function setAuthToken(token: string, user?: UserProfile): Promise<void> {
  setStoredAuthToken(token);
  if (user) {
    state.user = user;
  }
  state.authRequired = false;
  state.initError = '';
  state.adminPreviewAsUser = false;
  await initApp(true);
}

async function logoutToGuest(): Promise<void> {
  setStoredAuthToken(null);
  state.adminPreviewAsUser = false;
  state.user = null;
  state.books = [];
  state.activeBookId = '';
  resetBookScopedState();
  if (typeof localStorage !== 'undefined') {
    localStorage.removeItem(ACTIVE_BOOK_STORAGE_KEY);
  }
  await initApp(true);
}

function openAuthRequired(): void {
  state.authRequired = true;
  state.initError = '';
}

function appendAiMessage(msg: AiChatMessage): void {
  state.aiMessages.push(msg);
}

function clearAiMessages(): void {
  state.aiMessages = createDefaultWelcomeMessage();
}

function hasAuthToken(): boolean {
  return Boolean(getStoredAuthToken());
}

const appStore = reactive({
  ...toRefs(state),
  activeBook,
  expenseCategories,
  incomeCategories,
  activeWallets,
  isRealAdmin,
  isAdmin,
  isAuthenticated,
  firstInstall,
  initApp,
  refreshAuthStatus,
  switchBook,
  refreshProfile,
  refreshBooks,
  refreshBookData,
  refreshWallets,
  refreshCategories,
  refreshTags,
  refreshRecurring,
  refreshTransactions,
  refreshBudgets,
  refreshStats,
  refreshDailyReport,
  createTransaction,
  updateTransaction,
  deleteTransaction,
  batchDeleteTransactions,
  showToast,
  togglePrivacy,
  setCurrentMonth,
  openAvatarModal,
  closeAvatarModal,
  logoutToGuest,
  setAuthToken,
  openAuthRequired,
  appendAiMessage,
  clearAiMessages,
  hasAuthToken,
});

export type AppStore = typeof appStore;

export function useAppStore(): AppStore {
  return appStore;
}
