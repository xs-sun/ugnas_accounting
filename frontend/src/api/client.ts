import type {
  Book,
  BookMember,
  Wallet,
  WalletSummary,
  Category,
  Tag,
  Transaction,
  MonthlyReport,
  TrendReport,
  DailyReportItem,
  Snapshot,
  ImportedItem,
  UserProfile,
} from '../types';

import UGOSCore from '@ugreen-nas/core';
import cloudWindow from '@ugreen-nas/core/cloudWindow';

let ugToken: string | null = null;
let ugTokenPromise: Promise<string | null> | null = null;

export async function getOfficialUGToken(): Promise<string | null> {
  if (ugToken) return ugToken;
  if (!ugTokenPromise) {
    ugTokenPromise = (async () => {
      try {
        UGOSCore.init();
        const info: any = await cloudWindow.useCapacity('getThirdToken');
        if (info?.third_token) {
          ugToken = info.third_token;
          if (typeof localStorage !== 'undefined') {
            localStorage.setItem('ug_auth_token', ugToken);
          }
          return ugToken;
        }
      } catch (err) {
        // Standalone or non-iframe mode fallback
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

    // Persist into localStorage if discovered from URL
    if (token) localStorage.setItem('ug_auth_token', token);
    if (userId) localStorage.setItem('ug_auth_user_id', userId);
    if (userName) localStorage.setItem('ug_auth_user_name', userName);
    if (userType) localStorage.setItem('ug_auth_user_type', userType);
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
  if (token && (!userId || !userName)) {
    const claims = parseJwt(token);
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
    userType: userType || 'admin',
  };
}

export function setCustomUserSession(userId: string, userName?: string, userType?: string) {
  if (typeof localStorage !== 'undefined') {
    if (userId) localStorage.setItem('ug_auth_user_id', userId);
    if (userName) localStorage.setItem('ug_auth_user_name', userName);
    if (userType) localStorage.setItem('ug_auth_user_type', userType);
  }
}

async function getAuthHeaders(): Promise<Record<string, string>> {
  const officialToken = await getOfficialUGToken();
  const session = resolveSession();
  const headers: Record<string, string> = {
    'Accept': 'application/json',
  };

  const activeToken = officialToken || session.token;
  if (activeToken) {
    headers['Ugreen-Ttk'] = activeToken;
    headers['Authorization'] = `Bearer ${activeToken}`;
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

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  const authHeaders = await getAuthHeaders();
  const mergedHeaders = {
    ...authHeaders,
    ...(options.headers as Record<string, string> || {}),
  };

  const response = await fetch(url, {
    ...options,
    headers: mergedHeaders,
  });

  if (!response.ok) {
    let errorMsg = `HTTP ${response.status}: ${response.statusText}`;
    try {
      const errJson = await response.json();
      if (errJson.message) errorMsg = errJson.message;
    } catch {
      // keep fallback msg
    }
    throw new Error(errorMsg);
  }

  const resData = await response.json();
  if (resData.code !== 0 && resData.code !== undefined && resData.code !== 200 && resData.code !== 201) {
    throw new Error(resData.message || 'Request failed');
  }

  return resData.data as T;
}

export const api = {
  // User Profile
  async getProfile(): Promise<UserProfile> {
    return request<UserProfile>('/api/v1/user/profile');
  },

  // Health
  async getHealth(): Promise<{ status: string; app_id: string; version: string }> {
    return request('/api/v1/health');
  },

  // Books
  async listBooks(): Promise<Book[]> {
    return request<Book[]>('/api/v1/books');
  },
  async getBook(id: string): Promise<Book> {
    return request<Book>(`/api/v1/books/${id}`);
  },
  async createBook(data: { name: string; currency?: string; description?: string }): Promise<Book> {
    return request<Book>('/api/v1/books', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  async updateBook(id: string, data: { name?: string; description?: string; currency?: string }): Promise<Book> {
    return request<Book>(`/api/v1/books/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  async deleteBook(id: string): Promise<void> {
    return request<void>(`/api/v1/books/${id}`, { method: 'DELETE' });
  },
  async listMembers(bookId: string): Promise<BookMember[]> {
    return request<BookMember[]>(`/api/v1/books/${bookId}/members`);
  },
  async addMember(bookId: string, data: { user_id: string; user_name?: string; role: string }): Promise<any> {
    return request(`/api/v1/books/${bookId}/members`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  async removeMember(bookId: string, userId: string): Promise<void> {
    return request<void>(`/api/v1/books/${bookId}/members/${userId}`, { method: 'DELETE' });
  },

  // Wallets
  async listWallets(bookId?: string): Promise<Wallet[]> {
    const query = bookId ? `?book_id=${encodeURIComponent(bookId)}` : '';
    return request<Wallet[]>(`/api/v1/wallets${query}`);
  },
  async getWalletSummary(bookId?: string): Promise<WalletSummary> {
    const query = bookId ? `?book_id=${encodeURIComponent(bookId)}` : '';
    return request<WalletSummary>(`/api/v1/wallets/summary${query}`);
  },
  async createWallet(data: Partial<Wallet>): Promise<Wallet> {
    return request<Wallet>('/api/v1/wallets', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  async updateWallet(id: string, data: Partial<Wallet>): Promise<Wallet> {
    return request<Wallet>(`/api/v1/wallets/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  async deleteWallet(id: string): Promise<void> {
    return request<void>(`/api/v1/wallets/${id}`, { method: 'DELETE' });
  },

  // Categories & Tags
  async listCategories(bookId?: string, type?: string): Promise<Category[]> {
    const params = new URLSearchParams();
    if (bookId) params.set('book_id', bookId);
    if (type) params.set('type', type);
    const qs = params.toString() ? `?${params.toString()}` : '';
    return request<Category[]>(`/api/v1/categories${qs}`);
  },
  async createCategory(data: Partial<Category>): Promise<Category> {
    return request<Category>('/api/v1/categories', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  async updateCategory(id: string, data: Partial<Category>): Promise<Category> {
    return request<Category>(`/api/v1/categories/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  async deleteCategory(id: string): Promise<void> {
    return request<void>(`/api/v1/categories/${id}`, { method: 'DELETE' });
  },
  async listTags(bookId?: string): Promise<Tag[]> {
    const query = bookId ? `?book_id=${encodeURIComponent(bookId)}` : '';
    return request<Tag[]>(`/api/v1/tags${query}`);
  },
  async createTag(data: Partial<Tag>): Promise<Tag> {
    return request<Tag>('/api/v1/tags', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  async deleteTag(id: string): Promise<void> {
    return request<void>(`/api/v1/tags/${id}`, { method: 'DELETE' });
  },

  // Transactions
  async listTransactions(params: {
    book_id?: string;
    start_date?: string;
    end_date?: string;
    wallet_id?: string;
    category_id?: string;
    type?: string;
    page?: number;
    page_size?: number;
  }): Promise<{ items: Transaction[]; total: number; page: number; page_size: number }> {
    const sp = new URLSearchParams();
    if (params.book_id) sp.set('book_id', params.book_id);
    if (params.start_date) sp.set('start_date', params.start_date);
    if (params.end_date) sp.set('end_date', params.end_date);
    if (params.wallet_id) sp.set('wallet_id', params.wallet_id);
    if (params.category_id) sp.set('category_id', params.category_id);
    if (params.type) sp.set('type', params.type);
    if (params.page) sp.set('page', params.page.toString());
    if (params.page_size) sp.set('page_size', params.page_size.toString());

    return request<{ items: Transaction[]; total: number; page: number; page_size: number }>(
      `/api/v1/transactions?${sp.toString()}`
    );
  },
  async getTransaction(id: string): Promise<Transaction> {
    return request<Transaction>(`/api/v1/transactions/${id}`);
  },
  async createTransaction(data: any): Promise<Transaction> {
    return request<Transaction>('/api/v1/transactions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  async updateTransaction(id: string, data: any): Promise<Transaction> {
    return request<Transaction>(`/api/v1/transactions/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  async deleteTransaction(id: string): Promise<void> {
    return request<void>(`/api/v1/transactions/${id}`, { method: 'DELETE' });
  },
  async uploadAttachment(txId: string, file: File): Promise<{ attachment_id: string; file_name: string }> {
    const authHeaders = await getAuthHeaders();
    const formData = new FormData();
    formData.append('file', file);

    const res = await fetch(`/api/v1/transactions/${txId}/attachment`, {
      method: 'POST',
      headers: authHeaders,
      body: formData,
    });
    if (!res.ok) {
      throw new Error(`Upload failed: ${res.statusText}`);
    }
    const json = await res.json();
    return json.data;
  },

  // Reports
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

  // Import / Export
  getExportUrl(bookId?: string): string {
    return bookId ? `/api/v1/export?book_id=${encodeURIComponent(bookId)}` : '/api/v1/export';
  },
  async importWeChat(file: File): Promise<{ items: ImportedItem[]; count: number }> {
    const authHeaders = await getAuthHeaders();
    const formData = new FormData();
    formData.append('file', file);
    const res = await fetch('/api/v1/imports/wechat', {
      method: 'POST',
      headers: authHeaders,
      body: formData,
    });
    if (!res.ok) throw new Error(`WeChat import failed: ${res.statusText}`);
    const json = await res.json();
    return json.data;
  },
  async importAlipay(file: File): Promise<{ items: ImportedItem[]; count: number }> {
    const authHeaders = await getAuthHeaders();
    const formData = new FormData();
    formData.append('file', file);
    const res = await fetch('/api/v1/imports/alipay', {
      method: 'POST',
      headers: authHeaders,
      body: formData,
    });
    if (!res.ok) throw new Error(`Alipay import failed: ${res.statusText}`);
    const json = await res.json();
    return json.data;
  },
  async commitImport(bookId: string, walletId: string, items: ImportedItem[]): Promise<{ inserted: number }> {
    return request<{ inserted: number }>('/api/v1/imports/commit', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ book_id: bookId, wallet_id: walletId, items }),
    });
  },

  // Snapshots
  async listSnapshots(): Promise<Snapshot[]> {
    return request<Snapshot[]>('/api/v1/snapshots');
  },
  async createSnapshot(): Promise<{ id: string; file_name: string; file_size: number; sha256: string }> {
    return request('/api/v1/snapshots/create', { method: 'POST' });
  },
  getSnapshotDownloadUrl(id: string): string {
    return `/api/v1/snapshots/${id}/download`;
  },
  async deleteSnapshot(id: string): Promise<void> {
    return request<void>(`/api/v1/snapshots/${id}`, { method: 'DELETE' });
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
        if (j.message) err = j.message;
      } catch {}
      throw new Error(`Restore failed: ${err}`);
    }
  },
};
