import type {
  Book,
  BookMember,
  InviteEmailResponse,
  Wallet,
  WalletSummary,
  Category,
  Tag,
  Transaction,
  TransactionListParams,
  TransactionListResponse,
  CreateTransactionPayload,
  MonthlyReport,
  TrendReport,
  DailyReportItem,
  StatsPeriod,
  StatsReport,
  BudgetOverview,
  RecurringRule,
  CreateRecurringPayload,
  Snapshot,
  ImportedItem,
  UserProfile,
  AuthStatusResponse,
  GatewayAuthResponse,
  SendCodeRequest,
  SendCodeResponse,
  RegisterRequest,
  LoginRequest,
  AuthResponse,
  ForgotPasswordRequest,
  ForgotPasswordResponse,
  UpdateProfileRequest,
  BindEmailRequest,
  AiChatResponse,
  AiOcrResponse,
  AiInsightsResponse,
  AiTestResponse,
  AdminOverview,
  AdminAiConfig,
  AdminAiUsageResponse,
  AdminRegConfig,
  AdminUserItem,
  AdminSystemConfig,
  AdminSmtpTestResponse,
} from '../types';

import { ensureUGOSDesktopWindowSize } from '@/utils/ugosWindow';

export const AUTH_TOKEN_KEY = 'cat7_auth_token';
const UG_TOKEN_TIMEOUT_MS = 1000;

export class ApiError extends Error {
  status: number;
  code?: number;

  constructor(message: string, status: number, code?: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

let ugToken: string | null = null;
let ugTokenPromise: Promise<string | null> | null = null;
let ugTokenResolved = false;
let unauthorizedHandler: (() => void) | null = null;

export function setUnauthorizedHandler(handler: (() => void) | null): void {
  unauthorizedHandler = handler;
}

export function isEmbeddedUGOSWindow(): boolean {
  if (typeof window === 'undefined') return false;
  try {
    return window.self !== window.top && Boolean(window.name);
  } catch {
    return Boolean(window.name);
  }
}

let ugosSdkPromise: Promise<{
  cloudWindow: typeof import('@ugreen-nas/core/cloudWindow').default;
  isMobile: boolean;
}> | null = null;

async function loadInitializedUGOSCloudWindow() {
  if (!ugosSdkPromise) {
    ugosSdkPromise = (async () => {
      const [coreMod, cwMod, pluginsMod] = await Promise.all([
        import('@ugreen-nas/core'),
        import('@ugreen-nas/core/cloudWindow'),
        import('@ugreen-nas/core/plugins').catch(() => null),
      ]);
      const UGOSCore = coreMod.default;
      if (UGOSCore && typeof UGOSCore.init === 'function') {
        await UGOSCore.init();
      }
      return {
        cloudWindow: cwMod.default,
        isMobile: Boolean(pluginsMod?.envInfo?.$isMobile),
      };
    })().catch((err) => {
      ugosSdkPromise = null;
      throw err;
    });
  }
  return ugosSdkPromise;
}

export async function initUGOSCoreIfEmbedded(): Promise<void> {
  if (!isEmbeddedUGOSWindow()) return;
  try {
    const { cloudWindow, isMobile } = await loadInitializedUGOSCloudWindow();
    await ensureUGOSDesktopWindowSize(cloudWindow, { isMobile });
  } catch (err) {
    console.warn('[UGOS] Core init warning:', err);
  }
}

export function getStoredAuthToken(): string {
  if (typeof localStorage === 'undefined') return '';
  return localStorage.getItem(AUTH_TOKEN_KEY) || '';
}

export function setStoredAuthToken(token: string | null): void {
  if (typeof localStorage === 'undefined') return;
  if (token && token.trim()) {
    localStorage.setItem(AUTH_TOKEN_KEY, token.trim());
  } else {
    localStorage.removeItem(AUTH_TOKEN_KEY);
  }
}

export function clearGatewayAuthCache(): void {
  ugToken = null;
  ugTokenPromise = null;
  ugTokenResolved = false;
  if (typeof localStorage !== 'undefined') {
    localStorage.removeItem('ug_auth_token');
    localStorage.removeItem('ug_auth_user_id');
    localStorage.removeItem('ug_auth_user_name');
    localStorage.removeItem('ug_auth_user_type');
  }
}

export async function getOfficialUGToken(forceRetry = false): Promise<string | null> {
  if (ugToken) return ugToken;
  if (forceRetry) {
    ugTokenPromise = null;
    ugTokenResolved = false;
  }
  if (ugTokenResolved) return null;
  if (!isEmbeddedUGOSWindow()) {
    ugTokenResolved = true;
    return null;
  }
  if (!ugTokenPromise) {
    ugTokenPromise = (async () => {
      let timeoutId: ReturnType<typeof setTimeout> | null = null;
      try {
        const { cloudWindow } = await loadInitializedUGOSCloudWindow();
        if (!cloudWindow || typeof cloudWindow.useCapacity !== 'function') {
          return null;
        }
        const info: any = await Promise.race([
          cloudWindow.useCapacity('getThirdToken', undefined, UG_TOKEN_TIMEOUT_MS),
          new Promise<null>((resolve) => {
            timeoutId = setTimeout(() => resolve(null), UG_TOKEN_TIMEOUT_MS + 150);
          }),
        ]);
        if (info?.third_token) {
          ugToken = info.third_token;
          if (typeof localStorage !== 'undefined' && ugToken) {
            localStorage.setItem('ug_auth_token', ugToken);
          }
          return ugToken;
        }
      } catch {
        // Standalone, mobile WebView, or non-UGOS-host fallback
      } finally {
        if (timeoutId) clearTimeout(timeoutId);
        ugTokenResolved = true;
      }
      return null;
    })();
  }
  return ugTokenPromise;
}

function getCookie(name: string): string | null {
  if (typeof document === 'undefined') return null;
  const match = document.cookie.match(new RegExp('(^|;\\s*)(' + name + ')=([^;]*)'));
  return match ? decodeURIComponent(match[3]) : null;
}

function parseJwt(token: string): any {
  try {
    const parts = token.split('.');
    if (parts.length === 3) {
      const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
      const jsonPayload = decodeURIComponent(
        atob(base64)
          .split('')
          .map((c) => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
          .join('')
      );
      return JSON.parse(jsonPayload);
    }
  } catch {
    // not a valid JWT
  }
  return null;
}

export interface AuthSession {
  userId: string;
  userName: string;
  userType: string;
  token: string;
}

export function resolveSession(): AuthSession {
  let token = '';
  let userId = '';
  let userName = '';
  let userType = '';

  // 1. Check URL search parameters
  if (typeof window !== 'undefined' && window.location) {
    const sp = new URLSearchParams(window.location.search);
    token = sp.get('token') || sp.get('ttk') || sp.get('ugreen_token') || '';
    userId = sp.get('user_id') || sp.get('userId') || sp.get('uid') || sp.get('ugreen_user_id') || '';
    userName = sp.get('user_name') || sp.get('username') || sp.get('name') || sp.get('ugreen_user_name') || '';
    userType = sp.get('user_type') || sp.get('userType') || sp.get('role') || '';

    if (token && typeof localStorage !== 'undefined') localStorage.setItem('ug_auth_token', token);
    if (userId && typeof localStorage !== 'undefined') localStorage.setItem('ug_auth_user_id', userId);
    if (userName && typeof localStorage !== 'undefined') localStorage.setItem('ug_auth_user_name', userName);
    if (userType && typeof localStorage !== 'undefined') localStorage.setItem('ug_auth_user_type', userType);
  }

  // 2. Check localStorage
  if (typeof localStorage !== 'undefined') {
    if (!token) token = localStorage.getItem('ug_auth_token') || localStorage.getItem('token') || localStorage.getItem('ttk') || '';
    if (!userId) userId = localStorage.getItem('ug_auth_user_id') || localStorage.getItem('user_id') || localStorage.getItem('uid') || '';
    if (!userName) userName = localStorage.getItem('ug_auth_user_name') || localStorage.getItem('user_name') || localStorage.getItem('username') || '';
    if (!userType) userType = localStorage.getItem('ug_auth_user_type') || localStorage.getItem('user_type') || localStorage.getItem('role') || '';
  }

  // 3. Check Cookies
  if (!token) {
    token = getCookie('Ugreen-Ttk') || getCookie('token') || getCookie('ttk') || getCookie('ugreen_token') || getCookie('ug_token') || '';
  }
  if (!userId) {
    userId = getCookie('Ugreen-User-ID') || getCookie('user_id') || getCookie('userId') || getCookie('uid') || getCookie('ugreen_user_id') || '';
  }
  if (!userName) {
    userName = getCookie('Ugreen-User-Name') || getCookie('user_name') || getCookie('username') || getCookie('name') || '';
  }

  // 4. Decode JWT if token present and userId/userName missing
  const bearerToken = getStoredAuthToken() || token;
  if (bearerToken && (!userId || !userName)) {
    const claims = parseJwt(bearerToken);
    if (claims) {
      if (!userId) userId = claims.user_id || claims.userId || claims.uid || claims.sub || claims.id || '';
      if (!userName) userName = claims.username || claims.user_name || claims.name || claims.nickname || '';
      if (!userType) userType = claims.user_type || claims.userType || claims.role || (claims.is_admin ? 'admin' : '');
    }
  }

  return {
    token,
    userId,
    userName: userName || userId || '',
    userType,
  };
}

export function setCustomUserSession(userId: string, userName?: string, userType?: string): void {
  if (typeof localStorage !== 'undefined') {
    if (userId) localStorage.setItem('ug_auth_user_id', userId);
    if (userName) localStorage.setItem('ug_auth_user_name', userName);
    if (userType) localStorage.setItem('ug_auth_user_type', userType);
  }
}

export async function getAuthHeaders(): Promise<Record<string, string>> {
  const headers: Record<string, string> = {
    Accept: 'application/json',
  };
  const cat7Token = getStoredAuthToken();
  if (cat7Token) {
    headers['Authorization'] = `Bearer ${cat7Token}`;
  }
  return headers;
}

/**
 * 仅在用户主动触发「绿联云授权登录」时收集网关凭据头，与普通业务会话解耦。
 */
export async function getGatewayAuthHeaders(): Promise<Record<string, string>> {
  const headers: Record<string, string> = {
    Accept: 'application/json',
  };
  const officialToken = await getOfficialUGToken(true);
  const session = resolveSession();
  const ugActiveToken = officialToken || session.token;
  if (ugActiveToken) {
    headers['Ugreen-Ttk'] = ugActiveToken;
    headers['Authorization'] = `Bearer ${ugActiveToken}`;
  }

  if (session.userId) {
    headers['Ugreen-User-ID'] = session.userId;
  }
  if (session.userName) {
    headers['Ugreen-User-Name'] = session.userName;
  }
  if (session.userType) {
    headers['Ugreen-User-Type'] = session.userType;
  }

  return headers;
}

function handleUnauthorizedResponse(url: string, status: number): void {
  if (status === 401 && !url.startsWith('/api/v1/auth/')) {
    setStoredAuthToken(null);
    clearGatewayAuthCache();
    unauthorizedHandler?.();
  }
}

async function requestWithHeaders<T>(
  url: string,
  baseHeaders: Record<string, string>,
  options: RequestInit = {}
): Promise<T> {
  const mergedHeaders: Record<string, string> = {
    ...baseHeaders,
    ...((options.headers as Record<string, string>) || {}),
  };

  const response = await fetch(url, {
    ...options,
    headers: mergedHeaders,
  });

  if (!response.ok) {
    let errorMsg = `HTTP ${response.status}: ${response.statusText}`;
    let errCode: number | undefined = response.status;
    try {
      const errJson = await response.json();
      if (errJson?.message) errorMsg = errJson.message;
      if (typeof errJson?.code === 'number') errCode = errJson.code;
    } catch {
      // keep fallback msg
    }
    handleUnauthorizedResponse(url, response.status);
    throw new ApiError(errorMsg, response.status, errCode);
  }

  const resData = await response.json();
  if (resData.code !== 0 && resData.code !== undefined && resData.code !== 200 && resData.code !== 201) {
    throw new ApiError(resData.message || 'Request failed', response.status, resData.code);
  }

  return resData.data as T;
}

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  const authHeaders = await getAuthHeaders();
  return requestWithHeaders<T>(url, authHeaders, options);
}

function jsonRequest<T>(url: string, method: 'POST' | 'PUT' | 'PATCH' | 'DELETE', body?: unknown): Promise<T> {
  return request<T>(url, {
    method,
    headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

export const api = {
  // ==================== Health ====================
  async getHealth(): Promise<{ status: string; app_id: string; version: string }> {
    return request('/api/v1/health');
  },

  // ==================== Auth ====================
  async getAuthStatus(): Promise<AuthStatusResponse> {
    return request<AuthStatusResponse>('/api/v1/auth/status');
  },
  async gatewayLogin(): Promise<GatewayAuthResponse> {
    const gwHeaders = await getGatewayAuthHeaders();
    return requestWithHeaders<GatewayAuthResponse>('/api/v1/auth/gateway', gwHeaders, {
      method: 'POST',
    });
  },
  async sendCode(data: SendCodeRequest): Promise<SendCodeResponse> {
    return jsonRequest<SendCodeResponse>('/api/v1/auth/send-code', 'POST', data);
  },
  async register(data: RegisterRequest): Promise<AuthResponse> {
    return jsonRequest<AuthResponse>('/api/v1/auth/register', 'POST', data);
  },
  async login(data: LoginRequest): Promise<AuthResponse> {
    return jsonRequest<AuthResponse>('/api/v1/auth/login', 'POST', data);
  },
  async forgotPassword(data: ForgotPasswordRequest): Promise<ForgotPasswordResponse> {
    return jsonRequest<ForgotPasswordResponse>('/api/v1/auth/forgot-password', 'POST', data);
  },

  // ==================== User ====================
  async getProfile(): Promise<UserProfile> {
    return request<UserProfile>('/api/v1/user/profile');
  },
  async updateProfile(data: UpdateProfileRequest): Promise<UserProfile> {
    return jsonRequest<UserProfile>('/api/v1/user/profile', 'PUT', data);
  },
  async bindEmail(data: BindEmailRequest): Promise<UserProfile> {
    return jsonRequest<UserProfile>('/api/v1/user/bind-email', 'POST', data);
  },

  // ==================== Books ====================
  async listBooks(): Promise<Book[]> {
    return request<Book[]>('/api/v1/books');
  },
  async getBook(id: string): Promise<Book> {
    return request<Book>(`/api/v1/books/${encodeURIComponent(id)}`);
  },
  async createBook(data: {
    name: string;
    currency?: string;
    description?: string;
    color?: string;
    budget?: number;
  }): Promise<Book> {
    return jsonRequest<Book>('/api/v1/books', 'POST', data);
  },
  async updateBook(
    id: string,
    data: {
      name?: string;
      description?: string;
      currency?: string;
      color?: string;
      budget?: number;
    }
  ): Promise<Book> {
    return jsonRequest<Book>(`/api/v1/books/${encodeURIComponent(id)}`, 'PUT', data);
  },
  async deleteBook(id: string): Promise<void> {
    return request<void>(`/api/v1/books/${encodeURIComponent(id)}`, { method: 'DELETE' });
  },
  async listMembers(bookId: string): Promise<BookMember[]> {
    return request<BookMember[]>(`/api/v1/books/${encodeURIComponent(bookId)}/members`);
  },
  async addMember(bookId: string, data: { user_id: string; user_name?: string; role: string }): Promise<BookMember> {
    return jsonRequest<BookMember>(`/api/v1/books/${encodeURIComponent(bookId)}/members`, 'POST', data);
  },
  async removeMember(bookId: string, userId: string): Promise<void> {
    return request<void>(`/api/v1/books/${encodeURIComponent(bookId)}/members/${encodeURIComponent(userId)}`, {
      method: 'DELETE',
    });
  },
  async inviteMemberByEmail(
    bookId: string,
    data: { email: string; role?: string }
  ): Promise<InviteEmailResponse> {
    return jsonRequest<InviteEmailResponse>(`/api/v1/books/${encodeURIComponent(bookId)}/invite-email`, 'POST', data);
  },
  async createVirtualMember(
    bookId: string,
    data: { name: string; color?: string }
  ): Promise<BookMember> {
    return jsonRequest<BookMember>(`/api/v1/books/${encodeURIComponent(bookId)}/virtual-members`, 'POST', data);
  },
  async joinBookByCode(data: { invite_code: string }): Promise<Book> {
    return jsonRequest<Book>('/api/v1/books/join', 'POST', data);
  },

  // ==================== Wallets ====================
  async listWallets(bookId?: string): Promise<Wallet[]> {
    const query = bookId ? `?book_id=${encodeURIComponent(bookId)}` : '';
    return request<Wallet[]>(`/api/v1/wallets${query}`);
  },
  async getWalletSummary(bookId?: string): Promise<WalletSummary> {
    const query = bookId ? `?book_id=${encodeURIComponent(bookId)}` : '';
    return request<WalletSummary>(`/api/v1/wallets/summary${query}`);
  },
  async createWallet(data: Partial<Wallet>): Promise<Wallet> {
    return jsonRequest<Wallet>('/api/v1/wallets', 'POST', data);
  },
  async updateWallet(id: string, data: Partial<Wallet>): Promise<Wallet> {
    return jsonRequest<Wallet>(`/api/v1/wallets/${encodeURIComponent(id)}`, 'PUT', data);
  },
  async deleteWallet(id: string): Promise<void> {
    return request<void>(`/api/v1/wallets/${encodeURIComponent(id)}`, { method: 'DELETE' });
  },

  // ==================== Categories & Tags ====================
  async listCategories(bookId?: string, type?: string): Promise<Category[]> {
    const params = new URLSearchParams();
    if (bookId) params.set('book_id', bookId);
    if (type) params.set('type', type);
    const qs = params.toString() ? `?${params.toString()}` : '';
    return request<Category[]>(`/api/v1/categories${qs}`);
  },
  async createCategory(data: Partial<Category>): Promise<Category> {
    return jsonRequest<Category>('/api/v1/categories', 'POST', data);
  },
  async updateCategory(id: string, data: Partial<Category>): Promise<Category> {
    return jsonRequest<Category>(`/api/v1/categories/${encodeURIComponent(id)}`, 'PUT', data);
  },
  async deleteCategory(id: string): Promise<void> {
    return request<void>(`/api/v1/categories/${encodeURIComponent(id)}`, { method: 'DELETE' });
  },
  async listTags(bookId?: string): Promise<Tag[]> {
    const query = bookId ? `?book_id=${encodeURIComponent(bookId)}` : '';
    return request<Tag[]>(`/api/v1/tags${query}`);
  },
  async createTag(data: Partial<Tag>): Promise<Tag> {
    return jsonRequest<Tag>('/api/v1/tags', 'POST', data);
  },
  async deleteTag(id: string): Promise<void> {
    return request<void>(`/api/v1/tags/${encodeURIComponent(id)}`, { method: 'DELETE' });
  },

  // ==================== Transactions ====================
  async listTransactions(params: TransactionListParams): Promise<TransactionListResponse> {
    const sp = new URLSearchParams();
    if (params.book_id) sp.set('book_id', params.book_id);
    if (params.start_date) sp.set('start_date', params.start_date);
    if (params.end_date) sp.set('end_date', params.end_date);
    if (params.wallet_id) sp.set('wallet_id', params.wallet_id);
    if (params.category_id) sp.set('category_id', params.category_id);
    if (params.type) sp.set('type', params.type);
    if (params.search) sp.set('search', params.search);
    if (params.page) sp.set('page', params.page.toString());
    if (params.page_size) sp.set('page_size', params.page_size.toString());

    const qs = sp.toString();
    return request<TransactionListResponse>(`/api/v1/transactions${qs ? `?${qs}` : ''}`);
  },
  async getTransaction(id: string): Promise<Transaction> {
    return request<Transaction>(`/api/v1/transactions/${encodeURIComponent(id)}`);
  },
  async createTransaction(data: CreateTransactionPayload | Record<string, any>): Promise<Transaction> {
    return jsonRequest<Transaction>('/api/v1/transactions', 'POST', data);
  },
  async updateTransaction(id: string, data: Partial<CreateTransactionPayload> | Record<string, any>): Promise<Transaction> {
    return jsonRequest<Transaction>(`/api/v1/transactions/${encodeURIComponent(id)}`, 'PUT', data);
  },
  async deleteTransaction(id: string): Promise<void> {
    return request<void>(`/api/v1/transactions/${encodeURIComponent(id)}`, { method: 'DELETE' });
  },
  async batchDeleteTransactions(ids: string[]): Promise<{ deleted: number }> {
    return jsonRequest<{ deleted: number }>('/api/v1/transactions/batch-delete', 'POST', { ids });
  },
  async uploadAttachment(txId: string, file: File): Promise<{ attachment_id: string; file_name: string }> {
    const formData = new FormData();
    formData.append('file', file);
    return request<{ attachment_id: string; file_name: string }>(
      `/api/v1/transactions/${encodeURIComponent(txId)}/attachment`,
      {
        method: 'POST',
        body: formData,
      }
    );
  },

  // ==================== Budgets ====================
  async getBudgets(bookId: string, year?: number, month?: number): Promise<BudgetOverview> {
    const sp = new URLSearchParams();
    sp.set('book_id', bookId);
    if (year) sp.set('year', String(year));
    if (month) sp.set('month', String(month));
    return request<BudgetOverview>(`/api/v1/budgets?${sp.toString()}`);
  },
  async updateTotalBudget(data: { book_id: string; budget: number }): Promise<BudgetOverview> {
    return jsonRequest<BudgetOverview>('/api/v1/budgets/total', 'PUT', data);
  },
  async updateCategoryBudget(data: {
    book_id: string;
    category_id: string;
    limit_amount: number;
  }): Promise<BudgetOverview> {
    return jsonRequest<BudgetOverview>('/api/v1/budgets/category', 'PUT', data);
  },

  // ==================== Recurring Rules ====================
  async listRecurring(bookId: string): Promise<RecurringRule[]> {
    return request<RecurringRule[]>(`/api/v1/recurring?book_id=${encodeURIComponent(bookId)}`);
  },
  async createRecurring(data: CreateRecurringPayload): Promise<RecurringRule> {
    return jsonRequest<RecurringRule>('/api/v1/recurring', 'POST', data);
  },
  async updateRecurring(id: string, data: Partial<CreateRecurringPayload> & { is_active?: boolean }): Promise<RecurringRule> {
    return jsonRequest<RecurringRule>(`/api/v1/recurring/${encodeURIComponent(id)}`, 'PUT', data);
  },
  async deleteRecurring(id: string): Promise<void> {
    return request<void>(`/api/v1/recurring/${encodeURIComponent(id)}`, { method: 'DELETE' });
  },
  async toggleRecurring(id: string): Promise<RecurringRule> {
    return jsonRequest<RecurringRule>(`/api/v1/recurring/${encodeURIComponent(id)}/toggle`, 'POST');
  },

  // ==================== Reports & Stats ====================
  async getStats(params: {
    book_id: string;
    period?: StatsPeriod;
    type?: 'expense' | 'income';
    year?: number;
    month?: number;
  }): Promise<StatsReport> {
    const sp = new URLSearchParams();
    sp.set('book_id', params.book_id);
    if (params.period) sp.set('period', params.period);
    if (params.type) sp.set('type', params.type);
    if (params.year) sp.set('year', String(params.year));
    if (params.month) sp.set('month', String(params.month));
    return request<StatsReport>(`/api/v1/reports/stats?${sp.toString()}`);
  },
  async getMonthlyReport(bookId?: string, year?: number, month?: number): Promise<MonthlyReport> {
    const sp = new URLSearchParams();
    if (bookId) sp.set('book_id', bookId);
    if (year) sp.set('year', year.toString());
    if (month) sp.set('month', month.toString());
    return request<MonthlyReport>(`/api/v1/reports/monthly?${sp.toString()}`);
  },
  async getTrendReport(bookId?: string, months?: number): Promise<TrendReport> {
    const sp = new URLSearchParams();
    if (bookId) sp.set('book_id', bookId);
    if (months) sp.set('months', months.toString());
    return request<TrendReport>(`/api/v1/reports/trend?${sp.toString()}`);
  },
  async getDailyReport(bookId?: string, year?: number, month?: number): Promise<DailyReportItem[]> {
    const sp = new URLSearchParams();
    if (bookId) sp.set('book_id', bookId);
    if (year) sp.set('year', year.toString());
    if (month) sp.set('month', month.toString());
    return request<DailyReportItem[]>(`/api/v1/reports/daily?${sp.toString()}`);
  },

  // ==================== AI ====================
  async aiChat(data: { book_id: string; message: string; is_voice?: boolean }): Promise<AiChatResponse> {
    return jsonRequest<AiChatResponse>('/api/v1/ai/chat', 'POST', data);
  },
  async aiOcr(data: { book_id: string; file?: File; demo?: string }): Promise<AiOcrResponse> {
    if (data.file) {
      const formData = new FormData();
      formData.append('book_id', data.book_id);
      formData.append('file', data.file);
      return request<AiOcrResponse>('/api/v1/ai/ocr', {
        method: 'POST',
        body: formData,
      });
    }
    return jsonRequest<AiOcrResponse>('/api/v1/ai/ocr', 'POST', {
      book_id: data.book_id,
      demo: data.demo || 'starbucks',
    });
  },
  async getAiInsights(params: {
    book_id: string;
    period?: StatsPeriod;
    type?: 'expense' | 'income';
    seed?: number;
  }): Promise<AiInsightsResponse> {
    const sp = new URLSearchParams();
    sp.set('book_id', params.book_id);
    if (params.period) sp.set('period', params.period);
    if (params.type) sp.set('type', params.type);
    if (params.seed !== undefined) sp.set('seed', String(params.seed));
    return request<AiInsightsResponse>(`/api/v1/ai/insights?${sp.toString()}`);
  },
  async testAiConnection(data?: {
    base_url?: string;
    api_key?: string;
    model?: string;
  }): Promise<AiTestResponse> {
    return jsonRequest<AiTestResponse>('/api/v1/ai/test', 'POST', data || {});
  },

  // ==================== Admin ====================
  async getAdminOverview(): Promise<AdminOverview> {
    return request<AdminOverview>('/api/v1/admin/overview');
  },
  async getAdminAiConfig(): Promise<AdminAiConfig> {
    return request<AdminAiConfig>('/api/v1/admin/ai-config');
  },
  async updateAdminAiConfig(data: Partial<AdminAiConfig>): Promise<AdminAiConfig> {
    return jsonRequest<AdminAiConfig>('/api/v1/admin/ai-config', 'PUT', data);
  },
  async getAdminAiUsage(search?: string): Promise<AdminAiUsageResponse> {
    const qs = search ? `?search=${encodeURIComponent(search)}` : '';
    return request<AdminAiUsageResponse>(`/api/v1/admin/ai-usage${qs}`);
  },
  async getAdminRegConfig(): Promise<AdminRegConfig> {
    return request<AdminRegConfig>('/api/v1/admin/reg-config');
  },
  async updateAdminRegConfig(data: Partial<AdminRegConfig>): Promise<AdminRegConfig> {
    return jsonRequest<AdminRegConfig>('/api/v1/admin/reg-config', 'PUT', data);
  },
  async listAdminUsers(search?: string): Promise<AdminUserItem[]> {
    const qs = search ? `?search=${encodeURIComponent(search)}` : '';
    return request<AdminUserItem[]>(`/api/v1/admin/users${qs}`);
  },
  async updateAdminUser(
    id: string,
    data: { daily_ai_limit?: number; is_active?: boolean; role?: string }
  ): Promise<AdminUserItem> {
    return jsonRequest<AdminUserItem>(`/api/v1/admin/users/${encodeURIComponent(id)}`, 'PUT', data);
  },
  async getAdminSystemConfig(): Promise<AdminSystemConfig> {
    return request<AdminSystemConfig>('/api/v1/admin/system-config');
  },
  async updateAdminSystemConfig(data: Partial<AdminSystemConfig>): Promise<AdminSystemConfig> {
    return jsonRequest<AdminSystemConfig>('/api/v1/admin/system-config', 'PUT', data);
  },
  async testAdminSmtp(data?: { email?: string }): Promise<AdminSmtpTestResponse> {
    return jsonRequest<AdminSmtpTestResponse>('/api/v1/admin/smtp-test', 'POST', data || {});
  },

  // ==================== Import / Export ====================
  getExportUrl(bookId?: string): string {
    return bookId ? `/api/v1/export?book_id=${encodeURIComponent(bookId)}` : '/api/v1/export';
  },
  async importWeChat(file: File): Promise<{ items: ImportedItem[]; count: number }> {
    const formData = new FormData();
    formData.append('file', file);
    return request<{ items: ImportedItem[]; count: number }>('/api/v1/imports/wechat', {
      method: 'POST',
      body: formData,
    });
  },
  async importAlipay(file: File): Promise<{ items: ImportedItem[]; count: number }> {
    const formData = new FormData();
    formData.append('file', file);
    return request<{ items: ImportedItem[]; count: number }>('/api/v1/imports/alipay', {
      method: 'POST',
      body: formData,
    });
  },
  async commitImport(bookId: string, walletId: string, items: ImportedItem[]): Promise<{ inserted: number }> {
    return jsonRequest<{ inserted: number }>('/api/v1/imports/commit', 'POST', {
      book_id: bookId,
      wallet_id: walletId,
      items,
    });
  },

  // ==================== Snapshots ====================
  async listSnapshots(): Promise<Snapshot[]> {
    return request<Snapshot[]>('/api/v1/snapshots');
  },
  async createSnapshot(): Promise<{ id: string; file_name: string; file_size: number; sha256: string }> {
    return request('/api/v1/snapshots/create', { method: 'POST' });
  },
  getSnapshotDownloadUrl(id: string): string {
    return `/api/v1/snapshots/${encodeURIComponent(id)}/download`;
  },
  async deleteSnapshot(id: string): Promise<void> {
    return request<void>(`/api/v1/snapshots/${encodeURIComponent(id)}`, { method: 'DELETE' });
  },
  async restoreSnapshot(file: File): Promise<void> {
    const authHeaders = await getAuthHeaders();
    const formData = new FormData();
    formData.append('file', file);
    const res = await fetch('/api/v1/snapshots/restore', {
      method: 'POST',
      headers: authHeaders,
      body: formData,
    });
    if (!res.ok) {
      let err = res.statusText;
      try {
        const j = await res.json();
        if (j?.message) err = j.message;
      } catch {
        // ignore parse error
      }
      handleUnauthorizedResponse('/api/v1/snapshots/restore', res.status);
      throw new ApiError(`Restore failed: ${err}`, res.status);
    }
  },
};
