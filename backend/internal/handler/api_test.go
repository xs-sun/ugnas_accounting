package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
	"com.xsguyu.accounting/backend/internal/service"
)

type testEnv struct {
	db                   *repository.DB
	dataDir              string
	handler              http.Handler
	bookRepo             *repository.BookRepo
	walletRepo           *repository.WalletRepo
	catRepo              *repository.CategoryRepo
	txRepo               *repository.TransactionRepo
	userRepo             *repository.UserRepo
	budgetRepo           *repository.BudgetRepo
	recurringRepo        *repository.RecurringRepo
	settingsRepo         *repository.SettingsRepo
	activityRepo         *repository.ActivityRepo
	authSvc              *service.AuthService
	recorder             *service.ActivityRecorder
	hub                  *service.Hub
	recurringScheduler   *service.RecurringScheduler
	budgetAlertScheduler *service.BudgetAlertScheduler
	autoBackupScheduler  *service.AutoBackupScheduler
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dataDir := t.TempDir()
	db, err := repository.InitDB(dataDir)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	bookRepo := repository.NewBookRepo(db)
	walletRepo := repository.NewWalletRepo(db)
	catRepo := repository.NewCategoryRepo(db)
	txRepo := repository.NewTransactionRepo(db)
	snapshotRepo := repository.NewSnapshotRepo(db)
	userRepo := repository.NewUserRepo(db)
	budgetRepo := repository.NewBudgetRepo(db)
	recurringRepo := repository.NewRecurringRepo(db)
	settingsRepo := repository.NewSettingsRepo(db, dataDir)
	aiChatRepo := repository.NewAiChatRepo(db)

	snapshotSvc := service.NewSnapshotService(db, snapshotRepo, dataDir)
	importSvc := service.NewImportService(db, txRepo, catRepo, walletRepo)
	exportSvc := service.NewExportService(db)
	authSvc := service.NewAuthService(userRepo, settingsRepo)
	aiSvc := service.NewAiService(bookRepo, catRepo, walletRepo, txRepo, budgetRepo, userRepo, settingsRepo)
	activityRepo := repository.NewActivityRepo(db)
	hub := service.NewHub()
	recorder := service.NewActivityRecorder(activityRepo, budgetRepo, hub)
	recSched := service.NewRecurringScheduler(recurringRepo, txRepo, hub, recorder)
	budgetAlertSched := service.NewBudgetAlertScheduler(recorder)
	bakSched := service.NewAutoBackupScheduler(snapshotSvc, settingsRepo)

	authH := NewAuthHandler(userRepo, bookRepo, walletRepo, settingsRepo, authSvc)
	userH := NewUserHandler(bookRepo, walletRepo, txRepo, userRepo, settingsRepo, dataDir)
	bookH := NewBookHandler(bookRepo, userRepo, authSvc, hub, recorder)
	walletH := NewWalletHandler(walletRepo, bookRepo, hub, recorder)
	catH := NewCategoryHandler(catRepo, hub, recorder)
	txH := NewTransactionHandler(txRepo, bookRepo, walletRepo, dataDir, hub, recorder)
	reportH := NewReportHandler(txRepo)
	budgetH := NewBudgetHandler(budgetRepo, bookRepo, hub, recorder)
	recH := NewRecurringHandler(recurringRepo, hub, recorder)
	aiH := NewAiHandler(aiSvc)
	aiChatH := NewAiChatHandler(aiChatRepo, bookRepo, dataDir)
	adminH := NewAdminHandler(settingsRepo, userRepo, authSvc)
	impExpH := NewImportExportHandler(importSvc, exportSvc, hub, recorder, aiSvc, bookRepo)
	snapH := NewSnapshotHandler(snapshotSvc)
	streamH := NewStreamHandler(hub, bookRepo, userRepo, authSvc)
	activityH := NewActivityHandler(activityRepo, bookRepo)

	router := NewRouter(
		RouterConfig{AppID: "com.xsguyu.accounting", Version: "0.3.0", StaticDir: filepath.Join(dataDir, "nonexistent_www")},
		authH, userH, bookH, walletH, catH, txH, reportH, budgetH, recH, aiH, adminH, impExpH, snapH, streamH, activityH, aiChatH,
	)

	chain := CORSMiddleware(NewAuthMiddleware(userRepo, authSvc)(router))
	return &testEnv{
		db:                   db,
		dataDir:              dataDir,
		handler:              chain,
		bookRepo:             bookRepo,
		walletRepo:           walletRepo,
		catRepo:              catRepo,
		txRepo:               txRepo,
		userRepo:             userRepo,
		budgetRepo:           budgetRepo,
		recurringRepo:        recurringRepo,
		settingsRepo:         settingsRepo,
		activityRepo:         activityRepo,
		authSvc:              authSvc,
		recorder:             recorder,
		hub:                  hub,
		recurringScheduler:   recSched,
		budgetAlertScheduler: budgetAlertSched,
		autoBackupScheduler:  bakSched,
	}
}

func doJSONReq(t *testing.T, h http.Handler, method, path string, body interface{}, headers map[string]string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		buf = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	var resp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	return rr, resp
}

func TestSchemaMigrationIdempotency(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "accounting.db")

	// Simulate legacy v0.1.7 database without new columns or tables
	legacyDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	_, err = legacyDB.Exec(`
		CREATE TABLE books (id TEXT PRIMARY KEY, name TEXT NOT NULL, currency TEXT NOT NULL DEFAULT 'CNY', description TEXT DEFAULT '', owner_id TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now')));
		INSERT INTO books (id, name, owner_id) VALUES ('old_b1', '旧账本', 'u_old');
		CREATE TABLE book_members (book_id TEXT NOT NULL, user_id TEXT NOT NULL, user_name TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'editor', created_at TEXT NOT NULL DEFAULT (datetime('now')), PRIMARY KEY (book_id, user_id));
		CREATE TABLE wallets (id TEXT PRIMARY KEY, book_id TEXT NOT NULL, type TEXT NOT NULL, name TEXT NOT NULL, balance INTEGER NOT NULL DEFAULT 0, credit_limit INTEGER NOT NULL DEFAULT 0, billing_day INTEGER DEFAULT 1, repayment_day INTEGER DEFAULT 20, card_last_four TEXT DEFAULT '', exclude_net_asset INTEGER NOT NULL DEFAULT 0, is_archived INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now')));
		CREATE TABLE categories (id TEXT PRIMARY KEY, book_id TEXT NOT NULL, type TEXT NOT NULL, name TEXT NOT NULL, icon TEXT NOT NULL DEFAULT 'tag', color TEXT NOT NULL DEFAULT '#0E8C7F', parent_id TEXT DEFAULT NULL, created_at TEXT NOT NULL DEFAULT (datetime('now')));
		CREATE TABLE transactions (id TEXT PRIMARY KEY, book_id TEXT NOT NULL, type TEXT NOT NULL, amount INTEGER NOT NULL, date TEXT NOT NULL, wallet_id TEXT NOT NULL, to_wallet_id TEXT DEFAULT NULL, category_id TEXT DEFAULT NULL, note TEXT DEFAULT '', order_no TEXT DEFAULT '', created_by TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now')));
	`)
	if err != nil {
		t.Fatalf("seed legacy db: %v", err)
	}
	_ = legacyDB.Close()

	// Run InitDB twice to ensure full idempotency
	db1, err := repository.InitDB(dir)
	if err != nil {
		t.Fatalf("first InitDB failed: %v", err)
	}
	_ = db1.Close()

	db2, err := repository.InitDB(dir)
	if err != nil {
		t.Fatalf("second InitDB failed: %v", err)
	}
	defer db2.Close()

	// Verify backfilled invite_code on legacy book
	var inviteCode string
	var budget int64
	if err := db2.QueryRow(`SELECT invite_code, budget FROM books WHERE id = 'old_b1'`).Scan(&inviteCode, &budget); err != nil {
		t.Fatalf("query migrated book: %v", err)
	}
	if !strings.HasPrefix(inviteCode, "CAT7-") || budget != 450000 {
		t.Fatalf("unexpected migrated book values: invite=%s budget=%d", inviteCode, budget)
	}

	// Verify all 6 new tables exist
	for _, tbl := range []string{"users", "email_codes", "category_budgets", "recurring_rules", "ai_usage_logs", "system_settings"} {
		var name string
		if err := db2.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name); err != nil {
			t.Fatalf("expected table %s to exist: %v", tbl, err)
		}
	}
}

func TestAuthAndUserLifecycle(t *testing.T) {
	env := setupTestEnv(t)

	// Health check returns v0.3.0
	rr, resp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/health", nil, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("health status %d", rr.Code)
	}
	data := resp["data"].(map[string]interface{})
	if data["version"] != "0.3.0" {
		t.Fatalf("expected version 0.3.0, got %v", data["version"])
	}

	// Send verification code for register
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/send-code", map[string]string{
		"email":   "xiaohe@cat7.app",
		"purpose": "register",
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("send-code failed: %d %s", rr.Code, rr.Body.String())
	}
	demoCode := resp["data"].(map[string]interface{})["demo_code"].(string)
	if len(demoCode) != 6 {
		t.Fatalf("expected 6-digit demo_code, got %s", demoCode)
	}

	// Register first user (becomes admin)
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"name":     "林小禾",
		"email":    "xiaohe@cat7.app",
		"code":     demoCode,
		"password": "password123",
	}, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", rr.Code, rr.Body.String())
	}
	regData := resp["data"].(map[string]interface{})
	token := regData["token"].(string)
	if !strings.HasPrefix(token, "cat7_") {
		t.Fatalf("expected cat7_ token, got %s", token)
	}

	// Login with password
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "xiaohe@cat7.app",
		"password": "password123",
		"mode":     "password",
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("password login failed: %d %s", rr.Code, rr.Body.String())
	}

	// Login with code
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/send-code", map[string]string{
		"email":   "xiaohe@cat7.app",
		"purpose": "login",
	}, nil)
	loginCode := resp["data"].(map[string]interface{})["demo_code"].(string)
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email": "xiaohe@cat7.app",
		"code":  loginCode,
		"mode":  "code",
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("code login failed: %d", rr.Code)
	}

	// Forgot password (also verify purpose="forgot" alias)
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/send-code", map[string]string{
		"email":   "xiaohe@cat7.app",
		"purpose": "forgot",
	}, nil)
	resetCode := resp["data"].(map[string]interface{})["demo_code"].(string)
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/forgot-password", map[string]string{
		"email":        "xiaohe@cat7.app",
		"code":         resetCode,
		"new_password": "newpassword456",
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("forgot password failed: %d", rr.Code)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "xiaohe@cat7.app",
		"password": "newpassword456",
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("login with new password failed: %d", rr.Code)
	}

	// Update profile with Base64 avatar upload & fetch avatar binary
	authHeaders := map[string]string{"Authorization": "Bearer " + token}
	tinyPNG := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	rr, resp = doJSONReq(t, env.handler, http.MethodPut, "/api/v1/user/profile", map[string]string{
		"name":   "林小禾Pro",
		"avatar": tinyPNG,
	}, authHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("update profile failed: %d %s", rr.Code, rr.Body.String())
	}
	prof := resp["data"].(map[string]interface{})
	avatarURL := prof["avatar"].(string)
	if !strings.HasPrefix(avatarURL, "/api/v1/avatars/") || prof["user_name"] != "林小禾Pro" {
		t.Fatalf("unexpected profile after update: %+v", prof)
	}

	avReq := httptest.NewRequest(http.MethodGet, avatarURL, nil)
	avRec := httptest.NewRecorder()
	env.handler.ServeHTTP(avRec, avReq)
	if avRec.Code != http.StatusOK || avRec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("serve avatar failed: code=%d ct=%s", avRec.Code, avRec.Header().Get("Content-Type"))
	}

	// Path traversal check on avatars
	badReq := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/..%2Faccounting.db", nil)
	badRec := httptest.NewRecorder()
	env.handler.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for avatar path traversal, got %d", badRec.Code)
	}

	// UGOS user bind-email
	ugosHeaders := map[string]string{
		"Ugreen-User-ID":   "ug_1001",
		"Ugreen-User-Name": "UG用户",
		"Ugreen-User-Type": "users",
	}
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/send-code", map[string]string{
		"email":   "ug1001@cat7.app",
		"purpose": "bind",
	}, nil)
	bindCode := resp["data"].(map[string]interface{})["demo_code"].(string)
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/user/bind-email", map[string]string{
		"email": "ug1001@cat7.app",
		"code":  bindCode,
	}, ugosHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("bind email failed: %d %s", rr.Code, rr.Body.String())
	}
	if resp["data"].(map[string]interface{})["email_bound"] != true {
		t.Fatalf("expected email_bound=true")
	}

	// Registration rules: require invite code and closed registration
	rr, _ = doJSONReq(t, env.handler, http.MethodPut, "/api/v1/admin/reg-config", map[string]interface{}{
		"allow_register":      true,
		"require_invite_code": true,
		"invite_code":         "CAT7-VIP-999",
	}, authHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("update reg-config failed: %d", rr.Code)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"name":        "BlockedUser",
		"email":       "blocked@cat7.app",
		"password":    "pass1234",
		"invite_code": "WRONG",
	}, nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for wrong invite code, got %d", rr.Code)
	}

	// Valid invite code succeeds
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"name":        "陈屿",
		"email":       "chenyu@cat7.app",
		"password":    "pass1234",
		"invite_code": "CAT7-VIP-999",
	}, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 with valid invite code, got %d %s", rr.Code, rr.Body.String())
	}
	user2Token := resp["data"].(map[string]interface{})["token"].(string)
	user2ID := resp["data"].(map[string]interface{})["user"].(map[string]interface{})["user_id"].(string)

	// Non-admin cannot access admin API
	rr, _ = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/admin/overview", nil, map[string]string{
		"Authorization": "Bearer " + user2Token,
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", rr.Code)
	}

	// Admin cannot disable self -> 400
	adminID := regData["user"].(map[string]interface{})["user_id"].(string)
	rr, _ = doJSONReq(t, env.handler, http.MethodPut, "/api/v1/admin/users/"+adminID, map[string]interface{}{
		"is_active": false,
	}, authHeaders)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when admin disables self, got %d", rr.Code)
	}

	// Admin disables user2 -> token rejected with 403
	rr, _ = doJSONReq(t, env.handler, http.MethodPut, "/api/v1/admin/users/"+user2ID, map[string]interface{}{
		"is_active": false,
	}, authHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("disable user failed: %d", rr.Code)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/user/profile", nil, map[string]string{
		"Authorization": "Bearer " + user2Token,
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disabled user, got %d", rr.Code)
	}
}

func TestBooksWalletsCategoriesTransactionsAndBudgets(t *testing.T) {
	env := setupTestEnv(t)
	ownerH := map[string]string{"Ugreen-User-ID": "u_owner", "Ugreen-User-Name": "小禾", "Ugreen-User-Type": "admin"}
	guestH := map[string]string{"Ugreen-User-ID": "u_guest", "Ugreen-User-Name": "访客", "Ugreen-User-Type": "users"}

	// First GET /books auto-seeds one default book
	rr, resp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/books", nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("list books failed: %d", rr.Code)
	}
	books := resp["data"].([]interface{})
	if len(books) != 1 {
		t.Fatalf("expected 1 auto-seeded book, got %d", len(books))
	}
	b0 := books[0].(map[string]interface{})
	bookID := b0["id"].(string)
	inviteCode := b0["invite_code"].(string)
	if b0["name"] != "个人日常账本" || int64(b0["budget"].(float64)) != 450000 || !strings.HasPrefix(inviteCode, "CAT7-") {
		t.Fatalf("unexpected auto-seeded book: %+v", b0)
	}

	// Verify 10 expense top cats + 36 subcats + 5 income top cats + 10 subcats = 61 categories
	cats, err := env.catRepo.ListCategories(context.Background(), bookID, "")
	if err != nil || len(cats) != 61 {
		t.Fatalf("expected 61 preset categories, got %d (err=%v)", len(cats), err)
	}
	var foodCatID, shopCatID, coffeeSubID string
	for _, c := range cats {
		if c.Type == "expense" && c.ParentID == nil && c.Name == "餐饮" {
			foodCatID = c.ID
		}
		if c.Type == "expense" && c.ParentID == nil && c.Name == "购物" {
			shopCatID = c.ID
		}
		if c.Type == "expense" && c.ParentID != nil && c.Name == "咖啡饮品" {
			coffeeSubID = c.ID
		}
	}
	if foodCatID == "" || shopCatID == "" || coffeeSubID == "" {
		t.Fatalf("missing expected preset categories")
	}

	// Verify 4 preset wallets with zero initial balance
	wallets, err := env.walletRepo.ListWallets(context.Background(), bookID, false)
	if err != nil || len(wallets) != 4 {
		t.Fatalf("expected 4 preset wallets, got %d", len(wallets))
	}
	wechatWalletID := wallets[0].ID

	// Test deleting the seeded book does NOT auto-seed again on next GET /books
	rr, _ = doJSONReq(t, env.handler, http.MethodDelete, "/api/v1/books/"+bookID, nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete book failed: %d", rr.Code)
	}
	rr, resp = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/books", nil, ownerH)
	if len(resp["data"].([]interface{})) != 0 {
		t.Fatalf("expected 0 books after explicit deletion, got %v", resp["data"])
	}

	// Create a new book for detailed flow tests
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books", map[string]interface{}{
		"name":   "家庭账本",
		"color":  "#3E8E63",
		"budget": 600000,
	}, ownerH)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create book failed: %d", rr.Code)
	}
	bNew := resp["data"].(map[string]interface{})
	bookID = bNew["id"].(string)
	inviteCode = bNew["invite_code"].(string)

	cats, _ = env.catRepo.ListCategories(context.Background(), bookID, "")
	for _, c := range cats {
		if c.Type == "expense" && c.ParentID == nil && c.Name == "餐饮" {
			foodCatID = c.ID
		}
		if c.Type == "expense" && c.ParentID != nil && c.Name == "咖啡饮品" {
			coffeeSubID = c.ID
		}
		if c.Type == "income" && c.ParentID == nil && c.Name == "工资" {
			shopCatID = c.ID // reuse variable for income cat check
		}
	}
	wallets, _ = env.walletRepo.ListWallets(context.Background(), bookID, false)
	wechatWalletID = wallets[0].ID

	// Test Wallet PUT balance update fix
	rr, resp = doJSONReq(t, env.handler, http.MethodPut, "/api/v1/wallets/"+wechatWalletID, map[string]interface{}{
		"balance":    500000,
		"group_name": "日常消费",
		"color":      "#3E8E63",
	}, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("update wallet failed: %d", rr.Code)
	}
	if int64(resp["data"].(map[string]interface{})["balance"].(float64)) != 500000 {
		t.Fatalf("expected wallet balance 500000, got %v", resp["data"])
	}

	// Invite virtual member, email member, and join by code
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/"+bookID+"/virtual-members", map[string]string{
		"name":  "家里猫咪",
		"color": "#DE8A1E",
	}, ownerH)
	if rr.Code != http.StatusCreated {
		t.Fatalf("add virtual member failed: %d", rr.Code)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/"+bookID+"/invite-email", map[string]string{
		"email": "partner@cat7.app",
		"role":  "editor",
	}, ownerH)
	if rr.Code != http.StatusCreated {
		t.Fatalf("invite email failed: %d", rr.Code)
	}
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/join", map[string]string{
		"invite_code": inviteCode,
	}, guestH)
	if rr.Code != http.StatusOK {
		t.Fatalf("join by code failed: %d %s", rr.Code, rr.Body.String())
	}

	// Transactions: create with subcategory, title, include_budget=true and another with include_budget=false
	today := time.Now().Format("2006-01-02")
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/transactions", map[string]interface{}{
		"book_id":        bookID,
		"type":           "expense",
		"amount":         3850,
		"date":           today,
		"tx_time":        "10:15",
		"wallet_id":      wechatWalletID,
		"category_id":    coffeeSubID,
		"title":          "瑞幸生椰拿铁",
		"sub_category":   "咖啡饮品",
		"note":           "早起提神",
		"include_budget": true,
	}, ownerH)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create tx1 failed: %d %s", rr.Code, rr.Body.String())
	}
	tx1ID := resp["data"].(map[string]interface{})["id"].(string)

	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/transactions", map[string]interface{}{
		"book_id":        bookID,
		"type":           "expense",
		"amount":         10000,
		"date":           today,
		"wallet_id":      wechatWalletID,
		"category_id":    foodCatID,
		"title":          "公司垫付聚餐",
		"include_budget": false,
		"reimburse":      true,
	}, ownerH)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create tx2 failed: %d", rr.Code)
	}
	tx2ID := resp["data"].(map[string]interface{})["id"].(string)

	// Query transactions by parent category ID should match subcategory tx1 as well!
	rr, resp = doJSONReq(t, env.handler, http.MethodGet,
		fmt.Sprintf("/api/v1/transactions?book_id=%s&category_id=%s&search=拿铁&page_size=200", bookID, foodCatID), nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("list tx failed: %d", rr.Code)
	}
	txListData := resp["data"].(map[string]interface{})
	if int(txListData["total"].(float64)) != 1 || int(txListData["page_size"].(float64)) != 200 {
		t.Fatalf("expected 1 matched tx and page_size=200, got %+v", txListData)
	}

	// Budgets overview: only tx1 (3850) counts toward budget, tx2 (include_budget=false) does not!
	now := time.Now()
	rr, resp = doJSONReq(t, env.handler, http.MethodGet,
		fmt.Sprintf("/api/v1/budgets?book_id=%s&year=%d&month=%d", bookID, now.Year(), int(now.Month())), nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("get budgets failed: %d", rr.Code)
	}
	bov := resp["data"].(map[string]interface{})
	if int64(bov["total_spent"].(float64)) != 3850 || int64(bov["total_budget"].(float64)) != 600000 {
		t.Fatalf("expected total_spent=3850 total_budget=600000, got %+v", bov)
	}

	// Update category budget & total budget
	rr, _ = doJSONReq(t, env.handler, http.MethodPut, "/api/v1/budgets/total", map[string]interface{}{
		"book_id": bookID,
		"budget":  500000,
	}, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("update total budget failed: %d", rr.Code)
	}
	rr, resp = doJSONReq(t, env.handler, http.MethodPut, "/api/v1/budgets/category", map[string]interface{}{
		"book_id":      bookID,
		"category_id":  foodCatID,
		"limit_amount": 150000,
	}, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("update category budget failed: %d", rr.Code)
	}

	// Stats report: rolls up subcategory coffeeSubID into top category 餐饮
	rr, resp = doJSONReq(t, env.handler, http.MethodGet,
		fmt.Sprintf("/api/v1/reports/stats?book_id=%s&period=month&type=expense&year=%d&month=%d", bookID, now.Year(), int(now.Month())), nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("get stats report failed: %d", rr.Code)
	}
	stats := resp["data"].(map[string]interface{})
	if int64(stats["total_expense"].(float64)) != 13850 || int64(stats["budget_used"].(float64)) != 3850 {
		t.Fatalf("unexpected stats totals: %+v", stats)
	}
	statCats := stats["categories"].([]interface{})
	if len(statCats) != 1 || statCats[0].(map[string]interface{})["category_name"] != "餐饮" {
		t.Fatalf("expected subcategory rolled up into 餐饮, got %+v", statCats)
	}

	// Batch delete both transactions -> wallet balance should return to 500000
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/transactions/batch-delete", map[string]interface{}{
		"ids": []string{tx1ID, tx2ID},
	}, ownerH)
	if rr.Code != http.StatusOK || int(resp["data"].(map[string]interface{})["deleted"].(float64)) != 2 {
		t.Fatalf("batch delete failed: %d %+v", rr.Code, resp)
	}
	wAfter, _ := env.walletRepo.GetWallet(context.Background(), wechatWalletID)
	if wAfter.Balance != 500000 {
		t.Fatalf("expected wallet balance restored to 500000 after batch delete, got %d", wAfter.Balance)
	}

	// Category duplicate validation & cascading delete of subcategories
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/categories", map[string]interface{}{
		"book_id": bookID,
		"type":    "expense",
		"name":    "测试一级",
	}, ownerH)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create custom category failed: %d", rr.Code)
	}
	customParentID := resp["data"].(map[string]interface{})["id"].(string)
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/categories", map[string]interface{}{
		"book_id":   bookID,
		"type":      "expense",
		"name":      "测试一级",
		"parent_id": customParentID,
	}, ownerH)
	if rr.Code != http.StatusCreated {
		t.Fatalf("subcategory with same name under different parent_id should succeed, got %d", rr.Code)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/categories", map[string]interface{}{
		"book_id": bookID,
		"type":    "expense",
		"name":    "测试一级",
	}, ownerH)
	if rr.Code != http.StatusConflict {
		t.Fatalf("duplicate top-level category should return 409, got %d", rr.Code)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodDelete, "/api/v1/categories/"+customParentID, nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete parent category failed: %d", rr.Code)
	}
	catsAfter, _ := env.catRepo.ListCategories(context.Background(), bookID, "expense")
	for _, c := range catsAfter {
		if c.Name == "测试一级" {
			t.Fatalf("expected parent and child categories deleted together")
		}
	}
}

func TestRecurringAiAdminAndSchedulers(t *testing.T) {
	env := setupTestEnv(t)
	adminH := map[string]string{"Ugreen-User-ID": "u_admin", "Ugreen-User-Name": "管理员", "Ugreen-User-Type": "admin"}

	// Seed default book
	_, resp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/books", nil, adminH)
	bookID := resp["data"].([]interface{})[0].(map[string]interface{})["id"].(string)
	cats, _ := env.catRepo.ListCategories(context.Background(), bookID, "expense")
	wallets, _ := env.walletRepo.ListWallets(context.Background(), bookID, false)

	// Recurring CRUD + Toggle + Scheduler execution
	today := time.Now().Format("2006-01-02")
	rr, resp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/recurring", map[string]interface{}{
		"book_id":     bookID,
		"type":        "expense",
		"category_id": cats[0].ID,
		"title":       "每月房租",
		"amount":      185000,
		"cycle":       "每月",
		"start_date":  today,
		"next_date":   today,
		"wallet_id":   wallets[0].ID,
	}, adminH)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create recurring failed: %d %s", rr.Code, rr.Body.String())
	}
	ruleID := resp["data"].(map[string]interface{})["id"].(string)

	// Run scheduler once -> executes due rule
	executed := env.recurringScheduler.RunOnce(context.Background())
	if executed != 1 {
		t.Fatalf("expected 1 recurring rule executed, got %d", executed)
	}
	wAfter, _ := env.walletRepo.GetWallet(context.Background(), wallets[0].ID)
	if wAfter.Balance != -185000 {
		t.Fatalf("expected wallet balance -185000 after recurring execution, got %d", wAfter.Balance)
	}

	// Toggle & Delete recurring
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/recurring/"+ruleID+"/toggle", nil, adminH)
	if rr.Code != http.StatusOK || resp["data"].(map[string]interface{})["is_active"] != false {
		t.Fatalf("toggle recurring failed: %d %+v", rr.Code, resp)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodDelete, "/api/v1/recurring/"+ruleID, nil, adminH)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete recurring failed: %d", rr.Code)
	}

	// AI Chat (expense draft) + AI Chat (query)
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/ai/chat", map[string]interface{}{
		"book_id": bookID,
		"message": "刚才在星巴克买咖啡花了 38.5 元用微信支付",
	}, adminH)
	if rr.Code != http.StatusOK {
		t.Fatalf("ai chat failed: %d %s", rr.Code, rr.Body.String())
	}
	chatData := resp["data"].(map[string]interface{})
	draft := chatData["draft"].(map[string]interface{})
	if int64(draft["amount"].(float64)) != 3850 || draft["category_name"] != "餐饮" {
		t.Fatalf("unexpected ai draft: %+v", draft)
	}

	// AI OCR Demo JSON & Multipart upload
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/ai/ocr", map[string]interface{}{
		"book_id": bookID,
		"demo":    "hema",
	}, adminH)
	if rr.Code != http.StatusOK {
		t.Fatalf("ai ocr demo failed: %d", rr.Code)
	}
	ocrDraft := resp["data"].(map[string]interface{})["draft"].(map[string]interface{})
	if int64(ocrDraft["amount"].(float64)) != 8650 {
		t.Fatalf("expected 8650 cents for hema ocr, got %v", ocrDraft["amount"])
	}

	var mpBuf bytes.Buffer
	mw := multipart.NewWriter(&mpBuf)
	_ = mw.WriteField("book_id", bookID)
	fw, _ := mw.CreateFormFile("file", "starbucks_receipt.jpg")
	_, _ = fw.Write([]byte("fakejpgdata"))
	_ = mw.Close()
	mpReq := httptest.NewRequest(http.MethodPost, "/api/v1/ai/ocr", &mpBuf)
	mpReq.Header.Set("Content-Type", mw.FormDataContentType())
	mpReq.Header.Set("Ugreen-User-ID", "u_admin")
	mpReq.Header.Set("Ugreen-User-Type", "admin")
	mpRec := httptest.NewRecorder()
	env.handler.ServeHTTP(mpRec, mpReq)
	if mpRec.Code != http.StatusOK {
		t.Fatalf("ai ocr multipart failed: %d %s", mpRec.Code, mpRec.Body.String())
	}

	// AI Insights & AI Test
	rr, resp = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/ai/insights?book_id="+bookID+"&period=month&type=expense&seed=1", nil, adminH)
	if rr.Code != http.StatusOK || len(resp["data"].(map[string]interface{})["tips"].([]interface{})) != 3 {
		t.Fatalf("ai insights failed: %d %+v", rr.Code, resp)
	}
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/ai/test", map[string]string{}, adminH)
	if rr.Code != http.StatusOK || resp["data"].(map[string]interface{})["connected"] != true {
		t.Fatalf("ai test failed: %d %+v", rr.Code, resp)
	}

	// Admin endpoints & daily AI limit 429 enforcement
	rr, _ = doJSONReq(t, env.handler, http.MethodPut, "/api/v1/admin/users/u_admin", map[string]interface{}{
		"daily_ai_limit": 3,
	}, adminH)
	if rr.Code != http.StatusOK {
		t.Fatalf("update user daily limit failed: %d", rr.Code)
	}
	// Already made 3 calls (1 chat + 2 ocr), next call must return 429
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/ai/chat", map[string]interface{}{
		"book_id": bookID,
		"message": "超限测试 10 元",
	}, adminH)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after reaching daily ai limit, got %d", rr.Code)
	}

	rr, resp = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/admin/overview", nil, adminH)
	if rr.Code != http.StatusOK || int(resp["data"].(map[string]interface{})["today_ai_calls"].(float64)) != 3 {
		t.Fatalf("admin overview failed: %d %+v", rr.Code, resp)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/admin/ai-usage?search=管理员", nil, adminH)
	if rr.Code != http.StatusOK {
		t.Fatalf("admin ai-usage failed: %d", rr.Code)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodPut, "/api/v1/admin/ai-config", map[string]interface{}{
		"provider": "OpenAI 兼容接口",
		"model":    "gpt-4o-mini",
	}, adminH)
	if rr.Code != http.StatusOK {
		t.Fatalf("admin ai-config failed: %d", rr.Code)
	}
	rr, resp = doJSONReq(t, env.handler, http.MethodPut, "/api/v1/admin/system-config", map[string]interface{}{
		"auto_backup_enabled": true,
		"auto_backup_retain":  2,
		"site_name":           "七喵记账NAS",
	}, adminH)
	if rr.Code != http.StatusOK {
		t.Fatalf("admin system-config failed: %d", rr.Code)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/admin/smtp-test", map[string]interface{}{}, adminH)
	if rr.Code != http.StatusOK {
		t.Fatalf("admin smtp-test failed: %d", rr.Code)
	}

	// Auto backup scheduler creates snapshot and prunes beyond retain=2
	for i := 0; i < 3; i++ {
		if _, err := env.autoBackupScheduler.ExecuteBackupAndPrune(context.Background(), 2); err != nil {
			t.Fatalf("backup prune %d failed: %v", i, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	rr, resp = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/snapshots", nil, adminH)
	if rr.Code != http.StatusOK || len(resp["data"].([]interface{})) != 2 {
		t.Fatalf("expected 2 retained snapshots, got %d %+v", rr.Code, resp)
	}

	// Swagger doc check
	swReq := httptest.NewRequest(http.MethodGet, "/api/swagger/doc.json", nil)
	swRec := httptest.NewRecorder()
	env.handler.ServeHTTP(swRec, swReq)
	if swRec.Code != http.StatusOK || !strings.Contains(swRec.Body.String(), "/api/v1/recurring") {
		t.Fatalf("swagger doc check failed: %d", swRec.Code)
	}

	// User profile metrics check
	rr, resp = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/user/profile", nil, adminH)
	if rr.Code != http.StatusOK {
		t.Fatalf("profile failed: %d", rr.Code)
	}
	prof := resp["data"].(map[string]interface{})
	if int(prof["total_transactions_count"].(float64)) != 1 || int(prof["streak_days"].(float64)) < 1 {
		t.Fatalf("expected user profile metrics populated, got %+v", prof)
	}

	// Legacy Reports Regression (/monthly, /trend, /daily)
	now := time.Now()
	rr, resp = doJSONReq(t, env.handler, http.MethodGet,
		fmt.Sprintf("/api/v1/reports/monthly?book_id=%s&year=%d&month=%d", bookID, now.Year(), int(now.Month())), nil, adminH)
	if rr.Code != http.StatusOK || int64(resp["data"].(map[string]interface{})["total_expense"].(float64)) != 185000 {
		t.Fatalf("monthly report regression failed: %d %+v", rr.Code, resp)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/reports/trend?book_id="+bookID+"&months=6", nil, adminH)
	if rr.Code != http.StatusOK {
		t.Fatalf("trend report regression failed: %d", rr.Code)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodGet,
		fmt.Sprintf("/api/v1/reports/daily?book_id=%s&year=%d&month=%d", bookID, now.Year(), int(now.Month())), nil, adminH)
	if rr.Code != http.StatusOK {
		t.Fatalf("daily report regression failed: %d", rr.Code)
	}

	// WeChat CSV Import Preview + Commit + Export CSV Regression
	wechatCSV := "\xef\xbb\xbf微信支付账单明细\n----------------------微信支付账单明细列表--------------------\n" +
		"交易时间,交易类型,交易对方,商品,收/支,金额(元),支付方式,当前状态,交易单号,商户单号,备注\n" +
		today + " 12:00:00,商户消费,星巴克咖啡,燕麦拿铁,支出,¥35.00,零钱,支付成功,WX10001,M10001,咖啡\n"
	var wxBuf bytes.Buffer
	wxW := multipart.NewWriter(&wxBuf)
	wxFile, _ := wxW.CreateFormFile("file", "wechat.csv")
	_, _ = wxFile.Write([]byte(wechatCSV))
	_ = wxW.Close()
	wxReq := httptest.NewRequest(http.MethodPost, "/api/v1/imports/wechat", &wxBuf)
	wxReq.Header.Set("Content-Type", wxW.FormDataContentType())
	wxReq.Header.Set("Ugreen-User-ID", "u_admin")
	wxReq.Header.Set("Ugreen-User-Type", "admin")
	wxRec := httptest.NewRecorder()
	env.handler.ServeHTTP(wxRec, wxReq)
	if wxRec.Code != http.StatusOK {
		t.Fatalf("wechat import preview failed: %d %s", wxRec.Code, wxRec.Body.String())
	}
	var wxResp map[string]interface{}
	_ = json.Unmarshal(wxRec.Body.Bytes(), &wxResp)
	parsedItems := wxResp["data"].(map[string]interface{})["items"].([]interface{})
	if len(parsedItems) != 1 {
		t.Fatalf("expected 1 parsed wechat item, got %d", len(parsedItems))
	}

	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/imports/commit", map[string]interface{}{
		"book_id":   bookID,
		"wallet_id": wallets[0].ID,
		"items":     parsedItems,
	}, adminH)
	if rr.Code != http.StatusOK || int(resp["data"].(map[string]interface{})["inserted"].(float64)) != 1 {
		t.Fatalf("import commit failed: %d %+v", rr.Code, resp)
	}

	expReq := httptest.NewRequest(http.MethodGet, "/api/v1/export?book_id="+bookID, nil)
	expReq.Header.Set("Ugreen-User-ID", "u_admin")
	expReq.Header.Set("Ugreen-User-Type", "admin")
	expRec := httptest.NewRecorder()
	env.handler.ServeHTTP(expRec, expReq)
	if expRec.Code != http.StatusOK || !strings.Contains(expRec.Body.String(), "35.00") || !strings.Contains(expRec.Body.String(), "1850.00") {
		t.Fatalf("export csv failed: code=%d body=%s", expRec.Code, expRec.Body.String())
	}
}

func TestModelContextAdminHelper(t *testing.T) {
	u1 := &model.UserContext{ID: "1", Type: "users", Role: "user"}
	if u1.IsAdmin() {
		t.Fatalf("expected non-admin")
	}
	u2 := &model.UserContext{ID: "2", Type: "users", Role: "admin"}
	if !u2.IsAdmin() {
		t.Fatalf("expected admin when role=admin")
	}
}

func TestAuthMiddlewareProductionVsDevFallback(t *testing.T) {
	env := setupTestEnv(t)

	// 1. Production mode (default, APP_ENV empty): missing gateway headers & token -> 401 Unauthorized
	t.Setenv("APP_ENV", "")
	t.Setenv("STRICT_AUTH", "")
	rr, _ := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/user/profile", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 in production mode without auth, got %d", rr.Code)
	}

	// Public routes and static SPA routes remain accessible without auth in production mode
	rr, _ = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/health", nil, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 on health route in production mode, got %d", rr.Code)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodGet, "/", nil, nil)
	if rr.Code == http.StatusUnauthorized {
		t.Fatalf("expected static root route not to return 401 in production mode, got %d", rr.Code)
	}

	// 2. Development mode (APP_ENV=development): missing auth falls back to dev user
	t.Setenv("APP_ENV", "development")
	rr, _ = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/user/profile", nil, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 in development fallback mode, got %d", rr.Code)
	}

	// 3. STRICT_AUTH=1 overrides APP_ENV=development
	t.Setenv("STRICT_AUTH", "1")
	rr, _ = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/user/profile", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when STRICT_AUTH=1, got %d", rr.Code)
	}
}

func TestAuthStatusAndGatewayQuickLoginFlow(t *testing.T) {
	env := setupTestEnv(t)
	t.Setenv("APP_ENV", "")
	t.Setenv("STRICT_AUTH", "")

	// 1. Fresh install: first_install=true, has_registered_users=false
	rr, resp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/auth/status", nil, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /auth/status failed: %d %v", rr.Code, resp)
	}
	statusData := resp["data"].(map[string]interface{})
	if statusData["first_install"] != true || statusData["has_registered_users"] != false {
		t.Fatalf("expected first_install=true, got %v", statusData)
	}

	// 2. Gateway login without headers -> 401
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/gateway", map[string]interface{}{}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without gateway headers, got %d", rr.Code)
	}

	// 3. Gateway login with UGOS headers (including percent-encoded UTF-8 name, unregistered user) -> need_register + bind_ticket
	gwHeaders := map[string]string{
		"Ugreen-User-ID":   "ugos_1001",
		"Ugreen-User-Name": "%E7%BB%BF%E8%81%94%E6%B7%98%E4%B8%83",
		"Ugreen-User-Type": "users",
	}
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/gateway", map[string]interface{}{}, gwHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /auth/gateway failed: %d %v", rr.Code, resp)
	}
	gwData := resp["data"].(map[string]interface{})
	if gwData["status"] != "need_register" {
		t.Fatalf("expected status=need_register, got %v", gwData)
	}
	if gwData["suggested_username"] != "绿联淘七" || gwData["suggested_name"] != "绿联淘七" {
		t.Fatalf("expected decoded chinese suggested name, got username=%v name=%v", gwData["suggested_username"], gwData["suggested_name"])
	}
	bindTicket, _ := gwData["bind_ticket"].(string)
	if !strings.HasPrefix(bindTicket, "cat7bind_") {
		t.Fatalf("expected cat7bind_ ticket prefix, got %q", bindTicket)
	}

	// 4. Enable email verification and confirm gateway bind registration enforces the exact same rule
	if err := env.settingsRepo.SaveRegConfig(context.Background(), &model.RegConfig{
		AllowRegister:       true,
		RequireEmailVerify:  true,
		DefaultDailyAiLimit: 30,
	}); err != nil {
		t.Fatalf("SaveRegConfig failed: %v", err)
	}

	// Missing verification code must be rejected even with bind_ticket
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{
		"username":    "taoqi_nas",
		"name":        "淘七",
		"email":       "taoqi@example.com",
		"password":    "pass123456",
		"avatar":      "/cats/cat_1.png",
		"bind_ticket": bindTicket,
	}, nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when email verification code is missing, got %d", rr.Code)
	}

	// Fetch demo_code and complete registration
	rr, codeResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/send-code", map[string]interface{}{
		"email":   "taoqi@example.com",
		"purpose": "register",
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("send-code failed: %d", rr.Code)
	}
	demoCode, _ := codeResp["data"].(map[string]interface{})["demo_code"].(string)
	if demoCode == "" {
		t.Fatalf("expected demo_code in test environment")
	}

	rr, regResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{
		"username":    "taoqi_nas",
		"name":        "淘七",
		"email":       "taoqi@example.com",
		"code":        demoCode,
		"password":    "pass123456",
		"avatar":      "/cats/cat_2.png",
		"bind_ticket": bindTicket,
	}, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("gateway bind register failed: %d %v", rr.Code, regResp)
	}
	regData := regResp["data"].(map[string]interface{})
	token, _ := regData["token"].(string)
	if !strings.HasPrefix(token, "cat7_") {
		t.Fatalf("expected cat7_ bearer token, got %q", token)
	}
	userMap := regData["user"].(map[string]interface{})
	if userMap["username"] != "taoqi_nas" || userMap["gateway_bound"] != true || userMap["role"] != "admin" {
		t.Fatalf("unexpected registered user profile: %v", userMap)
	}

	// 5. Status reflects registered state
	rr, resp = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/auth/status", nil, nil)
	statusData = resp["data"].(map[string]interface{})
	if statusData["first_install"] != false || statusData["has_registered_users"] != true {
		t.Fatalf("expected first_install=false after registration, got %v", statusData)
	}

	// 6. Subsequent gateway login authenticates immediately with backend token
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/gateway", map[string]interface{}{}, gwHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("second gateway login failed: %d %v", rr.Code, resp)
	}
	gwData = resp["data"].(map[string]interface{})
	if gwData["status"] != "authenticated" {
		t.Fatalf("expected authenticated on repeat gateway login, got %v", gwData)
	}

	// 7. Subsequent login via username + password and standalone Bearer token usage without gateway headers
	rr, loginResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/login", map[string]interface{}{
		"account":  "taoqi_nas",
		"password": "pass123456",
		"mode":     "password",
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("username password login failed: %d %v", rr.Code, loginResp)
	}
	loginToken, _ := loginResp["data"].(map[string]interface{})["token"].(string)
	if !strings.HasPrefix(loginToken, "cat7_") {
		t.Fatalf("expected cat7_ login token, got %q", loginToken)
	}

	rr, profResp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/user/profile", nil, map[string]string{
		"Authorization": "Bearer " + loginToken,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected bearer token profile access without gateway headers, got %d %v", rr.Code, profResp)
	}
	if profResp["data"].(map[string]interface{})["username"] != "taoqi_nas" {
		t.Fatalf("profile mismatch: %v", profResp)
	}
}

func TestLegacyGatewayUserUpgradePreservesBooks(t *testing.T) {
	env := setupTestEnv(t)
	t.Setenv("APP_ENV", "")
	t.Setenv("STRICT_AUTH", "")

	legacyHeaders := map[string]string{
		"Ugreen-User-ID":   "ugos_legacy_88",
		"Ugreen-User-Name": "老张",
		"Ugreen-User-Type": "users",
	}

	// Legacy implicit gateway request creates user & custom book without email/password
	rr, profBefore := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/user/profile", nil, legacyHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("legacy profile init failed: %d", rr.Code)
	}
	legacyUserID := profBefore["data"].(map[string]interface{})["user_id"].(string)

	rr, createBookResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books", map[string]interface{}{
		"name":     "老张家庭账本",
		"currency": "CNY",
	}, legacyHeaders)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("legacy create book failed: %d %v", rr.Code, createBookResp)
	}

	// Status should still indicate no formally registered account yet
	_, statusResp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/auth/status", nil, nil)
	if statusResp["data"].(map[string]interface{})["first_install"] != true {
		t.Fatalf("expected first_install=true before legacy user completes registration")
	}

	// Gateway quick login asks legacy user to complete email/username/password registration
	rr, gwResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/gateway", map[string]interface{}{}, legacyHeaders)
	if rr.Code != http.StatusOK {
		t.Fatalf("legacy gateway login failed: %d %v", rr.Code, gwResp)
	}
	gwData := gwResp["data"].(map[string]interface{})
	if gwData["status"] != "need_register" {
		t.Fatalf("expected legacy user without email/password to need_register, got %v", gwData)
	}
	bindTicket := gwData["bind_ticket"].(string)

	rr, regResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{
		"username":    "laozhang",
		"name":        "老张已升级",
		"email":       "laozhang@example.com",
		"password":    "secret123",
		"avatar":      "/cats/cat_3.png",
		"bind_ticket": bindTicket,
	}, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("legacy user upgrade failed: %d %v", rr.Code, regResp)
	}
	regData := regResp["data"].(map[string]interface{})
	upgradedUser := regData["user"].(map[string]interface{})
	if upgradedUser["user_id"] != legacyUserID {
		t.Fatalf("expected same user_id=%s after upgrade, got %v", legacyUserID, upgradedUser["user_id"])
	}

	// Access books purely via issued backend Bearer token (without any gateway headers)
	bearer := regData["token"].(string)
	rr, booksResp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/books", nil, map[string]string{
		"Authorization": "Bearer " + bearer,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /books with token failed: %d %v", rr.Code, booksResp)
	}
	rawBooks := fmt.Sprintf("%v", booksResp["data"])
	if !strings.Contains(rawBooks, "老张家庭账本") {
		t.Fatalf("expected legacy book preserved after upgrade, got %s", rawBooks)
	}
}

func TestSingleUseInviteCodeAndTransactionMemberInfo(t *testing.T) {
	env := setupTestEnv(t)
	ownerH := map[string]string{"Ugreen-User-ID": "40001", "Ugreen-User-Name": "owner_one", "Ugreen-User-Type": "admin"}
	userBH := map[string]string{"Ugreen-User-ID": "40002", "Ugreen-User-Name": "member_two", "Ugreen-User-Type": "users"}
	userCH := map[string]string{"Ugreen-User-ID": "40003", "Ugreen-User-Name": "outsider_three", "Ugreen-User-Type": "users"}

	// Owner creates a book
	rr, resp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books", map[string]interface{}{
		"name": "安全协作账本",
	}, ownerH)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create book failed: %d %s", rr.Code, rr.Body.String())
	}
	bookData := resp["data"].(map[string]interface{})
	bookID := bookData["id"].(string)
	code1 := bookData["invite_code"].(string)
	if !strings.HasPrefix(code1, "CAT7-") || len(code1) < 13 {
		t.Fatalf("expected high-entropy invite code, got %q", code1)
	}

	// User B joins using code1 -> succeeds once
	rr, joinResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/join", map[string]string{
		"invite_code": code1,
	}, userBH)
	if rr.Code != http.StatusOK {
		t.Fatalf("user B join failed: %d %s", rr.Code, rr.Body.String())
	}
	// Non-owner should NOT see invite_code in response
	if joinedBook := joinResp["data"].(map[string]interface{}); joinedBook["invite_code"] != "" && joinedBook["invite_code"] != nil {
		t.Fatalf("non-owner should not receive invite_code, got %v", joinedBook["invite_code"])
	}

	// User C tries to reuse code1 -> MUST fail (410 Gone)
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/join", map[string]string{
		"invite_code": code1,
	}, userCH)
	if rr.Code != http.StatusGone {
		t.Fatalf("reusing single-use invite code must return 410, got %d %s", rr.Code, rr.Body.String())
	}

	// Non-owner cannot regenerate invite code
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/"+bookID+"/invite-code", map[string]string{
		"role": "editor",
	}, userBH)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("non-owner regenerate invite code should return 403, got %d", rr.Code)
	}

	// Owner generates code2, then immediately regenerates code3 -> code2 must be revoked
	rr, regenResp2 := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/"+bookID+"/invite-code", map[string]string{
		"role": "viewer",
	}, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("regenerate code2 failed: %d %s", rr.Code, rr.Body.String())
	}
	code2 := regenResp2["data"].(map[string]interface{})["invite_code"].(string)

	rr, regenResp3 := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/"+bookID+"/invite-code", map[string]string{
		"role": "viewer",
	}, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("regenerate code3 failed: %d %s", rr.Code, rr.Body.String())
	}
	code3 := regenResp3["data"].(map[string]interface{})["invite_code"].(string)
	if code2 == code3 {
		t.Fatalf("expected distinct regenerated invite codes, got %q", code2)
	}

	// Old unused code2 must be revoked
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/join", map[string]string{
		"invite_code": code2,
	}, userCH)
	if rr.Code != http.StatusGone {
		t.Fatalf("revoked invite code2 should return 410, got %d", rr.Code)
	}

	// User C joins with code3 -> succeeds with viewer role
	rr, joinResp3 := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/join", map[string]string{
		"invite_code": code3,
	}, userCH)
	if rr.Code != http.StatusOK {
		t.Fatalf("user C join with code3 failed: %d %s", rr.Code, rr.Body.String())
	}
	if joinResp3["data"].(map[string]interface{})["role"] != "viewer" {
		t.Fatalf("expected viewer role from code3, got %v", joinResp3["data"].(map[string]interface{})["role"])
	}

	// Verify transaction list includes created_by_name and created_by_avatar
	wallets, _ := env.walletRepo.ListWallets(context.Background(), bookID, false)
	rr, txResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/transactions", map[string]interface{}{
		"book_id":   bookID,
		"wallet_id": wallets[0].ID,
		"type":      "expense",
		"amount":    1500,
		"title":     "奶茶",
	}, userBH)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create tx by member B failed: %d %s", rr.Code, rr.Body.String())
	}
	createdTx := txResp["data"].(map[string]interface{})
	if createdTx["created_by_name"] != "member_two" {
		t.Fatalf("expected created_by_name=member_two, got %v", createdTx["created_by_name"])
	}

	rr, listResp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/transactions?book_id="+bookID, nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("list tx failed: %d", rr.Code)
	}
	items := listResp["data"].(map[string]interface{})["items"].([]interface{})
	if len(items) == 0 || items[0].(map[string]interface{})["created_by_name"] != "member_two" {
		t.Fatalf("expected listed tx to include created_by_name=member_two, got %v", items)
	}
}

func TestRefreshTokenFlowAndRevocation(t *testing.T) {
	t.Setenv("STRICT_AUTH", "1")
	env := setupTestEnv(t)

	// Register a new user and verify both access token and 180-day refresh_token are issued
	_, codeResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/send-code", map[string]string{
		"email":   "nasowner@cat7.app",
		"purpose": "register",
	}, nil)
	code := codeResp["data"].(map[string]interface{})["demo_code"].(string)

	rr, regResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "nasowner",
		"name":     "NAS主人",
		"email":    "nasowner@cat7.app",
		"code":     code,
		"password": "password123",
	}, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", rr.Code, rr.Body.String())
	}
	regData := regResp["data"].(map[string]interface{})
	access1, _ := regData["token"].(string)
	refresh1, _ := regData["refresh_token"].(string)
	if !strings.HasPrefix(access1, "cat7_") || !strings.HasPrefix(refresh1, "cat7r_") {
		t.Fatalf("expected cat7_ and cat7r_ tokens, got access=%q refresh=%q", access1, refresh1)
	}
	if int64(regData["refresh_expires_in"].(float64)) != 180*24*3600 {
		t.Fatalf("expected 180-day refresh_expires_in, got %v", regData["refresh_expires_in"])
	}

	// Refresh using refresh_token -> returns valid new access token & sliding refresh token
	rr, refResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": refresh1,
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("refresh token failed: %d %s", rr.Code, rr.Body.String())
	}
	refData := refResp["data"].(map[string]interface{})
	access2, _ := refData["token"].(string)
	if !strings.HasPrefix(access2, "cat7_") {
		t.Fatalf("expected refreshed cat7_ token, got %q", access2)
	}

	// Verify refreshed access token works for protected endpoint
	rr, _ = doJSONReq(t, env.handler, http.MethodGet, "/api/v1/user/profile", nil, map[string]string{
		"Authorization": "Bearer " + access2,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("profile with refreshed token failed: %d", rr.Code)
	}

	// Logout revokes refresh_token -> subsequent refresh fails with 401
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/logout", map[string]string{
		"refresh_token": refresh1,
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("logout failed: %d", rr.Code)
	}
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": refresh1,
	}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after refresh token revocation, got %d %s", rr.Code, rr.Body.String())
	}

	// Legacy token upgrade: valid access token without refresh_token can still upgrade to get a refresh_token
	rr, upgResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/refresh", map[string]string{}, map[string]string{
		"Authorization": "Bearer " + access2,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("legacy access token refresh upgrade failed: %d %s", rr.Code, rr.Body.String())
	}
	if rt, _ := upgResp["data"].(map[string]interface{})["refresh_token"].(string); !strings.HasPrefix(rt, "cat7r_") {
		t.Fatalf("expected upgraded refresh_token, got %q", rt)
	}
}

func TestDefaultBookPersistenceAndCleanup(t *testing.T) {
	env := setupTestEnv(t)

	_, codeResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/send-code", map[string]string{
		"email":   "defbook@cat7.app",
		"purpose": "register",
	}, nil)
	code := codeResp["data"].(map[string]interface{})["demo_code"].(string)

	rr, regResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "defuser",
		"name":     "默认账本用户",
		"email":    "defbook@cat7.app",
		"code":     code,
		"password": "password123",
	}, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", rr.Code, rr.Body.String())
	}
	tok := regResp["data"].(map[string]interface{})["token"].(string)
	hdr := map[string]string{"Authorization": "Bearer " + tok}

	// Create second book
	rr, book2Resp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books", map[string]interface{}{
		"name":   "家庭共同账本",
		"budget": 800000,
	}, hdr)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create book 2 failed: %d %s", rr.Code, rr.Body.String())
	}
	book2ID := book2Resp["data"].(map[string]interface{})["id"].(string)

	// Set book 2 as default
	rr, setResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/"+book2ID+"/default", nil, hdr)
	if rr.Code != http.StatusOK {
		t.Fatalf("set default book failed: %d %s", rr.Code, rr.Body.String())
	}
	if isDef, _ := setResp["data"].(map[string]interface{})["is_default"].(bool); !isDef {
		t.Fatalf("expected is_default=true on set response, got %v", setResp["data"])
	}

	// Verify profile returns default_book_id and listBooks marks book 2 as is_default=true
	rr, profResp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/user/profile", nil, hdr)
	if rr.Code != http.StatusOK {
		t.Fatalf("get profile failed: %d", rr.Code)
	}
	if defID, _ := profResp["data"].(map[string]interface{})["default_book_id"].(string); defID != book2ID {
		t.Fatalf("expected profile default_book_id=%q, got %q", book2ID, defID)
	}

	rr, listResp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/books", nil, hdr)
	if rr.Code != http.StatusOK {
		t.Fatalf("list books failed: %d", rr.Code)
	}
	listItems := listResp["data"].([]interface{})
	defaultCount := 0
	for _, raw := range listItems {
		item := raw.(map[string]interface{})
		if isDef, _ := item["is_default"].(bool); isDef {
			defaultCount++
			if item["id"] != book2ID {
				t.Fatalf("unexpected default book id: %v", item["id"])
			}
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected 1 default book, got %d", defaultCount)
	}

	// Delete book 2 -> default_book_id should automatically clear
	rr, _ = doJSONReq(t, env.handler, http.MethodDelete, "/api/v1/books/"+book2ID, nil, hdr)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete book 2 failed: %d", rr.Code)
	}
	rr, profAfterDel := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/user/profile", nil, hdr)
	if defID, _ := profAfterDel["data"].(map[string]interface{})["default_book_id"].(string); defID != "" {
		t.Fatalf("expected cleared default_book_id after deleting book, got %q", defID)
	}

	// Set non-existent / unauthorized book as default -> 404
	rr, _ = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/nonexistent-id/default", nil, hdr)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown book default, got %d", rr.Code)
	}

	// Clear default endpoint works cleanly
	rr, _ = doJSONReq(t, env.handler, http.MethodDelete, "/api/v1/books/default", nil, hdr)
	if rr.Code != http.StatusOK {
		t.Fatalf("clear default book failed: %d", rr.Code)
	}
}

func TestAiCapabilitiesAndTranscribeAudio(t *testing.T) {
	env := setupTestEnv(t)

	// Mock OpenAI-compatible Whisper/STT server
	sttSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			http.Error(w, "bad multipart", http.StatusBadRequest)
			return
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing audio file", http.StatusBadRequest)
			return
		}
		f.Close()
		modelName := r.FormValue("model")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"text":"刚在星巴克买了拿铁花了 32 元，模型 %s 文件 %s"}`, modelName, hdr.Filename)
	}))
	defer sttSrv.Close()

	_, codeResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/send-code", map[string]string{
		"email":   "voice@cat7.app",
		"purpose": "register",
	}, nil)
	code := codeResp["data"].(map[string]interface{})["demo_code"].(string)

	rr, regResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "voiceuser",
		"name":     "语音测试用户",
		"email":    "voice@cat7.app",
		"code":     code,
		"password": "password123",
	}, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", rr.Code, rr.Body.String())
	}
	tok := regResp["data"].(map[string]interface{})["token"].(string)
	hdr := map[string]string{"Authorization": "Bearer " + tok}

	// Initially STT is not configured (default DeepSeek placeholder)
	rr, capResp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/ai/capabilities", nil, hdr)
	if rr.Code != http.StatusOK {
		t.Fatalf("get capabilities failed: %d", rr.Code)
	}
	if sttOK, _ := capResp["data"].(map[string]interface{})["stt_configured"].(bool); sttOK {
		t.Fatalf("expected stt_configured=false initially")
	}

	// Uploading audio before STT is configured returns 503 instead of fake text
	var preBody bytes.Buffer
	preMW := multipart.NewWriter(&preBody)
	part0, _ := preMW.CreateFormFile("file", "sample.webm")
	_, _ = part0.Write([]byte("fake-audio-bytes"))
	_ = preMW.Close()
	preReq := httptest.NewRequest(http.MethodPost, "/api/v1/ai/transcribe", &preBody)
	preReq.Header.Set("Content-Type", preMW.FormDataContentType())
	preReq.Header.Set("Authorization", "Bearer "+tok)
	preRR := httptest.NewRecorder()
	env.handler.ServeHTTP(preRR, preReq)
	if preRR.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when stt unconfigured, got %d %s", preRR.Code, preRR.Body.String())
	}

	// Save STT config pointing to mock Whisper server
	cfg, _ := env.settingsRepo.GetAiConfig(context.Background())
	cfg.SttBaseURL = sttSrv.URL + "/v1"
	cfg.SttAPIKey = "sk-real-stt-test"
	cfg.SttModel = "FunAudioLLM/SenseVoiceSmall"
	if err := env.settingsRepo.SaveAiConfig(context.Background(), cfg); err != nil {
		t.Fatalf("SaveAiConfig failed: %v", err)
	}

	rr, capResp2 := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/ai/capabilities", nil, hdr)
	if rr.Code != http.StatusOK {
		t.Fatalf("get capabilities 2 failed: %d", rr.Code)
	}
	if sttOK, _ := capResp2["data"].(map[string]interface{})["stt_configured"].(bool); !sttOK {
		t.Fatalf("expected stt_configured=true after configuration")
	}

	// Transcribe real uploaded audio bytes
	var audioBody bytes.Buffer
	mw := multipart.NewWriter(&audioBody)
	part, _ := mw.CreateFormFile("file", "record.webm")
	_, _ = part.Write([]byte("RIFF-AUDIO-PAYLOAD"))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/transcribe", &audioBody)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	trRR := httptest.NewRecorder()
	env.handler.ServeHTTP(trRR, req)
	if trRR.Code != http.StatusOK {
		t.Fatalf("transcribe failed: %d %s", trRR.Code, trRR.Body.String())
	}
	var trJSON map[string]interface{}
	_ = json.Unmarshal(trRR.Body.Bytes(), &trJSON)
	txt, _ := trJSON["data"].(map[string]interface{})["text"].(string)
	if !strings.Contains(txt, "星巴克") || !strings.Contains(txt, "FunAudioLLM/SenseVoiceSmall") {
		t.Fatalf("unexpected transcribe text: %q", txt)
	}

	// Test voice text polishing endpoint
	rr, polResp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/ai/polish", map[string]string{
		"text": "嗯...那个，今天午饭吃牛肉面花了 32 元",
	}, hdr)
	if rr.Code != http.StatusOK {
		t.Fatalf("polish failed: %d %s", rr.Code, rr.Body.String())
	}
	if pTxt, _ := polResp["data"].(map[string]interface{})["text"].(string); !strings.Contains(pTxt, "牛肉面") {
		t.Fatalf("unexpected polish text: %q", pTxt)
	}
}

func TestTransactionMemberFieldAndStats(t *testing.T) {
	env := setupTestEnv(t)
	ownerH := map[string]string{"Ugreen-User-ID": "u_owner", "Ugreen-User-Name": "小禾", "Ugreen-User-Type": "admin"}

	// Auto-seeded book + preset wallet/category
	rr, resp := doJSONReq(t, env.handler, http.MethodGet, "/api/v1/books", nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("list books failed: %d", rr.Code)
	}
	bookID := resp["data"].([]interface{})[0].(map[string]interface{})["id"].(string)
	wallets, _ := env.walletRepo.ListWallets(context.Background(), bookID, false)
	cats, _ := env.catRepo.ListCategories(context.Background(), bookID, "expense")
	var coffeeSubID, foodCatID string
	for _, c := range cats {
		if c.ParentID == nil && c.Name == "餐饮" {
			foodCatID = c.ID
		}
		if c.ParentID != nil && c.Name == "咖啡饮品" {
			coffeeSubID = c.ID
		}
	}
	if coffeeSubID == "" || foodCatID == "" {
		t.Fatalf("missing preset categories")
	}

	// Add a virtual member （家里猫咪）
	rr, resp = doJSONReq(t, env.handler, http.MethodPost, "/api/v1/books/"+bookID+"/virtual-members", map[string]string{
		"name":  "家里猫咪",
		"color": "#DE8A1E",
	}, ownerH)
	if rr.Code != http.StatusCreated {
		t.Fatalf("add virtual member failed: %d %s", rr.Code, rr.Body.String())
	}
	virtID := resp["data"].(map[string]interface{})["user_id"].(string)
	if !strings.HasPrefix(virtID, "virt_") {
		t.Fatalf("expected virt_ member id, got %q", virtID)
	}

	today := time.Now().Format("2006-01-02")
	createTx := func(memberID string, amount int, title string) (int, map[string]interface{}) {
		body := map[string]interface{}{
			"book_id": bookID, "type": "expense", "amount": amount, "date": today,
			"wallet_id": wallets[0].ID, "category_id": coffeeSubID, "title": title,
		}
		if memberID != "" {
			body["member_id"] = memberID
		}
		rr, resp := doJSONReq(t, env.handler, http.MethodPost, "/api/v1/transactions", body, ownerH)
		return rr.Code, resp
	}

	// 1. Owner creates a tx for the virtual member: member fields must follow member_id, not creator.
	code, resp := createTx(virtID, 3850, "猫咪的咖啡")
	if code != http.StatusCreated {
		t.Fatalf("create tx for virtual member failed: %d %s", code, rr.Body.String())
	}
	txVirt := resp["data"].(map[string]interface{})
	if txVirt["member_id"] != virtID || txVirt["member_name"] != "家里猫咪" {
		t.Fatalf("expected member=家里猫咪(%s), got %+v", virtID, txVirt)
	}
	if txVirt["created_by"] != "u_owner" {
		t.Fatalf("created_by must stay the creator, got %v", txVirt["created_by"])
	}
	virtTxID := txVirt["id"].(string)

	// 2. No member_id -> defaults to the creator.
	code, resp = createTx("", 10000, "我的午饭")
	if code != http.StatusCreated {
		t.Fatalf("create tx without member failed: %d", code)
	}
	txMine := resp["data"].(map[string]interface{})
	if txMine["member_id"] != "u_owner" || txMine["member_name"] != "小禾" {
		t.Fatalf("expected default member=小禾(u_owner), got %+v", txMine)
	}

	// 3. member_id of a non-member must be rejected.
	code, _ = createTx("u_stranger", 100, "陌生人")
	if code != http.StatusBadRequest {
		t.Fatalf("non-member member_id must return 400, got %d", code)
	}

	// 4. List filter by member_id returns only that member's tx.
	rr, resp = doJSONReq(t, env.handler, http.MethodGet,
		fmt.Sprintf("/api/v1/transactions?book_id=%s&member_id=%s", bookID, virtID), nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("list by member failed: %d", rr.Code)
	}
	items := resp["data"].(map[string]interface{})["items"].([]interface{})
	if len(items) != 1 || items[0].(map[string]interface{})["id"] != virtTxID {
		t.Fatalf("expected exactly the virtual member tx, got %+v", items)
	}

	// 5. Stats report groups by member (same month, expense).
	now := time.Now()
	rr, resp = doJSONReq(t, env.handler, http.MethodGet,
		fmt.Sprintf("/api/v1/reports/stats?book_id=%s&period=month&type=expense&year=%d&month=%d", bookID, now.Year(), int(now.Month())), nil, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("get stats failed: %d", rr.Code)
	}
	stats := resp["data"].(map[string]interface{})
	members := stats["members"].([]interface{})
	if len(members) != 2 {
		t.Fatalf("expected 2 members in stats, got %+v", members)
	}
	byID := map[string]map[string]interface{}{}
	for _, m := range members {
		mm := m.(map[string]interface{})
		byID[mm["member_id"].(string)] = mm
	}
	if int64(byID[virtID]["amount"].(float64)) != 3850 || byID[virtID]["member_name"] != "家里猫咪" {
		t.Fatalf("virtual member stats wrong: %+v", byID[virtID])
	}
	if int64(byID["u_owner"]["amount"].(float64)) != 10000 {
		t.Fatalf("owner stats wrong: %+v", byID["u_owner"])
	}
	if int(byID[virtID]["percentage"].(float64)) != 27 || int(byID["u_owner"]["percentage"].(float64)) != 72 {
		t.Fatalf("unexpected member percentages: %+v", members)
	}

	// 6. Update member_id moves the tx to the owner.
	rr, resp = doJSONReq(t, env.handler, http.MethodPut, "/api/v1/transactions/"+virtTxID,
		map[string]interface{}{"member_id": "u_owner"}, ownerH)
	if rr.Code != http.StatusOK {
		t.Fatalf("update member failed: %d %s", rr.Code, rr.Body.String())
	}
	updated := resp["data"].(map[string]interface{})
	if updated["member_id"] != "u_owner" || updated["member_name"] != "小禾" {
		t.Fatalf("expected member moved to owner, got %+v", updated)
	}

	// 7. Category usage counts: 咖啡饮品 used twice, 餐饮 roll-up is 2.
	cats, _ = env.catRepo.ListCategories(context.Background(), bookID, "expense")
	usage := map[string]int{}
	for _, c := range cats {
		usage[c.ID] = c.UsageCount
	}
	if usage[coffeeSubID] != 2 || usage[foodCatID] != 2 {
		t.Fatalf("expected usage 2/2 for 咖啡饮品/餐饮， got %d/%d", usage[coffeeSubID], usage[foodCatID])
	}
}
