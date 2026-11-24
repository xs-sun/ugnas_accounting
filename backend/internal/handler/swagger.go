package handler

import (
	"encoding/json"
	"fmt"
)

// SwaggerDocJSON returns the complete OpenAPI 3.0.0 specification for SevenCats Accounting v0.3.0
func SwaggerDocJSON(appID, version string) []byte {
	doc := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "七喵记账 (SevenCats Accounting) API Documentation",
			"description": "基于绿联私有云 (UGOS Pro) 的本地私有化多账本记账与 AI 智能财务平台 API 文档（v0.3.0）。\n\n支持双模鉴权（本地邮箱登录 + UGOS 网关）、一/二级分类与分类预算、定时自动记账、AI 聊着记与 OCR 小票识别、深度报表统计、微信/支付宝 CSV 导入导出、SQLite 无锁热快照与轻量隐私保护后台管理。",
			"version":     version,
			"contact": map[string]interface{}{
				"name": "七喵工作室 (xsguyu studio)",
				"url":  "https://ugnas.xsguyu.com",
			},
		},
		"servers": []map[string]interface{}{
			{"url": "/", "description": "当前 NAS 本地运行实例"},
		},
		"tags": []map[string]interface{}{
			{"name": "System", "description": "系统状态与健康检查"},
			{"name": "Auth", "description": "邮箱注册、密码/验证码登录与密码找回"},
			{"name": "User", "description": "个人资料、3D 小猫头像上传与邮箱绑定"},
			{"name": "Books", "description": "多账本管理、邀请口令加入、邮件邀请与虚拟成员"},
			{"name": "Wallets", "description": "资产账户体系与动态净资产汇总"},
			{"name": "Categories & Tags", "description": "一/二级收支分类树与正交标签"},
			{"name": "Transactions", "description": "流水记账、多维筛选与批量删除"},
			{"name": "Budgets", "description": "总预算与分分类月度预算水位"},
			{"name": "Recurring", "description": "周期性定时自动记账规则"},
			{"name": "Reports", "description": "周/月/年深度统计大盘与走势分析"},
			{"name": "AI", "description": "AI 聊着记、小票 OCR、财务诊断与模型连通性探测"},
			{"name": "Admin", "description": "管理中台（仅聚合宏观计量与系统配置，零私人账单访问）"},
			{"name": "Import & Export", "description": "微信/支付宝账单解析导入与 CSV 导出"},
			{"name": "Snapshots", "description": "SQLite VACUUM INTO 在线热快照与恢复"},
		},
		"paths": map[string]interface{}{
			"/api/v1/health": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"System"}, "summary": "系统健康检查", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "正常"}}},
			},
			"/api/v1/auth/send-code": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Auth"}, "summary": "发送邮箱验证码", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "已发送"}}},
			},
			"/api/v1/auth/register": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Auth"}, "summary": "邮箱注册账号", "responses": map[string]interface{}{"201": map[string]interface{}{"description": "注册成功"}}},
			},
			"/api/v1/auth/login": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Auth"}, "summary": "密码或验证码登录", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "登录成功"}}},
			},
			"/api/v1/auth/forgot-password": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Auth"}, "summary": "邮箱验证码重置密码", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "重置成功"}}},
			},
			"/api/v1/user/profile": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"User"}, "summary": "获取个人资料与财务健康指标", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"put": map[string]interface{}{"tags": []string{"User"}, "summary": "修改昵称或上传头像 (支持 Base64)", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/user/bind-email": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"User"}, "summary": "绑定邮箱", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "绑定成功"}}},
			},
			"/api/v1/avatars/{filename}": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"User"}, "summary": "获取用户上传的本地头像图片", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "图片二进制流"}}},
			},
			"/api/v1/books": map[string]interface{}{
				"get":  map[string]interface{}{"tags": []string{"Books"}, "summary": "获取账本列表 (空时自动初始化默认账本)", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"post": map[string]interface{}{"tags": []string{"Books"}, "summary": "创建账本 (含默认一/二级分类、钱包与分类预算)", "responses": map[string]interface{}{"201": map[string]interface{}{"description": "创建成功"}}},
			},
			"/api/v1/books/join": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Books"}, "summary": "通过 CAT7-XXXX 口令加入账本", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "加入成功"}}},
			},
			"/api/v1/books/{id}": map[string]interface{}{
				"get":    map[string]interface{}{"tags": []string{"Books"}, "summary": "获取账本详情", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"put":    map[string]interface{}{"tags": []string{"Books"}, "summary": "更新账本信息", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"delete": map[string]interface{}{"tags": []string{"Books"}, "summary": "删除账本", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/books/{id}/invite-email": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Books"}, "summary": "通过邮箱邀请成员加入账本", "responses": map[string]interface{}{"201": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/books/{id}/virtual-members": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Books"}, "summary": "添加虚拟成员", "responses": map[string]interface{}{"201": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/wallets": map[string]interface{}{
				"get":  map[string]interface{}{"tags": []string{"Wallets"}, "summary": "获取账户列表", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"post": map[string]interface{}{"tags": []string{"Wallets"}, "summary": "创建账户", "responses": map[string]interface{}{"201": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/wallets/{id}": map[string]interface{}{
				"put":    map[string]interface{}{"tags": []string{"Wallets"}, "summary": "更新账户 (含余额校准)", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"delete": map[string]interface{}{"tags": []string{"Wallets"}, "summary": "归档账户", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/wallets/summary": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Wallets"}, "summary": "获取资产负债汇总", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/categories": map[string]interface{}{
				"get":  map[string]interface{}{"tags": []string{"Categories & Tags"}, "summary": "获取一/二级分类列表", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"post": map[string]interface{}{"tags": []string{"Categories & Tags"}, "summary": "创建分类", "responses": map[string]interface{}{"201": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/categories/{id}": map[string]interface{}{
				"put":    map[string]interface{}{"tags": []string{"Categories & Tags"}, "summary": "修改分类", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"delete": map[string]interface{}{"tags": []string{"Categories & Tags"}, "summary": "删除分类并同步清理子分类", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/transactions": map[string]interface{}{
				"get":  map[string]interface{}{"tags": []string{"Transactions"}, "summary": "多维筛选交易流水 (支持 search、一级分类穿透及 page_size<=500)", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"post": map[string]interface{}{"tags": []string{"Transactions"}, "summary": "新增流水", "responses": map[string]interface{}{"201": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/transactions/batch-delete": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Transactions"}, "summary": "批量删除流水并原子回滚余额", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/transactions/{id}": map[string]interface{}{
				"get":    map[string]interface{}{"tags": []string{"Transactions"}, "summary": "获取流水详情", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"put":    map[string]interface{}{"tags": []string{"Transactions"}, "summary": "更新流水", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"delete": map[string]interface{}{"tags": []string{"Transactions"}, "summary": "删除流水", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/budgets": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Budgets"}, "summary": "获取月度总预算及分分类预算执行水位", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/budgets/total": map[string]interface{}{
				"put": map[string]interface{}{"tags": []string{"Budgets"}, "summary": "修改账本总预算", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/budgets/category": map[string]interface{}{
				"put": map[string]interface{}{"tags": []string{"Budgets"}, "summary": "设置或修改分类预算上限", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/recurring": map[string]interface{}{
				"get":  map[string]interface{}{"tags": []string{"Recurring"}, "summary": "获取定时记账规则列表", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"post": map[string]interface{}{"tags": []string{"Recurring"}, "summary": "新建定时记账规则", "responses": map[string]interface{}{"201": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/recurring/{id}": map[string]interface{}{
				"put":    map[string]interface{}{"tags": []string{"Recurring"}, "summary": "更新定时记账规则", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"delete": map[string]interface{}{"tags": []string{"Recurring"}, "summary": "删除定时记账规则", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/recurring/{id}/toggle": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Recurring"}, "summary": "启停定时记账规则", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/reports/stats": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Reports"}, "summary": "获取周/月/年深度统计分析（子分类归并一级分类）", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/reports/monthly": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Reports"}, "summary": "获取月度收支报表", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/reports/trend": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Reports"}, "summary": "获取月度收支趋势", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/reports/daily": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Reports"}, "summary": "获取日度收支明细", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/ai/chat": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"AI"}, "summary": "AI 自然语言/语音记账与问答", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/ai/ocr": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"AI"}, "summary": "小票/账单图片 OCR 识别", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/ai/insights": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"AI"}, "summary": "AI 财务诊断与省钱建议", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/ai/test": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"AI"}, "summary": "测试 AI 模型服务连通性", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/admin/overview": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Admin"}, "summary": "管理中台宏观统计概览 (零私人账单访问)", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/admin/ai-config": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Admin"}, "summary": "获取 AI 配置", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"put": map[string]interface{}{"tags": []string{"Admin"}, "summary": "更新 AI 配置", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/admin/ai-usage": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Admin"}, "summary": "查询分用户 AI 调用与 Token 消耗", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/admin/reg-config": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Admin"}, "summary": "获取注册策略配置", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"put": map[string]interface{}{"tags": []string{"Admin"}, "summary": "更新注册策略配置", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/admin/users": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Admin"}, "summary": "查询系统用户列表", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/admin/users/{id}": map[string]interface{}{
				"put": map[string]interface{}{"tags": []string{"Admin"}, "summary": "调整用户角色、AI 日限额或启停状态", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/admin/system-config": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Admin"}, "summary": "获取系统自动备份与 SMTP 邮箱配置", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
				"put": map[string]interface{}{"tags": []string{"Admin"}, "summary": "更新系统自动备份与 SMTP 邮箱配置", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/admin/smtp-test": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Admin"}, "summary": "测试 SMTP 邮件发送", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/export": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Import & Export"}, "summary": "导出账本流水为 CSV", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "CSV 数据流"}}},
			},
			"/api/v1/imports/wechat": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Import & Export"}, "summary": "解析微信 CSV 账单", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/imports/alipay": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Import & Export"}, "summary": "解析支付宝 CSV 账单", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/imports/commit": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Import & Export"}, "summary": "批量提交导入流水", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/snapshots": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Snapshots"}, "summary": "获取历史热快照列表", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/snapshots/create": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Snapshots"}, "summary": "创建热快照", "responses": map[string]interface{}{"201": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/snapshots/restore": map[string]interface{}{
				"post": map[string]interface{}{"tags": []string{"Snapshots"}, "summary": "上传归档热恢复", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/snapshots/{id}": map[string]interface{}{
				"delete": map[string]interface{}{"tags": []string{"Snapshots"}, "summary": "删除热快照", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "成功"}}},
			},
			"/api/v1/snapshots/{id}/download": map[string]interface{}{
				"get": map[string]interface{}{"tags": []string{"Snapshots"}, "summary": "下载热快照文件", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "二进制文件流"}}},
			},
		},
	}

	bytes, _ := json.MarshalIndent(doc, "", "  ")
	return bytes
}

// SwaggerUIHTML returns modern, self-contained, responsive Swagger UI HTML page
func SwaggerUIHTML(docURL, title string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <link rel="icon" type="image/png" href="/cat7-logo.png" />
  <style>
    body {
      margin: 0;
      padding: 0;
      background-color: #FAFAF7;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    }
    .topbar {
      display: none !important;
    }
    .custom-header {
      background: linear-gradient(135deg, #DE8A1E 0%%, #EC9B2E 100%%);
      color: white;
      padding: 16px 24px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      box-shadow: 0 2px 8px rgba(222, 138, 30, 0.25);
    }
    .custom-header .brand {
      display: flex;
      align-items: center;
      gap: 12px;
      font-size: 18px;
      font-weight: 700;
      letter-spacing: 0.5px;
    }
    .custom-header img {
      width: 32px;
      height: 32px;
      border-radius: 8px;
      background: white;
      padding: 2px;
    }
    .custom-header a {
      color: white;
      text-decoration: none;
      font-size: 14px;
      background: rgba(255, 255, 255, 0.2);
      padding: 6px 14px;
      border-radius: 6px;
      transition: all 0.2s;
    }
    .custom-header a:hover {
      background: rgba(255, 255, 255, 0.35);
    }
    .swagger-ui .info .title {
      color: #18181B;
    }
    .swagger-ui .btn.authorize {
      color: #DE8A1E;
      border-color: #DE8A1E;
    }
    .swagger-ui .opblock.opblock-get .opblock-summary-method {
      background: #2F78A8;
    }
    .swagger-ui .opblock.opblock-post .opblock-summary-method {
      background: #DE8A1E;
    }
  </style>
</head>
<body>
  <div class="custom-header">
    <div class="brand">
      <img src="/cat7-logo.png" alt="Logo" />
      <span>七喵记账 · API 交互式文档 (Swagger UI)</span>
    </div>
    <a href="/">返回应用主页 &rarr;</a>
  </div>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = function() {
      window.ui = SwaggerUIBundle({
        url: "%s",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>`, title, docURL)
}
