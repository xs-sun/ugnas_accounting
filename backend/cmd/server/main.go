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
	Version = "0.1.7"
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
	// 1. Detect base app directory from executable location
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

	// Detect volume directory if installed under /volumeX/@appstore/com.xsguyu.accounting
	volumeDir := filepath.Dir(filepath.Dir(appRoot))
	ugDataCandidate := filepath.Join(volumeDir, "@appdata", AppID)
	ugLogCandidate := filepath.Join(volumeDir, "@applog", AppID)

	// 2. Resolve Data Directory
	// UGOS Pro standard: /volume*/@appdata/<appId>
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

	// 3. Resolve Log Directory
	// UGOS Pro standard: /volume*/@applog/<appId>
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

	// 4. Resolve Static Directory
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
	// Immediately initialize logging to stdout and stderr so no panic or startup message is lost
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

	// Switch working directory to appRoot if valid
	if appRoot != "" && appRoot != "." {
		_ = os.Chdir(appRoot)
	}

	// Ensure directories are writable with graceful fallback to /tmp
	dataDir = ensureWritableDir(dataDir, "data")
	logDir = ensureWritableDir(logDir, "log")

	// Setup multi-logging (stdout + stderr + log files)
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

	// Initialize SQLite Database with WAL, mmap, and pre-agg triggers
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

	// Initialize Services
	snapshotService := service.NewSnapshotService(db, snapshotRepo, dataDir)
	importService := service.NewImportService(db)
	exportService := service.NewExportService(db)

	// Initialize Handlers
	userHandler := handler.NewUserHandler(bookRepo, walletRepo, txRepo)
	bookHandler := handler.NewBookHandler(bookRepo)
	walletHandler := handler.NewWalletHandler(walletRepo, bookRepo)
	catHandler := handler.NewCategoryHandler(catRepo)
	txHandler := handler.NewTransactionHandler(txRepo, bookRepo, walletRepo, dataDir)
	reportHandler := handler.NewReportHandler(txRepo)
	impExpHandler := handler.NewImportExportHandler(importService, exportService)
	snapshotHandler := handler.NewSnapshotHandler(snapshotService)

	routerConfig := handler.RouterConfig{
		AppID:     AppID,
		Version:   Version,
		StaticDir: staticDir,
	}

	router := handler.NewRouter(
		routerConfig,
		userHandler,
		bookHandler,
		walletHandler,
		catHandler,
		txHandler,
		reportHandler,
		impExpHandler,
		snapshotHandler,
	)

	// Chain middlewares: CORS -> Auth -> Router
	handlerChain := handler.CORSMiddleware(handler.AuthMiddleware(router))

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
