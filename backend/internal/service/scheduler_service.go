package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	_ "time/tzdata" // 内嵌时区库：容器里也可能没有系统 zoneinfo，Asia/Shanghai 必须能加载

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

// 定时入账的执行时刻：北京时间 9:00。规则本身只有日期没有时间，
// 流水上显示的 09:00 是写死的；以前调度器只比日期，过了午夜就入账，
// 所以一条“今天到期”的规则会在凌晨 0 点就被记上。
const recurringExecHour = 9

var beijingLoc = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.Local
}()

// shouldPostRule 判断一条到期规则现在该不该入账（now 已换算成北京时间）：
// next_date < 今天 → 已逾期（比如 NAS 关机几天），直接补记；
// next_date == 今天 → 只有到了当天 9:00 才入账；
// next_date > 今天 → 还没到期（正常情况下 ListDueRules 不会返回这种）。
func shouldPostRule(r model.RecurringRule, now time.Time) bool {
	today := now.Format("2006-01-02")
	if r.NextDate < today {
		return true
	}
	if r.NextDate > today {
		return false
	}
	return now.Hour() >= recurringExecHour
}

type RecurringScheduler struct {
	recurringRepo *repository.RecurringRepo
	txRepo        *repository.TransactionRepo
	hub           *Hub
	recorder      *ActivityRecorder
}

func NewRecurringScheduler(recurringRepo *repository.RecurringRepo, txRepo *repository.TransactionRepo, hub *Hub, recorder *ActivityRecorder) *RecurringScheduler {
	return &RecurringScheduler{
		recurringRepo: recurringRepo,
		txRepo:        txRepo,
		hub:           hub,
		recorder:      recorder,
	}
}

func ComputeNextDate(dateStr, cycle string) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		t = time.Now()
	}
	switch strings.TrimSpace(cycle) {
	case "每天", "daily":
		return t.AddDate(0, 0, 1).Format("2006-01-02")
	case "每周", "weekly":
		return t.AddDate(0, 0, 7).Format("2006-01-02")
	case "每两周", "biweekly":
		return t.AddDate(0, 0, 14).Format("2006-01-02")
	case "每年", "yearly":
		return t.AddDate(1, 0, 0).Format("2006-01-02")
	case "工作日", "workday":
		next := t.AddDate(0, 0, 1)
		for next.Weekday() == time.Saturday || next.Weekday() == time.Sunday {
			next = next.AddDate(0, 0, 1)
		}
		return next.Format("2006-01-02")
	default: // "每月" / "monthly"
		return t.AddDate(0, 1, 0).Format("2006-01-02")
	}
}

func (s *RecurringScheduler) RunOnce(ctx context.Context) int {
	// “今天”和“9 点”都按北京时间算，不依赖 NAS 的系统时区设置。
	now := time.Now().In(beijingLoc)
	today := now.Format("2006-01-02")
	rules, err := s.recurringRepo.ListDueRules(ctx, today)
	if err != nil {
		log.Printf("[WARN] RecurringScheduler ListDueRules error: %v", err)
		return 0
	}

	executed := 0
	for _, r := range rules {
		if !shouldPostRule(r, now) {
			continue
		}
		if r.Amount <= 0 || r.WalletID == "" {
			continue
		}
		var catPtr *string
		if r.CategoryID != "" {
			cid := r.CategoryID
			catPtr = &cid
		}
		tx := &model.Transaction{
			BookID:        r.BookID,
			Type:          r.Type,
			Amount:        r.Amount,
			Date:          r.NextDate,
			TxTime:        "09:00",
			WalletID:      r.WalletID,
			CategoryID:    catPtr,
			Title:         r.Title,
			SubCategory:   r.SubCategory,
			Note:          "定时自动记账",
			IncludeBudget: !r.ExcludeStats,
			CreatedBy:     r.CreatedBy,
		}
		if err := s.txRepo.CreateTransaction(ctx, tx, nil); err != nil {
			log.Printf("[WARN] RecurringScheduler CreateTransaction for rule %s failed: %v", r.ID, err)
			continue
		}
		nextDate := ComputeNextDate(r.NextDate, r.Cycle)
		if err := s.recurringRepo.MarkRuleExecuted(ctx, r.ID, nextDate); err != nil {
			log.Printf("[WARN] RecurringScheduler MarkRuleExecuted %s failed: %v", r.ID, err)
			continue
		}
		executed++
		amt := r.Amount
		txID := tx.ID
		if s.recorder != nil {
			s.recorder.Record(ctx, model.BookActivity{
				BookID:      r.BookID,
				UserID:      r.CreatedBy,
				ActorName:   "定时任务",
				Action:      model.ActivityRecurringPosted,
				Summary:     fmt.Sprintf("定时记账入账「%s」%s", r.Title, formatCentsYuan(r.Amount)),
				AmountCents: &amt,
				TxID:        &txID,
			}, ChangeTransaction)
			// 定时入账也可能把当月预算顶过线，立即检查一次。
			s.recorder.CheckBudgets(ctx, r.BookID)
		} else if s.hub != nil {
			s.hub.Publish(r.BookID, ChangeTransaction, "定时任务")
		}
	}
	return executed
}

func (s *RecurringScheduler) Start(ctx context.Context) {
	go func() {
		s.RunOnce(ctx)
		ticker := time.NewTicker(1 * time.Minute)
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

type AutoBackupScheduler struct {
	snapshotSvc  *SnapshotService
	settingsRepo *repository.SettingsRepo
}

func NewAutoBackupScheduler(snapshotSvc *SnapshotService, settingsRepo *repository.SettingsRepo) *AutoBackupScheduler {
	return &AutoBackupScheduler{
		snapshotSvc:  snapshotSvc,
		settingsRepo: settingsRepo,
	}
}

func (s *AutoBackupScheduler) ExecuteBackupAndPrune(ctx context.Context, retain int) (*model.Snapshot, error) {
	snap, err := s.snapshotSvc.CreateSnapshot(ctx, "system_auto")
	if err != nil {
		return nil, err
	}
	if retain > 0 {
		list, err := s.snapshotSvc.ListSnapshots(ctx)
		if err == nil && len(list) > retain {
			for _, old := range list[retain:] {
				_ = s.snapshotSvc.DeleteSnapshot(ctx, old.ID)
			}
		}
	}
	return snap, nil
}

func (s *AutoBackupScheduler) RunOnce(ctx context.Context) (bool, error) {
	cfg, err := s.settingsRepo.GetSystemConfig(ctx)
	if err != nil || !cfg.AutoBackupEnabled {
		return false, err
	}

	now := time.Now()
	periodKey := now.Format("2006-01-02")
	switch {
	case strings.Contains(cfg.AutoBackupCycle, "每周"):
		y, w := now.ISOWeek()
		periodKey = fmt.Sprintf("%04d-W%02d", y, w)
	case strings.Contains(cfg.AutoBackupCycle, "每月"):
		periodKey = now.Format("2006-01")
	}

	lastPeriod, _ := s.settingsRepo.GetSetting(ctx, "last_auto_backup_period")
	if lastPeriod == periodKey {
		return false, nil
	}

	if _, err := s.ExecuteBackupAndPrune(ctx, cfg.AutoBackupRetain); err != nil {
		return false, err
	}
	_ = s.settingsRepo.SetSetting(ctx, "last_auto_backup_period", periodKey)
	return true, nil
}

func (s *AutoBackupScheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := s.RunOnce(ctx); err != nil {
					log.Printf("[WARN] AutoBackupScheduler failed: %v", err)
				}
			}
		}
	}()
}
