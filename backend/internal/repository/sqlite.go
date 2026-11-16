package repository

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	Path string
}

func InitDB(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("[WARN] MkdirAll dataDir '%s': %v", dataDir, err)
	}

	testFile := filepath.Join(dataDir, fmt.Sprintf(".perm_test_%d", os.Getpid()))
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		fallbackDir := filepath.Join(os.TempDir(), "com.xsguyu.accounting", "data")
		log.Printf("[WARN] dataDir '%s' is not writable: %v, falling back to '%s'", dataDir, err, fallbackDir)
		_ = os.MkdirAll(fallbackDir, 0755)
		dataDir = fallbackDir
	} else {
		_ = os.Remove(testFile)
	}

	dbPath := filepath.Join(dataDir, "accounting.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Connection pool settings for SQLite
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA mmap_size = 268435456;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA temp_store = MEMORY;",
	}

	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			log.Printf("[WARN] Failed to execute pragma '%s': %v", p, err)
		}
	}

	schema := `
	CREATE TABLE IF NOT EXISTS books (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		currency TEXT NOT NULL DEFAULT 'CNY',
		description TEXT DEFAULT '',
		owner_id TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS book_members (
		book_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		user_name TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'editor',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		PRIMARY KEY (book_id, user_id),
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS wallets (
		id TEXT PRIMARY KEY,
		book_id TEXT NOT NULL,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		balance INTEGER NOT NULL DEFAULT 0,
		credit_limit INTEGER NOT NULL DEFAULT 0,
		billing_day INTEGER DEFAULT 1,
		repayment_day INTEGER DEFAULT 20,
		card_last_four TEXT DEFAULT '',
		exclude_net_asset INTEGER NOT NULL DEFAULT 0,
		is_archived INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS categories (
		id TEXT PRIMARY KEY,
		book_id TEXT NOT NULL,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		icon TEXT NOT NULL DEFAULT 'tag',
		color TEXT NOT NULL DEFAULT '#0E8C7F',
		parent_id TEXT DEFAULT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE,
		FOREIGN KEY (parent_id) REFERENCES categories(id) ON DELETE SET NULL
	);

	CREATE TABLE IF NOT EXISTS tags (
		id TEXT PRIMARY KEY,
		book_id TEXT NOT NULL,
		name TEXT NOT NULL,
		color TEXT NOT NULL DEFAULT '#5D6470',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS transactions (
		id TEXT PRIMARY KEY,
		book_id TEXT NOT NULL,
		type TEXT NOT NULL,
		amount INTEGER NOT NULL,
		date TEXT NOT NULL,
		wallet_id TEXT NOT NULL,
		to_wallet_id TEXT DEFAULT NULL,
		category_id TEXT DEFAULT NULL,
		note TEXT DEFAULT '',
		order_no TEXT DEFAULT '',
		created_by TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE,
		FOREIGN KEY (wallet_id) REFERENCES wallets(id) ON DELETE RESTRICT,
		FOREIGN KEY (to_wallet_id) REFERENCES wallets(id) ON DELETE RESTRICT,
		FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE SET NULL
	);

	CREATE TABLE IF NOT EXISTS transaction_tags (
		transaction_id TEXT NOT NULL,
		tag_id TEXT NOT NULL,
		PRIMARY KEY (transaction_id, tag_id),
		FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE CASCADE,
		FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS attachments (
		id TEXT PRIMARY KEY,
		transaction_id TEXT NOT NULL,
		file_name TEXT NOT NULL,
		file_path TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		mime_type TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS daily_summaries (
		date TEXT NOT NULL,
		book_id TEXT NOT NULL,
		type TEXT NOT NULL,
		category_id TEXT NOT NULL,
		total_amount INTEGER NOT NULL DEFAULT 0,
		count INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (date, book_id, type, category_id)
	);

	CREATE INDEX IF NOT EXISTS idx_daily_summaries_query 
	ON daily_summaries (book_id, type, date, category_id, total_amount, count);

	CREATE INDEX IF NOT EXISTS idx_trans_book_date ON transactions(book_id, date);
	CREATE INDEX IF NOT EXISTS idx_trans_wallet ON transactions(wallet_id);
	CREATE INDEX IF NOT EXISTS idx_wallets_book ON wallets(book_id);
	CREATE INDEX IF NOT EXISTS idx_categories_book ON categories(book_id);

	CREATE TABLE IF NOT EXISTS snapshots (
		id TEXT PRIMARY KEY,
		file_name TEXT NOT NULL,
		file_path TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		sha256 TEXT NOT NULL,
		created_by TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	-- Triggers for daily_summaries
	CREATE TRIGGER IF NOT EXISTS trg_tx_insert_summary
	AFTER INSERT ON transactions
	WHEN NEW.type IN ('expense', 'income')
	BEGIN
		INSERT INTO daily_summaries (date, book_id, type, category_id, total_amount, count)
		VALUES (NEW.date, NEW.book_id, NEW.type, COALESCE(NEW.category_id, 'uncategorized'), NEW.amount, 1)
		ON CONFLICT(date, book_id, type, category_id) DO UPDATE SET
			total_amount = total_amount + NEW.amount,
			count = count + 1;
	END;

	CREATE TRIGGER IF NOT EXISTS trg_tx_delete_summary
	AFTER DELETE ON transactions
	WHEN OLD.type IN ('expense', 'income')
	BEGIN
		UPDATE daily_summaries
		SET total_amount = total_amount - OLD.amount,
			count = count - 1
		WHERE date = OLD.date AND book_id = OLD.book_id AND type = OLD.type 
		  AND category_id = COALESCE(OLD.category_id, 'uncategorized');
	END;
	`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Printf("[INFO] Database initialized at %s with WAL + mmap + triggers", dbPath)
	return &DB{DB: db, Path: dbPath}, nil
}
