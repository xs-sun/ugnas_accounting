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

export interface Book {
  id: string;
  name: string;
  currency: string;
  description: string;
  owner_id: string;
  role?: string;
  created_at: string;
  updated_at: string;
}

export interface BookMember {
  book_id: string;
  user_id: string;
  user_name: string;
  role: 'owner' | 'editor' | 'viewer';
  created_at: string;
}

export interface Wallet {
  id: string;
  book_id: string;
  type: WalletType;
  name: string;
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
  total_assets: number;
  total_liabilities: number;
  net_assets: number;
}

export interface Category {
  id: string;
  book_id: string;
  type: 'expense' | 'income';
  name: string;
  icon: string;
  color: string;
  parent_id?: string | null;
  created_at: string;
}

export interface Tag {
  id: string;
  book_id: string;
  name: string;
  color: string;
  created_at: string;
}

export interface Transaction {
  id: string;
  book_id: string;
  type: TransactionType;
  amount: number; // in cents
  date: string;
  wallet_id: string;
  to_wallet_id?: string | null;
  category_id?: string | null;
  note: string;
  order_no?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
  tags?: Tag[];
  attachments?: string[];
  category_name?: string;
  category_icon?: string;
  category_color?: string;
  wallet_name?: string;
  to_wallet_name?: string;
}

export interface CategoryReportItem {
  category_id: string;
  category_name: string;
  category_icon: string;
  category_color: string;
  amount: number;
  total_amount?: number;
  count: number;
  percentage: number;
}

export interface MonthlyReport {
  year: number;
  month: number;
  total_expense: number;
  total_income: number;
  net_savings: number;
  categories: CategoryReportItem[];
}

export interface TrendMonthItem {
  month: string;
  expense: number;
  income: number;
}

export interface TrendReport {
  months: TrendMonthItem[];
}

export interface DailyReportItem {
  date: string;
  expense: number;
  income: number;
}

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

export interface UserProfile {
  user_id: string;
  user_name: string;
  user_type: string;
  created_books_count: number;
  joined_books_count: number;
  total_books_count: number;
  total_transactions_count: number;
  total_wallets_count: number;
}

