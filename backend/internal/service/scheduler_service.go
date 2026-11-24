package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

type RecurringScheduler struct {
	recurringRepo *repository.RecurringRepo
	txRepo        *repository.TransactionRepo
}

func NewRecurringScheduler(recurringRepo *repository.RecurringRepo, txRepo *repository.TransactionRepo) *RecurringScheduler {
	return &RecurringScheduler{
		recurringRepo: recurringRepo,
		txRepo:        txRepo,
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
	today := time.Now().Format("2006-01-02")
	rules, err := s.recurringRepo.ListDueRules(ctx, today)
	if err != nil {
		log.Printf("[WARN] RecurringScheduler ListDueRules error: %v", err)
		return 0
	}

	executed := 0
	for _, r := range rules {
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
