package repository

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	Path string
}

func ensureColumn(db *sql.DB, table, col, ddl string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s);", table))
	if err != nil {
		return fmt.Errorf("pragma table_info(%s): %w", table, err)
	}
	defer rows.Close()

	var (
		cid     int
		name    string
		colType string
		notNull int
		dflt    sql.NullString
		pk      int
	)
	for rows.Next() {
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan table_info(%s): %w", table, err)
		}
		if name == col {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", table, col, ddl)
	if _, err := db.Exec(alterSQL); err != nil {
		return fmt.Errorf("alter table %s add %s: %w", table, col, err)
	}
	return nil
}

func randomInviteToken8() string {
	const alphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	out := make([]byte, 8)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			out[i] = alphabet[i%len(alphabet)]
			continue
		}
		out[i] = alphabet[n.Int64()]
	}
	return "CAT7-" + string(out)
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
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
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

	// Idempotent column migrations for existing 10 tables
	columnMigrations := []struct {
		table string
		col   string
		ddl   string
	}{
		{"books", "color", "TEXT NOT NULL DEFAULT '#DE8A1E'"},
		{"books", "budget", "INTEGER NOT NULL DEFAULT 450000"},
		{"books", "invite_code", "TEXT NOT NULL DEFAULT ''"},

		{"book_members", "avatar", "TEXT DEFAULT ''"},
		{"book_members", "color", "TEXT DEFAULT '#DE8A1E'"},
		{"book_members", "is_virtual", "INTEGER NOT NULL DEFAULT 0"},
		{"book_members", "status", "TEXT NOT NULL DEFAULT 'active'"},

		{"wallets", "group_name", "TEXT DEFAULT '电子钱包'"},
		{"wallets", "color", "TEXT DEFAULT '#DE8A1E'"},

		{"categories", "bg_color", "TEXT DEFAULT '#FFF6E8'"},
		{"categories", "is_custom", "INTEGER NOT NULL DEFAULT 0"},
		{"categories", "sort_order", "INTEGER NOT NULL DEFAULT 0"},

		{"transactions", "title", "TEXT DEFAULT ''"},
		{"transactions", "sub_category", "TEXT DEFAULT ''"},
		{"transactions", "tx_time", "TEXT DEFAULT ''"},
		{"transactions", "include_budget", "INTEGER NOT NULL DEFAULT 1"},
		{"transactions", "reimburse", "INTEGER NOT NULL DEFAULT 0"},
	}

	for _, m := range columnMigrations {
		if err := ensureColumn(db, m.table, m.col, m.ddl); err != nil {
			return nil, fmt.Errorf("column migration %s.%s failed: %w", m.table, m.col, err)
		}
	}

	// New tables for v0.2.0
	newTablesSchema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT DEFAULT '',
		name TEXT NOT NULL,
		email TEXT UNIQUE,
		password_hash TEXT DEFAULT '',
		avatar TEXT DEFAULT '/avatars/cat-1.jpg',
		role TEXT NOT NULL DEFAULT 'user',
		email_bound INTEGER NOT NULL DEFAULT 0,
		gateway_uid TEXT DEFAULT '',
		daily_ai_limit INTEGER NOT NULL DEFAULT 30,
		is_active INTEGER NOT NULL DEFAULT 1,
		last_active_at TEXT NOT NULL DEFAULT (datetime('now')),
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS email_codes (
		id TEXT PRIMARY KEY,
		email TEXT NOT NULL,
		code TEXT NOT NULL,
		purpose TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		used INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS category_budgets (
		book_id TEXT NOT NULL,
		category_id TEXT NOT NULL,
		limit_amount INTEGER NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		PRIMARY KEY (book_id, category_id),
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS recurring_rules (
		id TEXT PRIMARY KEY,
		book_id TEXT NOT NULL,
		type TEXT NOT NULL,
		category_id TEXT NOT NULL,
		sub_category TEXT DEFAULT '',
		title TEXT NOT NULL,
		amount INTEGER NOT NULL,
		cycle TEXT NOT NULL,
		start_date TEXT NOT NULL,
		end_rule TEXT DEFAULT '不结束',
		next_date TEXT NOT NULL,
		executed_count INTEGER NOT NULL DEFAULT 0,
		wallet_id TEXT NOT NULL,
		exclude_stats INTEGER NOT NULL DEFAULT 0,
		is_active INTEGER NOT NULL DEFAULT 1,
		created_by TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS ai_usage_logs (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		action TEXT NOT NULL,
		model TEXT DEFAULT '',
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS system_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	-- 账本动态：谁、在哪个账本、干了什么。随账本删除级联清理。
	CREATE TABLE IF NOT EXISTS book_activities (
		id TEXT PRIMARY KEY,
		book_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		actor_name TEXT NOT NULL DEFAULT '',
		action TEXT NOT NULL,
		summary TEXT NOT NULL DEFAULT '',
		amount_cents INTEGER DEFAULT NULL,
		tx_id TEXT DEFAULT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
	);

	-- 预算提醒已通知档位：同一账本、同一预算项、同一月份、同一档位只提醒一次。
	-- 用量回落到档位之下时删除对应行，下次重新越线会再次提醒。
	CREATE TABLE IF NOT EXISTS budget_alert_state (
		book_id TEXT NOT NULL,
		budget_key TEXT NOT NULL,
		month TEXT NOT NULL,
		level INTEGER NOT NULL,
		notified_at TEXT NOT NULL DEFAULT (datetime('now')),
		PRIMARY KEY (book_id, budget_key, month, level),
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS book_invite_codes (
		code TEXT PRIMARY KEY,
		book_id TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'editor',
		created_by TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		expires_at TEXT NOT NULL,
		used_by TEXT NOT NULL DEFAULT '',
		used_at TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS refresh_tokens (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		token_hash TEXT NOT NULL UNIQUE,
		expires_at INTEGER NOT NULL,
		last_used_at INTEGER NOT NULL,
		revoked_at INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_activities_book_created ON book_activities(book_id, created_at DESC, id DESC);
	CREATE INDEX IF NOT EXISTS idx_book_invite_codes_book ON book_invite_codes(book_id, used_at);
	CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id, revoked_at);
	CREATE INDEX IF NOT EXISTS idx_email_codes_lookup ON email_codes(email, purpose, used);
	CREATE INDEX IF NOT EXISTS idx_recurring_book_next ON recurring_rules(book_id, is_active, next_date);
	CREATE INDEX IF NOT EXISTS idx_ai_usage_user_date ON ai_usage_logs(user_id, created_at);
	CREATE INDEX IF NOT EXISTS idx_books_invite_code ON books(invite_code);
	`
	if _, err := db.Exec(newTablesSchema); err != nil {
		return nil, fmt.Errorf("failed to initialize v0.2.0 tables: %w", err)
	}

	userMigrations := []struct {
		col string
		ddl string
	}{
		{"username", "TEXT DEFAULT ''"},
		{"gateway_uid", "TEXT DEFAULT ''"},
		{"default_book_id", "TEXT DEFAULT ''"},
	}
	for _, m := range userMigrations {
		if err := ensureColumn(db, "users", m.col, m.ddl); err != nil {
			return nil, fmt.Errorf("column migration users.%s failed: %w", m.col, err)
		}
	}

	userIndexSQL := `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_unique
		ON users(LOWER(username)) WHERE username IS NOT NULL AND username != '';
	CREATE UNIQUE INDEX IF NOT EXISTS idx_users_gateway_uid_unique
		ON users(gateway_uid) WHERE gateway_uid IS NOT NULL AND gateway_uid != '';
	`
	if _, err := db.Exec(userIndexSQL); err != nil {
		return nil, fmt.Errorf("failed to create user unique indexes: %w", err)
	}

	// Backfill invite codes for historical books that lack one
	if err := backfillBookInviteCodes(db); err != nil {
		log.Printf("[WARN] backfill invite codes: %v", err)
	}

	log.Printf("[INFO] Database initialized at %s with WAL + mmap + triggers + v0.2.0 schema", dbPath)
	return &DB{DB: db, Path: dbPath}, nil
}

func backfillBookInviteCodes(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, owner_id, COALESCE(invite_code, '') FROM books`)
	if err != nil {
		return err
	}
	type bookRow struct {
		id         string
		ownerID    string
		inviteCode string
	}
	var books []bookRow
	for rows.Next() {
		var b bookRow
		if err := rows.Scan(&b.id, &b.ownerID, &b.inviteCode); err == nil {
			books = append(books, b)
		}
	}
	rows.Close()

	for _, b := range books {
		code := b.inviteCode
		if code == "" {
			for attempt := 0; attempt < 20; attempt++ {
				candidate := randomInviteToken8()
				var cnt int
				_ = db.QueryRow(`SELECT count(*) FROM book_invite_codes WHERE UPPER(code) = UPPER(?)`, candidate).Scan(&cnt)
				if cnt == 0 {
					code = candidate
					break
				}
			}
			if code == "" {
				continue
			}
			_, _ = db.Exec(`UPDATE books SET invite_code = ? WHERE id = ?`, code, b.id)
		}
		var existingCnt int
		_ = db.QueryRow(`SELECT count(*) FROM book_invite_codes WHERE book_id = ?`, b.id).Scan(&existingCnt)
		if existingCnt == 0 && code != "" {
			_, _ = db.Exec(`
				INSERT OR IGNORE INTO book_invite_codes (code, book_id, role, created_by, expires_at)
				VALUES (UPPER(?), ?, 'editor', ?, datetime('now', '+30 days'))
			`, code, b.id, b.ownerID)
		}
	}
	return nil
}
