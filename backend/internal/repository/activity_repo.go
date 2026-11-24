package repository

import (
	"context"
	"fmt"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"github.com/google/uuid"
)

type ActivityRepo struct {
	db *DB
}

func NewActivityRepo(db *DB) *ActivityRepo {
	return &ActivityRepo{db: db}
}

// Record 写入一条动态。调用方不传 ID 时自动生成。
// created_at 用服务端本地时间显式写入（SQLite 的 datetime('now') 是 UTC，
// 直接存会在中国时区显示慢 8 小时；服务器跑在 NAS/本地时区，本地时间才是对的）。
func (r *ActivityRepo) Record(ctx context.Context, a *model.BookActivity) error {
	if a.ID == "" {
		a.ID = "act_" + uuid.NewString()[:8]
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	a.CreatedAt = now
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO book_activities (id, book_id, user_id, actor_name, action, summary, amount_cents, tx_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, a.ID, a.BookID, a.UserID, a.ActorName, a.Action, a.Summary, a.AmountCents, a.TxID, now)
	return err
}

// List 按时间倒序取动态。before 为上一页最后一条的 ID（游标）；为空取最新一页。
// 返回的 hasMore 表示是否还有更旧的记录。
func (r *ActivityRepo) List(ctx context.Context, bookID string, limit int, before string) ([]model.BookActivity, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	var beforeCreatedAt, beforeID string
	if before != "" {
		_ = r.db.QueryRowContext(ctx, `SELECT created_at, id FROM book_activities WHERE id = ? AND book_id = ?`, before, bookID).Scan(&beforeCreatedAt, &beforeID)
	}

	query := `
		SELECT id, book_id, user_id, actor_name, action, summary, amount_cents, tx_id, created_at
		FROM book_activities WHERE book_id = ?
	`
	args := []interface{}{bookID}
	if beforeCreatedAt != "" {
		query += ` AND (created_at < ? OR (created_at = ? AND id < ?))`
		args = append(args, beforeCreatedAt, beforeCreatedAt, beforeID)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit+1)

	dbRows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer dbRows.Close()

	items := make([]model.BookActivity, 0, limit)
	for dbRows.Next() {
		var a model.BookActivity
		if err := dbRows.Scan(&a.ID, &a.BookID, &a.UserID, &a.ActorName, &a.Action, &a.Summary, &a.AmountCents, &a.TxID, &a.CreatedAt); err != nil {
			return nil, false, err
		}
		items = append(items, a)
	}
	hasMore := false
	if len(items) > limit {
		hasMore = true
		items = items[:limit]
	}
	return items, hasMore, nil
}

// Prune 按保留策略清理：90 天前的删掉；超过最近 1000 条的部分删掉。
// 每次写入动态后顺手调用，失败不影响主流程（调用方只记日志）。
func (r *ActivityRepo) Prune(ctx context.Context, bookID string) error {
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM book_activities
		WHERE book_id = ? AND created_at < datetime('now', '-90 days')
	`, bookID); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM book_activities WHERE id IN (
			SELECT id FROM book_activities
			WHERE book_id = ? ORDER BY created_at DESC, id DESC
			LIMIT -1 OFFSET 1000
		)
	`, bookID); err != nil {
		return err
	}
	return nil
}

// Count 返回该账本的动态条数（测试与保留策略校验用）。
func (r *ActivityRepo) Count(ctx context.Context, bookID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM book_activities WHERE book_id = ?`, bookID).Scan(&n)
	return n, err
}

// NotifiedLevels 返回某预算项当月已提醒过的档位集合。
func (r *ActivityRepo) NotifiedLevels(ctx context.Context, bookID, budgetKey, month string) (map[int]bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT level FROM budget_alert_state
		WHERE book_id = ? AND budget_key = ? AND month = ?
	`, bookID, budgetKey, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int]bool)
	for rows.Next() {
		var lv int
		if err := rows.Scan(&lv); err == nil {
			out[lv] = true
		}
	}
	return out, nil
}

// MarkNotified 记录某档位已提醒（幂等）。
func (r *ActivityRepo) MarkNotified(ctx context.Context, bookID, budgetKey, month string, level int) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO budget_alert_state (book_id, budget_key, month, level)
		VALUES (?, ?, ?, ?)
	`, bookID, budgetKey, month, level)
	return err
}

// ClearLevelsAbove 删除高于给定档位的已提醒记录（用量回落时调用，
// 下次重新越线会再次提醒）。level 为当前档位：如当前 0 档，清除 80/100。
func (r *ActivityRepo) ClearLevelsAbove(ctx context.Context, bookID, budgetKey, month string, level int) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM budget_alert_state
		WHERE book_id = ? AND budget_key = ? AND month = ? AND level > ?
	`, bookID, budgetKey, month, level)
	return err
}

// ListAllBookIDs 返回全部账本 ID（每日预算检查遍历用）。
func (r *ActivityRepo) ListAllBookIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM books ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan book id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
