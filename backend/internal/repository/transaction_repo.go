package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"github.com/google/uuid"
)

type TransactionRepo struct {
	db *DB
}

func NewTransactionRepo(db *DB) *TransactionRepo {
	return &TransactionRepo{db: db}
}

// 流水归属成员的通用列与连接：member_id 为空时兜底为创建人（历史数据）。
// 三处流水查询（详情、列表、统计 Top）共用，保持 Scan 顺序一致：
// member_id、member_name、member_avatar、member_color，紧跟在 created_by 三元组之后。
const txMemberColumns = `,
       COALESCE(NULLIF(t.member_id, ''), t.created_by),
       COALESCE(NULLIF(mm.user_name, ''), NULLIF(mu.name, ''), COALESCE(NULLIF(t.member_id, ''), t.created_by), ''),
       COALESCE(NULLIF(mu.avatar, ''), NULLIF(mm.avatar, ''), ''),
       COALESCE(mm.color, '#DE8A1E')`

const txMemberJoins = `
	LEFT JOIN book_members mm ON t.book_id = mm.book_id AND COALESCE(NULLIF(t.member_id, ''), t.created_by) = mm.user_id
	LEFT JOIN users mu ON COALESCE(NULLIF(t.member_id, ''), t.created_by) = mu.id`

func (r *TransactionRepo) CreateTransaction(ctx context.Context, tx *model.Transaction, tagIDs []string) error {
	dbTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer dbTx.Rollback()

	if tx.ID == "" {
		tx.ID = uuid.NewString()
	}

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

	incBudgetInt := 1
	if !tx.IncludeBudget {
		incBudgetInt = 0
	}
	reimburseInt := 0
	if tx.Reimburse {
		reimburseInt = 1
	}
	// 未指定归属成员时默认是创建人本人。
	if tx.MemberID == "" {
		tx.MemberID = tx.CreatedBy
	}

	insQuery := `
	INSERT INTO transactions (
		id, book_id, type, amount, date, tx_time, wallet_id, to_wallet_id, category_id,
		title, sub_category, note, order_no, include_budget, reimburse, created_by, member_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = dbTx.ExecContext(ctx, insQuery,
		tx.ID, tx.BookID, tx.Type, tx.Amount, tx.Date, tx.TxTime, tx.WalletID, tx.ToWalletID, tx.CategoryID,
		tx.Title, tx.SubCategory, tx.Note, tx.OrderNo, incBudgetInt, reimburseInt, tx.CreatedBy, tx.MemberID,
	)
	if err != nil {
		return err
	}

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
	SELECT t.id, t.book_id, t.type, t.amount, t.date, COALESCE(t.tx_time, ''), t.wallet_id, t.to_wallet_id, t.category_id,
	       COALESCE(t.title, ''), COALESCE(t.sub_category, ''), COALESCE(t.note, ''), COALESCE(t.order_no, ''),
	       COALESCE(t.include_budget, 1), COALESCE(t.reimburse, 0), t.created_by,
	       COALESCE(NULLIF(bm.user_name, ''), NULLIF(u.name, ''), t.created_by, ''),
	       COALESCE(NULLIF(u.avatar, ''), NULLIF(bm.avatar, ''), '')` + txMemberColumns + `,
	       t.created_at, t.updated_at,
	       COALESCE(c.name, ''), COALESCE(c.icon, ''), COALESCE(c.color, ''), COALESCE(c.bg_color, ''),
	       COALESCE(w.name, ''), COALESCE(tw.name, '')
	FROM transactions t
	LEFT JOIN categories c ON t.category_id = c.id
	LEFT JOIN wallets w ON t.wallet_id = w.id
	LEFT JOIN wallets tw ON t.to_wallet_id = tw.id
	LEFT JOIN book_members bm ON t.book_id = bm.book_id AND t.created_by = bm.user_id
	LEFT JOIN users u ON t.created_by = u.id` + txMemberJoins + `
	WHERE t.id = ?
	`
	var tx model.Transaction
	var toW, catID sql.NullString
	var incBudgetInt, reimburseInt int
	tx.Tags = []model.Tag{}

	err := r.db.QueryRowContext(ctx, query, txID).Scan(
		&tx.ID, &tx.BookID, &tx.Type, &tx.Amount, &tx.Date, &tx.TxTime, &tx.WalletID, &toW, &catID,
		&tx.Title, &tx.SubCategory, &tx.Note, &tx.OrderNo,
		&incBudgetInt, &reimburseInt, &tx.CreatedBy, &tx.CreatedByName, &tx.CreatedByAvatar,
		&tx.MemberID, &tx.MemberName, &tx.MemberAvatar, &tx.MemberColor,
		&tx.CreatedAt, &tx.UpdatedAt,
		&tx.CategoryName, &tx.CategoryIcon, &tx.CategoryColor, &tx.CategoryBgColor,
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
	tx.IncludeBudget = incBudgetInt == 1
	tx.Reimburse = reimburseInt == 1

	tx.Tags = r.fetchTxTags(ctx, txID)
	return &tx, nil
}

func (r *TransactionRepo) fetchTxTags(ctx context.Context, txID string) []model.Tag {
	tags := []model.Tag{}
	tagRows, err := r.db.QueryContext(ctx, `
		SELECT tg.id, tg.book_id, tg.name, tg.color, tg.created_at
		FROM tags tg
		JOIN transaction_tags tt ON tg.id = tt.tag_id
		WHERE tt.transaction_id = ?
	`, txID)
	if err == nil {
		defer tagRows.Close()
		for tagRows.Next() {
			var tag model.Tag
			if err := tagRows.Scan(&tag.ID, &tag.BookID, &tag.Name, &tag.Color, &tag.CreatedAt); err == nil {
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

func (r *TransactionRepo) ListTransactions(ctx context.Context, bookID string, walletID, categoryID, txType, startDate, endDate, search string, limit, offset int, memberID string) ([]model.Transaction, int, error) {
	where := []string{"1=1"}
	var args []interface{}

	if bookID != "" {
		where = append(where, "t.book_id = ?")
		args = append(args, bookID)
	}
	if memberID != "" {
		where = append(where, "COALESCE(NULLIF(t.member_id, ''), t.created_by) = ?")
		args = append(args, memberID)
	}
	if walletID != "" {
		where = append(where, "(t.wallet_id = ? OR t.to_wallet_id = ?)")
		args = append(args, walletID, walletID)
	}
	if categoryID != "" {
		where = append(where, "(t.category_id = ? OR c.parent_id = ?)")
		args = append(args, categoryID, categoryID)
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
	if strings.TrimSpace(search) != "" {
		kw := "%" + strings.TrimSpace(search) + "%"
		where = append(where, "(t.title LIKE ? OR t.note LIKE ? OR t.sub_category LIKE ? OR c.name LIKE ? OR w.name LIKE ?)")
		args = append(args, kw, kw, kw, kw, kw)
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countQuery := fmt.Sprintf(`
		SELECT count(*)
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		LEFT JOIN wallets w ON t.wallet_id = w.id
		WHERE %s
	`, whereClause)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(`
	SELECT t.id, t.book_id, t.type, t.amount, t.date, COALESCE(t.tx_time, ''), t.wallet_id, t.to_wallet_id, t.category_id,
	       COALESCE(t.title, ''), COALESCE(t.sub_category, ''), COALESCE(t.note, ''), COALESCE(t.order_no, ''),
	       COALESCE(t.include_budget, 1), COALESCE(t.reimburse, 0), t.created_by,
	       COALESCE(NULLIF(bm.user_name, ''), NULLIF(u.name, ''), t.created_by, ''),
	       COALESCE(NULLIF(u.avatar, ''), NULLIF(bm.avatar, ''), '')` + txMemberColumns + `,
	       t.created_at, t.updated_at,
	       COALESCE(c.name, ''), COALESCE(c.icon, ''), COALESCE(c.color, ''), COALESCE(c.bg_color, ''),
	       COALESCE(w.name, ''), COALESCE(tw.name, '')
	FROM transactions t
	LEFT JOIN categories c ON t.category_id = c.id
	LEFT JOIN wallets w ON t.wallet_id = w.id
	LEFT JOIN wallets tw ON t.to_wallet_id = tw.id
	LEFT JOIN book_members bm ON t.book_id = bm.book_id AND t.created_by = bm.user_id
	LEFT JOIN users u ON t.created_by = u.id` + txMemberJoins + `
	WHERE %s
	ORDER BY t.date DESC, COALESCE(t.tx_time, '') DESC, t.created_at DESC
	LIMIT ? OFFSET ?
	`, whereClause)

	argsWithLimit := append(append([]interface{}{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, query, argsWithLimit...)
	if err != nil {
		return nil, 0, err
	}

	transactions := make([]model.Transaction, 0)
	for rows.Next() {
		var tx model.Transaction
		var toW, catID sql.NullString
		var incBudgetInt, reimburseInt int
		tx.Tags = []model.Tag{}

		if err := rows.Scan(
			&tx.ID, &tx.BookID, &tx.Type, &tx.Amount, &tx.Date, &tx.TxTime, &tx.WalletID, &toW, &catID,
			&tx.Title, &tx.SubCategory, &tx.Note, &tx.OrderNo,
			&incBudgetInt, &reimburseInt, &tx.CreatedBy, &tx.CreatedByName, &tx.CreatedByAvatar,
			&tx.MemberID, &tx.MemberName, &tx.MemberAvatar, &tx.MemberColor,
			&tx.CreatedAt, &tx.UpdatedAt,
			&tx.CategoryName, &tx.CategoryIcon, &tx.CategoryColor, &tx.CategoryBgColor,
			&tx.WalletName, &tx.ToWalletName,
		); err != nil {
			rows.Close()
			return nil, 0, err
		}
		if toW.Valid {
			tx.ToWalletID = &toW.String
		}
		if catID.Valid {
			tx.CategoryID = &catID.String
		}
		tx.IncludeBudget = incBudgetInt == 1
		tx.Reimburse = reimburseInt == 1

		transactions = append(transactions, tx)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, err
	}
	rows.Close()

	for i := range transactions {
		transactions[i].Tags = r.fetchTxTags(ctx, transactions[i].ID)
	}

	return transactions, total, nil
}

func (r *TransactionRepo) deleteTxWithin(ctx context.Context, dbTx *sql.Tx, txID string) error {
	var oldTx model.Transaction
	var toW, catID sql.NullString
	query := `SELECT id, book_id, type, amount, date, wallet_id, to_wallet_id, category_id FROM transactions WHERE id = ?`
	err := dbTx.QueryRowContext(ctx, query, txID).Scan(
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

	_, err = dbTx.ExecContext(ctx, `DELETE FROM transactions WHERE id = ?`, txID)
	return err
}

func (r *TransactionRepo) DeleteTransaction(ctx context.Context, txID string) error {
	dbTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer dbTx.Rollback()

	if err := r.deleteTxWithin(ctx, dbTx, txID); err != nil {
		return err
	}
	return dbTx.Commit()
}

func (r *TransactionRepo) BatchDeleteTransactions(ctx context.Context, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	dbTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer dbTx.Rollback()

	deleted := 0
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		err := r.deleteTxWithin(ctx, dbTx, id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return 0, err
		}
		deleted++
	}

	if err := dbTx.Commit(); err != nil {
		return 0, err
	}
	return deleted, nil
}

func toBoolValue(v interface{}) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case float64:
		return b == 1, true
	case int:
		return b == 1, true
	case int64:
		return b == 1, true
	}
	return false, false
}

func (r *TransactionRepo) UpdateTransaction(ctx context.Context, txID string, updates map[string]interface{}) (*model.Transaction, error) {
	dbTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer dbTx.Rollback()

	var oldTx model.Transaction
	var toW, catID sql.NullString
	var incBudgetInt, reimburseInt int
	query := `
	SELECT id, book_id, type, amount, date, COALESCE(tx_time, ''), wallet_id, to_wallet_id, category_id,
	       COALESCE(title, ''), COALESCE(sub_category, ''), COALESCE(note, ''), COALESCE(order_no, ''),
	       COALESCE(include_budget, 1), COALESCE(reimburse, 0), created_by, COALESCE(NULLIF(member_id, ''), created_by), created_at, updated_at
	FROM transactions WHERE id = ?`
	err = dbTx.QueryRowContext(ctx, query, txID).Scan(
		&oldTx.ID, &oldTx.BookID, &oldTx.Type, &oldTx.Amount, &oldTx.Date, &oldTx.TxTime, &oldTx.WalletID, &toW, &catID,
		&oldTx.Title, &oldTx.SubCategory, &oldTx.Note, &oldTx.OrderNo,
		&incBudgetInt, &reimburseInt, &oldTx.CreatedBy, &oldTx.MemberID, &oldTx.CreatedAt, &oldTx.UpdatedAt,
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
	oldTx.IncludeBudget = incBudgetInt == 1
	oldTx.Reimburse = reimburseInt == 1

	newTx := oldTx
	if v, ok := updates["type"]; ok {
		if s, ok := v.(string); ok && (s == "expense" || s == "income" || s == "transfer") {
			newTx.Type = s
		}
	}
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
	if v, ok := updates["tx_time"]; ok {
		if s, ok := v.(string); ok {
			newTx.TxTime = s
		}
	}
	if v, ok := updates["title"]; ok {
		if s, ok := v.(string); ok {
			newTx.Title = s
		}
	}
	if v, ok := updates["sub_category"]; ok {
		if s, ok := v.(string); ok {
			newTx.SubCategory = s
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
	if v, ok := updates["to_wallet_id"]; ok {
		if v == nil {
			newTx.ToWalletID = nil
		} else if s, ok := v.(string); ok {
			if s == "" {
				newTx.ToWalletID = nil
			} else {
				newTx.ToWalletID = &s
			}
		}
	}
	if v, ok := updates["category_id"]; ok {
		if v == nil {
			newTx.CategoryID = nil
		} else if s, ok := v.(string); ok {
			if s == "" {
				newTx.CategoryID = nil
			} else {
				newTx.CategoryID = &s
			}
		}
	}
	if v, ok := updates["include_budget"]; ok {
		if b, valid := toBoolValue(v); valid {
			newTx.IncludeBudget = b
		}
	}
	if v, ok := updates["reimburse"]; ok {
		if b, valid := toBoolValue(v); valid {
			newTx.Reimburse = b
		}
	}
	if v, ok := updates["member_id"]; ok {
		if s, ok := v.(string); ok && s != "" {
			newTx.MemberID = s
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

	// Update daily_summaries
	if oldTx.Type == "expense" || oldTx.Type == "income" {
		oldCat := "uncategorized"
		if oldTx.CategoryID != nil && *oldTx.CategoryID != "" {
			oldCat = *oldTx.CategoryID
		}
		_, _ = dbTx.ExecContext(ctx, `
			UPDATE daily_summaries SET total_amount = total_amount - ?, count = count - 1
			WHERE date = ? AND book_id = ? AND type = ? AND category_id = ?
		`, oldTx.Amount, oldTx.Date, oldTx.BookID, oldTx.Type, oldCat)
	}
	if newTx.Type == "expense" || newTx.Type == "income" {
		newCat := "uncategorized"
		if newTx.CategoryID != nil && *newTx.CategoryID != "" {
			newCat = *newTx.CategoryID
		}
		_, _ = dbTx.ExecContext(ctx, `
			INSERT INTO daily_summaries (date, book_id, type, category_id, total_amount, count)
			VALUES (?, ?, ?, ?, ?, 1)
			ON CONFLICT(date, book_id, type, category_id) DO UPDATE SET
				total_amount = total_amount + ?,
				count = count + 1
		`, newTx.Date, newTx.BookID, newTx.Type, newCat, newTx.Amount, newTx.Amount)
	}

	newIncInt := 0
	if newTx.IncludeBudget {
		newIncInt = 1
	}
	newReimbInt := 0
	if newTx.Reimburse {
		newReimbInt = 1
	}

	updQuery := `
	UPDATE transactions SET
		type = ?, amount = ?, date = ?, tx_time = ?, wallet_id = ?, to_wallet_id = ?, category_id = ?,
		title = ?, sub_category = ?, note = ?, include_budget = ?, reimburse = ?, member_id = ?, updated_at = datetime('now')
	WHERE id = ?
	`
	_, err = dbTx.ExecContext(ctx, updQuery,
		newTx.Type, newTx.Amount, newTx.Date, newTx.TxTime, newTx.WalletID, newTx.ToWalletID, newTx.CategoryID,
		newTx.Title, newTx.SubCategory, newTx.Note, newIncInt, newReimbInt, newTx.MemberID, txID,
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
			rows.Close()
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
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	report.NetSavings = report.TotalIncome - report.TotalExpense

	for i := range report.Categories {
		item := &report.Categories[i]
		if report.TotalExpense > 0 {
			item.Percentage = float64((item.TotalAmount*10000)/report.TotalExpense) / 100.0
		}
		var name, icon, color, bgColor string
		if item.CategoryID != "uncategorized" && item.CategoryID != "" {
			if err := r.db.QueryRowContext(ctx, `SELECT name, icon, color, COALESCE(bg_color, '#FFF6E8') FROM categories WHERE id = ?`, item.CategoryID).Scan(&name, &icon, &color, &bgColor); err == nil {
				item.CategoryName = name
				item.CategoryIcon = icon
				item.CategoryColor = color
				item.BgColor = bgColor
			} else {
				item.CategoryName = "未分类"
				item.CategoryIcon = "tag"
				item.CategoryColor = "#DE8A1E"
				item.BgColor = "#FFF6E8"
			}
		} else {
			item.CategoryName = "未分类"
			item.CategoryIcon = "tag"
			item.CategoryColor = "#DE8A1E"
			item.BgColor = "#FFF6E8"
		}
	}

	sort.Slice(report.Categories, func(i, j int) bool {
		return report.Categories[i].TotalAmount > report.Categories[j].TotalAmount
	})

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

	sort.Strings(ymList)
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

func (r *TransactionRepo) GetStatsReport(ctx context.Context, bookID, period, txType string, year, month int) (*model.StatsReport, error) {
	now := time.Now()
	if year <= 0 {
		year = now.Year()
	}
	if month < 1 || month > 12 {
		month = int(now.Month())
	}
	if period != "week" && period != "month" && period != "year" {
		period = "month"
	}
	if txType != "expense" && txType != "income" {
		txType = "expense"
	}

	var startDate, endDate string
	loc := now.Location()
	switch period {
	case "week":
		ref := now
		if year != now.Year() || month != int(now.Month()) {
			ref = time.Date(year, time.Month(month), 15, 0, 0, 0, 0, loc)
		}
		weekday := int(ref.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startT := ref.AddDate(0, 0, -(weekday - 1))
		endT := startT.AddDate(0, 0, 6)
		startDate = startT.Format("2006-01-02")
		endDate = endT.Format("2006-01-02")
	case "year":
		startDate = fmt.Sprintf("%04d-01-01", year)
		endDate = fmt.Sprintf("%04d-12-31", year)
	default:
		firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
		lastDay := firstDay.AddDate(0, 1, -1)
		startDate = firstDay.Format("2006-01-02")
		endDate = lastDay.Format("2006-01-02")
	}

	var budget int64 = 450000
	if bookID != "" {
		_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(budget, 450000) FROM books WHERE id = ?`, bookID).Scan(&budget)
	}

	where := []string{"t.date >= ?", "t.date <= ?"}
	args := []interface{}{startDate, endDate}
	if bookID != "" {
		where = append(where, "t.book_id = ?")
		args = append(args, bookID)
	}
	whereSQL := strings.Join(where, " AND ")

	var totalExpense, totalIncome, budgetUsed int64
	sumQuery := fmt.Sprintf(`
		SELECT t.type, COALESCE(SUM(t.amount), 0), COALESCE(SUM(CASE WHEN COALESCE(t.include_budget, 1) = 1 THEN t.amount ELSE 0 END), 0)
		FROM transactions t
		WHERE %s AND t.type IN ('expense', 'income')
		GROUP BY t.type
	`, whereSQL)
	sumRows, err := r.db.QueryContext(ctx, sumQuery, args...)
	if err != nil {
		return nil, err
	}
	for sumRows.Next() {
		var t string
		var amt, bAmt int64
		if err := sumRows.Scan(&t, &amt, &bAmt); err == nil {
			if t == "expense" {
				totalExpense = amt
				budgetUsed = bAmt
			} else if t == "income" {
				totalIncome = amt
			}
		}
	}
	sumRows.Close()

	netSavings := totalIncome - totalExpense
	savingsRate := 0
	if totalIncome > 0 && netSavings > 0 {
		savingsRate = int((netSavings * 100) / totalIncome)
	}

	// Aggregate categories rolling up secondary subcategories to their parent top-level category
	catArgs := append(append([]interface{}{}, args...), txType)
	catQuery := fmt.Sprintf(`
		SELECT COALESCE(p.id, c.id, 'uncategorized') as top_id,
		       COALESCE(p.name, c.name, '未分类') as top_name,
		       COALESCE(p.icon, c.icon, 'tag') as top_icon,
		       COALESCE(p.color, c.color, '#64748B') as top_color,
		       COALESCE(p.bg_color, c.bg_color, '#F1F5F9') as top_bg,
		       COALESCE(SUM(t.amount), 0) as total_amt,
		       COUNT(*) as cnt
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		LEFT JOIN categories p ON c.parent_id = p.id
		WHERE %s AND t.type = ?
		GROUP BY top_id
		ORDER BY total_amt DESC
	`, whereSQL)

	catRows, err := r.db.QueryContext(ctx, catQuery, catArgs...)
	if err != nil {
		return nil, err
	}
	categories := make([]model.StatsCategoryItem, 0)
	var typeTotal int64
	if txType == "expense" {
		typeTotal = totalExpense
	} else {
		typeTotal = totalIncome
	}
	for catRows.Next() {
		var item model.StatsCategoryItem
		if err := catRows.Scan(&item.CategoryID, &item.CategoryName, &item.CategoryIcon, &item.CategoryColor, &item.BgColor, &item.Amount, &item.Count); err != nil {
			catRows.Close()
			return nil, err
		}
		if typeTotal > 0 {
			item.Percentage = int((item.Amount * 100) / typeTotal)
		}
		categories = append(categories, item)
	}
	catRows.Close()

	// Build daily or monthly series
	series := make([]model.StatsSeriesItem, 0)
	if period == "year" {
		seriesMap := make(map[string]*model.StatsSeriesItem)
		for m := 1; m <= 12; m++ {
			ym := fmt.Sprintf("%04d-%02d", year, m)
			seriesMap[ym] = &model.StatsSeriesItem{
				Date:  ym,
				Label: fmt.Sprintf("%d月", m),
			}
		}
		sQuery := fmt.Sprintf(`
			SELECT substr(t.date, 1, 7) as ym, t.type, COALESCE(SUM(t.amount), 0)
			FROM transactions t
			WHERE %s AND t.type IN ('expense', 'income')
			GROUP BY ym, t.type
		`, whereSQL)
		sRows, err := r.db.QueryContext(ctx, sQuery, args...)
		if err == nil {
			for sRows.Next() {
				var ym, t string
				var amt int64
				if err := sRows.Scan(&ym, &t, &amt); err == nil {
					if slot, ok := seriesMap[ym]; ok {
						if t == "expense" {
							slot.Expense = amt
						} else if t == "income" {
							slot.Income = amt
						}
					}
				}
			}
			sRows.Close()
		}
		for m := 1; m <= 12; m++ {
			ym := fmt.Sprintf("%04d-%02d", year, m)
			series = append(series, *seriesMap[ym])
		}
	} else {
		seriesMap := make(map[string]*model.StatsSeriesItem)
		startT, _ := time.Parse("2006-01-02", startDate)
		endT, _ := time.Parse("2006-01-02", endDate)
		var orderedKeys []string
		for d := startT; !d.After(endT); d = d.AddDate(0, 0, 1) {
			ds := d.Format("2006-01-02")
			orderedKeys = append(orderedKeys, ds)
			lbl := d.Format("02")
			if period == "week" {
				lbl = d.Format("01/02")
			}
			seriesMap[ds] = &model.StatsSeriesItem{
				Date:  ds,
				Label: lbl,
			}
		}
		sQuery := fmt.Sprintf(`
			SELECT t.date, t.type, COALESCE(SUM(t.amount), 0)
			FROM transactions t
			WHERE %s AND t.type IN ('expense', 'income')
			GROUP BY t.date, t.type
		`, whereSQL)
		sRows, err := r.db.QueryContext(ctx, sQuery, args...)
		if err == nil {
			for sRows.Next() {
				var ds, t string
				var amt int64
				if err := sRows.Scan(&ds, &t, &amt); err == nil {
					if slot, ok := seriesMap[ds]; ok {
						if t == "expense" {
							slot.Expense = amt
						} else if t == "income" {
							slot.Income = amt
						}
					}
				}
			}
			sRows.Close()
		}
		for _, k := range orderedKeys {
			series = append(series, *seriesMap[k])
		}
	}

	// Top 10 transactions of the selected type
	topArgs := append(append([]interface{}{}, args...), txType)
	topQuery := fmt.Sprintf(`
		SELECT t.id, t.book_id, t.type, t.amount, t.date, COALESCE(t.tx_time, ''), t.wallet_id, t.to_wallet_id, t.category_id,
		       COALESCE(t.title, ''), COALESCE(t.sub_category, ''), COALESCE(t.note, ''), COALESCE(t.order_no, ''),
		       COALESCE(t.include_budget, 1), COALESCE(t.reimburse, 0), t.created_by,
		       COALESCE(NULLIF(bm.user_name, ''), NULLIF(u.name, ''), t.created_by, ''),
		       COALESCE(NULLIF(u.avatar, ''), NULLIF(bm.avatar, ''), '')` + txMemberColumns + `,
		       t.created_at, t.updated_at,
		       COALESCE(c.name, ''), COALESCE(c.icon, ''), COALESCE(c.color, ''), COALESCE(c.bg_color, ''),
		       COALESCE(w.name, ''), COALESCE(tw.name, '')
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		LEFT JOIN wallets w ON t.wallet_id = w.id
		LEFT JOIN wallets tw ON t.to_wallet_id = tw.id
		LEFT JOIN book_members bm ON t.book_id = bm.book_id AND t.created_by = bm.user_id
		LEFT JOIN users u ON t.created_by = u.id` + txMemberJoins + `
		WHERE %s AND t.type = ?
		ORDER BY t.amount DESC, t.date DESC
		LIMIT 10
	`, whereSQL)

	topRows, err := r.db.QueryContext(ctx, topQuery, topArgs...)
	if err != nil {
		return nil, err
	}
	topTxs := make([]model.Transaction, 0)
	for topRows.Next() {
		var tx model.Transaction
		var toW, catID sql.NullString
		var incBudgetInt, reimburseInt int
		tx.Tags = []model.Tag{}
		if err := topRows.Scan(
			&tx.ID, &tx.BookID, &tx.Type, &tx.Amount, &tx.Date, &tx.TxTime, &tx.WalletID, &toW, &catID,
			&tx.Title, &tx.SubCategory, &tx.Note, &tx.OrderNo,
			&incBudgetInt, &reimburseInt, &tx.CreatedBy, &tx.CreatedByName, &tx.CreatedByAvatar,
			&tx.MemberID, &tx.MemberName, &tx.MemberAvatar, &tx.MemberColor,
			&tx.CreatedAt, &tx.UpdatedAt,
			&tx.CategoryName, &tx.CategoryIcon, &tx.CategoryColor, &tx.CategoryBgColor,
			&tx.WalletName, &tx.ToWalletName,
		); err != nil {
			topRows.Close()
			return nil, err
		}
		if toW.Valid {
			tx.ToWalletID = &toW.String
		}
		if catID.Valid {
			tx.CategoryID = &catID.String
		}
		tx.IncludeBudget = incBudgetInt == 1
		tx.Reimburse = reimburseInt == 1
		topTxs = append(topTxs, tx)
	}
	topRows.Close()

	// 按归属成员聚合（与分类聚合同一口径：所选周期 + 所选类型）。
	memArgs := append(append([]interface{}{}, args...), txType)
	memQuery := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(t.member_id, ''), t.created_by) as mid,
		       COALESCE(NULLIF(mm.user_name, ''), NULLIF(mu.name, ''), COALESCE(NULLIF(t.member_id, ''), t.created_by), '') as mname,
		       COALESCE(NULLIF(mu.avatar, ''), NULLIF(mm.avatar, ''), '') as mavatar,
		       COALESCE(mm.color, '#DE8A1E') as mcolor,
		       COALESCE(mm.is_virtual, 0) as mvirtual,
		       COALESCE(SUM(t.amount), 0) as total_amt,
		       COUNT(*) as cnt
		FROM transactions t
		LEFT JOIN book_members mm ON t.book_id = mm.book_id AND COALESCE(NULLIF(t.member_id, ''), t.created_by) = mm.user_id
		LEFT JOIN users mu ON COALESCE(NULLIF(t.member_id, ''), t.created_by) = mu.id
		WHERE %s AND t.type = ?
		GROUP BY mid
		ORDER BY total_amt DESC
	`, whereSQL)

	memRows, err := r.db.QueryContext(ctx, memQuery, memArgs...)
	if err != nil {
		return nil, err
	}
	members := make([]model.StatsMemberItem, 0)
	for memRows.Next() {
		var item model.StatsMemberItem
		var isVirt int
		if err := memRows.Scan(&item.MemberID, &item.MemberName, &item.MemberAvatar, &item.MemberColor, &isVirt, &item.Amount, &item.Count); err != nil {
			memRows.Close()
			return nil, err
		}
		item.IsVirtual = isVirt == 1
		if typeTotal > 0 {
			item.Percentage = int((item.Amount * 100) / typeTotal)
		}
		members = append(members, item)
	}
	memRows.Close()

	return &model.StatsReport{
		Period:          period,
		Type:            txType,
		StartDate:       startDate,
		EndDate:         endDate,
		TotalExpense:    totalExpense,
		TotalIncome:     totalIncome,
		NetSavings:      netSavings,
		SavingsRate:     savingsRate,
		Budget:          budget,
		BudgetUsed:      budgetUsed,
		Categories:      categories,
		Members:         members,
		DailySeries:     series,
		TopTransactions: topTxs,
	}, nil
}

// GetRangeStats 聚合任意起止日期区间内的收支：总额、按一级分类、按月。
// categoryID 为空表示不过滤；非空时包含该分类本身及其子分类的流水。
func (r *TransactionRepo) GetRangeStats(ctx context.Context, bookID, startDate, endDate, txType, categoryID string) (*model.RangeStats, error) {
	if txType != "expense" && txType != "income" {
		txType = ""
	}
	where := []string{"t.book_id = ?", "t.date >= ?", "t.date <= ?"}
	args := []interface{}{bookID, startDate, endDate}
	if categoryID != "" {
		where = append(where, "(t.category_id = ? OR c.parent_id = ?)")
		args = append(args, categoryID, categoryID)
	}
	whereSQL := strings.Join(where, " AND ")
	joinCat := "LEFT JOIN categories c ON t.category_id = c.id"

	res := &model.RangeStats{
		StartDate:  startDate,
		EndDate:    endDate,
		Categories: make([]model.StatsCategoryItem, 0),
		Monthly:    make([]model.RangeMonthItem, 0),
	}

	sumQuery := fmt.Sprintf(`
		SELECT t.type, COALESCE(SUM(t.amount), 0)
		FROM transactions t %s
		WHERE %s AND t.type IN ('expense', 'income')
		GROUP BY t.type
	`, joinCat, whereSQL)
	sumRows, err := r.db.QueryContext(ctx, sumQuery, args...)
	if err != nil {
		return nil, err
	}
	for sumRows.Next() {
		var t string
		var amt int64
		if err := sumRows.Scan(&t, &amt); err == nil {
			if t == "expense" {
				res.TotalExpense = amt
			} else if t == "income" {
				res.TotalIncome = amt
			}
		}
	}
	sumRows.Close()
	res.NetSavings = res.TotalIncome - res.TotalExpense

	catArgs := append([]interface{}{}, args...)
	catTypeFilter := ""
	if txType != "" {
		catTypeFilter = " AND t.type = ?"
		catArgs = append(catArgs, txType)
	}
	catQuery := fmt.Sprintf(`
		SELECT COALESCE(p.id, c.id, 'uncategorized') as top_id,
		       COALESCE(p.name, c.name, '未分类') as top_name,
		       COALESCE(p.icon, c.icon, 'tag') as top_icon,
		       COALESCE(p.color, c.color, '#64748B') as top_color,
		       COALESCE(p.bg_color, c.bg_color, '#F1F5F9') as top_bg,
		       COALESCE(SUM(t.amount), 0) as total_amt,
		       COUNT(*) as cnt
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		LEFT JOIN categories p ON c.parent_id = p.id
		WHERE %s%s
		GROUP BY top_id
		ORDER BY total_amt DESC
	`, whereSQL, catTypeFilter)
	catRows, err := r.db.QueryContext(ctx, catQuery, catArgs...)
	if err != nil {
		return nil, err
	}
	typeTotal := res.TotalExpense + res.TotalIncome
	if txType == "expense" {
		typeTotal = res.TotalExpense
	} else if txType == "income" {
		typeTotal = res.TotalIncome
	}
	for catRows.Next() {
		var item model.StatsCategoryItem
		if err := catRows.Scan(&item.CategoryID, &item.CategoryName, &item.CategoryIcon, &item.CategoryColor, &item.BgColor, &item.Amount, &item.Count); err != nil {
			catRows.Close()
			return nil, err
		}
		if typeTotal > 0 {
			item.Percentage = int((item.Amount * 100) / typeTotal)
		}
		res.Categories = append(res.Categories, item)
	}
	catRows.Close()

	monQuery := fmt.Sprintf(`
		SELECT substr(t.date, 1, 7) as ym, t.type, COALESCE(SUM(t.amount), 0)
		FROM transactions t %s
		WHERE %s AND t.type IN ('expense', 'income')
		GROUP BY ym, t.type
		ORDER BY ym ASC
	`, joinCat, whereSQL)
	monRows, err := r.db.QueryContext(ctx, monQuery, args...)
	if err != nil {
		return nil, err
	}
	monMap := make(map[string]*model.RangeMonthItem)
	var monOrder []string
	for monRows.Next() {
		var ym, t string
		var amt int64
		if err := monRows.Scan(&ym, &t, &amt); err != nil {
			continue
		}
		slot, ok := monMap[ym]
		if !ok {
			slot = &model.RangeMonthItem{Month: ym}
			monMap[ym] = slot
			monOrder = append(monOrder, ym)
		}
		if t == "expense" {
			slot.Expense = amt
		} else if t == "income" {
			slot.Income = amt
		}
	}
	monRows.Close()
	for _, ym := range monOrder {
		res.Monthly = append(res.Monthly, *monMap[ym])
	}
	return res, nil
}

// GetMemberTotals 按归属成员汇总一段时间内的收支（用于 Agent query_members 工具）。
// 归属成员是流水的 member_id（为空时为创建人），不是创建人本身。
func (r *TransactionRepo) GetMemberTotals(ctx context.Context, bookID, startDate, endDate string) ([]model.MemberTotal, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(NULLIF(member_id, ''), created_by), t.type, COALESCE(SUM(amount), 0), COUNT(*)
		FROM transactions t
		WHERE t.book_id = ? AND t.date >= ? AND t.date <= ? AND t.type IN ('expense', 'income')
		GROUP BY COALESCE(NULLIF(member_id, ''), created_by), t.type
	`, bookID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	totals := make(map[string]*model.MemberTotal)
	var order []string
	for rows.Next() {
		var userID, t string
		var amt int64
		var cnt int
		if err := rows.Scan(&userID, &t, &amt, &cnt); err != nil {
			continue
		}
		slot, ok := totals[userID]
		if !ok {
			slot = &model.MemberTotal{UserID: userID}
			totals[userID] = slot
			order = append(order, userID)
		}
		if t == "expense" {
			slot.Expense = amt
		} else if t == "income" {
			slot.Income = amt
		}
		slot.Count += cnt
	}
	out := make([]model.MemberTotal, 0, len(order))
	for _, id := range order {
		out = append(out, *totals[id])
	}
	return out, rows.Err()
}
