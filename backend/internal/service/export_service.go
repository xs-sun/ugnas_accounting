package service

import (
	"context"
	"fmt"
	"strings"

	"com.xsguyu.accounting/backend/internal/repository"
)

type ExportService struct {
	db *repository.DB
}

func NewExportService(db *repository.DB) *ExportService {
	return &ExportService{db: db}
}

func (s *ExportService) ExportCSV(ctx context.Context, bookID string) ([]byte, error) {
	where := []string{"1=1"}
	var args []interface{}
	if bookID != "" {
		where = append(where, "t.book_id = ?")
		args = append(args, bookID)
	}

	query := fmt.Sprintf(`
	SELECT t.date, t.type, COALESCE(c.name, '未分类') as category,
	       COALESCE(w.name, '') as wallet, t.amount, t.note
	FROM transactions t
	LEFT JOIN categories c ON t.category_id = c.id
	LEFT JOIN wallets w ON t.wallet_id = w.id
	WHERE %s
	ORDER BY t.date DESC
	`, strings.Join(where, " AND "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []string
	// Prepend UTF-8 BOM
	lines = append(lines, "\ufeff交易日期,交易类型,分类,账户,金额(元),备注")

	for rows.Next() {
		var dt, tType, cat, wallet, note string
		var amt int64
		if err := rows.Scan(&dt, &tType, &cat, &wallet, &amt, &note); err != nil {
			return nil, err
		}

		amtYuan := fmt.Sprintf("%.2f", float64(amt)/100.0)
		escapedNote := `"` + strings.ReplaceAll(note, `"`, `""`) + `"`
		lines = append(lines, fmt.Sprintf("%s,%s,%s,%s,%s,%s", dt, tType, cat, wallet, amtYuan, escapedNote))
	}

	return []byte(strings.Join(lines, "\n") + "\n"), rows.Err()
}
