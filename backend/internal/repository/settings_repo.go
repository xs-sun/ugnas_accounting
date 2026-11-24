package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"github.com/google/uuid"
)

type SettingsRepo struct {
	db      *DB
	dataDir string
}

func NewSettingsRepo(db *DB, dataDir string) *SettingsRepo {
	return &SettingsRepo{db: db, dataDir: dataDir}
}

func (r *SettingsRepo) GetSetting(ctx context.Context, key string) (string, error) {
	var val string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM system_settings WHERE key = ?`, key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return val, err
}

func (r *SettingsRepo) SetSetting(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO system_settings (key, value, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = datetime('now')
	`, key, value)
	return err
}

func (r *SettingsRepo) GetOrCreateHMACSecret(ctx context.Context) string {
	val, err := r.GetSetting(ctx, "auth_hmac_secret")
	if err == nil && val != "" {
		return val
	}
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	secret := hex.EncodeToString(buf)
	_ = r.SetSetting(ctx, "auth_hmac_secret", secret)
	return secret
}

func (r *SettingsRepo) GetAiConfig(ctx context.Context) (*model.AiConfig, error) {
	cfg := &model.AiConfig{
		Enabled:         true,
		Provider:        "DeepSeek",
		BaseURL:         "https://api.deepseek.com/v1",
		Model:           "deepseek-chat",
		DailyLimit:      30,
		Temperature:     0.3,
		OcrEnabled:      true,
		InsightsEnabled: true,
	}
	raw, err := r.GetSetting(ctx, "ai_config")
	if err == nil && raw != "" {
		_ = json.Unmarshal([]byte(raw), cfg)
	}
	cfg.HasAPIKey = strings.TrimSpace(cfg.APIKey) != ""
	return cfg, nil
}

func (r *SettingsRepo) SaveAiConfig(ctx context.Context, cfg *model.AiConfig) error {
	cfg.HasAPIKey = strings.TrimSpace(cfg.APIKey) != ""
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return r.SetSetting(ctx, "ai_config", string(data))
}

func (r *SettingsRepo) GetRegConfig(ctx context.Context) (*model.RegConfig, error) {
	cfg := &model.RegConfig{
		AllowRegister:       true,
		RequireInviteCode:   false,
		RequireEmailVerify:  false,
		InviteCode:          "CAT7-NAS-2026",
		DefaultDailyAiLimit: 30,
	}
	raw, err := r.GetSetting(ctx, "reg_config")
	if err == nil && raw != "" {
		_ = json.Unmarshal([]byte(raw), cfg)
	}
	if cfg.DefaultDailyAiLimit <= 0 {
		cfg.DefaultDailyAiLimit = 30
	}
	return cfg, nil
}

func (r *SettingsRepo) SaveRegConfig(ctx context.Context, cfg *model.RegConfig) error {
	if cfg.DefaultDailyAiLimit <= 0 {
		cfg.DefaultDailyAiLimit = 30
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return r.SetSetting(ctx, "reg_config", string(data))
}

func (r *SettingsRepo) GetSystemConfig(ctx context.Context) (*model.SystemConfig, error) {
	cfg := &model.SystemConfig{
		AutoBackupEnabled: false,
		AutoBackupCycle:   "每天 03:00",
		AutoBackupRetain:  14,
		BackupPath:        r.dataDir + "/snapshots",
		SmtpPort:          465,
		SmtpTLS:           true,
		SiteName:          "七喵记账",
	}
	raw, err := r.GetSetting(ctx, "system_config")
	if err == nil && raw != "" {
		_ = json.Unmarshal([]byte(raw), cfg)
	}
	if cfg.AutoBackupRetain <= 0 {
		cfg.AutoBackupRetain = 14
	}
	if cfg.BackupPath == "" {
		cfg.BackupPath = r.dataDir + "/snapshots"
	}
	if cfg.SiteName == "" {
		cfg.SiteName = "七喵记账"
	}
	cfg.HasSmtpPass = strings.TrimSpace(cfg.SmtpPass) != ""
	return cfg, nil
}

func (r *SettingsRepo) SaveSystemConfig(ctx context.Context, cfg *model.SystemConfig) error {
	if cfg.AutoBackupRetain <= 0 {
		cfg.AutoBackupRetain = 14
	}
	cfg.HasSmtpPass = strings.TrimSpace(cfg.SmtpPass) != ""
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return r.SetSetting(ctx, "system_config", string(data))
}

func (r *SettingsRepo) RecordAIUsage(ctx context.Context, userID, action, modelName string, inputTokens, outputTokens int64) error {
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ai_usage_logs (id, user_id, action, model, input_tokens, output_tokens, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "ai_"+uuid.NewString()[:8], userID, action, modelName, inputTokens, outputTokens, nowStr)
	return err
}

func (r *SettingsRepo) GetTodayAIUsageCount(ctx context.Context, userID string) int {
	todayPrefix := time.Now().Format("2006-01-02") + "%"
	var cnt int
	_ = r.db.QueryRowContext(ctx, `
		SELECT count(*) FROM ai_usage_logs WHERE user_id = ? AND created_at LIKE ?
	`, userID, todayPrefix).Scan(&cnt)
	return cnt
}

func (r *SettingsRepo) GetAdminUsers(ctx context.Context, search string) ([]model.AdminUserItem, error) {
	todayPrefix := time.Now().Format("2006-01-02") + "%"
	q := `
		SELECT u.id, u.name, COALESCE(u.email, ''), COALESCE(u.avatar, '/avatars/cat-1.jpg'),
		       u.role, u.is_active, u.daily_ai_limit, u.last_active_at
		FROM users u
		WHERE 1=1
	`
	var args []interface{}
	if strings.TrimSpace(search) != "" {
		kw := "%" + strings.TrimSpace(search) + "%"
		q += ` AND (u.name LIKE ? OR COALESCE(u.email, '') LIKE ? OR u.id LIKE ?)`
		args = append(args, kw, kw, kw)
	}
	q += ` ORDER BY u.created_at ASC`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	users := make([]model.AdminUserItem, 0)
	for rows.Next() {
		var it model.AdminUserItem
		var isAct int
		if err := rows.Scan(&it.ID, &it.Name, &it.Email, &it.Avatar, &it.Role, &isAct, &it.DailyAiLimit, &it.LastActiveAt); err != nil {
			rows.Close()
			return nil, err
		}
		it.IsActive = isAct == 1
		users = append(users, it)
	}
	rows.Close()

	for i := range users {
		u := &users[i]
		_ = r.db.QueryRowContext(ctx, `
			SELECT count(DISTINCT b.id) FROM books b
			LEFT JOIN book_members bm ON b.id = bm.book_id
			WHERE b.owner_id = ? OR bm.user_id = ?
		`, u.ID, u.ID).Scan(&u.BooksCount)

		_ = r.db.QueryRowContext(ctx, `SELECT count(*) FROM transactions WHERE created_by = ?`, u.ID).Scan(&u.TxCount)
		_ = r.db.QueryRowContext(ctx, `SELECT count(*) FROM ai_usage_logs WHERE user_id = ? AND created_at LIKE ?`, u.ID, todayPrefix).Scan(&u.TodayAiCount)
		_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0) FROM ai_usage_logs WHERE user_id = ?`, u.ID).Scan(&u.InputTokens, &u.OutputTokens)
	}

	return users, nil
}

func (r *SettingsRepo) GetAdminOverview(ctx context.Context) (*model.AdminOverview, error) {
	users, err := r.GetAdminUsers(ctx, "")
	if err != nil {
		return nil, err
	}
	aiCfg, _ := r.GetAiConfig(ctx)

	now := time.Now()
	todayPrefix := now.Format("2006-01-02") + "%"
	sevenDaysAgo := now.AddDate(0, 0, -6).Format("2006-01-02") + " 00:00:00"

	ov := &model.AdminOverview{
		TotalUsers:    len(users),
		AiProvider:    aiCfg.Provider,
		AiModel:       aiCfg.Model,
		DailyActivity: make([]model.AdminDailyActivity, 0, 7),
		UserStats:     users,
	}

	for _, u := range users {
		if u.IsActive {
			ov.ActiveUsers++
		}
		if u.LastActiveAt >= sevenDaysAgo {
			ov.ActiveUsers7d++
		}
	}

	_ = r.db.QueryRowContext(ctx, `SELECT count(*) FROM books`).Scan(&ov.TotalBooks)
	_ = r.db.QueryRowContext(ctx, `SELECT count(*) FROM transactions`).Scan(&ov.TotalTransactionsCount)
	_ = r.db.QueryRowContext(ctx, `SELECT count(*) FROM ai_usage_logs WHERE created_at LIKE ?`, todayPrefix).Scan(&ov.TodayAiCalls)
	_ = r.db.QueryRowContext(ctx, `SELECT count(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0) FROM ai_usage_logs`).Scan(&ov.TotalAiCalls, &ov.TotalInputTokens, &ov.TotalOutputTokens)
	ov.TotalTokens = ov.TotalInputTokens + ov.TotalOutputTokens

	for offset := -6; offset <= 0; offset++ {
		d := now.AddDate(0, 0, offset)
		ds := d.Format("2006-01-02")
		lbl := fmt.Sprintf("%d日", d.Day())
		var txCnt, aiCnt int
		_ = r.db.QueryRowContext(ctx, `SELECT count(*) FROM transactions WHERE date = ?`, ds).Scan(&txCnt)
		_ = r.db.QueryRowContext(ctx, `SELECT count(*) FROM ai_usage_logs WHERE created_at LIKE ?`, ds+"%").Scan(&aiCnt)
		ov.DailyActivity = append(ov.DailyActivity, model.AdminDailyActivity{
			Date:    ds,
			Label:   lbl,
			TxCount: txCnt,
			AiCalls: aiCnt,
		})
	}

	return ov, nil
}

func (r *SettingsRepo) GetAdminAIUsage(ctx context.Context, search string) (totalCalls, todayCalls int, totalIn, totalOut int64, users []model.AdminUserItem, logs []model.AiUsageLog, err error) {
	users, err = r.GetAdminUsers(ctx, search)
	if err != nil {
		return
	}
	todayPrefix := time.Now().Format("2006-01-02") + "%"
	_ = r.db.QueryRowContext(ctx, `SELECT count(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0) FROM ai_usage_logs`).Scan(&totalCalls, &totalIn, &totalOut)
	_ = r.db.QueryRowContext(ctx, `SELECT count(*) FROM ai_usage_logs WHERE created_at LIKE ?`, todayPrefix).Scan(&todayCalls)

	q := `
		SELECT l.id, l.user_id, COALESCE(u.name, l.user_id), l.action, l.model, l.input_tokens, l.output_tokens, l.created_at
		FROM ai_usage_logs l
		LEFT JOIN users u ON l.user_id = u.id
		WHERE 1=1
	`
	var args []interface{}
	if strings.TrimSpace(search) != "" {
		kw := "%" + strings.TrimSpace(search) + "%"
		q += ` AND (COALESCE(u.name, '') LIKE ? OR COALESCE(u.email, '') LIKE ? OR l.user_id LIKE ?)`
		args = append(args, kw, kw, kw)
	}
	q += ` ORDER BY l.created_at DESC LIMIT 50`

	rows, qErr := r.db.QueryContext(ctx, q, args...)
	if qErr != nil {
		err = qErr
		return
	}
	defer rows.Close()

	logs = make([]model.AiUsageLog, 0)
	for rows.Next() {
		var item model.AiUsageLog
		if sErr := rows.Scan(&item.ID, &item.UserID, &item.UserName, &item.Action, &item.Model, &item.InputTokens, &item.OutputTokens, &item.CreatedAt); sErr == nil {
			logs = append(logs, item)
		}
	}
	return
}
