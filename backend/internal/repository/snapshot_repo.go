package repository

import (
	"context"
	"database/sql"
	"errors"

	"com.xsguyu.accounting/backend/internal/model"
)

type SnapshotRepo struct {
	db *DB
}

func NewSnapshotRepo(db *DB) *SnapshotRepo {
	return &SnapshotRepo{db: db}
}

func (r *SnapshotRepo) CreateSnapshot(ctx context.Context, s *model.Snapshot) error {
	query := `INSERT INTO snapshots (id, file_name, file_path, file_size, sha256, created_by) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, s.ID, s.FileName, s.FilePath, s.FileSize, s.SHA256, s.CreatedBy)
	return err
}

func (r *SnapshotRepo) ListSnapshots(ctx context.Context) ([]model.Snapshot, error) {
	query := `SELECT id, file_name, file_path, file_size, sha256, created_by, created_at FROM snapshots ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Snapshot
	for rows.Next() {
		var s model.Snapshot
		if err := rows.Scan(&s.ID, &s.FileName, &s.FilePath, &s.FileSize, &s.SHA256, &s.CreatedBy, &s.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *SnapshotRepo) GetSnapshot(ctx context.Context, id string) (*model.Snapshot, error) {
	query := `SELECT id, file_name, file_path, file_size, sha256, created_by, created_at FROM snapshots WHERE id = ?`
	var s model.Snapshot
	err := r.db.QueryRowContext(ctx, query, id).Scan(&s.ID, &s.FileName, &s.FilePath, &s.FileSize, &s.SHA256, &s.CreatedBy, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SnapshotRepo) DeleteSnapshot(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM snapshots WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
