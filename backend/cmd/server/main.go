package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"com.xsguyu.accounting/backend/internal/handler"
	"com.xsguyu.accounting/backend/internal/repository"
	"com.xsguyu.accounting/backend/internal/service"
	_ "modernc.org/sqlite"
)

const (
	AppID   = "com.xsguyu.accounting"
	Version = "0.3.0"
)

func isDir(p string) bool {
	if p == "" {
		return false
	}
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func ensureWritableDir(targetDir, fallbackName string) string {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fallback := filepath.Join(os.TempDir(), AppID, fallbackName)
		log.Printf("[WARN] Cannot create directory '%s': %v, falling back to '%s'", targetDir, err, fallback)
		_ = os.MkdirAll(fallback, 0755)
		return fallback
	}

	testFile := filepath.Join(targetDir, fmt.Sprintf(".perm_test_%d", os.Getpid()))
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		fallback := filepath.Join(os.TempDir(), AppID, fallbackName)
		log.Printf("[WARN] Directory '%s' is not writable: %v, falling back to '%s'", targetDir, err, fallback)
		_ = os.MkdirAll(fallback, 0755)
		return fallback
	}
	_ = os.Remove(testFile)
	return targetDir
}

func resolveAppPaths() (appRoot, dataDir, logDir, staticDir string) {
	if execPath, err := os.Executable(); err == nil {
		if evalPath, err := filepath.EvalSymlinks(execPath); err == nil {
			execPath = evalPath
		}
		parent := filepath.Dir(execPath)
		if filepath.Base(parent) == "bin" {
			appRoot = filepath.Dir(parent)
		} else {
			appRoot = parent
		}
	}
	if appRoot == "" {
		appRoot = "."
	}

	volumeDir := filepath.Dir(filepath.Dir(appRoot))
	ugDataCandidate := filepath.Join(volumeDir, "@appdata", AppID)
	ugLogCandidate := filepath.Join(volumeDir, "@applog", AppID)

	if envData := os.Getenv("UGAPP_DATA_DIR"); envData != "" {
		dataDir = envData
	} else if isDir(ugDataCandidate) {
		dataDir = ugDataCandidate
	} else if isDir(filepath.Join(appRoot, "data")) {
		dataDir = filepath.Join(appRoot, "data")
	} else if isDir("./data") {
		dataDir = "./data"
	} else if isDir(filepath.Join(volumeDir, "@appdata")) {
		dataDir = ugDataCandidate
	} else {
		dataDir = filepath.Join(appRoot, "data")
	}

	if envLog := os.Getenv("UGAPP_LOG_DIR"); envLog != "" {
		logDir = envLog
	} else if isDir(ugLogCandidate) {
		logDir = ugLogCandidate
	} else if isDir(filepath.Join(appRoot, "log")) {
		logDir = filepath.Join(appRoot, "log")
	} else if isDir(filepath.Join(appRoot, "logs")) {
		logDir = filepath.Join(appRoot, "logs")
	} else if isDir("./log") {
		logDir = "./log"
	} else if isDir("./logs") {
		logDir = "./logs"
	} else if isDir(filepath.Join(volumeDir, "@applog")) {
		logDir = ugLogCandidate
	} else {
		logDir = filepath.Join(appRoot, "log")
	}

	if envStatic := os.Getenv("STATIC_DIR"); envStatic != "" && isDir(envStatic) {
		staticDir = envStatic
	} else if installDir := os.Getenv("UGAPP_INSTALL_DIR"); installDir != "" && isDir(filepath.Join(installDir, "www")) {
		staticDir = filepath.Join(installDir, "www")
	} else if isDir(filepath.Join(appRoot, "www")) {
		staticDir = filepath.Join(appRoot, "www")
	} else if isDir("rootfs_common/www") {
		staticDir = "rootfs_common/www"
	} else if isDir("../rootfs_common/www") {
		staticDir = "../rootfs_common/www"
	} else if isDir("frontend/dist") {
		staticDir = "frontend/dist"
	} else {
		staticDir = filepath.Join(appRoot, "www")
	}

	return appRoot, dataDir, logDir, staticDir
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	log.SetOutput(io.MultiWriter(os.Stdout, os.Stderr))

	var port int
	flag.IntVar(&port, "port", 21010, "Port for server to listen on (default 21010)")
	flag.Parse()

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && port == 21010 {
			port = p
		}
	}

	appRoot, dataDir, logDir, staticDir := resolveAppPaths()

	if appRoot != "" && appRoot != "." {
		_ = os.Chdir(appRoot)
	}

	dataDir = ensureWritableDir(dataDir, "data")
	logDir = ensureWritableDir(logDir, "log")

	writers := []io.Writer{os.Stdout, os.Stderr}
	logFilePath := filepath.Join(logDir, "accounting.log")
	if logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		writers = append(writers, logFile)
	}
	ugLogFilePath := filepath.Join(logDir, AppID+".log")
	if ugLogFile, err := os.OpenFile(ugLogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		writers = append(writers, ugLogFile)
	}
	log.SetOutput(io.MultiWriter(writers...))

	log.Printf("[INFO] Starting %s (v%s)", AppID, Version)
	log.Printf("[INFO] App root: %s", appRoot)
	log.Printf("[INFO] Data directory: %s", dataDir)
	log.Printf("[INFO] Log directory: %s", logDir)
	log.Printf("[INFO] Static frontend directory: %s", staticDir)
	log.Printf("[INFO] Target port: %d", port)

	db, err := repository.InitDB(dataDir)
	if err != nil {
		log.Fatalf("[FATAL] Database initialization failed: %v", err)
	}
	defer db.Close()

	// Initialize Repositories
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

	// Initialize Services
	snapshotService := service.NewSnapshotService(db, snapshotRepo, dataDir)
	importService := service.NewImportService(db, txRepo, catRepo, walletRepo)
	exportService := service.NewExportService(db)
	authService := service.NewAuthService(userRepo, settingsRepo)
	aiService := service.NewAiService(bookRepo, catRepo, walletRepo, txRepo, budgetRepo, userRepo, settingsRepo)
	// 本地 OCR（RapidOCR）：程序 + 模型由 build.sh 打进安装包；找不到时
	// OCR 自动走远程视觉，不影响启动。
	if ocrProvider := service.ResolveRapidOCRProvider(); ocrProvider != nil {
		aiService.SetOCRProvider(ocrProvider)
		log.Printf("[INFO] 本地 OCR 已启用: %s", ocrProvider.BinaryPath())
	} else {
		log.Printf("[INFO] 本地 OCR 不可用（未找到 RapidOCR 程序或模型），OCR 将走远程视觉")
	}
	// SSE 实时同步的进程内订阅中心（单进程常驻，即时直推，无外部组件）。
	hub := service.NewHub()
	activityRepo := repository.NewActivityRepo(db)
	activityRecorder := service.NewActivityRecorder(activityRepo, budgetRepo, hub)
	recurringScheduler := service.NewRecurringScheduler(recurringRepo, txRepo, hub, activityRecorder)
	budgetAlertScheduler := service.NewBudgetAlertScheduler(activityRecorder)
	autoBackupScheduler := service.NewAutoBackupScheduler(snapshotService, settingsRepo)

	// Initialize Handlers
	authHandler := handler.NewAuthHandler(userRepo, bookRepo, walletRepo, settingsRepo, authService)
	userHandler := handler.NewUserHandler(bookRepo, walletRepo, txRepo, userRepo, settingsRepo, dataDir)
	bookHandler := handler.NewBookHandler(bookRepo, userRepo, authService, hub, activityRecorder)
	walletHandler := handler.NewWalletHandler(walletRepo, bookRepo, hub, activityRecorder)
	catHandler := handler.NewCategoryHandler(catRepo, hub, activityRecorder)
	txHandler := handler.NewTransactionHandler(txRepo, bookRepo, walletRepo, dataDir, hub, activityRecorder)
	reportHandler := handler.NewReportHandler(txRepo)
	budgetHandler := handler.NewBudgetHandler(budgetRepo, bookRepo, hub, activityRecorder)
	recurringHandler := handler.NewRecurringHandler(recurringRepo, hub, activityRecorder)
	aiHandler := handler.NewAiHandler(aiService)
	aiChatHandler := handler.NewAiChatHandler(aiChatRepo, bookRepo, dataDir)
	adminHandler := handler.NewAdminHandler(settingsRepo, userRepo, authService)
	impExpHandler := handler.NewImportExportHandler(importService, exportService, hub, activityRecorder, aiService, bookRepo)
	snapshotHandler := handler.NewSnapshotHandler(snapshotService)
	streamHandler := handler.NewStreamHandler(hub, bookRepo, userRepo, authService)
	activityHandler := handler.NewActivityHandler(activityRepo, bookRepo)

	routerConfig := handler.RouterConfig{
		AppID:     AppID,
		Version:   Version,
		StaticDir: staticDir,
	}

	router := handler.NewRouter(
		routerConfig,
		authHandler,
		userHandler,
		bookHandler,
		walletHandler,
		catHandler,
		txHandler,
		reportHandler,
		budgetHandler,
		recurringHandler,
		aiHandler,
		adminHandler,
		impExpHandler,
		snapshotHandler,
		streamHandler,
		activityHandler,
		aiChatHandler,
	)

	authMiddleware := handler.NewAuthMiddleware(userRepo, authService)
	handlerChain := handler.CORSMiddleware(authMiddleware(router))

	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handlerChain,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[FATAL] Failed to bind TCP listener on %s: %v", addr, err)
	}
	log.Printf("[INFO] Server listening on http://0.0.0.0:%d", port)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start background schedulers
	recurringScheduler.Start(ctx)
	budgetAlertScheduler.Start(ctx)
	autoBackupScheduler.Start(ctx)

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[FATAL] Server HTTP listener failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("[INFO] Received shutdown signal, initiating graceful shutdown...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Server forced to shutdown: %v", err)
	} else {
		log.Println("[INFO] Server terminated gracefully.")
	}
}
