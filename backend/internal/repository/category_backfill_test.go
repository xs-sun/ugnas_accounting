package repository

import (
	"context"
	"testing"
)

// 回归：预置分类只在建账本时写入一次，老账本拿不到后来新增的分类
//（2026-11-26 Justin 反馈：交通里没有车辆保养、车险）。
// EnsurePresetCategories 在启动时把缺失的预置分类补进所有账本，幂等。
func TestEnsurePresetCategoriesBackfill(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(t.TempDir())
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	// 模拟老账本：直接插账本行，不经过 CreateBook，所以没有任何预置分类。
	if _, err := db.ExecContext(ctx, `INSERT INTO books (id, name, currency, owner_id) VALUES ('b1', '老账本', 'CNY', 'u1')`); err != nil {
		t.Fatalf("insert book: %v", err)
	}
	catRepo := NewCategoryRepo(db)

	total := 0
	for _, c := range presetBookCategories {
		total += 1 + len(c.subs)
	}
	added, err := catRepo.EnsurePresetCategories(ctx)
	if err != nil {
		t.Fatalf("EnsurePresetCategories: %v", err)
	}
	if added != total {
		t.Fatalf("added = %d, want %d", added, total)
	}
	cats, err := catRepo.ListCategories(ctx, "b1", "expense")
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	hasSub := func(parent, sub string) bool {
		var parentID string
		for _, c := range cats {
			if c.ParentID == nil && c.Name == parent {
				parentID = c.ID
			}
		}
		if parentID == "" {
			return false
		}
		for _, c := range cats {
			if c.ParentID != nil && *c.ParentID == parentID && c.Name == sub {
				return true
			}
		}
		return false
	}
	if !hasSub("交通", "车辆保养") || !hasSub("交通", "车险") {
		t.Fatalf("backfilled categories missing 车辆保养/车险")
	}

	// 幂等：再跑一次什么都不补。
	if added, err := catRepo.EnsurePresetCategories(ctx); err != nil || added != 0 {
		t.Fatalf("second run added = %d, err = %v, want 0, nil", added, err)
	}

	// 删掉一个二级分类再跑，应只补回这一个。
	if _, err := db.ExecContext(ctx, `DELETE FROM categories WHERE book_id = 'b1' AND name = '车辆保养'`); err != nil {
		t.Fatalf("delete sub: %v", err)
	}
	if added, err := catRepo.EnsurePresetCategories(ctx); err != nil || added != 1 {
		t.Fatalf("after delete added = %d, err = %v, want 1, nil", added, err)
	}
}
