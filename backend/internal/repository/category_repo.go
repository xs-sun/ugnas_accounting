package repository

import (
	"context"
	"database/sql"
	"errors"

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
		c.Color = "#0E8C7F"
	}

	query := `INSERT INTO categories (id, book_id, type, name, icon, color, parent_id) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, c.ID, c.BookID, c.Type, c.Name, c.Icon, c.Color, c.ParentID)
	return err
}

func (r *CategoryRepo) GetCategory(ctx context.Context, catID string) (*model.Category, error) {
	query := `SELECT id, book_id, type, name, icon, color, parent_id, created_at FROM categories WHERE id = ?`
	var c model.Category
	var pid sql.NullString
	err := r.db.QueryRowContext(ctx, query, catID).Scan(
		&c.ID, &c.BookID, &c.Type, &c.Name, &c.Icon, &c.Color, &pid, &c.CreatedAt,
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
	return &c, nil
}

func (r *CategoryRepo) ListCategories(ctx context.Context, bookID string, catType string) ([]model.Category, error) {
	query := `SELECT id, book_id, type, name, icon, color, parent_id, created_at FROM categories WHERE 1=1`
	var args []interface{}
	if bookID != "" {
		query += " AND book_id = ?"
		args = append(args, bookID)
	}
	if catType != "" {
		query += " AND type = ?"
		args = append(args, catType)
	}
	query += " ORDER BY created_at ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]model.Category, 0)
	for rows.Next() {
		var c model.Category
		var pid sql.NullString
		if err := rows.Scan(&c.ID, &c.BookID, &c.Type, &c.Name, &c.Icon, &c.Color, &pid, &c.CreatedAt); err != nil {
			return nil, err
		}
		if pid.Valid {
			c.ParentID = &pid.String
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

func (r *CategoryRepo) UpdateCategory(ctx context.Context, c *model.Category) error {
	query := `UPDATE categories SET name = ?, icon = ?, color = ?, parent_id = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, c.Name, c.Icon, c.Color, c.ParentID, c.ID)
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
	res, err := r.db.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, catID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
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
