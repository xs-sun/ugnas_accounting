package repository

import (
	"context"
	"database/sql"
	"errors"

	"com.xsguyu.accounting/backend/internal/model"
	"github.com/google/uuid"
)

type RecurringRepo struct {
	db *DB
}

func NewRecurringRepo(db *DB) *RecurringRepo {
	return &RecurringRepo{db: db}
}

const recurringSelectSQL = `
	SELECT r.id, r.book_id, r.type, r.category_id,
	       COALESCE(c.name, ''), COALESCE(c.icon, 'tag'), COALESCE(c.color, '#DE8A1E'), COALESCE(c.bg_color, '#FFF6E8'),
	       COALESCE(r.sub_category, ''), r.title, r.amount, r.cycle, r.start_date, COALESCE(r.end_rule, '不结束'),
	       r.next_date, r.executed_count, r.wallet_id, COALESCE(w.name, ''),
	       r.exclude_stats, r.is_active, r.created_by, r.created_at, r.updated_at
	FROM recurring_rules r
	LEFT JOIN categories c ON r.category_id = c.id
	LEFT JOIN wallets w ON r.wallet_id = w.id
`

func scanRecurringRule(row interface {
	Scan(dest ...interface{}) error
}) (*model.RecurringRule, error) {
	var rr model.RecurringRule
	var excludeInt, activeInt int
	err := row.Scan(
		&rr.ID, &rr.BookID, &rr.Type, &rr.CategoryID,
		&rr.CategoryName, &rr.CategoryIcon, &rr.CategoryColor, &rr.BgColor,
		&rr.SubCategory, &rr.Title, &rr.Amount, &rr.Cycle, &rr.StartDate, &rr.EndRule,
		&rr.NextDate, &rr.ExecutedCount, &rr.WalletID, &rr.WalletName,
		&excludeInt, &activeInt, &rr.CreatedBy, &rr.CreatedAt, &rr.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rr.ExcludeStats = excludeInt == 1
	rr.IsActive = activeInt == 1
	return &rr, nil
}

func (r *RecurringRepo) ListRules(ctx context.Context, bookID string) ([]model.RecurringRule, error) {
	query := recurringSelectSQL + ` WHERE 1=1`
	var args []interface{}
	if bookID != "" {
		query += ` AND r.book_id = ?`
		args = append(args, bookID)
	}
	query += ` ORDER BY r.created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]model.RecurringRule, 0)
	for rows.Next() {
		rr, err := scanRecurringRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rr)
	}
	return rules, rows.Err()
}

func (r *RecurringRepo) GetRule(ctx context.Context, id string) (*model.RecurringRule, error) {
	row := r.db.QueryRowContext(ctx, recurringSelectSQL+` WHERE r.id = ?`, id)
	return scanRecurringRule(row)
}

func (r *RecurringRepo) CreateRule(ctx context.Context, rr *model.RecurringRule) error {
	if rr.ID == "" {
		rr.ID = "rec_" + uuid.NewString()[:8]
	}
	if rr.EndRule == "" {
		rr.EndRule = "不结束"
	}
	if rr.NextDate == "" {
		rr.NextDate = rr.StartDate
	}
	excludeInt := 0
	if rr.ExcludeStats {
		excludeInt = 1
	}
	activeInt := 1
	if !rr.IsActive {
		activeInt = 0
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO recurring_rules (
			id, book_id, type, category_id, sub_category, title, amount, cycle, start_date, end_rule,
			next_date, executed_count, wallet_id, exclude_stats, is_active, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, rr.ID, rr.BookID, rr.Type, rr.CategoryID, rr.SubCategory, rr.Title, rr.Amount, rr.Cycle, rr.StartDate, rr.EndRule,
		rr.NextDate, rr.ExecutedCount, rr.WalletID, excludeInt, activeInt, rr.CreatedBy)
	return err
}

func (r *RecurringRepo) UpdateRule(ctx context.Context, rr *model.RecurringRule) error {
	excludeInt := 0
	if rr.ExcludeStats {
		excludeInt = 1
	}
	activeInt := 0
	if rr.IsActive {
		activeInt = 1
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE recurring_rules SET
			type = ?, category_id = ?, sub_category = ?, title = ?, amount = ?, cycle = ?,
			start_date = ?, end_rule = ?, next_date = ?, wallet_id = ?, exclude_stats = ?,
			is_active = ?, updated_at = datetime('now')
		WHERE id = ?
	`, rr.Type, rr.CategoryID, rr.SubCategory, rr.Title, rr.Amount, rr.Cycle,
		rr.StartDate, rr.EndRule, rr.NextDate, rr.WalletID, excludeInt, activeInt, rr.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *RecurringRepo) DeleteRule(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM recurring_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *RecurringRepo) ToggleRule(ctx context.Context, id string) (*model.RecurringRule, error) {
	rr, err := r.GetRule(ctx, id)
	if err != nil {
		return nil, err
	}
	newActive := 1
	if rr.IsActive {
		newActive = 0
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE recurring_rules SET is_active = ?, updated_at = datetime('now') WHERE id = ?`, newActive, id); err != nil {
		return nil, err
	}
	return r.GetRule(ctx, id)
}

func (r *RecurringRepo) ListDueRules(ctx context.Context, today string) ([]model.RecurringRule, error) {
	query := recurringSelectSQL + ` WHERE r.is_active = 1 AND r.next_date <= ? ORDER BY r.next_date ASC`
	rows, err := r.db.QueryContext(ctx, query, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]model.RecurringRule, 0)
	for rows.Next() {
		rr, err := scanRecurringRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rr)
	}
	return rules, rows.Err()
}

func (r *RecurringRepo) MarkRuleExecuted(ctx context.Context, id, nextDate string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE recurring_rules
		SET executed_count = executed_count + 1, next_date = ?, updated_at = datetime('now')
		WHERE id = ?
	`, nextDate, id)
	return err
}
