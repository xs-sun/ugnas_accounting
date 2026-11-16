package repository

import (
	"context"
	"database/sql"
	"errors"

	"com.xsguyu.accounting/backend/internal/model"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")
var ErrForbidden = errors.New("forbidden")

type BookRepo struct {
	db *DB
}

func NewBookRepo(db *DB) *BookRepo {
	return &BookRepo{db: db}
}

func (r *BookRepo) CreateBook(ctx context.Context, book *model.Book, userName string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if book.ID == "" {
		book.ID = uuid.NewString()
	}
	if book.Currency == "" {
		book.Currency = "CNY"
	}

	query := `INSERT INTO books (id, name, currency, description, owner_id) VALUES (?, ?, ?, ?, ?)`
	if _, err := tx.ExecContext(ctx, query, book.ID, book.Name, book.Currency, book.Description, book.OwnerID); err != nil {
		return err
	}

	// Add owner to book_members
	mQuery := `INSERT INTO book_members (book_id, user_id, user_name, role) VALUES (?, ?, ?, 'owner')`
	if _, err := tx.ExecContext(ctx, mQuery, book.ID, book.OwnerID, userName); err != nil {
		return err
	}

	// Seed default categories
	defaultCategories := []struct {
		name  string
		cType string
		icon  string
		color string
	}{
		{"餐饮美食", "expense", "food", "#D9533D"},
		{"交通出行", "expense", "trans", "#3F82A6"},
		{"购物生活", "expense", "shop", "#D6932C"},
		{"休闲娱乐", "expense", "fun", "#7B5CA8"},
		{"居家水电", "expense", "home", "#AF6829"},
		{"医疗健康", "expense", "health", "#B84D78"},
		{"工资薪金", "income", "salary", "#0E8C7F"},
		{"投资理财", "income", "invest", "#0B6E64"},
		{"其他收入", "income", "other", "#0E8C7F"},
	}

	catQuery := `INSERT INTO categories (id, book_id, type, name, icon, color) VALUES (?, ?, ?, ?, ?, ?)`
	for _, c := range defaultCategories {
		cID := uuid.NewString()
		if _, err := tx.ExecContext(ctx, catQuery, cID, book.ID, c.cType, c.name, c.icon, c.color); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *BookRepo) GetBook(ctx context.Context, bookID, userID string) (*model.Book, error) {
	query := `
	SELECT b.id, b.name, b.currency, b.description, b.owner_id, b.created_at, b.updated_at, COALESCE(bm.role, '')
	FROM books b
	LEFT JOIN book_members bm ON b.id = bm.book_id AND bm.user_id = ?
	WHERE b.id = ? AND (b.owner_id = ? OR bm.user_id IS NOT NULL)
	`
	var b model.Book
	err := r.db.QueryRowContext(ctx, query, userID, bookID, userID).Scan(
		&b.ID, &b.Name, &b.Currency, &b.Description, &b.OwnerID, &b.CreatedAt, &b.UpdatedAt, &b.Role,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if b.Role == "" && b.OwnerID == userID {
		b.Role = "owner"
	}
	return &b, nil
}

func (r *BookRepo) ListBooks(ctx context.Context, userID string) ([]model.Book, error) {
	query := `
	SELECT DISTINCT b.id, b.name, b.currency, b.description, b.owner_id, b.created_at, b.updated_at,
	       COALESCE(bm.role, CASE WHEN b.owner_id = ? THEN 'owner' ELSE 'viewer' END) as role
	FROM books b
	LEFT JOIN book_members bm ON b.id = bm.book_id AND bm.user_id = ?
	WHERE b.owner_id = ? OR bm.user_id IS NOT NULL
	ORDER BY b.created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []model.Book
	for rows.Next() {
		var b model.Book
		if err := rows.Scan(&b.ID, &b.Name, &b.Currency, &b.Description, &b.OwnerID, &b.CreatedAt, &b.UpdatedAt, &b.Role); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}

func (r *BookRepo) UpdateBook(ctx context.Context, book *model.Book) error {
	query := `UPDATE books SET name = ?, currency = ?, description = ?, updated_at = datetime('now') WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, book.Name, book.Currency, book.Description, book.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BookRepo) DeleteBook(ctx context.Context, bookID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM books WHERE id = ?`, bookID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BookRepo) AddMember(ctx context.Context, member *model.BookMember) error {
	// First check if book exists
	var dummy string
	err := r.db.QueryRowContext(ctx, `SELECT id FROM books WHERE id = ?`, member.BookID).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	// Check if already a member
	var cnt int
	_ = r.db.QueryRowContext(ctx, `SELECT count(*) FROM book_members WHERE book_id = ? AND user_id = ?`, member.BookID, member.UserID).Scan(&cnt)
	if cnt > 0 {
		return ErrConflict
	}

	query := `INSERT INTO book_members (book_id, user_id, user_name, role) VALUES (?, ?, ?, ?)`
	_, err = r.db.ExecContext(ctx, query, member.BookID, member.UserID, member.UserName, member.Role)
	return err
}

func (r *BookRepo) ListMembers(ctx context.Context, bookID string) ([]model.BookMember, error) {
	query := `SELECT book_id, user_id, user_name, role, created_at FROM book_members WHERE book_id = ? ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, query, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []model.BookMember
	for rows.Next() {
		var m model.BookMember
		if err := rows.Scan(&m.BookID, &m.UserID, &m.UserName, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *BookRepo) GetMemberRole(ctx context.Context, bookID, userID string) (string, error) {
	var role string
	err := r.db.QueryRowContext(ctx, `SELECT role FROM book_members WHERE book_id = ? AND user_id = ?`, bookID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		// Check if owner
		var ownerID string
		if err := r.db.QueryRowContext(ctx, `SELECT owner_id FROM books WHERE id = ?`, bookID).Scan(&ownerID); err == nil && ownerID == userID {
			return "owner", nil
		}
		return "", ErrNotFound
	}
	return role, err
}

func (r *BookRepo) RemoveMember(ctx context.Context, bookID, userID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM book_members WHERE book_id = ? AND user_id = ?`, bookID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
