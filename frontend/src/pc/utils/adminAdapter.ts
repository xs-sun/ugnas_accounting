import { getAuthHeaders } from '@/api/client';

/** PC 端 12 个工作台与管理中台视图标识 */
export type PcViewId =
  | 'dashboard'
  | 'ledger'
  | 'calendar'
  | 'stats'
  | 'finance'
  | 'categories'
  | 'ledgers'
  | 'account'
  | 'admin-overview'
  | 'admin-ai'
  | 'admin-users'
  | 'admin-system';

/** 归一化后的管理中台用户信息（兼容前后端字段差异） */
export interface NormalizedAdminUser {
  id: string;
  name: string;
  email: string;
  avatar: string;
  role: string;
  isActive: boolean;
  dailyAiLimit: number;
  todayAiCount: number;
  inputTokens: number;
  outputTokens: number;
  booksCount: number;
  txCount: number;
  lastActive: string;
}

/** 归一化后的管理中台概览数据 */
export interface NormalizedAdminOverview {
  totalUsers: number;
  activeUsers: number;
  weeklyActiveUsers: number;
  totalBooks: number;
  totalTransactions: number;
  todayAiCalls: number;
  totalAiCalls: number;
  totalInputTokens: number;
  totalOutputTokens: number;
  totalTokens: number;
  aiProvider: string;
  aiModel: string;
  dailyActivity: Array<{
    date: string;
    label: string;
    txCount: number;
    aiCalls: number;
  }>;
  users: NormalizedAdminUser[];
}

/** 归一化后的 AI 模型配置 */
export interface NormalizedAiConfig {
  enabled: boolean;
  provider: string;
  baseUrl: string;
  apiKey: string;
  hasApiKey: boolean;
  model: string;
  dailyLimit: number;
  ocrEnabled: boolean;
  insightsEnabled: boolean;
}

/** 归一化后的注册策略配置 */
export interface NormalizedRegConfig {
  allowRegister: boolean;
  requireInviteCode: boolean;
  requireEmailVerify: boolean;
  inviteCode: string;
  defaultDailyAiLimit: number;
}

/** 归一化后的系统与备份配置 */
export interface NormalizedSystemConfig {
  autoBackupEnabled: boolean;
  autoBackupCycle: string;
  autoBackupRetain: number;
  backupPath: string;
  smtpHost: string;
  smtpPort: number;
  smtpUser: string;
  smtpPass: string;
  hasSmtpPass: boolean;
  smtpFrom: string;
  smtpTls: boolean;
}

/** 归一化管理端用户对象 */
export function normalizeAdminUser(raw: any): NormalizedAdminUser {
  return {
    id: String(raw?.id || raw?.user_id || ''),
    name: String(raw?.name || raw?.user_name || '未命名用户'),
    email: String(raw?.email || ''),
    avatar: String(raw?.avatar || ''),
    role: String(raw?.role || 'member'),
    isActive: Boolean(raw?.is_active ?? true),
    dailyAiLimit: Number(raw?.daily_ai_limit ?? 30),
    todayAiCount: Number(raw?.today_ai_count ?? 0),
    inputTokens: Number(raw?.input_tokens ?? 0),
    outputTokens: Number(raw?.output_tokens ?? 0),
    booksCount: Number(raw?.books_count ?? 0),
    txCount: Number(raw?.tx_count ?? 0),
    lastActive: String(raw?.last_active_at || raw?.last_active || '暂无记录'),
  };
}

/** 归一化管理中台总览响应 */
export function normalizeAdminOverview(raw: any): NormalizedAdminOverview {
  const rawUsers: any[] = Array.isArray(raw?.user_stats)
    ? raw.user_stats
    : Array.isArray(raw?.users)
      ? raw.users
      : [];
  const rawSeries: any[] = Array.isArray(raw?.daily_activity)
    ? raw.daily_activity
    : Array.isArray(raw?.dau_series)
      ? raw.dau_series
      : [];

  return {
    totalUsers: Number(raw?.total_users ?? rawUsers.length),
    activeUsers: Number(raw?.active_users ?? 0),
    weeklyActiveUsers: Number(raw?.active_users_7d ?? raw?.weekly_active_users ?? 0),
    totalBooks: Number(raw?.total_books ?? 0),
    totalTransactions: Number(raw?.total_transactions_count ?? raw?.total_transactions ?? 0),
    todayAiCalls: Number(raw?.today_ai_calls ?? 0),
    totalAiCalls: Number(raw?.total_ai_calls ?? 0),
    totalInputTokens: Number(raw?.total_input_tokens ?? 0),
    totalOutputTokens: Number(raw?.total_output_tokens ?? 0),
    totalTokens: Number(raw?.total_tokens ?? 0),
    aiProvider: String(raw?.ai_provider || 'DeepSeek'),
    aiModel: String(raw?.ai_model || 'deepseek-chat'),
    dailyActivity: rawSeries.map((item) => ({
      date: String(item?.date || ''),
      label: String(item?.label || ''),
      txCount: Number(item?.tx_count ?? 0),
      aiCalls: Number(item?.ai_calls ?? 0),
    })),
    users: rawUsers.map(normalizeAdminUser),
  };
}

/** 归一化 AI 模型配置 */
export function normalizeAiConfig(raw: any): NormalizedAiConfig {
  return {
    enabled: Boolean(raw?.enabled ?? true),
    provider: String(raw?.provider || 'DeepSeek'),
    baseUrl: String(raw?.base_url || 'https://api.deepseek.com/v1'),
    apiKey: String(raw?.api_key || ''),
    hasApiKey: Boolean(raw?.has_api_key ?? Boolean(raw?.api_key)),
    model: String(raw?.model || 'deepseek-chat'),
    dailyLimit: Number(raw?.daily_limit ?? 30),
    ocrEnabled: Boolean(raw?.ocr_enabled ?? true),
    insightsEnabled: Boolean(raw?.insights_enabled ?? true),
  };
}

/** 归一化注册策略配置 */
export function normalizeRegConfig(raw: any): NormalizedRegConfig {
  return {
    allowRegister: Boolean(raw?.allow_register ?? raw?.allow_public_register ?? true),
    requireInviteCode: Boolean(raw?.require_invite_code ?? false),
    requireEmailVerify: Boolean(raw?.require_email_verify ?? false),
    inviteCode: String(raw?.invite_code || 'CAT7-NAS-2026'),
    defaultDailyAiLimit: Number(raw?.default_daily_ai_limit ?? 30),
  };
}

/** 归一化系统与备份配置 */
export function normalizeSystemConfig(raw: any): NormalizedSystemConfig {
  return {
    autoBackupEnabled: Boolean(raw?.auto_backup_enabled ?? raw?.auto_backup ?? false),
    autoBackupCycle: String(raw?.auto_backup_cycle ?? raw?.backup_cycle ?? '每天 03:00'),
    autoBackupRetain: Number(raw?.auto_backup_retain ?? raw?.keep_count ?? 14),
    backupPath: String(raw?.backup_path || '/data/snapshots'),
    smtpHost: String(raw?.smtp_host || ''),
    smtpPort: Number(raw?.smtp_port ?? 465) || 465,
    smtpUser: String(raw?.smtp_user || ''),
    smtpPass: String(raw?.smtp_pass || ''),
    hasSmtpPass: Boolean(raw?.has_smtp_pass ?? Boolean(raw?.smtp_pass)),
    smtpFrom: String(raw?.smtp_from || ''),
    smtpTls: Boolean(raw?.smtp_tls ?? true),
  };
}

/** 带鉴权请求头的文件下载，兼容独立登录 Bearer Token 与 NAS 环境 */
export async function downloadWithAuth(
  url: string,
  fallbackFileName: string
): Promise<void> {
  const headers = await getAuthHeaders();
  const res = await fetch(url, {
    method: 'GET',
    headers,
  });

  if (!res.ok) {
    let msg = `下载失败 (${res.status})`;
    try {
      const json = await res.json();
      if (json?.message) msg = json.message;
    } catch {
      // 保持默认错误文案
    }
    throw new Error(msg);
  }

  const blob = await res.blob();
  const disposition = res.headers.get('Content-Disposition') || '';
  let fileName = fallbackFileName;
  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  const normalMatch = disposition.match(/filename="?([^";]+)"?/i);
  if (utf8Match?.[1]) {
    try {
      fileName = decodeURIComponent(utf8Match[1]);
    } catch {
      fileName = fallbackFileName;
    }
  } else if (normalMatch?.[1]) {
    fileName = normalMatch[1];
  }

  const objectUrl = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = objectUrl;
  link.download = fileName;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  setTimeout(() => URL.revokeObjectURL(objectUrl), 1500);
}
