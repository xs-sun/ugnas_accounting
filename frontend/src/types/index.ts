export type TransactionType = 'expense' | 'income' | 'transfer';

export type WalletType =
  | 'wechat'
  | 'alipay'
  | 'debit'
  | 'credit'
  | 'cash'
  | 'investment'
  | 'custom'
  | 'other';

// ==================== Auth & User ====================

export type VerifyCodePurpose = 'register' | 'login' | 'forgot' | 'bind' | string;

export interface SendCodeRequest {
  email: string;
  purpose: VerifyCodePurpose;
}

export interface SendCodeResponse {
  sent: boolean;
  expires_in: number;
  demo_code?: string;
  smtp_sent: boolean;
}

export interface RegisterRequest {
  username?: string;
  name: string;
  email: string;
  code?: string;
  password: string;
  invite_code?: string;
  avatar?: string;
  bind_ticket?: string;
}

export interface LoginRequest {
  account?: string;
  username?: string;
  email?: string;
  password?: string;
  code?: string;
  mode?: 'password' | 'code';
}

export interface AuthStatusResponse {
  first_install: boolean;
  has_registered_users: boolean;
  total_users: number;
  gateway_header_detected: boolean;
  allow_register: boolean;
  require_email_verify: boolean;
  require_invite_code: boolean;
}

export interface GatewayAuthResponse {
  status: 'authenticated' | 'need_register';
  token?: string;
  refresh_token?: string;
  expires_in?: number;
  refresh_expires_in?: number;
  user?: UserProfile;
  bind_ticket?: string;
  gateway_uid?: string;
  suggested_username?: string;
  suggested_name?: string;
}

export interface ForgotPasswordRequest {
  email: string;
  code: string;
  new_password: string;
}

export interface ForgotPasswordResponse {
  reset: boolean;
}

export interface UserProfile {
  user_id: string;
  username?: string;
  user_name: string;
  user_type: string;
  email: string;
  avatar: string;
  role: 'admin' | 'member' | 'guest' | string;
  email_bound: boolean;
  gateway_bound?: boolean;
  default_book_id?: string;
  daily_ai_limit: number;
  today_ai_count: number;
  streak_days: number;
  recorded_days: number;
  total_transactions_count: number;
  created_books_count: number;
  joined_books_count: number;
  total_books_count: number;
  total_wallets_count: number;
  health_score: number;
  health_label: string;
  savings_rate: number;
}

export interface AuthResponse {
  token: string;
  refresh_token?: string;
  expires_in?: number;
  refresh_expires_in?: number;
  user: UserProfile;
}

export interface UpdateProfileRequest {
  name?: string;
  avatar?: string;
}

export interface BindEmailRequest {
  email: string;
  code: string;
}

// ==================== Books & Members ====================

export interface BookMember {
  book_id: string;
  user_id: string;
  user_name: string;
  role: 'owner' | 'editor' | 'viewer' | string;
  avatar: string;
  color: string;
  is_virtual: boolean;
  status: string;
  is_me: boolean;
  created_at: string;
}

export interface Book {
  id: string;
  name: string;
  currency: string;
  description: string;
  owner_id: string;
  role?: string;
  is_default?: boolean;
  color: string;
  budget: number; // in cents
  invite_code: string;
  tx_count: number;
  members: BookMember[];
  created_at: string;
  updated_at: string;
}

export interface InviteEmailResponse {
  member: BookMember;
  invite_code: string;
  smtp_sent: boolean;
  single_use?: boolean;
}

export interface InviteCodeResponse {
  invite_code: string;
  role: 'editor' | 'viewer' | string;
  single_use: boolean;
}

// ==================== Wallets ====================

export interface Wallet {
  id: string;
  book_id: string;
  type: WalletType;
  name: string;
  group_name: string;
  color: string;
  balance: number; // in cents
  credit_limit: number; // in cents
  billing_day: number;
  repayment_day: number;
  card_last_four: string;
  exclude_net_asset: boolean;
  is_archived: boolean;
  created_at: string;
  updated_at: string;
}

export interface WalletSummary {
  total_assets: number; // in cents
  total_liabilities: number; // in cents
  net_assets: number; // in cents
}

// ==================== Categories & Tags ====================

export interface Category {
  id: string;
  book_id: string;
  type: 'expense' | 'income';
  name: string;
  icon: string;
  color: string;
  bg_color: string;
  is_custom: boolean;
  sort_order: number;
  usage_count?: number;
  parent_id?: string | null;
  created_at: string;
}

export interface CategoryTreeNode extends Category {
  children: Category[];
}

export interface Tag {
  id: string;
  book_id: string;
  name: string;
  color: string;
  created_at: string;
}

// ==================== Transactions ====================

export interface Transaction {
  id: string;
  book_id: string;
  type: TransactionType;
  amount: number; // in cents
  date: string; // YYYY-MM-DD
  tx_time: string; // HH:mm
  wallet_id: string;
  to_wallet_id?: string | null;
  category_id?: string | null;
  title: string;
  sub_category: string;
  note: string;
  include_budget: boolean;
  reimburse: boolean;
  order_no?: string;
  created_by: string;
  created_by_name?: string;
  created_by_avatar?: string;
  // 归属成员（真实或虚拟成员），可与创建人不同；为空时后端默认等于创建人。
  member_id?: string;
  member_name?: string;
  member_avatar?: string;
  member_color?: string;
  created_at: string;
  updated_at: string;
  tags?: Tag[];
  attachments?: string[];
  category_name?: string;
  category_icon?: string;
  category_color?: string;
  category_bg_color?: string;
  wallet_name?: string;
  to_wallet_name?: string;
}

export interface TransactionListParams {
  book_id?: string;
  start_date?: string;
  end_date?: string;
  wallet_id?: string;
  category_id?: string;
  type?: string;
  search?: string;
  member_id?: string;
  page?: number;
  page_size?: number;
}

export interface TransactionListResponse {
  items: Transaction[];
  total: number;
  page: number;
  page_size: number;
}

export interface CreateTransactionPayload {
  book_id: string;
  type: TransactionType;
  amount: number; // in cents
  date: string;
  tx_time?: string;
  wallet_id: string;
  to_wallet_id?: string | null;
  category_id?: string | null;
  title?: string;
  sub_category?: string;
  note?: string;
  include_budget?: boolean;
  reimburse?: boolean;
  member_id?: string;
  tag_ids?: string[];
}

// ==================== Budgets ====================

export interface CategoryBudgetItem {
  category_id: string;
  category_name: string;
  category_icon: string;
  category_color: string;
  bg_color: string;
  limit_amount: number; // in cents
  spent_amount: number; // in cents
  remaining_amount: number; // in cents
  percentage: number;
}

export interface BudgetOverview {
  book_id: string;
  year: number;
  month: number;
  total_budget: number; // in cents
  total_spent: number; // in cents
  remaining_budget: number; // in cents
  remaining_days: number;
  daily_allowance: number; // in cents
  percentage: number;
  categories: CategoryBudgetItem[];
}

// ==================== Book Activities ====================

export interface BookActivity {
  id: string;
  book_id: string;
  user_id: string;
  actor_name: string;
  action: string;
  summary: string;
  amount_cents?: number | null;
  tx_id?: string | null;
  created_at: string; // 服务端本地时间 "YYYY-MM-DD HH:MM:SS"
}

export interface ActivityListResponse {
  items: BookActivity[];
  has_more: boolean;
  next_before: string;
}

// 预算越线提醒（SSE budget-alert 事件的 payload）。
export interface BudgetAlertPayload {
  budgetId: string;
  budgetName: string;
  kind: 'total' | 'category';
  level: 80 | 100;
  used: number; // in cents
  budget: number; // in cents
  month: string; // "YYYY-MM"
}

// ==================== Recurring Rules ====================

export type RecurringCycle = 'daily' | 'weekly' | 'monthly' | 'yearly' | string;

export interface RecurringRule {
  id: string;
  book_id: string;
  type: 'expense' | 'income';
  category_id: string;
  category_name: string;
  category_icon: string;
  category_color: string;
  bg_color: string;
  sub_category: string;
  title: string;
  amount: number; // in cents
  cycle: RecurringCycle;
  start_date: string;
  end_rule: string;
  next_date: string;
  executed_count: number;
  wallet_id: string;
  wallet_name: string;
  exclude_stats: boolean;
  is_active: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CreateRecurringPayload {
  book_id: string;
  type: 'expense' | 'income';
  category_id: string;
  sub_category?: string;
  title: string;
  amount: number; // in cents
  cycle: RecurringCycle;
  start_date: string;
  end_rule?: string;
  wallet_id: string;
  exclude_stats?: boolean;
}

// ==================== Reports & Stats ====================

export type StatsPeriod = 'week' | 'month' | 'year';

export interface CategoryReportItem {
  category_id: string;
  category_name: string;
  category_icon: string;
  category_color: string;
  bg_color?: string;
  amount: number; // in cents
  total_amount?: number;
  count: number;
  percentage: number;
}

export interface MonthlyReport {
  year: number;
  month: number;
  total_expense: number; // in cents
  total_income: number; // in cents
  net_savings: number; // in cents
  categories: CategoryReportItem[];
}

export interface TrendMonthItem {
  month: string;
  expense: number; // in cents
  income: number; // in cents
}

export interface TrendReport {
  months: TrendMonthItem[];
}

export interface DailyReportItem {
  date: string;
  expense: number; // in cents
  income: number; // in cents
}

export interface StatsDailySeriesItem {
  date: string;
  label: string;
  expense: number; // in cents
  income: number; // in cents
}

export interface StatsReport {
  period: StatsPeriod;
  type: 'expense' | 'income';
  start_date: string;
  end_date: string;
  total_expense: number; // in cents
  total_income: number; // in cents
  net_savings: number; // in cents
  savings_rate: number;
  budget: number; // in cents
  budget_used: number; // in cents
  categories: CategoryReportItem[];
  members: StatsMemberItem[];
  daily_series: StatsDailySeriesItem[];
  top_transactions: Transaction[];
}

export interface StatsMemberItem {
  member_id: string;
  member_name: string;
  member_avatar: string;
  member_color: string;
  is_virtual: boolean;
  amount: number; // in cents
  percentage: number;
  count: number;
}

// ==================== AI Features ====================

export interface AiUsageInfo {
  today_count: number;
  daily_limit: number;
  remaining: number;
}

export interface AiDraft {
  type: 'expense' | 'income';
  amount: number; // in cents
  title: string;
  note: string;
  category_id: string;
  category_name: string;
  sub_category: string;
  category_icon: string;
  category_color: string;
  bg_color: string;
  wallet_id: string;
  wallet_name: string;
  date: string;
  member_id?: string;
  member_name?: string;
}

export interface AiChatResponse {
  reply: string;
  draft: AiDraft | null;
  usage: AiUsageInfo;
}

export interface AiOcrResponse {
  reply: string;
  draft: AiDraft;
  raw_text: string;
  usage: AiUsageInfo;
}

export interface AiInsightTip {
  tag: string;
  color: string;
  bg: string;
  title: string;
  desc: string;
}

export interface AiInsightsResponse {
  summary: string;
  tips: AiInsightTip[];
}

export interface AiTestResponse {
  connected: boolean;
  mode: string;
  latency_ms: number;
  model: string;
  message: string;
}

export interface AiCapabilitiesResponse {
  enabled: boolean;
  ocr_enabled: boolean;
  insights_enabled: boolean;
  stt_configured: boolean;
  stt_model: string;
}

export interface AiTranscribeResponse {
  text: string;
  raw_text?: string;
  usage: AiUsageInfo;
}

export interface AiPolishResponse {
  text: string;
  usage: AiUsageInfo;
}

export interface AiChatMessage {
  id: string;
  role: 'user' | 'assistant';
  text: string;
  isVoice?: boolean;
  image?: string;
  // 服务端图片文件名（<dataDir>/ai-images/ 下），加载后由 store 取回 blob 显示。
  imagePath?: string;
  imageLabel?: string;
  // OCR 识别出的原文：点上传的图片可查看；随聊天记录存服务端数据库。
  ocrText?: string;
  draft?: AiDraft | null;
  saved?: boolean;
  savedTxId?: string;
  editing?: boolean;
}

// ==================== Admin Console ====================

export interface AdminDauItem {
  date: string;
  label: string;
  tx_count: number;
  active_users?: number;
}

export interface AdminUserItem {
  user_id: string;
  user_name: string;
  email: string;
  avatar: string;
  role: string;
  is_active: boolean;
  daily_ai_limit: number;
  today_ai_count: number;
  input_tokens: number;
  output_tokens: number;
  books_count: number;
  tx_count: number;
  last_active: string;
}

export interface AdminOverview {
  total_users: number;
  active_users: number;
  weekly_active_users: number;
  total_transactions: number;
  weekly_new_transactions: number;
  total_tokens: number;
  ai_provider: string;
  ai_model: string;
  dau_series: AdminDauItem[];
  users: AdminUserItem[];
}

export interface AdminAiConfig {
  provider: string;
  model: string;
  vision_model?: string;
  base_url: string;
  api_key: string;
  has_api_key?: boolean;
  stt_base_url?: string;
  stt_api_key?: string;
  has_stt_api_key?: boolean;
  stt_model?: string;
  daily_limit?: number;
  ocr_enabled?: boolean;
  insights_enabled?: boolean;
  enabled?: boolean;
}

export interface AdminAiUsageResponse {
  total_input_tokens: number;
  total_output_tokens: number;
  estimated_cost_yuan?: number;
  users: AdminUserItem[];
}

export interface AdminRegConfig {
  allow_public_register: boolean;
  require_email_verify: boolean;
  require_invite_code: boolean;
  invite_code: string;
  default_daily_ai_limit?: number;
}

export interface AdminSystemConfig {
  smtp_host: string;
  smtp_port: number | string;
  smtp_user: string;
  smtp_pass: string;
  smtp_from?: string;
  smtp_tls?: boolean;
  auto_backup: boolean;
  backup_cycle: string;
  backup_path: string;
  keep_count: number;
}

export interface AdminSmtpTestResponse {
  sent: boolean;
  message: string;
}

// ==================== Snapshots & Import/Export ====================

export interface Snapshot {
  id: string;
  file_name: string;
  file_path: string;
  file_size: number;
  sha256: string;
  created_by: string;
  created_at: string;
}

export interface ImportedItem {
  date: string;
  type: 'expense' | 'income';
  counterparty: string;
  product: string;
  amount: number; // cents
  order_no: string;
  suggested_category_id?: string;
  note: string;
}

export type SuishoujiColumnTarget =
  | 'date'
  | 'type'
  | 'amount'
  | 'category'
  | 'subcategory'
  | 'wallet'
  | 'title'
  | 'note'
  | 'tags'
  | '';

export interface SuishoujiPreviewRow {
  row_index: number;
  date: string;
  tx_time: string;
  type: 'expense' | 'income';
  amount_cents: number;
  category_name: string;
  sub_category: string;
  wallet_name: string;
  title: string;
  note: string;
  valid: boolean;
  error?: string;
}

export interface SuishoujiParseResult {
  headers: string[];
  total_rows: number;
  rows: string[][];
  samples: string[][];
  auto_mapping: Record<string, SuishoujiColumnTarget>;
  ai_available: boolean;
  ai_reason: string;
  ai_suggestions: Record<string, SuishoujiColumnTarget>;
  preview: SuishoujiPreviewRow[];
  default_wallet_id: string;
}

export interface SuishoujiRowFailure {
  row: number;
  reason: string;
}

export interface SuishoujiCommitResult {
  inserted: number;
  duplicates: number;
  failed: number;
  failures: SuishoujiRowFailure[];
}

export const SUISHOUJI_TARGET_OPTIONS: { value: SuishoujiColumnTarget; label: string }[] = [
  { value: 'date', label: '日期' },
  { value: 'type', label: '收支类型' },
  { value: 'amount', label: '金额' },
  { value: 'category', label: '一级分类' },
  { value: 'subcategory', label: '二级分类' },
  { value: 'wallet', label: '账户' },
  { value: 'title', label: '标题/商家' },
  { value: 'note', label: '备注' },
  { value: 'tags', label: '标签' },
  { value: '', label: '忽略此列' },
];
