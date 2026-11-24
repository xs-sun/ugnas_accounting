package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

// 预算提醒档位：花到 80% 预警、100% 超支提醒。
var budgetAlertLevels = []int{80, 100}

// ActivityRecorder 是“写动态 + 发 SSE + 预算提醒检查”的统一入口。
// 所有账本变更点都走它，保证动态行、SSE 事件、预算提醒三者口径一致，
// 且一律按 book_id 隔离——调用方只传本账本的 ID，绝不跨账本。
//
// 动态写入失败只记日志，不让一次动态写失败拖垮主流程（记账本身已成功）。
// 方法对 nil 接收者安全：未配置时退化为只发 SSE（与旧行为一致）。
type ActivityRecorder struct {
	repo       *repository.ActivityRepo
	budgetRepo *repository.BudgetRepo
	hub        *Hub
}

func NewActivityRecorder(repo *repository.ActivityRepo, budgetRepo *repository.BudgetRepo, hub *Hub) *ActivityRecorder {
	return &ActivityRecorder{repo: repo, budgetRepo: budgetRepo, hub: hub}
}

// FormatYuan 把分转成 ¥x.xx 展示串（供 handler 拼动态摘要用）。
func FormatYuan(cents int64) string {
	return formatCentsYuan(cents)
}

// Record 写一条动态并推送 SSE 事件（事件里带动态摘要，前端动态流可实时更新）。
// 写库失败只记日志；随后顺手按保留策略清理旧动态（失败也只记日志）。
func (r *ActivityRecorder) Record(ctx context.Context, a model.BookActivity, change ChangeType) {
	if r == nil || r.hub == nil {
		return
	}
	if r.repo != nil {
		if err := r.repo.Record(ctx, &a); err != nil {
			log.Printf("[WARN] RecordActivity insert failed (book %s action %s): %v", a.BookID, a.Action, err)
		} else if err := r.repo.Prune(ctx, a.BookID); err != nil {
			log.Printf("[WARN] RecordActivity prune failed (book %s): %v", a.BookID, err)
		}
	}
	r.hub.PublishWithSummary(a.BookID, change, a.ActorName, a.Summary)
}

// CheckBudgets 检查某账本当月预算（月总预算 + 各分类预算），对新越线的档位发提醒。
// 提醒只发一次：同一账本、同一预算项、同一月份、同一档位已提醒过的跳过；
// 用量回落到档位之下时清除该档位的已提醒记录，下次重新越线会再次提醒。
// 每次调用返回本次新发的提醒（可能为空）。提醒同时写动态 + 推 SSE budget-alert 事件。
func (r *ActivityRecorder) CheckBudgets(ctx context.Context, bookID string) []BudgetAlertPayload {
	if r == nil || r.budgetRepo == nil || r.repo == nil || r.hub == nil || bookID == "" {
		return nil
	}
	now := time.Now()
	ov, err := r.budgetRepo.GetBudgetOverview(ctx, bookID, now.Year(), int(now.Month()))
	if err != nil {
		log.Printf("[WARN] CheckBudgets overview failed (book %s): %v", bookID, err)
		return nil
	}
	monthKey := now.Format("2006-01")
	alerts := make([]BudgetAlertPayload, 0)

	// 月总预算
	if ov.TotalBudget > 0 {
		pct := int((ov.TotalSpent * 100) / ov.TotalBudget)
		if a := r.evaluateBudget(ctx, bookID, "total", "本月总预算", "total", ov.TotalSpent, ov.TotalBudget, pct, monthKey); a != nil {
			alerts = append(alerts, *a)
		}
	}
	// 分类预算（只看设了上限的分类）
	for _, c := range ov.Categories {
		if c.LimitAmount <= 0 {
			continue
		}
		pct := int((c.SpentAmount * 100) / c.LimitAmount)
		if a := r.evaluateBudget(ctx, bookID, "category:"+c.CategoryID, "分类「"+c.CategoryName+"」", c.CategoryID, c.SpentAmount, c.LimitAmount, pct, monthKey); a != nil {
			alerts = append(alerts, *a)
		}
	}
	return alerts
}

// evaluateBudget 判断单个预算项是否新越线：是则记档位、写动态、推 SSE，返回提醒；否则返回 nil。
// 一次调用最多发一条提醒（取新越线档位里最高的），低档位同时记为已提醒，避免随后补发。
func (r *ActivityRecorder) evaluateBudget(ctx context.Context, bookID, budgetKey, budgetName, budgetID string, used, limit int64, pct int, monthKey string) *BudgetAlertPayload {
	notified, err := r.repo.NotifiedLevels(ctx, bookID, budgetKey, monthKey)
	if err != nil {
		log.Printf("[WARN] CheckBudgets notified levels failed (book %s key %s): %v", bookID, budgetKey, err)
		return nil
	}
	currentLevel := 0
	for _, lv := range budgetAlertLevels {
		if pct >= lv {
			currentLevel = lv
		}
	}
	// 用量回落：清除高于当前档位的已提醒记录，下次重新越线会再次提醒。
	if err := r.repo.ClearLevelsAbove(ctx, bookID, budgetKey, monthKey, currentLevel); err != nil {
		log.Printf("[WARN] CheckBudgets clear levels failed (book %s key %s): %v", bookID, budgetKey, err)
	}
	// 找出新越线的档位，取最高的一个发提醒。
	alertLevel := 0
	for _, lv := range budgetAlertLevels {
		if pct >= lv && !notified[lv] && lv > alertLevel {
			alertLevel = lv
		}
	}
	if alertLevel == 0 {
		return nil
	}
	// 把不高于提醒档位的档位都记为已提醒（一次跳过两档时不补发低档）。
	for _, lv := range budgetAlertLevels {
		if lv <= alertLevel {
			if err := r.repo.MarkNotified(ctx, bookID, budgetKey, monthKey, lv); err != nil {
				log.Printf("[WARN] CheckBudgets mark notified failed (book %s key %s level %d): %v", bookID, budgetKey, lv, err)
			}
		}
	}
	kind := "total"
	if budgetKey != "total" {
		kind = "category"
	}
	alert := BudgetAlertPayload{
		BudgetID:   budgetID,
		BudgetName: budgetName,
		Kind:       kind,
		Level:      alertLevel,
		Used:       used,
		Budget:     limit,
		Month:      monthKey,
	}
	summary := fmt.Sprintf("%s预算已使用 %d%%（%s / %s）", budgetName, pct, formatCentsYuan(used), formatCentsYuan(limit))
	usedCopy := used
	r.Record(ctx, model.BookActivity{
		BookID:      bookID,
		UserID:      "system",
		ActorName:   "预算提醒",
		Action:      model.ActivityBudgetAlert,
		Summary:     summary,
		AmountCents: &usedCopy,
	}, ChangeBudget)
	r.hub.PublishBudgetAlert(bookID, alert)
	return &alert
}

// CheckAllBudgets 遍历全部账本做一次预算检查（每日定时任务用），返回新发提醒总数。
func (r *ActivityRecorder) CheckAllBudgets(ctx context.Context) int {
	if r == nil || r.repo == nil {
		return 0
	}
	ids, err := r.repo.ListAllBookIDs(ctx)
	if err != nil {
		log.Printf("[WARN] CheckAllBudgets list books failed: %v", err)
		return 0
	}
	total := 0
	for _, id := range ids {
		total += len(r.CheckBudgets(ctx, id))
	}
	return total
}

// BudgetAlertScheduler 每天跑一次预算检查，补上“没有新记账但已越线”的情况
// （例如定时入账把预算顶过线、或管理员直接改了库里的数据）。
// 检查本身幂等（已提醒档位当月不再提醒），启动时先跑一次再按 24 小时 tick。
type BudgetAlertScheduler struct {
	recorder *ActivityRecorder
}

func NewBudgetAlertScheduler(recorder *ActivityRecorder) *BudgetAlertScheduler {
	return &BudgetAlertScheduler{recorder: recorder}
}

func (s *BudgetAlertScheduler) RunOnce(ctx context.Context) int {
	if s == nil || s.recorder == nil {
		return 0
	}
	return s.recorder.CheckAllBudgets(ctx)
}

func (s *BudgetAlertScheduler) Start(ctx context.Context) {
	go func() {
		s.RunOnce(ctx)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.RunOnce(ctx)
			}
		}
	}()
}
