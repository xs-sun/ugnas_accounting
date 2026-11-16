package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"com.xsguyu.accounting/backend/internal/model"
	"github.com/google/uuid"
)

type TransactionRepo struct {
	db *DB
}

func NewTransactionRepo(db *DB) *TransactionRepo {
	return &TransactionRepo{db: db}
}

func (r *TransactionRepo) CreateTransaction(ctx context.Context, tx *model.Transaction, tagIDs []string) error {
	dbTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer dbTx.Rollback()

	if tx.ID == "" {
		tx.ID = uuid.NewString()
	}

	// Validate foreign keys exist
	var dummy string
	if err := dbTx.QueryRowContext(ctx, `SELECT id FROM books WHERE id = ?`, tx.BookID).Scan(&dummy); err != nil {
		return fmt.Errorf("invalid book_id: %w", err)
	}
	if err := dbTx.QueryRowContext(ctx, `SELECT id FROM wallets WHERE id = ?`, tx.WalletID).Scan(&dummy); err != nil {
		return fmt.Errorf("invalid wallet_id: %w", err)
	}
	if tx.ToWalletID != nil && *tx.ToWalletID != "" {
		if err := dbTx.QueryRowContext(ctx, `SELECT id FROM wallets WHERE id = ?`, *tx.ToWalletID).Scan(&dummy); err != nil {
			return fmt.Errorf("invalid to_wallet_id: %w", err)
		}
	}
	if tx.CategoryID != nil && *tx.CategoryID != "" {
		if err := dbTx.QueryRowContext(ctx, `SELECT id FROM categories WHERE id = ?`, *tx.CategoryID).Scan(&dummy); err != nil {
			return fmt.Errorf("invalid category_id: %w", err)
		}
	}

	// Insert transaction
	insQuery := `
	INSERT INTO transactions (
		id, book_id, type, amount, date, wallet_id, to_wallet_id, category_id, note, order_no, created_by
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = dbTx.ExecContext(ctx, insQuery,
		tx.ID, tx.BookID, tx.Type, tx.Amount, tx.Date, tx.WalletID, tx.ToWalletID, tx.CategoryID, tx.Note, tx.OrderNo, tx.CreatedBy,
	)
	if err != nil {
		return err
	}

	// Balance Linkage
	switch tx.Type {
	case "expense":
		_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance - ?, updated_at = datetime('now') WHERE id = ?`, tx.Amount, tx.WalletID)
	case "income":
		_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance + ?, updated_at = datetime('now') WHERE id = ?`, tx.Amount, tx.WalletID)
	case "transfer":
		if tx.ToWalletID == nil || *tx.ToWalletID == "" {
			return errors.New("transfer requires to_wallet_id")
		}
		_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance - ?, updated_at = datetime('now') WHERE id = ?`, tx.Amount, tx.WalletID)
		if err == nil {
			_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance + ?, updated_at = datetime('now') WHERE id = ?`, tx.Amount, *tx.ToWalletID)
		}
	}
	if err != nil {
		return err
	}

	// Tag linkages
	for _, tagID := range tagIDs {
		_, err = dbTx.ExecContext(ctx, `INSERT INTO transaction_tags (transaction_id, tag_id) VALUES (?, ?)`, tx.ID, tagID)
		if err != nil {
			return err
		}
	}

	return dbTx.Commit()
}

func (r *TransactionRepo) AddAttachment(ctx context.Context, id, txID, fileName, filePath string, fileSize int64, mimeType string) error {
	query := `INSERT INTO attachments (id, transaction_id, file_name, file_path, file_size, mime_type) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, id, txID, fileName, filePath, fileSize, mimeType)
	return err
}

func (r *TransactionRepo) GetTransaction(ctx context.Context, txID string) (*model.Transaction, error) {
	query := `
	SELECT t.id, t.book_id, t.type, t.amount, t.date, t.wallet_id, t.to_wallet_id, t.category_id,
	       t.note, t.order_no, t.created_by, t.created_at, t.updated_at,
	       COALESCE(c.name, ''), COALESCE(c.icon, ''), COALESCE(c.color, ''),
	       COALESCE(w.name, ''), COALESCE(tw.name, '')
	FROM transactions t
	LEFT JOIN categories c ON t.category_id = c.id
	LEFT JOIN wallets w ON t.wallet_id = w.id
	LEFT JOIN wallets tw ON t.to_wallet_id = tw.id
	WHERE t.id = ?
	`
	var tx model.Transaction
	var toW, catID sql.NullString
	tx.Tags = []model.Tag{}

	err := r.db.QueryRowContext(ctx, query, txID).Scan(
		&tx.ID, &tx.BookID, &tx.Type, &tx.Amount, &tx.Date, &tx.WalletID, &toW, &catID,
		&tx.Note, &tx.OrderNo, &tx.CreatedBy, &tx.CreatedAt, &tx.UpdatedAt,
		&tx.CategoryName, &tx.CategoryIcon, &tx.CategoryColor,
		&tx.WalletName, &tx.ToWalletName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if toW.Valid {
		tx.ToWalletID = &toW.String
	}
	if catID.Valid {
		tx.CategoryID = &catID.String
	}

	// Fetch tags
	tagRows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.book_id, t.name, t.color, t.created_at
		FROM tags t
		JOIN transaction_tags tt ON t.id = tt.tag_id
		WHERE tt.transaction_id = ?
	`, txID)
	if err == nil {
		defer tagRows.Close()
		for tagRows.Next() {
			var tag model.Tag
			if err := tagRows.Scan(&tag.ID, &tag.BookID, &tag.Name, &tag.Color, &tag.CreatedAt); err == nil {
				tx.Tags = append(tx.Tags, tag)
			}
		}
	}

	return &tx, nil
}

func (r *TransactionRepo) ListTransactions(ctx context.Context, bookID string, walletID, categoryID, txType, startDate, endDate string, limit, offset int) ([]model.Transaction, int, error) {
	where := []string{"1=1"}
	var args []interface{}

	if bookID != "" {
		where = append(where, "t.book_id = ?")
		args = append(args, bookID)
	}
	if walletID != "" {
		where = append(where, "(t.wallet_id = ? OR t.to_wallet_id = ?)")
		args = append(args, walletID, walletID)
	}
	if categoryID != "" {
		where = append(where, "t.category_id = ?")
		args = append(args, categoryID)
	}
	if txType != "" {
		where = append(where, "t.type = ?")
		args = append(args, txType)
	}
	if startDate != "" {
		where = append(where, "t.date >= ?")
		args = append(args, startDate)
	}
	if endDate != "" {
		where = append(where, "t.date <= ?")
		args = append(args, endDate)
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countQuery := fmt.Sprintf("SELECT count(*) FROM transactions t WHERE %s", whereClause)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(`
	SELECT t.id, t.book_id, t.type, t.amount, t.date, t.wallet_id, t.to_wallet_id, t.category_id,
	       t.note, t.order_no, t.created_by, t.created_at, t.updated_at,
	       COALESCE(c.name, ''), COALESCE(c.icon, ''), COALESCE(c.color, ''),
	       COALESCE(w.name, ''), COALESCE(tw.name, '')
	FROM transactions t
	LEFT JOIN categories c ON t.category_id = c.id
	LEFT JOIN wallets w ON t.wallet_id = w.id
	LEFT JOIN wallets tw ON t.to_wallet_id = tw.id
	WHERE %s
	ORDER BY t.date DESC, t.created_at DESC
	LIMIT ? OFFSET ?
	`, whereClause)

	argsWithLimit := append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, argsWithLimit...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	transactions := make([]model.Transaction, 0)
	for rows.Next() {
		var tx model.Transaction
		var toW, catID sql.NullString
		tx.Tags = []model.Tag{}

		if err := rows.Scan(
			&tx.ID, &tx.BookID, &tx.Type, &tx.Amount, &tx.Date, &tx.WalletID, &toW, &catID,
			&tx.Note, &tx.OrderNo, &tx.CreatedBy, &tx.CreatedAt, &tx.UpdatedAt,
			&tx.CategoryName, &tx.CategoryIcon, &tx.CategoryColor,
			&tx.WalletName, &tx.ToWalletName,
		); err != nil {
			return nil, 0, err
		}
		if toW.Valid {
			tx.ToWalletID = &toW.String
		}
		if catID.Valid {
			tx.CategoryID = &catID.String
		}

		// Fetch tags
		tagRows, tagErr := r.db.QueryContext(ctx, `
			SELECT tg.id, tg.book_id, tg.name, tg.color, tg.created_at
			FROM tags tg
			JOIN transaction_tags tt ON tg.id = tt.tag_id
			WHERE tt.transaction_id = ?
		`, tx.ID)
		if tagErr == nil {
			for tagRows.Next() {
				var tag model.Tag
				if err := tagRows.Scan(&tag.ID, &tag.BookID, &tag.Name, &tag.Color, &tag.CreatedAt); err == nil {
					tx.Tags = append(tx.Tags, tag)
				}
			}
			tagRows.Close()
		}

		transactions = append(transactions, tx)
	}

	return transactions, total, rows.Err()
}

func (r *TransactionRepo) DeleteTransaction(ctx context.Context, txID string) error {
	dbTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer dbTx.Rollback()

	// Get transaction details for rollback
	var oldTx model.Transaction
	var toW, catID sql.NullString
	query := `SELECT id, book_id, type, amount, date, wallet_id, to_wallet_id, category_id FROM transactions WHERE id = ?`
	err = dbTx.QueryRowContext(ctx, query, txID).Scan(
		&oldTx.ID, &oldTx.BookID, &oldTx.Type, &oldTx.Amount, &oldTx.Date, &oldTx.WalletID, &toW, &catID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if toW.Valid {
		oldTx.ToWalletID = &toW.String
	}

	// Balance Rollback
	switch oldTx.Type {
	case "expense":
		_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance + ?, updated_at = datetime('now') WHERE id = ?`, oldTx.Amount, oldTx.WalletID)
	case "income":
		_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance - ?, updated_at = datetime('now') WHERE id = ?`, oldTx.Amount, oldTx.WalletID)
	case "transfer":
		_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance + ?, updated_at = datetime('now') WHERE id = ?`, oldTx.Amount, oldTx.WalletID)
		if err == nil && oldTx.ToWalletID != nil {
			_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance - ?, updated_at = datetime('now') WHERE id = ?`, oldTx.Amount, *oldTx.ToWalletID)
		}
	}
	if err != nil {
		return err
	}

	// Delete from transactions (triggers automatic rollback in daily_summaries via trg_tx_delete_summary)
	_, err = dbTx.ExecContext(ctx, `DELETE FROM transactions WHERE id = ?`, txID)
	if err != nil {
		return err
	}

	return dbTx.Commit()
}

func (r *TransactionRepo) UpdateTransaction(ctx context.Context, txID string, updates map[string]interface{}) (*model.Transaction, error) {
	dbTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer dbTx.Rollback()

	// Load existing transaction
	var oldTx model.Transaction
	var toW, catID sql.NullString
	query := `SELECT id, book_id, type, amount, date, wallet_id, to_wallet_id, category_id, note, order_no, created_by, created_at, updated_at FROM transactions WHERE id = ?`
	err = dbTx.QueryRowContext(ctx, query, txID).Scan(
		&oldTx.ID, &oldTx.BookID, &oldTx.Type, &oldTx.Amount, &oldTx.Date, &oldTx.WalletID, &toW, &catID,
		&oldTx.Note, &oldTx.OrderNo, &oldTx.CreatedBy, &oldTx.CreatedAt, &oldTx.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if toW.Valid {
		oldTx.ToWalletID = &toW.String
	}
	if catID.Valid {
		oldTx.CategoryID = &catID.String
	}

	// Calculate new values
	newTx := oldTx
	if v, ok := updates["amount"]; ok {
		switch val := v.(type) {
		case float64:
			newTx.Amount = int64(val)
		case int64:
			newTx.Amount = val
		case int:
			newTx.Amount = int64(val)
		}
	}
	if v, ok := updates["date"]; ok {
		if s, ok := v.(string); ok && s != "" {
			newTx.Date = s
		}
	}
	if v, ok := updates["note"]; ok {
		if s, ok := v.(string); ok {
			newTx.Note = s
		}
	}
	if v, ok := updates["wallet_id"]; ok {
		if s, ok := v.(string); ok && s != "" {
			newTx.WalletID = s
		}
	}
	if v, ok := updates["category_id"]; ok {
		if s, ok := v.(string); ok {
			newTx.CategoryID = &s
		}
	}

	// Rollback old balance
	switch oldTx.Type {
	case "expense":
		_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance + ?, updated_at = datetime('now') WHERE id = ?`, oldTx.Amount, oldTx.WalletID)
	case "income":
		_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance - ?, updated_at = datetime('now') WHERE id = ?`, oldTx.Amount, oldTx.WalletID)
	case "transfer":
		_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance + ?, updated_at = datetime('now') WHERE id = ?`, oldTx.Amount, oldTx.WalletID)
		if err == nil && oldTx.ToWalletID != nil {
			_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance - ?, updated_at = datetime('now') WHERE id = ?`, oldTx.Amount, *oldTx.ToWalletID)
		}
	}
	if err != nil {
		return nil, err
	}

	// Apply new balance
	switch newTx.Type {
	case "expense":
		_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance - ?, updated_at = datetime('now') WHERE id = ?`, newTx.Amount, newTx.WalletID)
	case "income":
		_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance + ?, updated_at = datetime('now') WHERE id = ?`, newTx.Amount, newTx.WalletID)
	case "transfer":
		_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance - ?, updated_at = datetime('now') WHERE id = ?`, newTx.Amount, newTx.WalletID)
		if err == nil && newTx.ToWalletID != nil {
			_, err = dbTx.ExecContext(ctx, `UPDATE wallets SET balance = balance + ?, updated_at = datetime('now') WHERE id = ?`, newTx.Amount, *newTx.ToWalletID)
		}
	}
	if err != nil {
		return nil, err
	}

	// Update daily_summaries if expense/income and date/amount/category changed
	if oldTx.Type == "expense" || oldTx.Type == "income" {
		oldCat := "uncategorized"
		if oldTx.CategoryID != nil && *oldTx.CategoryID != "" {
			oldCat = *oldTx.CategoryID
		}
		newCat := "uncategorized"
		if newTx.CategoryID != nil && *newTx.CategoryID != "" {
			newCat = *newTx.CategoryID
		}

		// Decrement old summary
		_, _ = dbTx.ExecContext(ctx, `
			UPDATE daily_summaries SET total_amount = total_amount - ?, count = count - 1
			WHERE date = ? AND book_id = ? AND type = ? AND category_id = ?
		`, oldTx.Amount, oldTx.Date, oldTx.BookID, oldTx.Type, oldCat)

		// Increment new summary
		_, _ = dbTx.ExecContext(ctx, `
			INSERT INTO daily_summaries (date, book_id, type, category_id, total_amount, count)
			VALUES (?, ?, ?, ?, ?, 1)
			ON CONFLICT(date, book_id, type, category_id) DO UPDATE SET
				total_amount = total_amount + ?,
				count = count + 1
		`, newTx.Date, newTx.BookID, newTx.Type, newCat, newTx.Amount, newTx.Amount)
	}

	// Update transactions table
	updQuery := `
	UPDATE transactions SET
		amount = ?, date = ?, wallet_id = ?, to_wallet_id = ?, category_id = ?, note = ?, updated_at = datetime('now')
	WHERE id = ?
	`
	_, err = dbTx.ExecContext(ctx, updQuery,
		newTx.Amount, newTx.Date, newTx.WalletID, newTx.ToWalletID, newTx.CategoryID, newTx.Note, txID,
	)
	if err != nil {
		return nil, err
	}

	if err := dbTx.Commit(); err != nil {
		return nil, err
	}

	return &newTx, nil
}

// Reporting Queries against daily_summaries
func (r *TransactionRepo) GetMonthlyReport(ctx context.Context, bookID string, year, month int) (*model.MonthlyReport, error) {
	prefix := fmt.Sprintf("%04d-%02d", year, month)

	where := []string{"date LIKE ?"}
	args := []interface{}{prefix + "%"}
	if bookID != "" {
		where = append(where, "book_id = ?")
		args = append(args, bookID)
	}

	query := fmt.Sprintf(`
	SELECT type, category_id, SUM(total_amount), SUM(count)
	FROM daily_summaries
	WHERE %s
	GROUP BY type, category_id
	`, strings.Join(where, " AND "))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	report := &model.MonthlyReport{
		Year:       year,
		Month:      month,
		Categories: []model.CategoryReportItem{},
	}

	for rows.Next() {
		var t, catID string
		var totalAmount int64
		var count int
		if err := rows.Scan(&t, &catID, &totalAmount, &count); err != nil {
			return nil, err
		}
		if t == "expense" {
			report.TotalExpense += totalAmount
			report.Categories = append(report.Categories, model.CategoryReportItem{
				CategoryID:  catID,
				Amount:      totalAmount,
				TotalAmount: totalAmount,
				Count:       count,
			})
		} else if t == "income" {
			report.TotalIncome += totalAmount
		}
	}

	report.NetSavings = report.TotalIncome - report.TotalExpense

	// Enrich category names, icons, colors & percentages
	for i := range report.Categories {
		item := &report.Categories[i]
		if report.TotalExpense > 0 {
			item.Percentage = float64(int64((float64(item.TotalAmount)/float64(report.TotalExpense)*100.0)*100)) / 100.0
		}
		var name, icon, color string
		if item.CategoryID != "uncategorized" && item.CategoryID != "" {
			if err := r.db.QueryRowContext(ctx, `SELECT name, icon, color FROM categories WHERE id = ?`, item.CategoryID).Scan(&name, &icon, &color); err == nil {
				item.CategoryName = name
				item.CategoryIcon = icon
				item.CategoryColor = color
			} else {
				item.CategoryName = "未分类"
				item.CategoryIcon = "tag"
				item.CategoryColor = "#0E8C7F"
			}
		} else {
			item.CategoryName = "未分类"
			item.CategoryIcon = "tag"
			item.CategoryColor = "#0E8C7F"
		}
	}

	// Sort categories by total_amount descending
	for i := 0; i < len(report.Categories); i++ {
		for j := i + 1; j < len(report.Categories); j++ {
			if report.Categories[i].TotalAmount < report.Categories[j].TotalAmount {
				report.Categories[i], report.Categories[j] = report.Categories[j], report.Categories[i]
			}
		}
	}

	return report, nil
}

func (r *TransactionRepo) GetTrendReport(ctx context.Context, bookID string, months int) (*model.TrendReport, error) {
	if months <= 0 {
		months = 6
	}

	where := []string{"1=1"}
	var args []interface{}
	if bookID != "" {
		where = append(where, "book_id = ?")
		args = append(args, bookID)
	}

	query := fmt.Sprintf(`
	SELECT substr(date, 1, 7) as ym, type, SUM(total_amount)
	FROM daily_summaries
	WHERE %s
	GROUP BY ym, type
	ORDER BY ym DESC
	LIMIT ?
	`, strings.Join(where, " AND "))

	args = append(args, months*2)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trendMap := make(map[string]*model.TrendMonthItem)
	var ymList []string

	for rows.Next() {
		var ym, t string
		var amount int64
		if err := rows.Scan(&ym, &t, &amount); err != nil {
			return nil, err
		}
		item, ok := trendMap[ym]
		if !ok {
			item = &model.TrendMonthItem{Month: ym}
			trendMap[ym] = item
			ymList = append(ymList, ym)
		}
		if t == "expense" {
			item.Expense += amount
		} else if t == "income" {
			item.Income += amount
		}
	}

	// Sort ascending by month
	for i := 0; i < len(ymList); i++ {
		for j := i + 1; j < len(ymList); j++ {
			if ymList[i] > ymList[j] {
				ymList[i], ymList[j] = ymList[j], ymList[i]
			}
		}
	}

	results := make([]model.TrendMonthItem, 0, len(ymList))
	for _, ym := range ymList {
		results = append(results, *trendMap[ym])
	}
	return &model.TrendReport{Months: results}, nil
}

func (r *TransactionRepo) GetDailyReport(ctx context.Context, bookID string, year, month int) ([]model.DailyReportItem, error) {
	prefix := fmt.Sprintf("%04d-%02d", year, month)

	where := []string{"date LIKE ?"}
	args := []interface{}{prefix + "%"}
	if bookID != "" {
		where = append(where, "book_id = ?")
		args = append(args, bookID)
	}

	query := fmt.Sprintf(`
	SELECT date, type, SUM(total_amount)
	FROM daily_summaries
	WHERE %s
	GROUP BY date, type
	ORDER BY date ASC
	`, strings.Join(where, " AND "))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dayMap := make(map[string]*model.DailyReportItem)
	var days []string

	for rows.Next() {
		var d, t string
		var amount int64
		if err := rows.Scan(&d, &t, &amount); err != nil {
			return nil, err
		}
		item, ok := dayMap[d]
		if !ok {
			item = &model.DailyReportItem{Date: d}
			dayMap[d] = item
			days = append(days, d)
		}
		if t == "expense" {
			item.Expense += amount
		} else if t == "income" {
			item.Income += amount
		}
	}

	results := make([]model.DailyReportItem, 0, len(days))
	for _, d := range days {
		results = append(results, *dayMap[d])
	}
	return results, nil
}
