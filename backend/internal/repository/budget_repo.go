package repository

import (
	"context"
	"fmt"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
)

type BudgetRepo struct {
	db *DB
}

func NewBudgetRepo(db *DB) *BudgetRepo {
	return &BudgetRepo{db: db}
}

func (r *BudgetRepo) GetBudgetOverview(ctx context.Context, bookID string, year, month int) (*model.BudgetOverview, error) {
	now := time.Now()
	if year <= 0 {
		year = now.Year()
	}
	if month < 1 || month > 12 {
		month = int(now.Month())
	}

	var totalBudget int64 = 450000
	if bookID != "" {
		_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(budget, 450000) FROM books WHERE id = ?`, bookID).Scan(&totalBudget)
	}

	monthPrefix := fmt.Sprintf("%04d-%02d%%", year, month)

	var totalSpent int64
	if bookID != "" {
		_ = r.db.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE book_id = ? AND type = 'expense' AND date LIKE ? AND COALESCE(include_budget, 1) = 1
		`, bookID, monthPrefix).Scan(&totalSpent)
	} else {
		_ = r.db.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE type = 'expense' AND date LIKE ? AND COALESCE(include_budget, 1) = 1
		`, monthPrefix).Scan(&totalSpent)
	}

	// Spent per top-level category (subcategories roll up to parent)
	spentMap := make(map[string]int64)
	if bookID != "" {
		rows, err := r.db.QueryContext(ctx, `
			SELECT COALESCE(c.parent_id, c.id, '') as top_id, COALESCE(SUM(t.amount), 0)
			FROM transactions t
			LEFT JOIN categories c ON t.category_id = c.id
			WHERE t.book_id = ? AND t.type = 'expense' AND t.date LIKE ? AND COALESCE(t.include_budget, 1) = 1
			GROUP BY top_id
		`, bookID, monthPrefix)
		if err == nil {
			for rows.Next() {
				var cid string
				var amt int64
				if err := rows.Scan(&cid, &amt); err == nil && cid != "" {
					spentMap[cid] = amt
				}
			}
			rows.Close()
		}
	}

	catRows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.icon, c.color, COALESCE(c.bg_color, '#FFF6E8'), COALESCE(cb.limit_amount, 0)
		FROM categories c
		LEFT JOIN category_budgets cb ON cb.book_id = c.book_id AND cb.category_id = c.id
		WHERE c.book_id = ? AND c.type = 'expense' AND (c.parent_id IS NULL OR c.parent_id = '')
		ORDER BY c.sort_order ASC, c.created_at ASC
	`, bookID)
	if err != nil {
		return nil, err
	}
	defer catRows.Close()

	catItems := make([]model.CategoryBudgetItem, 0)
	for catRows.Next() {
		var it model.CategoryBudgetItem
		if err := catRows.Scan(&it.CategoryID, &it.CategoryName, &it.CategoryIcon, &it.CategoryColor, &it.BgColor, &it.LimitAmount); err != nil {
			return nil, err
		}
		it.SpentAmount = spentMap[it.CategoryID]
		if it.LimitAmount > 0 {
			it.RemainingAmount = it.LimitAmount - it.SpentAmount
			it.Percentage = int((it.SpentAmount * 100) / it.LimitAmount)
		}
		catItems = append(catItems, it)
	}

	remainingBudget := totalBudget - totalSpent
	pct := 0
	if totalBudget > 0 {
		pct = int((totalSpent * 100) / totalBudget)
	}

	lastDay := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, now.Location()).Day()
	remainingDays := lastDay
	if year == now.Year() && month == int(now.Month()) {
		remainingDays = lastDay - now.Day() + 1
	} else if year < now.Year() || (year == now.Year() && month < int(now.Month())) {
		remainingDays = 0
	}
	if remainingDays < 0 {
		remainingDays = 0
	}

	var dailyAllowance int64
	if remainingBudget > 0 && remainingDays > 0 {
		dailyAllowance = remainingBudget / int64(remainingDays)
	}

	return &model.BudgetOverview{
		BookID:          bookID,
		Year:            year,
		Month:           month,
		TotalBudget:     totalBudget,
		TotalSpent:      totalSpent,
		RemainingBudget: remainingBudget,
		RemainingDays:   remainingDays,
		DailyAllowance:  dailyAllowance,
		Percentage:      pct,
		Categories:      catItems,
	}, nil
}

func (r *BudgetRepo) UpdateTotalBudget(ctx context.Context, bookID string, budget int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE books SET budget = ?, updated_at = datetime('now') WHERE id = ?`, budget, bookID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BudgetRepo) UpsertCategoryBudget(ctx context.Context, bookID, categoryID string, limitAmount int64) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO category_budgets (book_id, category_id, limit_amount, updated_at)
		VALUES (?, ?, ?, datetime('now'))
		ON CONFLICT(book_id, category_id) DO UPDATE SET
			limit_amount = excluded.limit_amount,
			updated_at = datetime('now')
	`, bookID, categoryID, limitAmount)
	return err
}
