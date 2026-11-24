package service

import (
	"fmt"
	"html"
	"strings"
	"time"
)

// EmailContent represents a rendered email with both plain-text fallback and branded HTML.
type EmailContent struct {
	Subject  string
	TextBody string
	HTMLBody string
}

type emailKvRow struct {
	Label string
	Value string
}

type brandedEmailOpts struct {
	SiteName   string
	Preheader  string
	BadgeText  string
	BadgeTone  string // "amber" | "green"
	Title      string
	Subtitle   string
	CodeLabel  string
	CodeValue  string
	CodeHint   string
	Rows       []emailKvRow
	NoticeText string
}

func resolveBrandSiteName(siteName string) string {
	s := strings.TrimSpace(siteName)
	if s == "" {
		return "七喵记账"
	}
	return s
}

func renderBrandedEmailShell(opts brandedEmailOpts) string {
	siteName := resolveBrandSiteName(opts.SiteName)
	safeSite := html.EscapeString(siteName)
	safePreheader := html.EscapeString(opts.Preheader)
	safeBadge := html.EscapeString(opts.BadgeText)
	safeTitle := html.EscapeString(opts.Title)
	safeSubtitle := html.EscapeString(opts.Subtitle)
	safeCodeLabel := html.EscapeString(opts.CodeLabel)
	safeCodeValue := html.EscapeString(opts.CodeValue)
	safeCodeHint := html.EscapeString(opts.CodeHint)
	safeNotice := html.EscapeString(opts.NoticeText)

	badgeBg := "#FFF6E8"
	badgeBorder := "#F2DFBE"
	badgeColor := "#DE8A1E"
	if opts.BadgeTone == "green" {
		badgeBg = "#ECF6F0"
		badgeBorder = "#CBE6D6"
		badgeColor = "#3E8E63"
	}

	var rowsHTML strings.Builder
	if len(opts.Rows) > 0 {
		rowsHTML.WriteString(`<table role="presentation" cellpadding="0" cellspacing="0" width="100%" style="background-color:#F8F9FB;border:1px solid #ECEEF2;border-radius:14px;margin-top:20px;">`)
		for idx, row := range opts.Rows {
			borderTop := ""
			if idx > 0 {
				borderTop = "border-top:1px solid #ECEEF2;"
			}
			rowsHTML.WriteString(fmt.Sprintf(`<tr>
  <td style="padding:11px 16px;%sfont-size:12px;color:#64748B;width:92px;white-space:nowrap;">%s</td>
  <td style="padding:11px 16px;%sfont-size:13px;font-weight:600;color:#18181B;text-align:right;word-break:break-all;">%s</td>
</tr>`, borderTop, html.EscapeString(row.Label), borderTop, html.EscapeString(row.Value)))
		}
		rowsHTML.WriteString(`</table>`)
	}

	highlightHTML := ""
	if strings.TrimSpace(opts.CodeValue) != "" {
		highlightHTML = fmt.Sprintf(`<table role="presentation" cellpadding="0" cellspacing="0" width="100%%" style="margin-top:22px;background-color:#FFF8EE;border:1.5px dashed #E8B872;border-radius:18px;">
  <tr>
    <td align="center" style="padding:24px 16px 22px 16px;">
      <div style="font-size:12px;font-weight:600;color:#9A5D12;letter-spacing:0.5px;">%s</div>
      <div style="margin-top:12px;font-family:'SF Mono',SFMono-Regular,Menlo,Monaco,Consolas,'Liberation Mono',monospace;font-size:34px;line-height:1.2;font-weight:800;letter-spacing:8px;padding-left:8px;color:#C67612;">%s</div>
      <div style="margin-top:12px;display:inline-block;padding:4px 12px;border-radius:999px;background-color:#FFF1D6;font-size:11px;font-weight:600;color:#8C5510;">%s</div>
    </td>
  </tr>
</table>`, safeCodeLabel, safeCodeValue, safeCodeHint)
	}

	noticeHTML := ""
	if safeNotice != "" {
		noticeHTML = fmt.Sprintf(`<div style="margin-top:20px;padding:12px 14px;border-radius:12px;background-color:#F8FAFC;border:1px solid #ECEEF2;font-size:12px;line-height:1.65;color:#64748B;">%s</div>`, safeNotice)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>%s</title>
</head>
<body style="margin:0;padding:0;background-color:#F5F6F8;font-family:-apple-system,BlinkMacSystemFont,'PingFang SC','Hiragino Sans GB','Microsoft YaHei','Inter',sans-serif;color:#18181B;">
  <div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent;mso-hide:all;">%s</div>
  <table role="presentation" cellpadding="0" cellspacing="0" width="100%%" style="background-color:#F5F6F8;">
    <tr>
      <td align="center" style="padding:32px 14px 36px 14px;">
        <table role="presentation" cellpadding="0" cellspacing="0" width="100%%" style="max-width:540px;background-color:#FFFFFF;border:1px solid #ECEEF2;border-radius:24px;overflow:hidden;box-shadow:0 12px 32px -12px rgba(15,23,42,0.08);">
          <!-- Top Amber Brand Strip -->
          <tr>
            <td style="height:5px;line-height:5px;font-size:0;background-color:#DE8A1E;background-image:linear-gradient(90deg,#F0A336 0%%,#DE8A1E 55%%,#C67612 100%%);">&nbsp;</td>
          </tr>
          <!-- Header -->
          <tr>
            <td style="padding:26px 28px 18px 28px;border-bottom:1px solid #F1F5F9;">
              <table role="presentation" cellpadding="0" cellspacing="0" width="100%%">
                <tr>
                  <td valign="middle">
                    <table role="presentation" cellpadding="0" cellspacing="0">
                      <tr>
                        <td align="center" valign="middle" width="40" height="40" style="width:40px;height:40px;border-radius:13px;background-color:#DE8A1E;background-image:linear-gradient(135deg,#ECA034 0%%,#DA8316 100%%);color:#FFFFFF;font-size:15px;font-weight:800;letter-spacing:-0.5px;">七喵</td>
                        <td style="padding-left:12px;">
                          <div style="font-size:16px;font-weight:700;color:#18181B;line-height:1.25;">%s</div>
                          <div style="font-size:11px;color:#94A3B8;margin-top:2px;">绿联云 NAS 高品质极简 AI 记账</div>
                        </td>
                      </tr>
                    </table>
                  </td>
                  <td align="right" valign="middle">
                    <span style="display:inline-block;padding:5px 11px;border-radius:999px;background-color:%s;border:1px solid %s;font-size:11px;font-weight:700;color:%s;white-space:nowrap;">%s</span>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <!-- Body -->
          <tr>
            <td style="padding:26px 28px 28px 28px;">
              <h1 style="margin:0;font-size:20px;font-weight:700;color:#18181B;line-height:1.35;">%s</h1>
              <p style="margin:10px 0 0 0;font-size:14px;line-height:1.7;color:#475569;">%s</p>
              %s
              %s
              %s
            </td>
          </tr>
        </table>
        <!-- Footer -->
        <table role="presentation" cellpadding="0" cellspacing="0" width="100%%" style="max-width:540px;">
          <tr>
            <td align="center" style="padding:18px 16px 0 16px;font-size:11px;line-height:1.6;color:#94A3B8;">
              本邮件由 %s 系统自动发送，请勿直接回复。<br />
              © %d %s · 个人与家庭轻量协同记账工作台
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		safeTitle,
		safePreheader,
		safeSite,
		badgeBg, badgeBorder, badgeColor, safeBadge,
		safeTitle,
		safeSubtitle,
		highlightHTML,
		rowsHTML.String(),
		noticeHTML,
		safeSite,
		time.Now().Year(),
		safeSite,
	)
}

// RenderVerificationCodeEmail renders both text and branded HTML content for 6-digit verification emails.
func RenderVerificationCodeEmail(siteName, toEmail, purpose, code string, validMinutes int) EmailContent {
	site := resolveBrandSiteName(siteName)
	if validMinutes <= 0 {
		validMinutes = 10
	}
	normPurpose := strings.ToLower(strings.TrimSpace(purpose))

	badge := "安全验证"
	title := "请查收您的邮箱验证码"
	subtitle := "您正在进行账号身份验证操作，请在页面中输入下方 6 位数字验证码以继续："
	sceneName := "邮箱安全验证"

	switch normPurpose {
	case "register":
		badge = "注册验证"
		title = "欢迎加入" + site
		subtitle = "您正在注册" + site + "新账号（或绑定绿联云 NAS 身份），请在页面中输入以下 6 位数字验证码完成邮箱校验："
		sceneName = "新账号注册"
	case "login":
		badge = "登录验证"
		title = "账号安全登录验证"
		subtitle = "您正在通过邮箱验证码登录" + site + "工作台，请在登录页输入以下 6 位数字验证码："
		sceneName = "安全登录验证"
	case "forgot":
		badge = "密码重置"
		title = "重置登录密码验证"
		subtitle = "您正在申请重置" + site + "账号的登录密码。为保障您的账本与资产隐私安全，请使用下方验证码完成身份确认："
		sceneName = "重置登录密码"
	case "bind":
		badge = "安全绑定"
		title = "绑定安全邮箱验证"
		subtitle = "您正在将此邮箱绑定至" + site + "账号，绑定后可用于找回密码、验证码登录与接收家庭账本邀请："
		sceneName = "绑定安全邮箱"
	}

	subject := fmt.Sprintf("%s · %s", site, sceneName)
	textBody := fmt.Sprintf(
		"【%s】%s\n\n您的验证码为：%s（%d 分钟内有效，验证后失效）。\n请求账号：%s\n\n安全提示：请勿将验证码告知或转发给他人；如非本人操作，请忽略此邮件。",
		site, sceneName, code, validMinutes, toEmail,
	)

	htmlBody := renderBrandedEmailShell(brandedEmailOpts{
		SiteName:   site,
		Preheader:  fmt.Sprintf("您的%s验证码为 %s（%d 分钟内有效），请勿泄露给他人。", sceneName, code, validMinutes),
		BadgeText:  badge,
		BadgeTone:  "amber",
		Title:      title,
		Subtitle:   subtitle,
		CodeLabel:  "一次性安全验证码（点击数字可直接选中复制）",
		CodeValue:  code,
		CodeHint:   fmt.Sprintf("有效期 %d 分钟 · 仅限本次验证使用", validMinutes),
		Rows: []emailKvRow{
			{Label: "验证邮箱", Value: toEmail},
			{Label: "操作类型", Value: sceneName},
			{Label: "发送时间", Value: time.Now().Format("2006-01-02 15:04:05")},
		},
		NoticeText: "安全提示：" + site + "官方人员不会以任何形式向您索取验证码或密码。若非您本人发起本次请求，请直接忽略此邮件。",
	})

	return EmailContent{
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}
}

// RenderBookInviteEmail renders a branded invitation email with the ledger invite code.
func RenderBookInviteEmail(siteName, inviterName, bookName, inviteCode string) EmailContent {
	site := resolveBrandSiteName(siteName)
	inviter := strings.TrimSpace(inviterName)
	if inviter == "" {
		inviter = "成员"
	}
	book := strings.TrimSpace(bookName)
	if book == "" {
		book = "共享账本"
	}

	subject := fmt.Sprintf("%s · %s 邀请您加入共享账本「%s」", site, inviter, book)
	textBody := fmt.Sprintf(
		"%s 邀请您加入%s共享账本「%s」。\n\n专属邀请口令：%s\n\n登录后进入「家庭账本 -> 通过口令加入」填入上方邀请口令即可共同记账。",
		inviter, site, book, inviteCode,
	)

	htmlBody := renderBrandedEmailShell(brandedEmailOpts{
		SiteName:   site,
		Preheader:  fmt.Sprintf("%s 邀请您加入共享账本「%s」，邀请口令：%s", inviter, book, inviteCode),
		BadgeText:  "账本邀请",
		BadgeTone:  "amber",
		Title:      "收到共享账本协作邀请",
		Subtitle:   fmt.Sprintf("%s 邀请您加入共享账本「%s」，登录后填写下方邀请口令即可共同管理收支明细与家庭预算。", inviter, book),
		CodeLabel:  "共享账本邀请口令",
		CodeValue:  inviteCode,
		CodeHint:   "进入「家庭账本 -> 通过口令加入」即可加入",
		Rows: []emailKvRow{
			{Label: "邀请人", Value: inviter},
			{Label: "目标账本", Value: book},
			{Label: "发送时间", Value: time.Now().Format("2006-01-02 15:04:05")},
		},
		NoticeText: "若您暂时不认识该邀请人或未期待加入此账本，可忽略本邮件，不会对您的个人账本数据产生任何影响。",
	})

	return EmailContent{
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}
}

// RenderSMTPTestEmail renders a branded confirmation email for admin SMTP connectivity checks.
func RenderSMTPTestEmail(siteName, smtpHost string, smtpPort int, fromAddr, toEmail string) EmailContent {
	site := resolveBrandSiteName(siteName)
	subject := site + " · SMTP 发信配置测试成功"
	textBody := fmt.Sprintf(
		"这是一封来自%s管理后台的 SMTP 测试邮件。收到此邮件说明您的发信配置（%s:%d）已正常工作。",
		site, smtpHost, smtpPort,
	)

	htmlBody := renderBrandedEmailShell(brandedEmailOpts{
		SiteName:  site,
		Preheader: "收到此测试邮件表示您的 SMTP 发信服务器配置已就绪，可用于验证码与协作邀请通知。",
		BadgeText: "连通正常",
		BadgeTone: "green",
		Title:     "SMTP 发信配置测试成功",
		Subtitle:  fmt.Sprintf("这是一封来自%s系统后台的连通性测试邮件，您的邮件发送服务已正常就绪，可用于注册验证码、密码找回与账本协作邀请。", site),
		Rows: []emailKvRow{
			{Label: "SMTP 主机", Value: fmt.Sprintf("%s:%d", smtpHost, smtpPort)},
			{Label: "发件账号", Value: fromAddr},
			{Label: "测试收件人", Value: toEmail},
			{Label: "测试时间", Value: time.Now().Format("2006-01-02 15:04:05")},
		},
		NoticeText: "如您修改了管理后台中的 SMTP 密码或端口设置，建议随时再次触发一次测试以确保通知通道畅通。",
	})

	return EmailContent{
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}
}
