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
var ErrInviteCodeUsed = errors.New("invite code already used")
var ErrInviteCodeExpired = errors.New("invite code expired")

type BookRepo struct {
	db *DB
}

func NewBookRepo(db *DB) *BookRepo {
	return &BookRepo{db: db}
}

type presetSubCat struct {
	name string
	icon string
}

type presetCat struct {
	name    string
	cType   string
	icon    string
	color   string
	bgColor string
	budget  int64
	subs    []presetSubCat
}

var presetBookCategories = []presetCat{
	{
		name: "餐饮", cType: "expense", icon: "bowl", color: "#D6553E", bgColor: "#FDF1EE", budget: 120000,
		subs: []presetSubCat{
			{name: "早餐", icon: "breakfast"},
			{name: "午餐", icon: "bento"},
			{name: "晚餐", icon: "bowl"},
			{name: "咖啡饮品", icon: "coffee"},
			{name: "买菜水果", icon: "fruit"},
			{name: "聚餐", icon: "dining"},
		},
	},
	{
		name: "购物", cType: "expense", icon: "bag", color: "#DE8A1E", bgColor: "#FFF6E8", budget: 80000,
		subs: []presetSubCat{
			{name: "日用品", icon: "bag"},
			{name: "生鲜超市", icon: "cart"},
			{name: "服饰鞋包", icon: "shirt"},
			{name: "数码家电", icon: "laptop"},
			{name: "美妆护肤", icon: "sparkles"},
		},
	},
	{
		name: "交通", cType: "expense", icon: "train", color: "#2F78A8", bgColor: "#EDF5FA", budget: 30000,
		subs: []presetSubCat{
			{name: "地铁公交", icon: "train"},
			{name: "打车出行", icon: "car"},
			{name: "加油停车", icon: "fuel"},
			{name: "高铁机票", icon: "plane"},
		},
	},
	{
		name: "居家", cType: "expense", icon: "home", color: "#3E8E63", bgColor: "#ECF6F0", budget: 190000,
		subs: []presetSubCat{
			{name: "房租物业", icon: "home"},
			{name: "水电燃气", icon: "bolt"},
			{name: "宽带话费", icon: "wifi"},
			{name: "家居维修", icon: "wrench"},
		},
	},
	{
		name: "娱乐", cType: "expense", icon: "gamepad", color: "#7E57A6", bgColor: "#F4EFF9", budget: 40000,
		subs: []presetSubCat{
			{name: "电影演出", icon: "film"},
			{name: "游戏充值", icon: "gamepad"},
			{name: "运动健身", icon: "dumbbell"},
			{name: "旅行出游", icon: "compass"},
		},
	},
	{
		name: "医疗", cType: "expense", icon: "medkit", color: "#C84B59", bgColor: "#FCEEEF",
		subs: []presetSubCat{
			{name: "药品购置", icon: "medkit"},
			{name: "体检挂号", icon: "sparkles"},
			{name: "商业保险", icon: "wallet"},
		},
	},
	{
		name: "学习", cType: "expense", icon: "book", color: "#456799", bgColor: "#EEF2F8",
		subs: []presetSubCat{
			{name: "书籍资料", icon: "book"},
			{name: "课程培训", icon: "laptop"},
			{name: "办公文具", icon: "tag"},
		},
	},
	{
		name: "宠物", cType: "expense", icon: "paw", color: "#B0743C", bgColor: "#F9F2EA",
		subs: []presetSubCat{
			{name: "粮食零食", icon: "bowl"},
			{name: "猫砂用品", icon: "bag"},
			{name: "洗护医疗", icon: "medkit"},
		},
	},
	{
		name: "人情", cType: "expense", icon: "gift", color: "#C45B7A", bgColor: "#FCF0F4",
		subs: []presetSubCat{
			{name: "红包礼金", icon: "wallet"},
			{name: "请客送礼", icon: "gift"},
		},
	},
	{
		name: "其他", cType: "expense", icon: "tag", color: "#64748B", bgColor: "#F1F5F9",
		subs: []presetSubCat{
			{name: "临时支出", icon: "tag"},
			{name: "杂项", icon: "sparkles"},
		},
	},
	{
		name: "工资", cType: "income", icon: "wallet", color: "#3E8E63", bgColor: "#ECF6F0",
		subs: []presetSubCat{
			{name: "基本薪资", icon: "wallet"},
			{name: "绩效补贴", icon: "star"},
		},
	},
	{
		name: "奖金", cType: "income", icon: "star", color: "#DE8A1E", bgColor: "#FFF6E8",
		subs: []presetSubCat{
			{name: "年终奖", icon: "star"},
			{name: "项目奖金", icon: "gift"},
		},
	},
	{
		name: "理财", cType: "income", icon: "trend", color: "#2F78A8", bgColor: "#EDF5FA",
		subs: []presetSubCat{
			{name: "基金收益", icon: "trend"},
			{name: "利息分红", icon: "wallet"},
		},
	},
	{
		name: "报销退款", cType: "income", icon: "refund", color: "#4B8B71", bgColor: "#EDF6F2",
		subs: []presetSubCat{
			{name: "差旅报销", icon: "plane"},
			{name: "退货退款", icon: "refund"},
		},
	},
	{
		name: "其他收入", cType: "income", icon: "tag", color: "#64748B", bgColor: "#F1F5F9",
		subs: []presetSubCat{
			{name: "闲置转卖", icon: "bag"},
			{name: "副业兼职", icon: "laptop"},
		},
	},
}

func (r *BookRepo) generateUniqueInviteCodeTx(ctx context.Context, tx *sql.Tx) string {
	for i := 0; i < 30; i++ {
		code := randomInviteToken8()
		var cnt int
		_ = tx.QueryRowContext(ctx, `SELECT count(*) FROM book_invite_codes WHERE UPPER(code) = UPPER(?)`, code).Scan(&cnt)
		if cnt == 0 {
			_ = tx.QueryRowContext(ctx, `SELECT count(*) FROM books WHERE UPPER(invite_code) = UPPER(?)`, code).Scan(&cnt)
		}
		if cnt == 0 {
			return code
		}
	}
	return "CAT7-" + uuid.NewString()[:8]
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
	if book.Color == "" {
		book.Color = "#DE8A1E"
	}
	if book.Budget <= 0 {
		book.Budget = 450000
	}
	if book.InviteCode == "" {
		book.InviteCode = r.generateUniqueInviteCodeTx(ctx, tx)
	}

	query := `INSERT INTO books (id, name, currency, description, color, budget, invite_code, owner_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := tx.ExecContext(ctx, query, book.ID, book.Name, book.Currency, book.Description, book.Color, book.Budget, book.InviteCode, book.OwnerID); err != nil {
		return err
	}

	if book.InviteCode != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO book_invite_codes (code, book_id, role, created_by, expires_at)
			VALUES (UPPER(?), ?, 'editor', ?, datetime('now', '+30 days'))
		`, book.InviteCode, book.ID, book.OwnerID); err != nil {
			return err
		}
	}

	ownerAvatar := "/avatars/cat-1.jpg"
	_ = tx.QueryRowContext(ctx, `SELECT COALESCE(avatar, '/avatars/cat-1.jpg') FROM users WHERE id = ?`, book.OwnerID).Scan(&ownerAvatar)
	if ownerAvatar == "" {
		ownerAvatar = "/avatars/cat-1.jpg"
	}

	mQuery := `INSERT INTO book_members (book_id, user_id, user_name, role, avatar, color, is_virtual, status) VALUES (?, ?, ?, 'owner', ?, ?, 0, 'active')`
	if _, err := tx.ExecContext(ctx, mQuery, book.ID, book.OwnerID, userName, ownerAvatar, book.Color); err != nil {
		return err
	}

	catQuery := `INSERT INTO categories (id, book_id, type, name, icon, color, bg_color, parent_id, is_custom, sort_order) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`
	budgetQuery := `INSERT INTO category_budgets (book_id, category_id, limit_amount) VALUES (?, ?, ?) ON CONFLICT(book_id, category_id) DO NOTHING`
	for idx, c := range presetBookCategories {
		parentID := uuid.NewString()
		if _, err := tx.ExecContext(ctx, catQuery, parentID, book.ID, c.cType, c.name, c.icon, c.color, c.bgColor, nil, idx+1); err != nil {
			return err
		}
		if c.budget > 0 {
			if _, err := tx.ExecContext(ctx, budgetQuery, book.ID, parentID, c.budget); err != nil {
				return err
			}
		}
		for sIdx, sub := range c.subs {
			subID := uuid.NewString()
			if _, err := tx.ExecContext(ctx, catQuery, subID, book.ID, c.cType, sub.name, sub.icon, c.color, c.bgColor, parentID, sIdx+1); err != nil {
				return err
			}
		}
	}

	defaultWallets := []struct {
		name      string
		wType     string
		groupName string
		color     string
	}{
		{"微信支付", "wechat", "电子钱包", "#3E8E63"},
		{"支付宝", "alipay", "电子钱包", "#2F78A8"},
		{"招商银行卡", "debit", "储蓄卡", "#DE8A1E"},
		{"现金钱包", "cash", "现金", "#64748B"},
	}
	walletQuery := `INSERT INTO wallets (id, book_id, type, name, group_name, color, balance) VALUES (?, ?, ?, ?, ?, ?, 0)`
	for _, w := range defaultWallets {
		if _, err := tx.ExecContext(ctx, walletQuery, uuid.NewString(), book.ID, w.wType, w.name, w.groupName, w.color); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *BookRepo) EnsureDefaultBookForUser(ctx context.Context, userID, userName string) error {
	if userID == "" {
		return nil
	}
	key := "user_seeded_" + userID
	var existing string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM system_settings WHERE key = ?`, key).Scan(&existing)
	if err == nil && existing == "1" {
		return nil
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO system_settings (key, value, updated_at) VALUES (?, '1', datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = '1', updated_at = datetime('now')
	`, key); err != nil {
		return err
	}
	if userName == "" {
		userName = userID
	}
	book := &model.Book{
		Name:        "个人日常账本",
		Currency:    "CNY",
		Description: "日常衣食住行 · 计入总资产",
		Color:       "#DE8A1E",
		Budget:      450000,
		OwnerID:     userID,
	}
	return r.CreateBook(ctx, book, userName)
}

func (r *BookRepo) getUserDefaultBookID(ctx context.Context, userID string) string {
	if userID == "" {
		return ""
	}
	var defaultBookID string
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(default_book_id, '') FROM users WHERE id = ?`, userID).Scan(&defaultBookID)
	return defaultBookID
}

func (r *BookRepo) populateBookExtras(ctx context.Context, b *model.Book, currentUserID string) {
	_ = r.db.QueryRowContext(ctx, `SELECT count(*) FROM transactions WHERE book_id = ?`, b.ID).Scan(&b.TxCount)
	members, err := r.ListMembersForUser(ctx, b.ID, currentUserID)
	if err == nil && members != nil {
		b.Members = members
	} else {
		b.Members = []model.BookMember{}
	}
}

func (r *BookRepo) GetBook(ctx context.Context, bookID, userID string) (*model.Book, error) {
	query := `
	SELECT b.id, b.name, b.currency, COALESCE(b.description, ''), COALESCE(b.color, '#DE8A1E'),
	       COALESCE(b.budget, 450000), COALESCE(b.invite_code, ''), b.owner_id, b.created_at, b.updated_at,
	       COALESCE(bm.role, '')
	FROM books b
	LEFT JOIN book_members bm ON b.id = bm.book_id AND bm.user_id = ?
	WHERE b.id = ? AND (b.owner_id = ? OR bm.user_id IS NOT NULL)
	`
	var b model.Book
	err := r.db.QueryRowContext(ctx, query, userID, bookID, userID).Scan(
		&b.ID, &b.Name, &b.Currency, &b.Description, &b.Color,
		&b.Budget, &b.InviteCode, &b.OwnerID, &b.CreatedAt, &b.UpdatedAt, &b.Role,
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
	if b.Role != "owner" {
		b.InviteCode = ""
	}
	defaultBookID := r.getUserDefaultBookID(ctx, userID)
	b.IsDefault = defaultBookID != "" && b.ID == defaultBookID
	r.populateBookExtras(ctx, &b, userID)
	return &b, nil
}

// IssueInviteCode 生成一条新的一次性邀请码；invalidateExisting 为 true 时同时作废该账本未使用的旧展示码并更新 books.invite_code。
func (r *BookRepo) IssueInviteCode(ctx context.Context, bookID, createdBy, role string, invalidateExisting bool) (string, error) {
	if role != "editor" && role != "viewer" {
		role = "editor"
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM books WHERE id = ?`, bookID).Scan(&exists); err != nil {
		return "", err
	}
	if exists == 0 {
		return "", ErrNotFound
	}

	if invalidateExisting {
		if _, err := tx.ExecContext(ctx, `
			UPDATE book_invite_codes
			SET used_by = 'revoked', used_at = datetime('now')
			WHERE book_id = ? AND used_at = ''
		`, bookID); err != nil {
			return "", err
		}
	}

	code := r.generateUniqueInviteCodeTx(ctx, tx)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO book_invite_codes (code, book_id, role, created_by, expires_at)
		VALUES (UPPER(?), ?, ?, ?, datetime('now', '+7 days'))
	`, code, bookID, role, createdBy); err != nil {
		return "", err
	}

	if invalidateExisting {
		if _, err := tx.ExecContext(ctx, `UPDATE books SET invite_code = ?, updated_at = datetime('now') WHERE id = ?`, code, bookID); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return code, nil
}

// ConsumeInviteCodeAndJoin 原子校验并核销一次性邀请码，确保一个邀请码只能使用一次。
func (r *BookRepo) ConsumeInviteCodeAndJoin(ctx context.Context, inviteCode, userID, userName string) (*model.Book, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()

	var (
		storedCode string
		bookID     string
		role       string
		usedAt     string
		isExpired  int
	)
	err = tx.QueryRowContext(ctx, `
		SELECT code, book_id, role, COALESCE(used_at, ''), (datetime('now') > expires_at)
		FROM book_invite_codes
		WHERE UPPER(code) = UPPER(?)
		LIMIT 1
	`, inviteCode).Scan(&storedCode, &bookID, &role, &usedAt, &isExpired)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, ErrNotFound
	}
	if err != nil {
		return nil, false, err
	}
	if usedAt != "" {
		return nil, false, ErrInviteCodeUsed
	}
	if isExpired != 0 {
		return nil, false, ErrInviteCodeExpired
	}

	// 已是该账本成员/拥有者时直接幂等返回，不消耗一次性邀请码
	var memberCnt int
	_ = tx.QueryRowContext(ctx, `SELECT count(*) FROM book_members WHERE book_id = ? AND user_id = ?`, bookID, userID).Scan(&memberCnt)
	if memberCnt == 0 {
		var ownerID string
		if err := tx.QueryRowContext(ctx, `SELECT owner_id FROM books WHERE id = ?`, bookID).Scan(&ownerID); err == nil && ownerID == userID {
			memberCnt = 1
		}
	}
	if memberCnt > 0 {
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		b, err := r.GetBook(ctx, bookID, userID)
		return b, false, err
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE book_invite_codes
		SET used_by = ?, used_at = datetime('now')
		WHERE code = ? AND used_at = ''
	`, userID, storedCode)
	if err != nil {
		return nil, false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, false, ErrInviteCodeUsed
	}

	if role != "editor" && role != "viewer" {
		role = "editor"
	}
	avatar := ""
	_ = tx.QueryRowContext(ctx, `SELECT COALESCE(avatar, '') FROM users WHERE id = ?`, userID).Scan(&avatar)
	if avatar == "" {
		avatar = "/avatars/cat-1.jpg"
	}
	if userName == "" {
		userName = userID
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO book_members (book_id, user_id, user_name, role, avatar, color, is_virtual, status)
		VALUES (?, ?, ?, ?, ?, '#DE8A1E', 0, 'active')
	`, bookID, userID, userName, role, avatar); err != nil {
		return nil, false, err
	}

	// 若该码是账本当前展示码，核销后同步清空，避免界面继续展示已失效口令
	_, _ = tx.ExecContext(ctx, `
		UPDATE books SET invite_code = '', updated_at = datetime('now')
		WHERE id = ? AND UPPER(invite_code) = UPPER(?)
	`, bookID, storedCode)

	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	b, err := r.GetBook(ctx, bookID, userID)
	return b, true, err
}

func (r *BookRepo) GetBookByInviteCode(ctx context.Context, inviteCode string) (*model.Book, error) {
	query := `
	SELECT id, name, currency, COALESCE(description, ''), COALESCE(color, '#DE8A1E'),
	       COALESCE(budget, 450000), COALESCE(invite_code, ''), owner_id, created_at, updated_at
	FROM books WHERE UPPER(invite_code) = UPPER(?) LIMIT 1
	`
	var b model.Book
	err := r.db.QueryRowContext(ctx, query, inviteCode).Scan(
		&b.ID, &b.Name, &b.Currency, &b.Description, &b.Color,
		&b.Budget, &b.InviteCode, &b.OwnerID, &b.CreatedAt, &b.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *BookRepo) ListBooks(ctx context.Context, userID string) ([]model.Book, error) {
	query := `
	SELECT DISTINCT b.id, b.name, b.currency, COALESCE(b.description, ''), COALESCE(b.color, '#DE8A1E'),
	       COALESCE(b.budget, 450000), COALESCE(b.invite_code, ''), b.owner_id, b.created_at, b.updated_at,
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

	var books []model.Book
	for rows.Next() {
		var b model.Book
		if err := rows.Scan(
			&b.ID, &b.Name, &b.Currency, &b.Description, &b.Color,
			&b.Budget, &b.InviteCode, &b.OwnerID, &b.CreatedAt, &b.UpdatedAt, &b.Role,
		); err != nil {
			rows.Close()
			return nil, err
		}
		books = append(books, b)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	defaultBookID := r.getUserDefaultBookID(ctx, userID)
	for i := range books {
		if books[i].Role != "owner" {
			books[i].InviteCode = ""
		}
		books[i].IsDefault = defaultBookID != "" && books[i].ID == defaultBookID
		r.populateBookExtras(ctx, &books[i], userID)
	}
	return books, nil
}

func (r *BookRepo) UpdateBook(ctx context.Context, book *model.Book) error {
	if book.Color == "" {
		book.Color = "#DE8A1E"
	}
	query := `UPDATE books SET name = ?, currency = ?, description = ?, color = ?, budget = ?, updated_at = datetime('now') WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, book.Name, book.Currency, book.Description, book.Color, book.Budget, book.ID)
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
	_, _ = r.db.ExecContext(ctx, `UPDATE users SET default_book_id = '' WHERE default_book_id = ?`, bookID)
	return nil
}

func (r *BookRepo) AddMember(ctx context.Context, member *model.BookMember) error {
	var dummy string
	err := r.db.QueryRowContext(ctx, `SELECT id FROM books WHERE id = ?`, member.BookID).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	var cnt int
	_ = r.db.QueryRowContext(ctx, `SELECT count(*) FROM book_members WHERE book_id = ? AND user_id = ?`, member.BookID, member.UserID).Scan(&cnt)
	if cnt > 0 {
		return ErrConflict
	}

	if member.Color == "" {
		member.Color = "#DE8A1E"
	}
	if member.Status == "" {
		member.Status = "active"
	}
	if member.Avatar == "" && !member.IsVirtual {
		_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(avatar, '') FROM users WHERE id = ?`, member.UserID).Scan(&member.Avatar)
		if member.Avatar == "" {
			member.Avatar = "/avatars/cat-1.jpg"
		}
	}
	isVirtualInt := 0
	if member.IsVirtual {
		isVirtualInt = 1
	}

	query := `INSERT INTO book_members (book_id, user_id, user_name, role, avatar, color, is_virtual, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = r.db.ExecContext(ctx, query, member.BookID, member.UserID, member.UserName, member.Role, member.Avatar, member.Color, isVirtualInt, member.Status)
	return err
}

func (r *BookRepo) ListMembers(ctx context.Context, bookID string) ([]model.BookMember, error) {
	return r.ListMembersForUser(ctx, bookID, "")
}

func (r *BookRepo) ListMembersForUser(ctx context.Context, bookID, currentUserID string) ([]model.BookMember, error) {
	query := `
	SELECT bm.book_id, bm.user_id, bm.user_name, bm.role,
	       COALESCE(NULLIF(bm.avatar, ''), COALESCE(u.avatar, '')),
	       COALESCE(bm.color, '#DE8A1E'),
	       COALESCE(bm.is_virtual, 0),
	       COALESCE(bm.status, 'active'),
	       bm.created_at
	FROM book_members bm
	LEFT JOIN users u ON bm.user_id = u.id
	WHERE bm.book_id = ?
	ORDER BY bm.created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []model.BookMember
	for rows.Next() {
		var m model.BookMember
		var isVirtInt int
		if err := rows.Scan(
			&m.BookID, &m.UserID, &m.UserName, &m.Role,
			&m.Avatar, &m.Color, &isVirtInt, &m.Status, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		m.IsVirtual = isVirtInt == 1
		m.IsMe = currentUserID != "" && m.UserID == currentUserID
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *BookRepo) GetMemberRole(ctx context.Context, bookID, userID string) (string, error) {
	var role string
	err := r.db.QueryRowContext(ctx, `SELECT role FROM book_members WHERE book_id = ? AND user_id = ?`, bookID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
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
	_, _ = r.db.ExecContext(ctx, `UPDATE users SET default_book_id = '' WHERE id = ? AND default_book_id = ?`, userID, bookID)
	return nil
}
