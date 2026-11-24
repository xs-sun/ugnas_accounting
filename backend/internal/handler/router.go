package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type RouterConfig struct {
	AppID     string
	Version   string
	StaticDir string
}

type Router struct {
	cfg        RouterConfig
	authH      *AuthHandler
	userH      *UserHandler
	bookH      *BookHandler
	walletH    *WalletHandler
	categoryH  *CategoryHandler
	txH        *TransactionHandler
	reportH    *ReportHandler
	budgetH    *BudgetHandler
	recurringH *RecurringHandler
	aiH        *AiHandler
	adminH     *AdminHandler
	impExpH    *ImportExportHandler
	snapshotH  *SnapshotHandler
	streamH    *StreamHandler
	activityH  *ActivityHandler
	fileServer http.Handler
}

func NewRouter(
	cfg RouterConfig,
	authH *AuthHandler,
	userH *UserHandler,
	bookH *BookHandler,
	walletH *WalletHandler,
	categoryH *CategoryHandler,
	txH *TransactionHandler,
	reportH *ReportHandler,
	budgetH *BudgetHandler,
	recurringH *RecurringHandler,
	aiH *AiHandler,
	adminH *AdminHandler,
	impExpH *ImportExportHandler,
	snapshotH *SnapshotHandler,
	streamH *StreamHandler,
	activityH *ActivityHandler,
) *Router {
	return &Router{
		cfg:        cfg,
		authH:      authH,
		userH:      userH,
		bookH:      bookH,
		walletH:    walletH,
		categoryH:  categoryH,
		txH:        txH,
		reportH:    reportH,
		budgetH:    budgetH,
		recurringH: recurringH,
		aiH:        aiH,
		adminH:     adminH,
		impExpH:    impExpH,
		snapshotH:  snapshotH,
		streamH:    streamH,
		activityH:  activityH,
		fileServer: http.FileServer(http.Dir(cfg.StaticDir)),
	}
}

var (
	reBookMemberItem     = regexp.MustCompile(`^/api/v1/books/([^/]+)/members/([^/]+)$`)
	reBookMembers        = regexp.MustCompile(`^/api/v1/books/([^/]+)/members$`)
	reBookInviteEmail    = regexp.MustCompile(`^/api/v1/books/([^/]+)/invite-email$`)
	reBookVirtualMembers = regexp.MustCompile(`^/api/v1/books/([^/]+)/virtual-members$`)
	reBookActivities     = regexp.MustCompile(`^/api/v1/books/([^/]+)/activities$`)
	reBookItem           = regexp.MustCompile(`^/api/v1/books/([^/]+)$`)

	reWalletItem   = regexp.MustCompile(`^/api/v1/wallets/([^/]+)$`)
	reCategoryItem = regexp.MustCompile(`^/api/v1/categories/([^/]+)$`)
	reTagItem      = regexp.MustCompile(`^/api/v1/tags/([^/]+)$`)

	reTxAttachment = regexp.MustCompile(`^/api/v1/transactions/([^/]+)/attachment$`)
	reTxItem       = regexp.MustCompile(`^/api/v1/transactions/([^/]+)$`)

	reRecurringToggle = regexp.MustCompile(`^/api/v1/recurring/([^/]+)/toggle$`)
	reRecurringItem   = regexp.MustCompile(`^/api/v1/recurring/([^/]+)$`)

	reAdminUserItem = regexp.MustCompile(`^/api/v1/admin/users/([^/]+)$`)

	reSnapshotDownload = regexp.MustCompile(`^/api/v1/snapshots/([^/]+)/download$`)
	reSnapshotItem     = regexp.MustCompile(`^/api/v1/snapshots/([^/]+)$`)
)

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Global path traversal protection for API endpoints
	reqPathOnly := strings.SplitN(r.RequestURI, "?", 2)[0]
	combinedLower := strings.ToLower(reqPathOnly + "|" + r.URL.RawPath + "|" + r.URL.EscapedPath() + "|" + path)
	if strings.HasPrefix(path, "/api/") && (strings.Contains(combinedLower, "..") || strings.Contains(combinedLower, "%2e")) {
		sendError(w, http.StatusBadRequest, "invalid path: traversal not allowed")
		return
	}

	// 0. Swagger UI & OpenAPI Specification Documentation
	if path == "/api/swagger/doc.json" || path == "/swagger/doc.json" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(SwaggerDocJSON(rt.cfg.AppID, rt.cfg.Version))
		return
	}

	if path == "/swagger" || path == "/swagger/" || path == "/api/swagger" || path == "/api/swagger/" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(SwaggerUIHTML("/api/swagger/doc.json", "七喵记账 - API 交互式文档 (Swagger UI)")))
		return
	}

	// 1. Health & Auth & Avatars
	if path == "/api/v1/stream" {
		if r.Method == http.MethodGet {
			rt.streamH.Stream(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/health" || path == "/health" {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
			return
		}
		sendJSON(w, 0, http.StatusOK, "success", map[string]interface{}{
			"status":  "ok",
			"app_id":  rt.cfg.AppID,
			"version": rt.cfg.Version,
		})
		return
	}

	if path == "/api/v1/auth/status" {
		if r.Method == http.MethodGet {
			rt.authH.Status(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/auth/gateway" {
		if r.Method == http.MethodPost {
			rt.authH.GatewayLogin(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/auth/send-code" {
		if r.Method == http.MethodPost {
			rt.authH.SendCode(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/auth/register" {
		if r.Method == http.MethodPost {
			rt.authH.Register(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/auth/login" {
		if r.Method == http.MethodPost {
			rt.authH.Login(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/auth/forgot-password" {
		if r.Method == http.MethodPost {
			rt.authH.ForgotPassword(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if strings.HasPrefix(path, "/api/v1/avatars/") {
		if r.Method == http.MethodGet {
			rawName := strings.TrimPrefix(path, "/api/v1/avatars/")
			if strings.Contains(r.URL.RawPath, "..") || strings.Contains(r.URL.RawPath, "%2e") || strings.Contains(r.URL.RawPath, "%2E") {
				sendError(w, http.StatusBadRequest, "invalid avatar filename")
				return
			}
			rt.userH.ServeAvatar(w, r, rawName)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	// 2. User Profile APIs
	if path == "/api/v1/user/profile" || path == "/api/v1/user/me" {
		switch r.Method {
		case http.MethodGet:
			rt.userH.GetProfile(w, r)
		case http.MethodPut:
			rt.userH.UpdateProfile(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/user/bind-email" {
		if r.Method == http.MethodPost {
			rt.userH.BindEmail(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	// 3. Books APIs (exact routes before regex routes)
	if path == "/api/v1/books" {
		switch r.Method {
		case http.MethodGet:
			rt.bookH.ListBooks(w, r)
		case http.MethodPost:
			rt.bookH.CreateBook(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/books/join" {
		if r.Method == http.MethodPost {
			rt.bookH.JoinByCode(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if m := reBookMemberItem.FindStringSubmatch(path); len(m) > 2 {
		bookID, targetUser := m[1], m[2]
		if r.Method == http.MethodDelete {
			rt.bookH.RemoveMember(w, r, bookID, targetUser)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if m := reBookInviteEmail.FindStringSubmatch(path); len(m) > 1 {
		if r.Method == http.MethodPost {
			rt.bookH.InviteByEmail(w, r, m[1])
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if m := reBookVirtualMembers.FindStringSubmatch(path); len(m) > 1 {
		if r.Method == http.MethodPost {
			rt.bookH.AddVirtualMember(w, r, m[1])
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if m := reBookActivities.FindStringSubmatch(path); len(m) > 1 {
		if r.Method == http.MethodGet {
			rt.activityH.ListActivities(w, r, m[1])
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if m := reBookMembers.FindStringSubmatch(path); len(m) > 1 {
		bookID := m[1]
		switch r.Method {
		case http.MethodGet:
			rt.bookH.ListMembers(w, r, bookID)
		case http.MethodPost:
			rt.bookH.AddMember(w, r, bookID)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if m := reBookItem.FindStringSubmatch(path); len(m) > 1 {
		bookID := m[1]
		switch r.Method {
		case http.MethodGet:
			rt.bookH.GetBook(w, r, bookID)
		case http.MethodPut:
			rt.bookH.UpdateBook(w, r, bookID)
		case http.MethodDelete:
			rt.bookH.DeleteBook(w, r, bookID)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	// 4. Wallets APIs
	if path == "/api/v1/wallets/summary" {
		if r.Method == http.MethodGet {
			rt.walletH.GetSummary(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/wallets" {
		switch r.Method {
		case http.MethodGet:
			rt.walletH.ListWallets(w, r)
		case http.MethodPost:
			rt.walletH.CreateWallet(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if m := reWalletItem.FindStringSubmatch(path); len(m) > 1 {
		walletID := m[1]
		switch r.Method {
		case http.MethodPut:
			rt.walletH.UpdateWallet(w, r, walletID)
		case http.MethodDelete:
			rt.walletH.DeleteWallet(w, r, walletID)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	// 5. Categories & Tags APIs
	if path == "/api/v1/categories" {
		switch r.Method {
		case http.MethodGet:
			rt.categoryH.ListCategories(w, r)
		case http.MethodPost:
			rt.categoryH.CreateCategory(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if m := reCategoryItem.FindStringSubmatch(path); len(m) > 1 {
		catID := m[1]
		switch r.Method {
		case http.MethodPut:
			rt.categoryH.UpdateCategory(w, r, catID)
		case http.MethodDelete:
			rt.categoryH.DeleteCategory(w, r, catID)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/tags" {
		switch r.Method {
		case http.MethodGet:
			rt.categoryH.ListTags(w, r)
		case http.MethodPost:
			rt.categoryH.CreateTag(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if m := reTagItem.FindStringSubmatch(path); len(m) > 1 {
		tagID := m[1]
		if r.Method == http.MethodDelete {
			rt.categoryH.DeleteTag(w, r, tagID)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	// 6. Transactions APIs (exact routes before regex routes)
	if path == "/api/v1/transactions/batch-delete" {
		if r.Method == http.MethodPost {
			rt.txH.BatchDelete(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if m := reTxAttachment.FindStringSubmatch(path); len(m) > 1 {
		txID := m[1]
		if r.Method == http.MethodPost {
			rt.txH.UploadAttachment(w, r, txID)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/transactions" {
		switch r.Method {
		case http.MethodGet:
			rt.txH.ListTransactions(w, r)
		case http.MethodPost:
			rt.txH.CreateTransaction(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if m := reTxItem.FindStringSubmatch(path); len(m) > 1 {
		txID := m[1]
		switch r.Method {
		case http.MethodGet:
			rt.txH.GetTransaction(w, r, txID)
		case http.MethodPut:
			rt.txH.UpdateTransaction(w, r, txID)
		case http.MethodDelete:
			rt.txH.DeleteTransaction(w, r, txID)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	// 7. Budgets APIs
	if path == "/api/v1/budgets" {
		if r.Method == http.MethodGet {
			rt.budgetH.GetBudgets(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/budgets/total" {
		if r.Method == http.MethodPut {
			rt.budgetH.UpdateTotalBudget(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/budgets/category" {
		if r.Method == http.MethodPut {
			rt.budgetH.UpdateCategoryBudget(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	// 8. Recurring Rules APIs
	if path == "/api/v1/recurring" {
		switch r.Method {
		case http.MethodGet:
			rt.recurringH.ListRules(w, r)
		case http.MethodPost:
			rt.recurringH.CreateRule(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if m := reRecurringToggle.FindStringSubmatch(path); len(m) > 1 {
		if r.Method == http.MethodPost {
			rt.recurringH.ToggleRule(w, r, m[1])
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if m := reRecurringItem.FindStringSubmatch(path); len(m) > 1 {
		id := m[1]
		switch r.Method {
		case http.MethodPut:
			rt.recurringH.UpdateRule(w, r, id)
		case http.MethodDelete:
			rt.recurringH.DeleteRule(w, r, id)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	// 9. Reports APIs
	if path == "/api/v1/reports/stats" {
		if r.Method == http.MethodGet {
			rt.reportH.StatsReport(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/reports/monthly" {
		if r.Method == http.MethodGet {
			rt.reportH.MonthlyReport(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/reports/trend" {
		if r.Method == http.MethodGet {
			rt.reportH.TrendReport(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/reports/daily" {
		if r.Method == http.MethodGet {
			rt.reportH.DailyReport(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	// 10. AI APIs
	if path == "/api/v1/ai/chat" {
		if r.Method == http.MethodPost {
			rt.aiH.Chat(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/ai/ocr" {
		if r.Method == http.MethodPost {
			rt.aiH.OCR(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/ai/insights" {
		if r.Method == http.MethodGet {
			rt.aiH.Insights(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/ai/test" {
		if r.Method == http.MethodPost {
			rt.aiH.TestConnection(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	// 11. Admin APIs
	if path == "/api/v1/admin/overview" {
		if r.Method == http.MethodGet {
			rt.adminH.Overview(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/admin/ai-config" {
		switch r.Method {
		case http.MethodGet:
			rt.adminH.GetAiConfig(w, r)
		case http.MethodPut:
			rt.adminH.UpdateAiConfig(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/admin/ai-usage" {
		if r.Method == http.MethodGet {
			rt.adminH.AIUsage(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/admin/reg-config" {
		switch r.Method {
		case http.MethodGet:
			rt.adminH.GetRegConfig(w, r)
		case http.MethodPut:
			rt.adminH.UpdateRegConfig(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/admin/users" {
		if r.Method == http.MethodGet {
			rt.adminH.ListUsers(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if m := reAdminUserItem.FindStringSubmatch(path); len(m) > 1 {
		if r.Method == http.MethodPut {
			rt.adminH.UpdateUser(w, r, m[1])
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/admin/system-config" {
		switch r.Method {
		case http.MethodGet:
			rt.adminH.GetSystemConfig(w, r)
		case http.MethodPut:
			rt.adminH.UpdateSystemConfig(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}
	if path == "/api/v1/admin/smtp-test" {
		if r.Method == http.MethodPost {
			rt.adminH.TestSMTP(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	// 12. Import / Export APIs
	if path == "/api/v1/export" {
		if r.Method == http.MethodGet {
			rt.impExpH.ExportLedger(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/imports/wechat" {
		if r.Method == http.MethodPost {
			rt.impExpH.ImportWeChat(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/imports/alipay" {
		if r.Method == http.MethodPost {
			rt.impExpH.ImportAlipay(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/imports/commit" {
		if r.Method == http.MethodPost {
			rt.impExpH.CommitImport(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/imports/suishouji/parse" {
		if r.Method == http.MethodPost {
			rt.impExpH.ParseSuishouji(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/imports/suishouji/preview" {
		if r.Method == http.MethodPost {
			rt.impExpH.PreviewSuishouji(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/imports/suishouji/commit" {
		if r.Method == http.MethodPost {
			rt.impExpH.CommitSuishouji(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	// 13. Snapshots APIs
	if path == "/api/v1/snapshots" {
		if r.Method == http.MethodGet {
			rt.snapshotH.ListSnapshots(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/snapshots/create" {
		if r.Method == http.MethodPost {
			rt.snapshotH.CreateSnapshot(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if path == "/api/v1/snapshots/restore" {
		if r.Method == http.MethodPost {
			rt.snapshotH.RestoreSnapshot(w, r)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if m := reSnapshotDownload.FindStringSubmatch(path); len(m) > 1 {
		snapID := m[1]
		if r.Method == http.MethodGet {
			rt.snapshotH.DownloadSnapshot(w, r, snapID)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	if m := reSnapshotItem.FindStringSubmatch(path); len(m) > 1 {
		snapID := m[1]
		if r.Method == http.MethodDelete {
			rt.snapshotH.DeleteSnapshot(w, r, snapID)
		} else {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		}
		return
	}

	// 14. Unknown /api/ routes
	if strings.HasPrefix(path, "/api/") {
		sendError(w, http.StatusNotFound, fmt.Sprintf("not found: %s", path))
		return
	}

	// 15. Static files & SPA Fallback
	rt.serveStatic(w, r)
}

func (rt *Router) serveStatic(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat(rt.cfg.StaticDir); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>七喵记账</title></head>
<body style="font-family: -apple-system, sans-serif; background: #F6F7F8; color: #171A20; padding: 40px; text-align: center;">
  <div style="background: #FFFFFF; border-radius: 20px; max-width: 480px; margin: 40px auto; padding: 32px; border: 1px solid #EDEEF1; box-shadow: 0 10px 30px -4px rgba(23,26,32,0.05);">
    <h2 style="color: #DE8A1E; margin-top: 0;">七喵记账</h2>
    <p style="color: #5D6470; font-size: 14px;">` + rt.cfg.AppID + ` (v` + rt.cfg.Version + `)</p>
    <p style="margin: 20px 0;">后端服务运行正常。</p>
    <p><a href="/api/v1/health" style="color: #DE8A1E; text-decoration: none; font-weight: 500;">/api/v1/health</a></p>
  </div>
</body>
</html>`))
		return
	}

	cleanPath := filepath.Clean(r.URL.Path)
	if cleanPath == "/" {
		cleanPath = "/index.html"
	}
	targetPath := filepath.Join(rt.cfg.StaticDir, cleanPath)
	if fi, err := os.Stat(targetPath); err == nil && !fi.IsDir() {
		rt.fileServer.ServeHTTP(w, r)
		return
	}

	indexPath := filepath.Join(rt.cfg.StaticDir, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		http.ServeFile(w, r, indexPath)
		return
	}

	rt.fileServer.ServeHTTP(w, r)
}
