package service

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"strings"
	"testing"

	"com.xsguyu.accounting/backend/internal/model"
)

func TestRenderVerificationCodeEmailPurposesAndEscaping(t *testing.T) {
	purposes := []struct {
		purpose      string
		wantBadge    string
		wantSubject  string
		wantSubtitle string
	}{
		{"register", "注册验证", "新账号注册", "创建新账号"},
		{"login", "登录验证", "安全登录验证", "验证码登录"},
		{"forgot", "密码重置", "重置登录密码", "重置"},
		{"bind", "安全绑定", "绑定安全邮箱", "绑定"},
	}

	for _, tc := range purposes {
		content := RenderVerificationCodeEmail("七喵记账", "user+tag@example.com", tc.purpose, "592011", 10)
		if !strings.Contains(content.Subject, tc.wantSubject) {
			t.Fatalf("purpose=%s unexpected subject: %s", tc.purpose, content.Subject)
		}
		if !strings.Contains(content.TextBody, "592011") || !strings.Contains(content.TextBody, "10 分钟内有效") {
			t.Fatalf("purpose=%s missing text body info: %s", tc.purpose, content.TextBody)
		}
		for _, fragment := range []string{tc.wantBadge, "592011", "#DE8A1E", "#FFF8EE", "user+tag@example.com"} {
			if !strings.Contains(content.HTMLBody, fragment) {
				t.Fatalf("purpose=%s missing HTML fragment %q", tc.purpose, fragment)
			}
		}
		if tc.purpose == "login" {
			if dumpPath := os.Getenv("CAT7_DUMP_EMAIL_HTML"); dumpPath != "" {
				_ = os.WriteFile(dumpPath, []byte(content.HTMLBody), 0644)
			}
		}
	}

	xssContent := RenderVerificationCodeEmail(`<script>alert(1)</script>`, `a"<b>@test.com`, "register", "123456", 10)
	if strings.Contains(xssContent.HTMLBody, "<script>alert(1)</script>") {
		t.Fatalf("expected HTML escaping for site name XSS payload")
	}
	if !strings.Contains(xssContent.HTMLBody, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("expected escaped script text in HTML body")
	}
}

func TestRenderInviteAndSMTPTestEmails(t *testing.T) {
	inv := RenderBookInviteEmail("七喵记账", "淘七", "家庭账本", "CAT7-8888")
	if !strings.Contains(inv.HTMLBody, "CAT7-8888") || !strings.Contains(inv.HTMLBody, "家庭账本") {
		t.Fatalf("invite email missing code or book name")
	}
	testMail := RenderSMTPTestEmail("七喵记账", "smtp.example.com", 465, "noreply@xsguyu.com", "me@example.com")
	if !strings.Contains(testMail.HTMLBody, "smtp.example.com:465") || !strings.Contains(testMail.HTMLBody, "#3E8E63") {
		t.Fatalf("smtp test email missing host info or green status badge")
	}
}

func TestBuildMIMEMessageMultipartAndHeaders(t *testing.T) {
	cfg := &model.SystemConfig{
		SiteName: "七喵记账",
		SmtpUser: "noreply@xsguyu.com",
		SmtpFrom: "noreply@xsguyu.com",
	}
	content := RenderVerificationCodeEmail(cfg.SiteName, "1152909413@qq.com", "login", "592011", 10)
	envFrom, raw, err := BuildMIMEMessage(cfg, "1152909413@qq.com", content)
	if err != nil {
		t.Fatalf("BuildMIMEMessage failed: %v", err)
	}
	if envFrom != "noreply@xsguyu.com" {
		t.Fatalf("expected pure envelope address, got %q", envFrom)
	}

	parsedMsg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("mail.ReadMessage failed: %v", err)
	}
	dec := new(mime.WordDecoder)
	decodedSubject, err := dec.DecodeHeader(parsedMsg.Header.Get("Subject"))
	if err != nil || !strings.Contains(decodedSubject, "七喵记账") {
		t.Fatalf("unexpected decoded subject: %q (err=%v)", decodedSubject, err)
	}
	decodedFrom, err := dec.DecodeHeader(parsedMsg.Header.Get("From"))
	if err != nil || !strings.Contains(decodedFrom, "七喵记账") || !strings.Contains(decodedFrom, "noreply@xsguyu.com") {
		t.Fatalf("unexpected decoded From header: %q (err=%v)", decodedFrom, err)
	}

	mediaType, params, err := mime.ParseMediaType(parsedMsg.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/alternative" {
		t.Fatalf("expected multipart/alternative content type, got %q (err=%v)", mediaType, err)
	}

	mr := multipart.NewReader(parsedMsg.Body, params["boundary"])
	var gotPlain, gotHTML string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart failed: %v", err)
		}
		rawPart, _ := io.ReadAll(part)
		decodedPart, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(strings.TrimSpace(string(rawPart)), "\r\n", ""))
		if err != nil {
			t.Fatalf("base64 decode failed for %s: %v", part.Header.Get("Content-Type"), err)
		}
		ct := part.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "text/plain") {
			gotPlain = string(decodedPart)
		} else if strings.HasPrefix(ct, "text/html") {
			gotHTML = string(decodedPart)
		}
	}

	if !strings.Contains(gotPlain, "592011") {
		t.Fatalf("plain part missing code: %s", gotPlain)
	}
	if !strings.Contains(gotHTML, "592011") || !strings.Contains(gotHTML, "七喵记账") {
		t.Fatalf("html part missing code or brand name")
	}

	// Also confirm custom display name in SmtpFrom extracts envelope address cleanly
	cfg2 := &model.SystemConfig{
		SiteName: "七喵记账",
		SmtpFrom: "家庭财务管家 <custom@xsguyu.com>",
	}
	envFrom2, _, err := BuildMIMEMessage(cfg2, "to@example.com", content)
	if err != nil || envFrom2 != "custom@xsguyu.com" {
		t.Fatalf("expected custom envelope addr extracted cleanly, got %q (err=%v)", envFrom2, err)
	}
}
