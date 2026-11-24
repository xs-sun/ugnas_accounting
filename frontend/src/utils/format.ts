export interface FormatMoneyOptions {
  /** 是否开启隐私脱敏 */
  privacy?: boolean;
  /** 是否显式展示正负号 */
  showSign?: boolean;
  /** 交易类型，配合 showSign 使用 */
  type?: 'expense' | 'income' | 'transfer' | string;
  /** 货币符号，默认 ¥ */
  symbol?: string;
  /** 紧凑格式，如 >= 1000 时缩略 */
  compact?: boolean;
  /** 小数位数，默认 2 */
  decimals?: number;
}

/** 分 -> 元 */
export function centsToYuan(cents: number | null | undefined): number {
  const num = Number(cents || 0);
  if (!Number.isFinite(num)) return 0;
  return Math.round(num) / 100;
}

/** 元 -> 分（四舍五入至整数分） */
export function yuanToCents(yuan: number | string | null | undefined): number {
  const num = typeof yuan === 'string' ? parseFloat(yuan) : Number(yuan || 0);
  if (!Number.isFinite(num)) return 0;
  return Math.round(num * 100);
}

/** 格式化金额（入参为 Int64 分 cents） */
export function formatMoney(
  cents: number | null | undefined,
  opts: FormatMoneyOptions = {}
): string {
  const {
    privacy = false,
    showSign = false,
    type = 'expense',
    symbol = '¥',
    compact = false,
    decimals = 2,
  } = opts;

  if (privacy) return '****';

  const rawCents = Number(cents || 0);
  const absYuan = Math.abs(rawCents) / 100;

  let formattedNum: string;
  if (compact && absYuan >= 10000) {
    formattedNum = `${(absYuan / 10000).toFixed(1)}w`;
  } else if (compact && absYuan >= 1000) {
    formattedNum = `${(absYuan / 1000).toFixed(1)}k`;
  } else {
    formattedNum = absYuan.toLocaleString('zh-CN', {
      minimumFractionDigits: decimals,
      maximumFractionDigits: decimals,
    });
  }

  if (showSign) {
    if (type === 'income') return `+${symbol}${formattedNum}`;
    if (type === 'expense') return `-${symbol}${formattedNum}`;
    return rawCents < 0 ? `-${symbol}${formattedNum}` : `${symbol}${formattedNum}`;
  }

  return rawCents < 0 ? `-${symbol}${formattedNum}` : `${symbol}${formattedNum}`;
}

/** 格式化 Token 数量 */
export function formatTokens(n: number | null | undefined): string {
  const val = Number(n || 0);
  if (!Number.isFinite(val) || val <= 0) return '0';
  if (val >= 1_000_000) return `${(val / 1_000_000).toFixed(2)}M`;
  if (val >= 1_000) return `${(val / 1_000).toFixed(1)}K`;
  return String(Math.round(val));
}

/** 返回本地时区今日 ISO 字符串 YYYY-MM-DD */
export function todayIso(date: Date = new Date()): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

/** 返回本地时区当前 HH:mm */
export function nowTimeHm(date: Date = new Date()): string {
  const h = String(date.getHours()).padStart(2, '0');
  const m = String(date.getMinutes()).padStart(2, '0');
  return `${h}:${m}`;
}

/** 根据 YYYY-MM-DD 返回 "M月D日 · 今天/昨天/周X" */
export function getWeekdayLabel(isoDate: string): string {
  if (!isoDate) return '';
  const clean = isoDate.slice(0, 10);
  const parts = clean.split('-');
  if (parts.length !== 3) return isoDate;

  const monthNum = Number(parts[1]);
  const dayNum = Number(parts[2]);
  const md = `${monthNum}月${dayNum}日`;

  const today = new Date();
  const todayStr = todayIso(today);
  const yesterday = new Date(today);
  yesterday.setDate(today.getDate() - 1);
  const yesterdayStr = todayIso(yesterday);

  if (clean === todayStr) return `${md} · 今天`;
  if (clean === yesterdayStr) return `${md} · 昨天`;

  const map = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
  const parsed = new Date(`${clean}T12:00:00`);
  if (Number.isNaN(parsed.getTime())) return md;
  return `${md} · ${map[parsed.getDay()]}`;
}

/** 安全计算简单加减法表达式，返回非负元数值（保留 2 位小数） */
export function evalSimpleExpr(expr: string): number {
  if (!expr) return 0;
  const clean = expr.replace(/[^0-9.+\-]/g, '');
  const tokens = clean.match(/[+\-]?[0-9]*\.?[0-9]+/g);
  if (!tokens) return 0;
  const sum = tokens.reduce((acc, token) => acc + parseFloat(token), 0);
  if (Number.isNaN(sum)) return 0;
  return Math.max(0, Math.round(sum * 100) / 100);
}

/** 格式化文件大小 */
export function formatFileSize(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
}

/** 动态时间：服务端本地时间 "YYYY-MM-DD HH:MM:SS" → 今天 HH:mm / 昨天 HH:mm / MM-DD HH:mm */
export function formatActivityTime(createdAt: string): string {
  if (!createdAt) return '';
  const m = createdAt.match(/^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})/);
  if (!m) return createdAt;
  const d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), Number(m[4]), Number(m[5]));
  const hm = `${m[4]}:${m[5]}`;
  const today = new Date();
  const startOf = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diffDays = Math.round((startOf(today) - startOf(d)) / 86400000);
  if (diffDays <= 0) return `今天 ${hm}`;
  if (diffDays === 1) return `昨天 ${hm}`;
  return `${m[2]}-${m[3]} ${hm}`;
}
