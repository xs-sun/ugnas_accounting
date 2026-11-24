import { getStoredAuthToken } from '@/api/client';
import type { BudgetAlertPayload } from '@/types';

// 账本实时流：对当前账本开一条 SSE（EventSource），服务端有变更就推 ledger-changed，
// 预算越线推 budget-alert。同一个流、同一套序号；切换账本、退出登录时由 store 重启/关闭。
// EventSource 自带断线重连；重连后服务端会再发 connected，store 收到后全量刷新一次做同步。

export type LedgerChangeType =
  | 'transaction'
  | 'wallet'
  | 'budget'
  | 'category'
  | 'member'
  | 'recurring'
  | 'book';

export interface LedgerStreamEvent {
  type: 'ledger-changed' | 'budget-alert' | 'connected';
  bookId: string;
  seq: number;
  change?: LedgerChangeType;
  actorName?: string;
  summary?: string;
  budgetAlert?: BudgetAlertPayload;
  at?: string;
}

type EventsHandler = (events: LedgerStreamEvent[]) => void;
type ConnectedHandler = (event: LedgerStreamEvent) => void;

let source: EventSource | null = null;
let currentBookId = '';
let lastSeq = -1;
let pending: LedgerStreamEvent[] = [];
let debounceTimer: ReturnType<typeof setTimeout> | null = null;

function parseEvent(data: string): LedgerStreamEvent | null {
  try {
    return JSON.parse(data) as LedgerStreamEvent;
  } catch {
    return null;
  }
}

// 防抖：突发多条事件合并成一次刷新。
function queueEvent(ev: LedgerStreamEvent, onEvents: EventsHandler): void {
  pending.push(ev);
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    debounceTimer = null;
    const batch = pending;
    pending = [];
    onEvents(batch);
  }, 300);
}

export function startLedgerStream(
  bookId: string,
  onEvents: EventsHandler,
  onConnected?: ConnectedHandler,
): void {
  stopLedgerStream();
  if (!bookId) return;
  const token = getStoredAuthToken();
  if (!token) return;
  currentBookId = bookId;
  const url = `/api/v1/stream?bookId=${encodeURIComponent(bookId)}&token=${encodeURIComponent(token)}`;
  source = new EventSource(url);
  source.addEventListener('connected', (e: MessageEvent) => {
    const ev = parseEvent(e.data);
    if (!ev) return;
    lastSeq = ev.seq;
    onConnected?.(ev);
  });
  source.addEventListener('ledger-changed', (e: MessageEvent) => {
    const ev = parseEvent(e.data);
    if (!ev) return;
    // 序号跳号说明中间丢了事件：不补拉单条，重连/下一次 connected 会全量刷新兜底。
    if (lastSeq >= 0 && ev.seq > lastSeq + 1) {
      console.warn('[stream] seq gap', lastSeq, '->', ev.seq);
    }
    lastSeq = ev.seq;
    queueEvent(ev, onEvents);
  });
  source.addEventListener('budget-alert', (e: MessageEvent) => {
    const ev = parseEvent(e.data);
    if (!ev) return;
    if (lastSeq >= 0 && ev.seq > lastSeq + 1) {
      console.warn('[stream] seq gap', lastSeq, '->', ev.seq);
    }
    lastSeq = ev.seq;
    queueEvent(ev, onEvents);
  });
  source.onerror = () => {
    // EventSource 会自动重连；重连成功后服务端再发 connected，store 全量同步一次。
  };
}

export function stopLedgerStream(): void {
  if (debounceTimer) {
    clearTimeout(debounceTimer);
    debounceTimer = null;
  }
  pending = [];
  if (source) {
    source.close();
    source = null;
  }
  currentBookId = '';
  lastSeq = -1;
}

export function getLedgerStreamBookId(): string {
  return currentBookId;
}
