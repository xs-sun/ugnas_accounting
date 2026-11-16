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
	userH      *UserHandler
	bookH      *BookHandler
	walletH    *WalletHandler
	categoryH  *CategoryHandler
	txH        *TransactionHandler
	reportH    *ReportHandler
	impExpH    *ImportExportHandler
	snapshotH  *SnapshotHandler
	fileServer http.Handler
}

func NewRouter(
	cfg RouterConfig,
	userH *UserHandler,
	bookH *BookHandler,
	walletH *WalletHandler,
	categoryH *CategoryHandler,
	txH *TransactionHandler,
	reportH *ReportHandler,
	impExpH *ImportExportHandler,
	snapshotH *SnapshotHandler,
) *Router {
	return &Router{
		cfg:        cfg,
		userH:      userH,
		bookH:      bookH,
		walletH:    walletH,
		categoryH:  categoryH,
		txH:        txH,
		reportH:    reportH,
		impExpH:    impExpH,
		snapshotH:  snapshotH,
		fileServer: http.FileServer(http.Dir(cfg.StaticDir)),
	}
}

// Regex patterns for parameterized paths
var (
	reBookMemberItem = regexp.MustCompile(`^/api/v1/books/([^/]+)/members/([^/]+)$`)
	reBookMembers    = regexp.MustCompile(`^/api/v1/books/([^/]+)/members$`)
	reBookItem       = regexp.MustCompile(`^/api/v1/books/([^/]+)$`)

	reWalletItem = regexp.MustCompile(`^/api/v1/wallets/([^/]+)$`)

	reCategoryItem = regexp.MustCompile(`^/api/v1/categories/([^/]+)$`)
	reTagItem      = regexp.MustCompile(`^/api/v1/tags/([^/]+)$`)

	reTxAttachment = regexp.MustCompile(`^/api/v1/transactions/([^/]+)/attachment$`)
	reTxItem       = regexp.MustCompile(`^/api/v1/transactions/([^/]+)$`)

	reSnapshotDownload = regexp.MustCompile(`^/api/v1/snapshots/([^/]+)/download$`)
	reSnapshotItem     = regexp.MustCompile(`^/api/v1/snapshots/([^/]+)$`)
)

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

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

	// 1. Health check & User profile
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

	if path == "/api/v1/user/profile" || path == "/api/v1/user/me" {
		if r.Method != http.MethodGet {
			sendError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
			return
		}
		rt.userH.GetProfile(w, r)
		return
	}

	// 2. Books APIs
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

	if m := reBookMemberItem.FindStringSubmatch(path); len(m) > 2 {
		bookID, targetUser := m[1], m[2]
		if r.Method == http.MethodDelete {
			rt.bookH.RemoveMember(w, r, bookID, targetUser)
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

	// 3. Wallets APIs
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

	// 4. Categories & Tags APIs
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

	// 5. Transactions APIs
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

	// 6. Reports APIs
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

	// 7. Import / Export APIs
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

	// 8. Snapshots APIs
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

	// 9. Unknown /api/ routes
	if strings.HasPrefix(path, "/api/") {
		sendError(w, http.StatusNotFound, fmt.Sprintf("not found: %s", path))
		return
	}

	// 10. Static files & SPA Fallback
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
    <h2 style="color: #0E8C7F; margin-top: 0;">七喵记账</h2>
    <p style="color: #5D6470; font-size: 14px;">` + rt.cfg.AppID + ` (v` + rt.cfg.Version + `)</p>
    <p style="margin: 20px 0;">后端服务运行正常。</p>
    <p><a href="/api/v1/health" style="color: #0E8C7F; text-decoration: none; font-weight: 500;">/api/v1/health</a></p>
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

	// SPA Fallback: serve index.html
	indexPath := filepath.Join(rt.cfg.StaticDir, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		http.ServeFile(w, r, indexPath)
		return
	}

	rt.fileServer.ServeHTTP(w, r)
}
