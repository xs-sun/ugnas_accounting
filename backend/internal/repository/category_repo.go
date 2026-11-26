package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"github.com/google/uuid"
)

type CategoryRepo struct {
	db *DB
}

func NewCategoryRepo(db *DB) *CategoryRepo {
	return &CategoryRepo{db: db}
}

func (r *CategoryRepo) CreateCategory(ctx context.Context, c *model.Category) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	if c.Icon == "" {
		c.Icon = "tag"
	}
	if c.Color == "" {
		c.Color = "#DE8A1E"
	}
	if c.BgColor == "" {
		c.BgColor = "#FFF6E8"
	}
	isCustomInt := 0
	if c.IsCustom {
		isCustomInt = 1
	}

	query := `INSERT INTO categories (id, book_id, type, name, icon, color, bg_color, parent_id, is_custom, sort_order) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, c.ID, c.BookID, c.Type, c.Name, c.Icon, c.Color, c.BgColor, c.ParentID, isCustomInt, c.SortOrder)
	return err
}

func (r *CategoryRepo) ExistsDuplicate(ctx context.Context, bookID, catType string, parentID *string, name string) (bool, error) {
	pid := ""
	if parentID != nil {
		pid = *parentID
	}
	var cnt int
	err := r.db.QueryRowContext(ctx,
		`SELECT count(*) FROM categories WHERE book_id = ? AND type = ? AND COALESCE(parent_id, '') = ? AND name = ?`,
		bookID, catType, pid, name,
	).Scan(&cnt)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func (r *CategoryRepo) GetCategory(ctx context.Context, catID string) (*model.Category, error) {
	query := `SELECT id, book_id, type, name, icon, color, COALESCE(bg_color, '#FFF6E8'), parent_id, COALESCE(is_custom, 0), COALESCE(sort_order, 0), created_at FROM categories WHERE id = ?`
	var c model.Category
	var pid sql.NullString
	var isCustomInt int
	err := r.db.QueryRowContext(ctx, query, catID).Scan(
		&c.ID, &c.BookID, &c.Type, &c.Name, &c.Icon, &c.Color, &c.BgColor, &pid, &isCustomInt, &c.SortOrder, &c.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if pid.Valid {
		c.ParentID = &pid.String
	}
	c.IsCustom = isCustomInt == 1
	return &c, nil
}

func (r *CategoryRepo) ListCategories(ctx context.Context, bookID string, catType string) ([]model.Category, error) {
	query := `SELECT id, book_id, type, name, icon, color, COALESCE(bg_color, '#FFF6E8'), parent_id, COALESCE(is_custom, 0), COALESCE(sort_order, 0), created_at FROM categories WHERE 1=1`
	var args []interface{}
	if bookID != "" {
		query += " AND book_id = ?"
		args = append(args, bookID)
	}
	if catType != "" {
		query += " AND type = ?"
		args = append(args, catType)
	}
	query += " ORDER BY sort_order ASC, created_at ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]model.Category, 0)
	for rows.Next() {
		var c model.Category
		var pid sql.NullString
		var isCustomInt int
		if err := rows.Scan(&c.ID, &c.BookID, &c.Type, &c.Name, &c.Icon, &c.Color, &c.BgColor, &pid, &isCustomInt, &c.SortOrder, &c.CreatedAt); err != nil {
			return nil, err
		}
		if pid.Valid {
			c.ParentID = &pid.String
		}
		c.IsCustom = isCustomInt == 1
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	// 记账选分类用的使用次数：近 90 天内本账本同类型流水按分类计数，
	// 一级分类含其二级合计；失败时静默为 0，不影响分类列表本身。
	r.attachUsageCounts(ctx, categories, bookID, catType)
	return categories, nil
}

// attachUsageCounts 统计近 90 天各分类的使用笔数并写回 UsageCount。
func (r *CategoryRepo) attachUsageCounts(ctx context.Context, categories []model.Category, bookID, catType string) {
	if bookID == "" || len(categories) == 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -90).Format("2006-01-02")
	rows, err := r.db.QueryContext(ctx, `
		SELECT category_id, COUNT(*)
		FROM transactions
		WHERE book_id = ? AND type = ? AND date >= ? AND category_id IS NOT NULL AND category_id <> ''
		GROUP BY category_id
	`, bookID, catType, cutoff)
	if err != nil {
		return
	}
	defer rows.Close()
	direct := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err == nil {
			direct[id] = n
		}
	}
	for i := range categories {
		c := &categories[i]
		c.UsageCount = direct[c.ID]
		// 一级分类的使用次数含其全部二级合计。
		if c.ParentID == nil {
			for j := range categories {
				child := &categories[j]
				if child.ParentID != nil && *child.ParentID == c.ID {
					c.UsageCount += direct[child.ID]
				}
			}
		}
	}
}

func (r *CategoryRepo) UpdateCategory(ctx context.Context, c *model.Category) error {
	query := `UPDATE categories SET name = ?, icon = ?, color = ?, bg_color = ?, parent_id = ?, sort_order = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, c.Name, c.Icon, c.Color, c.BgColor, c.ParentID, c.SortOrder, c.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *CategoryRepo) DeleteCategory(ctx context.Context, catID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, _ = tx.ExecContext(ctx, `DELETE FROM category_budgets WHERE category_id = ?`, catID)
	_, _ = tx.ExecContext(ctx, `DELETE FROM categories WHERE parent_id = ?`, catID)

	res, err := tx.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, catID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// Tag methods
func (r *CategoryRepo) CreateTag(ctx context.Context, tag *model.Tag) error {
	if tag.ID == "" {
		tag.ID = uuid.NewString()
	}
	if tag.Color == "" {
		tag.Color = "#5D6470"
	}

	query := `INSERT INTO tags (id, book_id, name, color) VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, tag.ID, tag.BookID, tag.Name, tag.Color)
	return err
}

func (r *CategoryRepo) ListTags(ctx context.Context, bookID string) ([]model.Tag, error) {
	query := `SELECT id, book_id, name, color, created_at FROM tags WHERE 1=1`
	var args []interface{}
	if bookID != "" {
		query += " AND book_id = ?"
		args = append(args, bookID)
	}
	query += " ORDER BY created_at ASC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]model.Tag, 0)
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.BookID, &t.Name, &t.Color, &t.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *CategoryRepo) DeleteTag(ctx context.Context, tagID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM tags WHERE id = ?`, tagID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// EnsurePresetCategories 把预置分类里缺失的项补进所有已有的账本。
// 预置分类只在建账本时写入一次（CreateBook），之后再加新分类（比如
// 交通/车辆保养、交通/车险）老账本是拿不到的；这个方法在每次启动时
// 跑一遍，按（账本、类型、父级、名称）比对、缺失才插入，幂等。
// 返回本次补进的分类条数。
func (r *CategoryRepo) EnsurePresetCategories(ctx context.Context) (int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM books`)
	if err != nil {
		return 0, err
	}
	var bookIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		bookIDs = append(bookIDs, id)
	}
	rows.Close()

	added := 0
	catQuery := `INSERT INTO categories (id, book_id, type, name, icon, color, bg_color, parent_id, is_custom, sort_order) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`
	budgetQuery := `INSERT INTO category_budgets (book_id, category_id, limit_amount) VALUES (?, ?, ?) ON CONFLICT(book_id, category_id) DO NOTHING`
	for _, bookID := range bookIDs {
		for idx, c := range presetBookCategories {
			var parentID string
			if err := r.db.QueryRowContext(ctx,
				`SELECT id FROM categories WHERE book_id = ? AND type = ? AND parent_id IS NULL AND name = ?`,
				bookID, c.cType, c.name).Scan(&parentID); err != nil {
				parentID = uuid.NewString()
				if _, err := r.db.ExecContext(ctx, catQuery, parentID, bookID, c.cType, c.name, c.icon, c.color, c.bgColor, nil, idx+1); err != nil {
					return added, err
				}
				added++
				if c.budget > 0 {
					if _, err := r.db.ExecContext(ctx, budgetQuery, bookID, parentID, c.budget); err != nil {
						return added, err
					}
				}
			}
			var maxOrder int
			_ = r.db.QueryRowContext(ctx,
				`SELECT COALESCE(MAX(sort_order), 0) FROM categories WHERE book_id = ? AND parent_id = ?`,
				bookID, parentID).Scan(&maxOrder)
			for sIdx, sub := range c.subs {
				dup, err := r.ExistsDuplicate(ctx, bookID, c.cType, &parentID, sub.name)
				if err != nil {
					return added, err
				}
				if dup {
					continue
				}
				if _, err := r.db.ExecContext(ctx, catQuery, uuid.NewString(), bookID, c.cType, sub.name, sub.icon, c.color, c.bgColor, parentID, maxOrder+sIdx+1); err != nil {
					return added, err
				}
				added++
			}
		}
	}
	return added, nil
}
