package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

const (
	// DefaultAccessTokenTTL Access Token 有效期 30 天
	DefaultAccessTokenTTL = 30 * 24 * time.Hour
	// DefaultRefreshTokenTTL Refresh Token 有效期 180 天（半年），每次刷新滑动续期
	DefaultRefreshTokenTTL = 180 * 24 * time.Hour
)

type AuthService struct {
	userRepo     *repository.UserRepo
	settingsRepo *repository.SettingsRepo
	hmacSecret   []byte
}

func NewAuthService(userRepo *repository.UserRepo, settingsRepo *repository.SettingsRepo) *AuthService {
	secret := settingsRepo.GetOrCreateHMACSecret(context.Background())
	return &AuthService{
		userRepo:     userRepo,
		settingsRepo: settingsRepo,
		hmacSecret:   []byte(secret),
	}
}

func (s *AuthService) HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	digest := deriveHash([]byte(password), salt)
	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(digest), nil
}

func (s *AuthService) VerifyPassword(password, stored string) bool {
	parts := strings.SplitN(stored, "$", 2)
	if len(parts) != 2 {
		return false
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	actual := deriveHash([]byte(password), salt)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func deriveHash(password, salt []byte) []byte {
	cur := append(append([]byte{}, salt...), password...)
	sum := sha256.Sum256(cur)
	for i := 0; i < 10000; i++ {
		h := sha256.New()
		h.Write(sum[:])
		h.Write(salt)
		copy(sum[:], h.Sum(nil))
	}
	return sum[:]
}

type tokenClaims struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
	Exp    int64  `json:"exp"`
}

// GatewayBindClaims stores HMAC-signed temporary gateway authorization context for registration completion.
type GatewayBindClaims struct {
	GatewayUID  string `json:"gw_uid"`
	GatewayName string `json:"gw_name"`
	GatewayType string `json:"gw_type"`
	ExistingUID string `json:"existing_uid,omitempty"`
	Exp         int64  `json:"exp"`
}

func (s *AuthService) issueSignedPayload(prefix string, payload interface{}) (string, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sig := s.signPayload(prefix + payloadEnc)
	return prefix + payloadEnc + "." + sig, nil
}

func (s *AuthService) verifySignedPayload(tokenStr, prefix string, out interface{}) error {
	if !strings.HasPrefix(tokenStr, prefix) {
		return errors.New("unexpected token prefix")
	}
	raw := strings.TrimPrefix(tokenStr, prefix)
	parts := strings.SplitN(raw, ".", 2)
	if len(parts) != 2 {
		return errors.New("malformed token")
	}
	expectedSig := s.signPayload(prefix + parts[0])
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return errors.New("invalid signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return err
	}
	return json.Unmarshal(payloadBytes, out)
}

func (s *AuthService) IssueToken(userID, role string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	claims := tokenClaims{
		UserID: userID,
		Role:   role,
		Exp:    time.Now().Add(ttl).Unix(),
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sig := s.signPayload(payloadEnc)
	return "cat7_" + payloadEnc + "." + sig, nil
}

func (s *AuthService) VerifyToken(tokenStr string) (*tokenClaims, error) {
	tokenStr = strings.TrimSpace(strings.TrimPrefix(tokenStr, "Bearer "))
	if !strings.HasPrefix(tokenStr, "cat7_") {
		return nil, errors.New("not a cat7 token")
	}
	raw := strings.TrimPrefix(tokenStr, "cat7_")
	parts := strings.SplitN(raw, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("malformed token")
	}
	expectedSig := s.signPayload(parts[0])
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return nil, errors.New("invalid signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var claims tokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, err
	}
	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}
	return &claims, nil
}

func hashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// IssueRefreshToken 生成持久化在 SQLite 中的高熵长期 Refresh Token（默认 180 天）。
func (s *AuthService) IssueRefreshToken(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	if s.userRepo == nil || strings.TrimSpace(userID) == "" {
		return "", errors.New("cannot issue refresh token")
	}
	if ttl <= 0 {
		ttl = DefaultRefreshTokenTTL
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	raw := "cat7r_" + base64.RawURLEncoding.EncodeToString(buf)
	expiresAt := time.Now().Add(ttl).Unix()
	if err := s.userRepo.CreateRefreshToken(ctx, userID, hashRefreshToken(raw), expiresAt); err != nil {
		return "", err
	}
	return raw, nil
}

// RefreshWithToken 校验 Refresh Token 并将有效期滑动顺延 180 天，返回用户与新的 Access Token。
func (s *AuthService) RefreshWithToken(ctx context.Context, rawRefreshToken string, accessTTL, refreshTTL time.Duration) (*model.User, string, string, error) {
	raw := strings.TrimSpace(rawRefreshToken)
	if !strings.HasPrefix(raw, "cat7r_") || s.userRepo == nil {
		return nil, "", "", errors.New("invalid refresh token")
	}
	if accessTTL <= 0 {
		accessTTL = DefaultAccessTokenTTL
	}
	if refreshTTL <= 0 {
		refreshTTL = DefaultRefreshTokenTTL
	}
	rec, err := s.userRepo.GetRefreshTokenByHash(ctx, hashRefreshToken(raw))
	if err != nil || rec == nil {
		return nil, "", "", errors.New("refresh token not found")
	}
	if rec.RevokedAt > 0 {
		return nil, "", "", errors.New("refresh token revoked")
	}
	if time.Now().Unix() > rec.ExpiresAt {
		return nil, "", "", errors.New("refresh token expired")
	}
	u, err := s.userRepo.GetUserByID(ctx, rec.UserID)
	if err != nil || u == nil {
		return nil, "", "", errors.New("user not found")
	}
	if !u.IsActive {
		return nil, "", "", errors.New("account disabled")
	}
	accessTok, err := s.IssueToken(u.ID, u.Role, accessTTL)
	if err != nil {
		return nil, "", "", err
	}
	_ = s.userRepo.ExtendRefreshToken(ctx, rec.ID, time.Now().Add(refreshTTL).Unix())
	return u, accessTok, raw, nil
}

// RevokeRefreshToken 吊销单条 Refresh Token（退出登录时调用）。
func (s *AuthService) RevokeRefreshToken(ctx context.Context, rawRefreshToken string) error {
	raw := strings.TrimSpace(rawRefreshToken)
	if raw == "" || s.userRepo == nil {
		return nil
	}
	return s.userRepo.RevokeRefreshTokenByHash(ctx, hashRefreshToken(raw))
}

// RevokeAllUserRefreshTokens 吊销某用户的全部 Refresh Token（改密或禁用账号时调用）。
func (s *AuthService) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	if userID == "" || s.userRepo == nil {
		return nil
	}
	return s.userRepo.RevokeAllUserRefreshTokens(ctx, userID)
}

// IssueGatewayBindTicket creates a short-lived signed ticket binding a verified UGOS gateway identity.
func (s *AuthService) IssueGatewayBindTicket(gwUID, gwName, gwType, existingUID string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	claims := GatewayBindClaims{
		GatewayUID:  gwUID,
		GatewayName: gwName,
		GatewayType: gwType,
		ExistingUID: existingUID,
		Exp:         time.Now().Add(ttl).Unix(),
	}
	return s.issueSignedPayload("cat7bind_", claims)
}

// VerifyGatewayBindTicket validates a gateway binding ticket and checks expiration.
func (s *AuthService) VerifyGatewayBindTicket(ticket string) (*GatewayBindClaims, error) {
	var claims GatewayBindClaims
	if err := s.verifySignedPayload(strings.TrimSpace(ticket), "cat7bind_", &claims); err != nil {
		return nil, err
	}
	if strings.TrimSpace(claims.GatewayUID) == "" {
		return nil, errors.New("missing gateway uid in ticket")
	}
	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("gateway authorization expired, please try again")
	}
	return &claims, nil
}

func (s *AuthService) signPayload(payload string) string {
	mac := hmac.New(sha256.New, s.hmacSecret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func RandomCatAvatar() string {
	n, err := rand.Int(rand.Reader, big.NewInt(6))
	if err != nil {
		return "/avatars/cat-1.jpg"
	}
	return fmt.Sprintf("/avatars/cat-%d.jpg", n.Int64()+1)
}

func GenerateCode6() string {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "849201"
	}
	return fmt.Sprintf("%06d", 100000+n.Int64())
}

func (s *AuthService) SendEmail(ctx context.Context, toEmail, subject, body string) (bool, error) {
	return s.SendEmailContent(ctx, toEmail, EmailContent{
		Subject:  subject,
		TextBody: body,
	})
}

func (s *AuthService) SendEmailContent(ctx context.Context, toEmail string, content EmailContent) (bool, error) {
	cfg, err := s.settingsRepo.GetSystemConfig(ctx)
	if err != nil || strings.TrimSpace(cfg.SmtpHost) == "" || strings.TrimSpace(cfg.SmtpUser) == "" {
		return false, nil
	}
	return true, SendSMTPMessage(cfg, toEmail, content)
}

func (s *AuthService) SendVerificationCodeEmail(ctx context.Context, toEmail, purpose, code string) (bool, error) {
	cfg, err := s.settingsRepo.GetSystemConfig(ctx)
	if err != nil || strings.TrimSpace(cfg.SmtpHost) == "" || strings.TrimSpace(cfg.SmtpUser) == "" {
		return false, nil
	}
	content := RenderVerificationCodeEmail(cfg.SiteName, toEmail, purpose, code, 10)
	return true, SendSMTPMessage(cfg, toEmail, content)
}

func (s *AuthService) SendBookInviteEmail(ctx context.Context, toEmail, inviterName, bookName, inviteCode string) (bool, error) {
	cfg, err := s.settingsRepo.GetSystemConfig(ctx)
	if err != nil || strings.TrimSpace(cfg.SmtpHost) == "" || strings.TrimSpace(cfg.SmtpUser) == "" {
		return false, nil
	}
	content := RenderBookInviteEmail(cfg.SiteName, inviterName, bookName, inviteCode)
	return true, SendSMTPMessage(cfg, toEmail, content)
}

func resolveFromAddresses(cfg *model.SystemConfig) (envelopeAddr, headerValue string) {
	raw := strings.TrimSpace(cfg.SmtpFrom)
	if raw == "" {
		raw = strings.TrimSpace(cfg.SmtpUser)
	}
	site := resolveBrandSiteName(cfg.SiteName)
	if parsed, err := mail.ParseAddress(raw); err == nil && strings.TrimSpace(parsed.Address) != "" {
		display := strings.TrimSpace(parsed.Name)
		if display == "" {
			display = site
		}
		return parsed.Address, fmt.Sprintf("%s <%s>", mime.BEncoding.Encode("UTF-8", display), parsed.Address)
	}
	return raw, fmt.Sprintf("%s <%s>", mime.BEncoding.Encode("UTF-8", site), raw)
}

func wrapBase64Lines(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var b strings.Builder
	b.Grow(len(encoded) + len(encoded)/38 + 4)
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		b.WriteString(encoded[i:end])
		b.WriteString("\r\n")
	}
	return b.String()
}

// BuildMIMEMessage constructs an RFC-compliant multipart/alternative or plain-text email payload.
func BuildMIMEMessage(cfg *model.SystemConfig, toEmail string, content EmailContent) (envelopeFrom string, raw []byte, err error) {
	envelopeFrom, headerFrom := resolveFromAddresses(cfg)

	var randBuf [12]byte
	if _, rErr := rand.Read(randBuf[:]); rErr != nil {
		return "", nil, rErr
	}
	boundary := "cat7alt_" + hex.EncodeToString(randBuf[:])

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", headerFrom))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.TrimSpace(toEmail)))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.BEncoding.Encode("UTF-8", content.Subject)))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msg.WriteString("MIME-Version: 1.0\r\n")

	if strings.TrimSpace(content.HTMLBody) != "" {
		msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary))
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		msg.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		msg.WriteString(wrapBase64Lines([]byte(content.TextBody)))
		msg.WriteString(fmt.Sprintf("\r\n--%s\r\n", boundary))
		msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		msg.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		msg.WriteString(wrapBase64Lines([]byte(content.HTMLBody)))
		msg.WriteString(fmt.Sprintf("\r\n--%s--\r\n", boundary))
	} else {
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		msg.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		msg.WriteString(wrapBase64Lines([]byte(content.TextBody)))
	}

	return envelopeFrom, []byte(msg.String()), nil
}

func SendSMTPWithConfig(cfg *model.SystemConfig, toEmail, subject, body string) error {
	return SendSMTPMessage(cfg, toEmail, EmailContent{
		Subject:  subject,
		TextBody: body,
	})
}

func SendSMTPMessage(cfg *model.SystemConfig, toEmail string, content EmailContent) error {
	host := strings.TrimSpace(cfg.SmtpHost)
	port := cfg.SmtpPort
	if port <= 0 {
		port = 465
	}
	envelopeFrom, msgBytes, err := BuildMIMEMessage(cfg, toEmail, content)
	if err != nil {
		return err
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	auth := smtp.PlainAuth("", cfg.SmtpUser, cfg.SmtpPass, host)
	if cfg.SmtpTLS && port == 465 {
		dialer := &net.Dialer{Timeout: 8 * time.Second}
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: host})
		if err != nil {
			return err
		}
		c, err := smtp.NewClient(conn, host)
		if err != nil {
			_ = conn.Close()
			return err
		}
		defer c.Close()
		if err := c.Auth(auth); err != nil {
			return err
		}
		if err := c.Mail(envelopeFrom); err != nil {
			return err
		}
		if err := c.Rcpt(toEmail); err != nil {
			return err
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write(msgBytes); err != nil {
			_ = w.Close()
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		return c.Quit()
	}

	return smtp.SendMail(addr, auth, envelopeFrom, []string{toEmail}, msgBytes)
}
