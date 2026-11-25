import { reactive, computed, toRefs, watch } from 'vue';
import {
  api,
  ApiError,
  clearGatewayAuthCache,
  getStoredAuthToken,
  getStoredRefreshToken,
  refreshAccessToken,
  setStoredAuthToken,
  setStoredRefreshToken,
  setUnauthorizedHandler,
} from '@/api/client';
import { todayIso, formatMoney } from '@/utils/format';
import {
  startLedgerStream,
  stopLedgerStream,
  type LedgerChangeType,
  type LedgerStreamEvent,
} from '@/composables/useLedgerStream';
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
  BookActivity,
  BudgetAlertPayload,
} from '@/types';

const ACTIVE_BOOK_STORAGE_KEY = 'cat7_active_book_id';
const PRIVACY_STORAGE_KEY = 'cat7_privacy_hidden';
// AI 聊天记录按账本存本地（localStorage），键名后跟账本 id。
const AI_MESSAGES_STORAGE_PREFIX = 'cat7_ai_messages_';
// 每个账本最多保留的聊天条数（防 localStorage 爆掉）。
const AI_MESSAGES_MAX_KEPT = 50;

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
  activities: BookActivity[];
  activitiesHasMore: boolean;
  activitiesNextBefore: string;
  activitiesLoading: boolean;
  latestBudgetAlert: BudgetAlertPayload | null;
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
  activities: [],
  activitiesHasMore: false,
  activitiesNextBefore: '',
  activitiesLoading: false,
  latestBudgetAlert: null,
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
  state.activities = [];
  state.activitiesHasMore = false;
  state.activitiesNextBefore = '';
  state.latestBudgetAlert = null;
}

function resetToUnauthenticated(): void {
  stopLedgerStream();
  setStoredAuthToken(null);
  setStoredRefreshToken(null);
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

async function refreshBooks(opts?: { preferDefault?: boolean }): Promise<Book[]> {
  const books = await api.listBooks();
  state.books = books || [];
  if (state.books.length > 0) {
    const defaultBook =
      state.books.find((b) => b.is_default) ||
      (state.user?.default_book_id
        ? state.books.find((b) => b.id === state.user?.default_book_id)
        : undefined);
    if (opts?.preferDefault && defaultBook) {
      state.activeBookId = defaultBook.id;
    } else {
      const matched = state.books.some((b) => b.id === state.activeBookId);
      if (!matched) {
        state.activeBookId = defaultBook?.id || state.books[0].id;
      }
    }
    if (typeof localStorage !== 'undefined' && state.activeBookId) {
      localStorage.setItem(ACTIVE_BOOK_STORAGE_KEY, state.activeBookId);
    }
    // 首次打开 / 账本列表刷新后，读当前账本的聊天记录。
    loadAiMessagesForBook(state.activeBookId);
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

// 账本动态：reset=true 拉最新一页（替换列表），否则用游标翻下一页（追加）。
async function refreshActivities(
  bookId?: string,
  opts: { reset?: boolean } = {}
): Promise<BookActivity[]> {
  const bid = resolveBookId(bookId);
  if (!bid) return [];
  if (state.activitiesLoading) return state.activities;
  const reset = opts.reset !== false;
  state.activitiesLoading = true;
  try {
    const res = await api.listActivities(bid, {
      limit: 20,
      before: reset ? undefined : state.activitiesNextBefore || undefined,
    });
    if (reset) {
      state.activities = res.items || [];
    } else {
      state.activities = [...state.activities, ...(res.items || [])];
    }
    state.activitiesHasMore = res.has_more;
    state.activitiesNextBefore = res.next_before || '';
    return state.activities;
  } finally {
    state.activitiesLoading = false;
  }
}

async function loadMoreActivities(): Promise<BookActivity[]> {
  if (!state.activitiesHasMore || state.activitiesLoading) return state.activities;
  return refreshActivities(undefined, { reset: false });
}

function dismissBudgetAlert(): void {
  state.latestBudgetAlert = null;
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
    refreshActivities(bid),
  ]);
}

async function initApp(
  force = false,
  opts?: { preferDefaultBook?: boolean }
): Promise<void> {
  if (!force && activeInitPromise) {
    return activeInitPromise;
  }
  const shouldUseDefaultBook = opts?.preferDefaultBook ?? !state.initialized;
  const currentSeq = ++initSeq;
  const run = (async () => {
    state.loading = true;
    state.initError = '';
    try {
      let token = getStoredAuthToken();
      const refreshTok = getStoredRefreshToken();
      if (!token && refreshTok) {
        const refreshed = await refreshAccessToken();
        if (currentSeq !== initSeq) return;
        if (refreshed.ok) {
          token = getStoredAuthToken();
        } else if (!refreshed.authInvalid) {
          state.user = null;
          state.initError = '连接服务失败，请检查网络或稍后重试';
          state.initialized = true;
          return;
        }
      }
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
      // 若老版本升级上来尚无 refresh_token，后台静默补领一个 180 天 refresh_token
      if (!getStoredRefreshToken()) {
        void refreshAccessToken();
      }
      await refreshBooks({ preferDefault: shouldUseDefaultBook });
      if (currentSeq !== initSeq) return;
      if (state.activeBookId) {
        await refreshBookData(state.activeBookId);
        restartLedgerStream();
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

async function setDefaultBook(bookId: string): Promise<void> {
  if (!bookId) return;
  await api.setDefaultBook(bookId);
  if (state.user) {
    state.user = { ...state.user, default_book_id: bookId };
  }
  state.books = state.books.map((b) => ({
    ...b,
    is_default: b.id === bookId,
  }));
}

async function clearDefaultBook(): Promise<void> {
  await api.clearDefaultBook();
  if (state.user) {
    state.user = { ...state.user, default_book_id: '' };
  }
  state.books = state.books.map((b) => ({
    ...b,
    is_default: false,
  }));
}

async function switchBook(bookId: string): Promise<void> {
  if (!bookId || bookId === state.activeBookId) return;
  state.activeBookId = bookId;
  state.latestBudgetAlert = null;
  // 切账本时读该账本的聊天记录（各账本分开存）。
  loadAiMessagesForBook(bookId);
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(ACTIVE_BOOK_STORAGE_KEY, bookId);
  }
  state.loading = true;
  try {
    await refreshBookData(bookId);
    restartLedgerStream();
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

// 收到 SSE 事件后按 change 类型定向刷新；connected（初连/重连）时全量刷新一次做同步。
// 任何变更都可能产生一条动态，所以每次都刷新动态列表；
// budget-alert 单独处理：弹 toast + 记下来给预算视图打标记 + 刷新预算。
function handleLedgerStreamEvents(events: LedgerStreamEvent[]): void {
  const bid = resolveBookId();
  if (!bid) return;
  const changes = new Set(
    events.map((e) => e.change).filter((c): c is LedgerChangeType => Boolean(c)),
  );
  const jobs: Array<Promise<unknown>> = [];
  const budgetAlert = events.find((e) => e.type === 'budget-alert' && e.budgetAlert);
  if (budgetAlert?.budgetAlert) {
    const alert = budgetAlert.budgetAlert;
    state.latestBudgetAlert = alert;
    showToast(
      `${alert.budgetName}预算已使用 ${alert.level}%（${formatMoney(alert.used)} / ${formatMoney(alert.budget)}）`,
      'info'
    );
    jobs.push(refreshBudgets(bid, state.currentYear, state.currentMonth));
  }
  jobs.push(refreshActivities(bid));
  if (changes.has('transaction') || changes.has('recurring')) {
    jobs.push(syncAfterTxMutation(bid));
  }
  if (changes.has('wallet')) {
    jobs.push(
      Promise.allSettled([
        refreshWallets(bid),
        refreshStats(state.statsPeriod, state.statsType, bid),
      ]),
    );
  }
  if (changes.has('budget')) {
    jobs.push(
      Promise.allSettled([
        refreshBudgets(bid, state.currentYear, state.currentMonth),
        refreshStats(state.statsPeriod, state.statsType, bid),
      ]),
    );
  }
  if (changes.has('category')) {
    jobs.push(refreshCategories(bid));
  }
  if (changes.has('member') || changes.has('book')) {
    jobs.push(Promise.allSettled([refreshBooks(), refreshBookData(bid)]));
  }
  if (jobs.length === 0) {
    jobs.push(refreshBookData(bid));
  }
  void Promise.allSettled(jobs);
}

function restartLedgerStream(): void {
  const bid = state.activeBookId;
  if (!bid || !state.user) {
    stopLedgerStream();
    return;
  }
  startLedgerStream(bid, handleLedgerStreamEvents, () => {
    void refreshBookData(bid);
  });
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

async function setAuthToken(
  token: string,
  user?: UserProfile,
  refreshToken?: string
): Promise<void> {
  setStoredAuthToken(token);
  if (refreshToken !== undefined) {
    setStoredRefreshToken(refreshToken);
  }
  if (user) {
    state.user = user;
  }
  state.authRequired = false;
  state.initError = '';
  state.adminPreviewAsUser = false;
  await initApp(true, { preferDefaultBook: true });
}

async function logoutToGuest(): Promise<void> {
  stopLedgerStream();
  const refreshTok = getStoredRefreshToken();
  setStoredAuthToken(null);
  setStoredRefreshToken(null);
  if (refreshTok) {
    void api.logout(refreshTok);
  }
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
  persistAiMessages();
}

function clearAiMessages(): void {
  state.aiMessages = createDefaultWelcomeMessage();
  if (typeof localStorage !== 'undefined' && state.activeBookId) {
    try {
      localStorage.removeItem(AI_MESSAGES_STORAGE_PREFIX + state.activeBookId);
    } catch {
      // ignore
    }
  }
}

function aiMessagesStorageKey(bookId: string): string {
  return AI_MESSAGES_STORAGE_PREFIX + bookId;
}

// 聊天记录落盘：文字、草稿、OCR 文本、已入账状态都存；
// 图片是 data URL（单张几百 KB），不存，避免 localStorage 爆掉。
function serializeAiMessages(messages: AiChatMessage[]): AiChatMessage[] {
  return messages.slice(-AI_MESSAGES_MAX_KEPT).map((m) => {
    const { image: _image, editing: _editing, ...rest } = m;
    void _image;
    void _editing;
    return rest as AiChatMessage;
  });
}

function persistAiMessages(): void {
  if (typeof localStorage === 'undefined' || !state.activeBookId) return;
  const key = aiMessagesStorageKey(state.activeBookId);
  try {
    localStorage.setItem(key, JSON.stringify(serializeAiMessages(state.aiMessages)));
  } catch {
    // 空间不足时只留最近 20 条再试一次，还不行就放弃（不影响记账本身）。
    try {
      localStorage.setItem(
        key,
        JSON.stringify(serializeAiMessages(state.aiMessages).slice(-20))
      );
    } catch {
      // ignore
    }
  }
}

function loadAiMessagesForBook(bookId: string): void {
  if (typeof localStorage === 'undefined' || !bookId) return;
  let raw: string | null = null;
  try {
    raw = localStorage.getItem(aiMessagesStorageKey(bookId));
  } catch {
    raw = null;
  }
  if (!raw) return;
  try {
    const parsed: unknown = JSON.parse(raw);
    if (Array.isArray(parsed) && parsed.length > 0) {
      state.aiMessages = (parsed as AiChatMessage[]).map((m) => ({
        ...m,
        editing: false,
      }));
    }
  } catch {
    // 损坏的历史直接丢弃，保留默认欢迎语。
  }
}

// 聊天记录有任何变化（新增、入账、OCR 文本回填、草稿编辑）都实时落盘。
watch(
  () => state.aiMessages,
  () => persistAiMessages(),
  { deep: true }
);

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
  setDefaultBook,
  clearDefaultBook,
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
  refreshActivities,
  loadMoreActivities,
  dismissBudgetAlert,
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
