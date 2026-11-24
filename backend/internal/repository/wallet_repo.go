package repository

import (
	"context"
	"database/sql"
	"errors"

	"com.xsguyu.accounting/backend/internal/model"
	"github.com/google/uuid"
)

type WalletRepo struct {
	db *DB
}

func NewWalletRepo(db *DB) *WalletRepo {
	return &WalletRepo{db: db}
}

func DefaultWalletGroupAndColor(wType string) (string, string) {
	switch wType {
	case "wechat":
		return "电子钱包", "#3E8E63"
	case "alipay":
		return "电子钱包", "#2F78A8"
	case "debit":
		return "储蓄卡", "#DE8A1E"
	case "credit":
		return "信用负债", "#D6553E"
	case "cash":
		return "现金", "#64748B"
	case "investment", "invest":
		return "投资理财", "#7E57A6"
	default:
		return "电子钱包", "#DE8A1E"
	}
}

func (r *WalletRepo) CreateWallet(ctx context.Context, w *model.Wallet) error {
	if w.ID == "" {
		w.ID = uuid.NewString()
	}
	defGroup, defColor := DefaultWalletGroupAndColor(w.Type)
	if w.GroupName == "" {
		w.GroupName = defGroup
	}
	if w.Color == "" {
		w.Color = defColor
	}

	excludeInt := 0
	if w.ExcludeNetAsset {
		excludeInt = 1
	}

	query := `
	INSERT INTO wallets (
		id, book_id, type, name, group_name, color, balance, credit_limit, billing_day, repayment_day, card_last_four, exclude_net_asset
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		w.ID, w.BookID, w.Type, w.Name, w.GroupName, w.Color, w.Balance, w.CreditLimit, w.BillingDay, w.RepaymentDay, w.CardLastFour, excludeInt,
	)
	return err
}

func (r *WalletRepo) GetWallet(ctx context.Context, walletID string) (*model.Wallet, error) {
	query := `
	SELECT id, book_id, type, name, COALESCE(group_name, '电子钱包'), COALESCE(color, '#DE8A1E'),
	       balance, credit_limit, billing_day, repayment_day, card_last_four,
	       exclude_net_asset, is_archived, created_at, updated_at
	FROM wallets WHERE id = ?
	`
	var w model.Wallet
	var excludeInt, archiveInt int
	err := r.db.QueryRowContext(ctx, query, walletID).Scan(
		&w.ID, &w.BookID, &w.Type, &w.Name, &w.GroupName, &w.Color,
		&w.Balance, &w.CreditLimit, &w.BillingDay, &w.RepaymentDay,
		&w.CardLastFour, &excludeInt, &archiveInt, &w.CreatedAt, &w.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	w.ExcludeNetAsset = excludeInt == 1
	w.IsArchived = archiveInt == 1
	return &w, nil
}

func (r *WalletRepo) ListWallets(ctx context.Context, bookID string, includeArchived bool) ([]model.Wallet, error) {
	query := `
	SELECT id, book_id, type, name, COALESCE(group_name, '电子钱包'), COALESCE(color, '#DE8A1E'),
	       balance, credit_limit, billing_day, repayment_day, card_last_four,
	       exclude_net_asset, is_archived, created_at, updated_at
	FROM wallets WHERE 1=1
	`
	var args []interface{}
	if bookID != "" {
		query += " AND book_id = ?"
		args = append(args, bookID)
	}
	if !includeArchived {
		query += " AND is_archived = 0"
	}
	query += " ORDER BY created_at ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	wallets := make([]model.Wallet, 0)
	for rows.Next() {
		var w model.Wallet
		var excludeInt, archiveInt int
		if err := rows.Scan(
			&w.ID, &w.BookID, &w.Type, &w.Name, &w.GroupName, &w.Color,
			&w.Balance, &w.CreditLimit, &w.BillingDay, &w.RepaymentDay,
			&w.CardLastFour, &excludeInt, &archiveInt, &w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, err
		}
		w.ExcludeNetAsset = excludeInt == 1
		w.IsArchived = archiveInt == 1
		wallets = append(wallets, w)
	}
	return wallets, rows.Err()
}

func (r *WalletRepo) UpdateWallet(ctx context.Context, w *model.Wallet) error {
	query := `
	UPDATE wallets SET
		name = ?, type = ?, group_name = ?, color = ?, balance = ?, credit_limit = ?,
		billing_day = ?, repayment_day = ?, card_last_four = ?, exclude_net_asset = ?,
		is_archived = ?, updated_at = datetime('now')
	WHERE id = ?
	`
	excludeInt := 0
	if w.ExcludeNetAsset {
		excludeInt = 1
	}
	archiveInt := 0
	if w.IsArchived {
		archiveInt = 1
	}

	res, err := r.db.ExecContext(ctx, query,
		w.Name, w.Type, w.GroupName, w.Color, w.Balance, w.CreditLimit,
		w.BillingDay, w.RepaymentDay, w.CardLastFour, excludeInt, archiveInt, w.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *WalletRepo) DeleteWallet(ctx context.Context, walletID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM wallets WHERE id = ?`, walletID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *WalletRepo) GetWalletSummary(ctx context.Context, bookID string) (*model.WalletSummary, error) {
	query := `SELECT balance, exclude_net_asset FROM wallets WHERE is_archived = 0`
	var args []interface{}
	if bookID != "" {
		query += " AND book_id = ?"
		args = append(args, bookID)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var totalAssets int64 = 0
	var totalLiabilities int64 = 0

	for rows.Next() {
		var bal int64
		var excludeInt int
		if err := rows.Scan(&bal, &excludeInt); err != nil {
			return nil, err
		}
		if excludeInt == 1 {
			continue
		}
		if bal >= 0 {
			totalAssets += bal
		} else {
			totalLiabilities += -bal
		}
	}

	summary := &model.WalletSummary{
		TotalAssets:      totalAssets,
		TotalLiabilities: totalLiabilities,
		NetAssets:        totalAssets - totalLiabilities,
	}
	return summary, nil
}
