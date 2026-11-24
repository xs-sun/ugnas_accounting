package service

import (
	"sync"
	"time"
)

// ChangeType 标识一次账本变更的类型，随 ledger-changed 事件下发，前端据此决定刷新哪部分数据。
type ChangeType string

const (
	ChangeTransaction ChangeType = "transaction"
	ChangeWallet      ChangeType = "wallet"
	ChangeBudget      ChangeType = "budget"
	ChangeCategory    ChangeType = "category"
	ChangeMember      ChangeType = "member"
	ChangeRecurring   ChangeType = "recurring"
	ChangeBook        ChangeType = "book"
)

// LedgerEvent 是下发给 SSE 订阅者的事件体。
// Type 为 "ledger-changed"（数据变了）、"budget-alert"（预算越线）
// 或 "connected"（刚连上 / 重连后发出的初始事件）。
// Seq 是该账本单调递增的序号，订阅者可据此发现丢事件（重连即全量刷新，保证最终一致）。
// Summary 是随 ledger-changed 附带的动态摘要（如“记了一笔午餐 ¥32.00”），
// 前端动态流收到即可实时更新，无需等下一次拉取。
type LedgerEvent struct {
	Type        string              `json:"type"`
	BookID      string              `json:"bookId"`
	Seq         int64               `json:"seq"`
	Change      ChangeType          `json:"change,omitempty"`
	ActorName   string              `json:"actorName,omitempty"`
	Summary     string              `json:"summary,omitempty"`
	BudgetAlert *BudgetAlertPayload `json:"budgetAlert,omitempty"`
	At          time.Time           `json:"at"`
}

// BudgetAlertPayload 是预算越线提醒的内容。
// Kind 为 "total"（月总预算）或 "category"（分类预算）；
// Level 为越过的档位（80 或 100）；Used/Budget 单位为分。
type BudgetAlertPayload struct {
	BudgetID   string `json:"budgetId"`
	BudgetName string `json:"budgetName"`
	Kind       string `json:"kind"`
	Level      int    `json:"level"`
	Used       int64  `json:"used"`
	Budget     int64  `json:"budget"`
	Month      string `json:"month"`
}

// subBuffer 是单个订阅者的事件缓冲。写请求 Publish 永不阻塞在订阅者身上：
// 缓冲满了就丢弃该事件（前端重连会全量刷新兜底），绝不能让一次慢客户端卡住一次记账请求。
const subBuffer = 64

// Hub 是进程内订阅中心：map[bookID] -> 订阅者集合。
// NAS 服务器是单进程常驻，推送是即时直推，不需要轮询数据库、不需要任何外部组件。
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[chan LedgerEvent]struct{}
	seq  map[string]int64
}

func NewHub() *Hub {
	return &Hub{
		subs: make(map[string]map[chan LedgerEvent]struct{}),
		seq:  make(map[string]int64),
	}
}

// Subscribe 为指定账本注册一个订阅者，返回事件通道和取消订阅函数。
// 调用方必须在连接结束时调用取消函数，否则通道会泄漏。
func (h *Hub) Subscribe(bookID string) (chan LedgerEvent, func()) {
	ch := make(chan LedgerEvent, subBuffer)
	h.mu.Lock()
	if h.subs[bookID] == nil {
		h.subs[bookID] = make(map[chan LedgerEvent]struct{})
	}
	h.subs[bookID][ch] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	unsub := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs[bookID], ch)
			if len(h.subs[bookID]) == 0 {
				delete(h.subs, bookID)
			}
			h.mu.Unlock()
		})
	}
	return ch, unsub
}

// Seq 返回该账本当前的序号（已发布的事件数）。
func (h *Hub) Seq(bookID string) int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.seq[bookID]
}

// SubscriberCount 返回该账本当前的订阅者数量（测试用）。
func (h *Hub) SubscriberCount(bookID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs[bookID])
}

// Publish 向该账本的全部订阅者推送一条 ledger-changed 事件，只推给该账本，不推给其他账本。
// 不排除操作者本人：操作者自己的页面本来就会刷新，收到事件只是再刷新一次（前端做了防抖）。
func (h *Hub) Publish(bookID string, change ChangeType, actorName string) LedgerEvent {
	return h.PublishWithSummary(bookID, change, actorName, "")
}

// PublishWithSummary 与 Publish 相同，额外附带一条动态摘要，
// 前端收到后可直接把这条动态插进动态流，无需等下一次拉取。
func (h *Hub) PublishWithSummary(bookID string, change ChangeType, actorName, summary string) LedgerEvent {
	return h.publishEvent(LedgerEvent{
		Type:      "ledger-changed",
		BookID:    bookID,
		Change:    change,
		ActorName: actorName,
		Summary:   summary,
		At:        time.Now(),
	})
}

// PublishBudgetAlert 向该账本的全部订阅者推送一条 budget-alert 事件。
func (h *Hub) PublishBudgetAlert(bookID string, alert BudgetAlertPayload) LedgerEvent {
	return h.publishEvent(LedgerEvent{
		Type:        "budget-alert",
		BookID:      bookID,
		Change:      ChangeBudget,
		ActorName:   "预算提醒",
		BudgetAlert: &alert,
		At:          time.Now(),
	})
}

func (h *Hub) publishEvent(ev LedgerEvent) LedgerEvent {
	h.mu.Lock()
	h.seq[ev.BookID]++
	ev.Seq = h.seq[ev.BookID]
	targets := make([]chan LedgerEvent, 0, len(h.subs[ev.BookID]))
	for ch := range h.subs[ev.BookID] {
		targets = append(targets, ch)
	}
	h.mu.Unlock()

	for _, ch := range targets {
		select {
		case ch <- ev:
		default:
			// 订阅者消费太慢：丢弃，不阻塞写请求；前端重连即全量刷新。
		}
	}
	return ev
}
