/** 移动端底部导航 Tab 类型 */
export type MobileTab = 'records' | 'calendar' | 'stats' | 'profile';

/** 记一笔抽屉初始模式 */
export type RecordInitialMode = 'ai' | 'expense' | 'income';

/** 账本成员与邀请抽屉内部视图状态 */
export type InviteSheetView = 'members' | 'invite' | 'virtual';

/** 全屏子页管理页面类型 */
export type SubpageType =
  | 'new-ledger'
  | 'budgets'
  | 'wallets'
  | 'categories'
  | 'recurring'
  | 'bind-email'
  | 'import-export';
