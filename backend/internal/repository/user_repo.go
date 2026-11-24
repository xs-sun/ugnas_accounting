package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"github.com/google/uuid"
)

type UserRepo struct {
	db *DB
}

func NewUserRepo(db *DB) *UserRepo {
	return &UserRepo{db: db}
}

func nowLocalStr() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

const userSelectColumns = `
	id, COALESCE(username, ''), name, email, COALESCE(password_hash, ''),
	COALESCE(avatar, '/avatars/cat-1.jpg'), role, email_bound,
	COALESCE(gateway_uid, ''), COALESCE(default_book_id, ''), daily_ai_limit, is_active, last_active_at, created_at
`

func (r *UserRepo) CountUsers(ctx context.Context) (int, error) {
	var cnt int
	err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&cnt)
	return cnt, err
}

func (r *UserRepo) CountRegisteredUsers(ctx context.Context) (int, error) {
	var cnt int
	err := r.db.QueryRowContext(ctx, `
		SELECT count(*) FROM users
		WHERE email_bound = 1 AND COALESCE(password_hash, '') != ''
	`).Scan(&cnt)
	return cnt, err
}

func (r *UserRepo) UpsertUserFromContext(ctx context.Context, uCtx *model.UserContext) (*model.User, error) {
	if uCtx == nil || uCtx.ID == "" {
		return nil, errors.New("missing user id")
	}
	u, err := r.GetUserByGatewayUID(ctx, uCtx.ID)
	nowStr := nowLocalStr()
	if err == nil && u != nil {
		_, _ = r.db.ExecContext(ctx, `UPDATE users SET last_active_at = ? WHERE id = ?`, nowStr, u.ID)
		u.LastActiveAt = nowStr
		return u, nil
	}

	role := "user"
	if uCtx.Type == "admin" || uCtx.Role == "admin" {
		role = "admin"
	}
	name := strings.TrimSpace(uCtx.Name)
	if name == "" {
		name = uCtx.ID
	}
	avatar := "/avatars/cat-1.jpg"
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO users (id, username, name, avatar, role, email_bound, gateway_uid, daily_ai_limit, is_active, last_active_at, created_at)
		VALUES (?, '', ?, ?, ?, 0, ?, 30, 1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET last_active_at = excluded.last_active_at
	`, uCtx.ID, name, avatar, role, uCtx.ID, nowStr, nowStr)
	if err != nil {
		return nil, err
	}
	return r.GetUserByID(ctx, uCtx.ID)
}

func (r *UserRepo) CreateUser(ctx context.Context, u *model.User) error {
	if u.ID == "" {
		u.ID = "u_" + uuid.NewString()[:8]
	}
	if u.Avatar == "" {
		u.Avatar = "/avatars/cat-1.jpg"
	}
	if u.Role == "" {
		u.Role = "user"
	}
	if u.DailyAiLimit <= 0 {
		u.DailyAiLimit = 30
	}
	nowStr := nowLocalStr()
	emailBoundInt := 0
	if u.EmailBound {
		emailBoundInt = 1
	}
	isActiveInt := 1
	if !u.IsActive {
		isActiveInt = 0
	}
	var emailVal sql.NullString
	if strings.TrimSpace(u.Email) != "" {
		emailVal = sql.NullString{String: strings.ToLower(strings.TrimSpace(u.Email)), Valid: true}
	}
	username := strings.TrimSpace(u.Username)
	gatewayUID := strings.TrimSpace(u.GatewayUID)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, username, name, email, password_hash, avatar, role, email_bound, gateway_uid, daily_ai_limit, is_active, last_active_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, u.ID, username, u.Name, emailVal, u.PasswordHash, u.Avatar, u.Role, emailBoundInt, gatewayUID, u.DailyAiLimit, isActiveInt, nowStr, nowStr)
	return err
}

func scanUserRow(row interface {
	Scan(dest ...interface{}) error
}) (*model.User, error) {
	var u model.User
	var email sql.NullString
	var emailBoundInt, isActiveInt int
	err := row.Scan(
		&u.ID, &u.Username, &u.Name, &email, &u.PasswordHash, &u.Avatar, &u.Role,
		&emailBoundInt, &u.GatewayUID, &u.DefaultBookID, &u.DailyAiLimit, &isActiveInt, &u.LastActiveAt, &u.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if email.Valid {
		u.Email = email.String
	}
	u.EmailBound = emailBoundInt == 1
	u.IsActive = isActiveInt == 1
	return &u, nil
}

func (r *UserRepo) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+userSelectColumns+` FROM users WHERE id = ?`, id)
	return scanUserRow(row)
}

func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	norm := strings.ToLower(strings.TrimSpace(email))
	row := r.db.QueryRowContext(ctx, `SELECT `+userSelectColumns+` FROM users WHERE LOWER(email) = ?`, norm)
	return scanUserRow(row)
}

func (r *UserRepo) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	norm := strings.ToLower(strings.TrimSpace(username))
	if norm == "" {
		return nil, ErrNotFound
	}
	row := r.db.QueryRowContext(ctx, `SELECT `+userSelectColumns+` FROM users WHERE LOWER(username) = ? AND username != ''`, norm)
	return scanUserRow(row)
}

func (r *UserRepo) GetUserByLoginIdentifier(ctx context.Context, identifier string) (*model.User, error) {
	norm := strings.ToLower(strings.TrimSpace(identifier))
	if norm == "" {
		return nil, ErrNotFound
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+userSelectColumns+`
		FROM users
		WHERE LOWER(email) = ? OR (COALESCE(username, '') != '' AND LOWER(username) = ?)
		LIMIT 1
	`, norm, norm)
	return scanUserRow(row)
}

func (r *UserRepo) GetUserByGatewayUID(ctx context.Context, gwUID string) (*model.User, error) {
	norm := strings.TrimSpace(gwUID)
	if norm == "" {
		return nil, ErrNotFound
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+userSelectColumns+`
		FROM users
		WHERE gateway_uid = ? OR id = ?
		ORDER BY CASE WHEN gateway_uid = ? THEN 0 ELSE 1 END
		LIMIT 1
	`, norm, norm, norm)
	return scanUserRow(row)
}

func (r *UserRepo) CompleteGatewayUserRegistration(
	ctx context.Context,
	existingUID, gatewayUID, username, name, email, passwordHash, avatar string,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	normEmail := strings.ToLower(strings.TrimSpace(email))
	nowStr := nowLocalStr()
	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET username = ?, name = ?, email = ?, password_hash = ?, avatar = ?,
		    email_bound = 1, gateway_uid = ?, last_active_at = ?
		WHERE id = ?
	`, strings.TrimSpace(username), strings.TrimSpace(name), normEmail, passwordHash, strings.TrimSpace(avatar), strings.TrimSpace(gatewayUID), nowStr, existingUID); err != nil {
		return err
	}
	_, _ = tx.ExecContext(ctx, `UPDATE book_members SET user_name = ?, avatar = ? WHERE user_id = ? AND is_virtual = 0`, strings.TrimSpace(name), strings.TrimSpace(avatar), existingUID)
	return tx.Commit()
}

func (r *UserRepo) TouchLastActive(ctx context.Context, id string) {
	_, _ = r.db.ExecContext(ctx, `UPDATE users SET last_active_at = ? WHERE id = ?`, nowLocalStr(), id)
}

func (r *UserRepo) UpdateUserProfile(ctx context.Context, id string, name, avatar *string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if name != nil && strings.TrimSpace(*name) != "" {
		cleanName := strings.TrimSpace(*name)
		if _, err := tx.ExecContext(ctx, `UPDATE users SET name = ? WHERE id = ?`, cleanName, id); err != nil {
			return err
		}
		_, _ = tx.ExecContext(ctx, `UPDATE book_members SET user_name = ? WHERE user_id = ? AND is_virtual = 0`, cleanName, id)
	}
	if avatar != nil && strings.TrimSpace(*avatar) != "" {
		cleanAv := strings.TrimSpace(*avatar)
		if _, err := tx.ExecContext(ctx, `UPDATE users SET avatar = ? WHERE id = ?`, cleanAv, id); err != nil {
			return err
		}
		_, _ = tx.ExecContext(ctx, `UPDATE book_members SET avatar = ? WHERE user_id = ? AND is_virtual = 0`, cleanAv, id)
	}
	return tx.Commit()
}

func (r *UserRepo) BindUserEmail(ctx context.Context, id, email string) error {
	norm := strings.ToLower(strings.TrimSpace(email))
	_, err := r.db.ExecContext(ctx, `UPDATE users SET email = ?, email_bound = 1 WHERE id = ?`, norm, id)
	return err
}

func (r *UserRepo) UpdateUserPassword(ctx context.Context, id, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id)
	return err
}

func (r *UserRepo) SetDefaultBook(ctx context.Context, userID, bookID string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE users SET default_book_id = ? WHERE id = ?`, strings.TrimSpace(bookID), userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepo) ClearDefaultBookByBookID(ctx context.Context, bookID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET default_book_id = '' WHERE default_book_id = ?`, bookID)
	return err
}

func (r *UserRepo) AdminUpdateUser(ctx context.Context, id string, role *string, dailyLimit *int, isActive *bool) error {
	u, err := r.GetUserByID(ctx, id)
	if err != nil {
		return err
	}
	if role != nil && (*role == "admin" || *role == "user") {
		u.Role = *role
	}
	if dailyLimit != nil && *dailyLimit >= 0 {
		u.DailyAiLimit = *dailyLimit
	}
	if isActive != nil {
		u.IsActive = *isActive
	}
	activeInt := 0
	if u.IsActive {
		activeInt = 1
	}
	res, err := r.db.ExecContext(ctx, `UPDATE users SET role = ?, daily_ai_limit = ?, is_active = ? WHERE id = ?`, u.Role, u.DailyAiLimit, activeInt, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepo) CreateEmailCode(ctx context.Context, email, code, purpose string, ttl time.Duration) error {
	norm := strings.ToLower(strings.TrimSpace(email))
	now := time.Now()
	expires := now.Add(ttl).Format("2006-01-02 15:04:05")
	created := now.Format("2006-01-02 15:04:05")
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO email_codes (id, email, code, purpose, expires_at, used, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?)
	`, uuid.NewString(), norm, code, purpose, expires, created)
	return err
}

func (r *UserRepo) VerifyAndConsumeEmailCode(ctx context.Context, email, code, purpose string) (bool, error) {
	norm := strings.ToLower(strings.TrimSpace(email))
	var id, expiresAt string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, expires_at FROM email_codes
		WHERE LOWER(email) = ? AND code = ? AND purpose = ? AND used = 0
		ORDER BY created_at DESC LIMIT 1
	`, norm, strings.TrimSpace(code), purpose).Scan(&id, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	expT, parseErr := time.ParseInLocation("2006-01-02 15:04:05", expiresAt, time.Now().Location())
	if parseErr != nil || time.Now().After(expT) {
		return false, nil
	}
	_, _ = r.db.ExecContext(ctx, `UPDATE email_codes SET used = 1 WHERE id = ?`, id)
	return true, nil
}

func (r *UserRepo) ComputeUserProfileMetrics(ctx context.Context, userID string, bookIDs []string) (streakDays, recordedDays, totalTx int, healthScore int, healthLabel string, savingsRate int) {
	if len(bookIDs) == 0 {
		return 0, 0, 0, 60, "健康良好", 0
	}

	placeholders := make([]string, len(bookIDs))
	args := make([]interface{}, len(bookIDs))
	for i, bid := range bookIDs {
		placeholders[i] = "?"
		args[i] = bid
	}
	inClause := strings.Join(placeholders, ",")

	_ = r.db.QueryRowContext(ctx, `SELECT count(*) FROM transactions WHERE book_id IN (`+inClause+`)`, args...).Scan(&totalTx)

	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT date FROM transactions WHERE book_id IN (`+inClause+`) ORDER BY date DESC`, args...)
	if err == nil {
		var dates []string
		for rows.Next() {
			var d string
			if err := rows.Scan(&d); err == nil && d != "" {
				dates = append(dates, d)
			}
		}
		rows.Close()
		recordedDays = len(dates)
		if len(dates) > 0 {
			dateSet := make(map[string]bool, len(dates))
			for _, d := range dates {
				dateSet[d] = true
			}
			cur := time.Now()
			if !dateSet[cur.Format("2006-01-02")] {
				cur = cur.AddDate(0, 0, -1)
			}
			for dateSet[cur.Format("2006-01-02")] {
				streakDays++
				cur = cur.AddDate(0, 0, -1)
			}
			if streakDays == 0 && recordedDays > 0 {
				// Fallback consecutive count from the latest recorded transaction date
				if latest, err := time.Parse("2006-01-02", dates[0]); err == nil {
					for dateSet[latest.Format("2006-01-02")] {
						streakDays++
						latest = latest.AddDate(0, 0, -1)
					}
				}
			}
		}
	}

	monthPrefix := time.Now().Format("2006-01")
	sumArgs := append(append([]interface{}{}, args...), monthPrefix+"%")
	var monthInc, monthExp int64
	sumRows, err := r.db.QueryContext(ctx, `
		SELECT type, COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE book_id IN (`+inClause+`) AND date LIKE ? AND type IN ('expense', 'income')
		GROUP BY type
	`, sumArgs...)
	if err == nil {
		for sumRows.Next() {
			var t string
			var amt int64
			if err := sumRows.Scan(&t, &amt); err == nil {
				if t == "income" {
					monthInc = amt
				} else if t == "expense" {
					monthExp = amt
				}
			}
		}
		sumRows.Close()
	}

	if monthInc > 0 && monthInc > monthExp {
		savingsRate = int(((monthInc - monthExp) * 100) / monthInc)
	}

	var totalBudget int64
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(budget), 0) FROM books WHERE id IN (`+inClause+`)`, args...).Scan(&totalBudget)

	score := 65
	if streakDays >= 7 {
		score += 12
	} else if streakDays >= 3 {
		score += 6
	}
	if savingsRate >= 30 {
		score += 15
	} else if savingsRate >= 10 {
		score += 8
	}
	if totalBudget > 0 {
		if monthExp <= totalBudget {
			score += 8
		} else {
			score -= 10
		}
	}
	if score > 99 {
		score = 99
	}
	if score < 40 {
		score = 40
	}
	healthScore = score
	switch {
	case score >= 85:
		healthLabel = "健康优秀"
	case score >= 70:
		healthLabel = "健康良好"
	case score >= 55:
		healthLabel = "尚可改善"
	default:
		healthLabel = "需要关注"
	}

	return streakDays, recordedDays, totalTx, healthScore, healthLabel, savingsRate
}

type RefreshTokenRecord struct {
	ID         string
	UserID     string
	TokenHash  string
	ExpiresAt  int64
	LastUsedAt int64
	RevokedAt  int64
	CreatedAt  int64
}

func (r *UserRepo) CreateRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt int64) error {
	now := time.Now().Unix()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, last_used_at, revoked_at, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?)
	`, uuid.NewString(), userID, tokenHash, expiresAt, now, now)
	return err
}

func (r *UserRepo) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
	var rec RefreshTokenRecord
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, last_used_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = ?
		LIMIT 1
	`, tokenHash).Scan(&rec.ID, &rec.UserID, &rec.TokenHash, &rec.ExpiresAt, &rec.LastUsedAt, &rec.RevokedAt, &rec.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *UserRepo) ExtendRefreshToken(ctx context.Context, id string, newExpiresAt int64) error {
	now := time.Now().Unix()
	_, err := r.db.ExecContext(ctx, `
		UPDATE refresh_tokens
		SET expires_at = ?, last_used_at = ?
		WHERE id = ? AND revoked_at = 0
	`, newExpiresAt, now, id)
	return err
}

func (r *UserRepo) RevokeRefreshTokenByHash(ctx context.Context, tokenHash string) error {
	now := time.Now().Unix()
	_, err := r.db.ExecContext(ctx, `
		UPDATE refresh_tokens SET revoked_at = ? WHERE token_hash = ? AND revoked_at = 0
	`, now, tokenHash)
	return err
}

func (r *UserRepo) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	now := time.Now().Unix()
	_, err := r.db.ExecContext(ctx, `
		UPDATE refresh_tokens SET revoked_at = ? WHERE user_id = ? AND revoked_at = 0
	`, now, userID)
	return err
}
