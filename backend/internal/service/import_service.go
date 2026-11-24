package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type ImportedItem struct {
	Date                string `json:"date"`
	Type                string `json:"type"` // expense, income
	Counterparty        string `json:"counterparty"`
	Product             string `json:"product"`
	Amount              int64  `json:"amount"` // cents
	OrderNo             string `json:"order_no"`
	SuggestedCategoryID string `json:"suggested_category_id,omitempty"`
	Note                string `json:"note"`
}

type ImportService struct {
	db         *repository.DB
	txRepo     *repository.TransactionRepo
	catRepo    *repository.CategoryRepo
	walletRepo *repository.WalletRepo
}

func NewImportService(db *repository.DB, txRepo *repository.TransactionRepo, catRepo *repository.CategoryRepo, walletRepo *repository.WalletRepo) *ImportService {
	return &ImportService{db: db, txRepo: txRepo, catRepo: catRepo, walletRepo: walletRepo}
}

func parseAmountCents(val string) int64 {
	val = strings.TrimSpace(val)
	val = strings.TrimPrefix(val, "¥")
	val = strings.TrimPrefix(val, "￥")
	val = strings.TrimSpace(val)
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(f * 100))
}

func (s *ImportService) ParseWeChatCSV(rawBytes []byte) ([]ImportedItem, error) {
	// Strip UTF-8 BOM if present
	if bytes.HasPrefix(rawBytes, []byte{0xef, 0xbb, 0xbf}) {
		rawBytes = rawBytes[3:]
	}

	text := string(rawBytes)
	lines := strings.Split(text, "\n")

	var items []ImportedItem
	headerFound := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "----------------------微信支付账单明细列表") {
			headerFound = true
			continue
		}
		if !headerFound {
			continue
		}
		if line == "" {
			continue
		}

		cols := strings.Split(line, ",")
		for i := range cols {
			cols[i] = strings.TrimSpace(cols[i])
		}

		if len(cols) >= 10 && cols[0] != "交易时间" {
			dtParts := strings.Split(cols[0], " ")
			dt := dtParts[0]

			tType := "expense"
			if cols[4] == "收入" {
				tType = "income"
			}

			amt := parseAmountCents(cols[5])
			if amt <= 0 {
				continue
			}

			counterparty := cols[2]
			catID := ""
			if strings.Contains(counterparty, "外卖") || strings.Contains(counterparty, "咖啡") ||
				strings.Contains(counterparty, "麦当劳") || strings.Contains(counterparty, "海底捞") {
				catID = "cat_food"
			} else if strings.Contains(counterparty, "滴滴") || strings.Contains(counterparty, "地铁") {
				catID = "cat_transport"
			} else if strings.Contains(counterparty, "超市") {
				catID = "cat_shopping"
			}

			note := cols[3]
			if len(cols) > 10 && cols[10] != "/" && cols[10] != "" {
				note = cols[10]
			}

			items = append(items, ImportedItem{
				Date:                dt,
				Type:                tType,
				Counterparty:        counterparty,
				Product:             cols[3],
				Amount:              amt,
				OrderNo:             cols[8],
				SuggestedCategoryID: catID,
				Note:                note,
			})
		}
	}

	return items, nil
}

func (s *ImportService) ParseAlipayCSV(rawBytes []byte) ([]ImportedItem, error) {
	// Try UTF-8 first, fallback to GBK
	var text string
	if isUTF8(rawBytes) {
		text = string(rawBytes)
	} else {
		reader := transform.NewReader(bytes.NewReader(rawBytes), simplifiedchinese.GBK.NewDecoder())
		decoded, err := io.ReadAll(reader)
		if err == nil {
			text = string(decoded)
		} else {
			text = string(rawBytes)
		}
	}

	lines := strings.Split(text, "\n")
	var items []ImportedItem
	headerFound := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "----------------------------------------------------") {
			headerFound = true
			continue
		}
		if !headerFound {
			continue
		}
		if line == "" {
			continue
		}

		cols := strings.Split(line, ",")
		for i := range cols {
			cols[i] = strings.TrimSpace(cols[i])
		}

		if len(cols) >= 10 && cols[0] != "交易时间" {
			dtParts := strings.Split(cols[0], " ")
			dt := dtParts[0]

			tType := "expense"
			if cols[5] == "收入" {
				tType = "income"
			}

			amt := parseAmountCents(cols[6])
			if amt <= 0 {
				continue
			}

			counterparty := cols[2]
			catID := ""
			if strings.Contains(counterparty, "外卖") || strings.Contains(counterparty, "星巴克") ||
				strings.Contains(counterparty, "肯德基") {
				catID = "cat_food"
			} else if strings.Contains(counterparty, "滴滴") || strings.Contains(counterparty, "铁路") ||
				strings.Contains(counterparty, "单车") {
				catID = "cat_transport"
			} else if strings.Contains(counterparty, "山姆") || strings.Contains(counterparty, "淘宝") ||
				strings.Contains(counterparty, "盒马") || strings.Contains(counterparty, "京东") {
				catID = "cat_shopping"
			}

			note := cols[4]
			if len(cols) > 11 && cols[11] != "/" && cols[11] != "" {
				note = cols[11]
			}

			items = append(items, ImportedItem{
				Date:                dt,
				Type:                tType,
				Counterparty:        counterparty,
				Product:             cols[4],
				Amount:              amt,
				OrderNo:             cols[9],
				SuggestedCategoryID: catID,
				Note:                note,
			})
		}
	}

	return items, nil
}

func isUTF8(b []byte) bool {
	return strings.ToValidUTF8(string(b), "") == string(b)
}

func (s *ImportService) CommitImport(ctx context.Context, bookID, walletID string, items []ImportedItem, userID string) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	inserted := 0
	today := time.Now().Format("2006-01-02")

	for _, it := range items {
		if it.OrderNo != "" {
			var exists int
			err := tx.QueryRowContext(ctx, `SELECT 1 FROM transactions WHERE book_id = ? AND order_no = ?`, bookID, it.OrderNo).Scan(&exists)
			if err == nil && exists == 1 {
				continue
			}
		}

		txID := "tx_imp_" + uuid.NewString()[:8]
		tType := it.Type
		if tType == "" {
			tType = "expense"
		}
		amt := it.Amount
		dt := it.Date
		if dt == "" {
			dt = today
		}

		var catIDVal *string
		if it.SuggestedCategoryID != "" {
			// Check if category exists in this book or map to a valid category ID
			var catFound string
			err := tx.QueryRowContext(ctx, `SELECT id FROM categories WHERE book_id = ? AND (id = ? OR name LIKE ?) LIMIT 1`,
				bookID, it.SuggestedCategoryID, "%"+it.SuggestedCategoryID+"%").Scan(&catFound)
			if err == nil && catFound != "" {
				catIDVal = &catFound
			} else {
				// Fallback to finding by type
				var fallbackCat string
				_ = tx.QueryRowContext(ctx, `SELECT id FROM categories WHERE book_id = ? AND type = ? LIMIT 1`, bookID, tType).Scan(&fallbackCat)
				if fallbackCat != "" {
					catIDVal = &fallbackCat
				}
			}
		}

		insQuery := `
		INSERT INTO transactions (id, book_id, type, amount, date, wallet_id, category_id, note, order_no, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		if _, err := tx.ExecContext(ctx, insQuery, txID, bookID, tType, amt, dt, walletID, catIDVal, it.Note, it.OrderNo, userID); err != nil {
			return 0, fmt.Errorf("failed to insert imported transaction: %w", err)
		}

		if tType == "expense" {
			if _, err := tx.ExecContext(ctx, `UPDATE wallets SET balance = balance - ?, updated_at = datetime('now') WHERE id = ?`, amt, walletID); err != nil {
				return 0, err
			}
		} else if tType == "income" {
			if _, err := tx.ExecContext(ctx, `UPDATE wallets SET balance = balance + ?, updated_at = datetime('now') WHERE id = ?`, amt, walletID); err != nil {
				return 0, err
			}
		}

		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return inserted, nil
}
