package handler

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// SwaggerDocJSON returns the complete OpenAPI 3.0.0 specification for SevenCats Accounting
func SwaggerDocJSON(appID, version string) []byte {
	doc := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "七喵记账 (SevenCats Accounting) API Documentation",
			"description": "基于绿联私有云 (UGOS Pro) 的本地私有化多账本记账与家庭资产管理系统 API 文档。\n\n包含多账本管理、多币种与四类钱包资产核算、两级收支分类、智能计算器记账、微信/支付宝账单导入与去重、莫兰迪图表统计、在线热快照等完整功能。",
			"version":     version,
			"contact": map[string]interface{}{
				"name": "七喵工作室 (xsguyu studio)",
				"url":  "https://ugnas.xsguyu.com",
			},
		},
		"servers": []map[string]interface{}{
			{
				"url":         "/",
				"description": "当前 NAS 本地运行实例",
			},
		},
		"tags": []map[string]interface{}{
			{"name": "System", "description": "系统状态与健康检查"},
			{"name": "Books", "description": "多账本管理与多成员协作 (所有者/编辑者/访客)"},
			{"name": "Wallets", "description": "资产账户体系 (资金/信用/理财/应收应付 & 转账核算)"},
			{"name": "Categories & Tags", "description": "两级收支分类树与正交标签体系"},
			{"name": "Transactions", "description": "核心记账交互、多维流水筛选与凭证附件"},
			{"name": "Reports", "description": "统计分析大盘、莫兰迪环形图、走势柱状图与成员贡献"},
			{"name": "Import & Export", "description": "微信/支付宝账单智能解析导入 & .ugledger 专有备份"},
			{"name": "Snapshots", "description": "SQLite 在线热快照 (VACUUM INTO) 容灾备份与极速恢复"},
		},
		"paths": map[string]interface{}{
			"/api/v1/health": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"System"},
					"summary":     "系统健康检查",
					"description": "获取当前后端运行状态、应用ID和版本号",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "服务正常运行",
						},
					},
				},
			},
			"/api/v1/books": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Books"},
					"summary":     "获取账本列表",
					"description": "获取当前用户参与的所有账本列表，包括个人账本与家庭共享账本",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "账本列表返回成功"},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Books"},
					"summary":     "创建新账本",
					"description": "创建新账本并初始化默认成员角色",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"name"},
									"properties": map[string]interface{}{
										"name":        map[string]interface{}{"type": "string", "example": "2026日常家庭账本"},
										"currency":    map[string]interface{}{"type": "string", "example": "CNY", "default": "CNY"},
										"description": map[string]interface{}{"type": "string", "example": "家庭日常开支共同维护"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "账本创建成功"},
					},
				},
			},
			"/api/v1/books/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Books"},
					"summary":     "获取指定账本详情",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "返回账本详情"},
						"404": map[string]interface{}{"description": "账本不存在"},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"Books"},
					"summary":     "修改账本信息",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"name":        map[string]interface{}{"type": "string"},
										"currency":    map[string]interface{}{"type": "string"},
										"description": map[string]interface{}{"type": "string"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "修改成功"},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Books"},
					"summary":     "删除账本 (深层入口 & 权限核验)",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "账本已安全删除"},
					},
				},
			},
			"/api/v1/books/{id}/members": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Books"},
					"summary":     "获取账本成员列表",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "返回成员列表及角色 (owner/editor/viewer)"},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Books"},
					"summary":     "添加/邀请账本成员",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"user_id", "user_name"},
									"properties": map[string]interface{}{
										"user_id":   map[string]interface{}{"type": "string", "example": "family_member_01"},
										"user_name": map[string]interface{}{"type": "string", "example": "老婆"},
										"role":      map[string]interface{}{"type": "string", "enum": []string{"owner", "editor", "viewer"}, "default": "editor"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "成员添加成功"},
					},
				},
			},
			"/api/v1/books/{id}/members/{userId}": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{"Books"},
					"summary":     "移除账本成员",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
						{"name": "userId", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "成员已移除"},
					},
				},
			},
			"/api/v1/wallets": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Wallets"},
					"summary":     "获取资产账户列表",
					"parameters": []map[string]interface{}{
						{"name": "book_id", "in": "query", "schema": map[string]interface{}{"type": "string"}, "description": "按账本筛选"},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "账户列表返回成功 (含余额, 额度, 分类图标)"},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Wallets"},
					"summary":     "创建新资产账户",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"name", "type"},
									"properties": map[string]interface{}{
										"name":            map[string]interface{}{"type": "string", "example": "招商银行储蓄卡"},
										"type":            map[string]interface{}{"type": "string", "enum": []string{"cash", "credit", "invest", "debt"}, "example": "cash"},
										"balance":         map[string]interface{}{"type": "integer", "description": "初始金额 (单位: 分)", "example": 500000},
										"credit_limit":    map[string]interface{}{"type": "integer", "description": "信用额度 (单位: 分)", "example": 2000000},
										"currency":        map[string]interface{}{"type": "string", "example": "CNY", "default": "CNY"},
										"icon":            map[string]interface{}{"type": "string", "example": "bank"},
										"color":           map[string]interface{}{"type": "string", "example": "#0E8C7F"},
										"include_in_net":  map[string]interface{}{"type": "boolean", "default": true},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "账户创建成功"},
					},
				},
			},
			"/api/v1/wallets/{id}": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{"Wallets"},
					"summary":     "更新账户信息与余额",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "账户已更新"},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Wallets"},
					"summary":     "删除资产账户",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "账户已删除"},
					},
				},
			},
			"/api/v1/wallets/transfer": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Wallets"},
					"summary":     "账户间转账 (原子级资金划转 & 手续费扣减)",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"from_wallet_id", "to_wallet_id", "amount"},
									"properties": map[string]interface{}{
										"book_id":        map[string]interface{}{"type": "string"},
										"from_wallet_id": map[string]interface{}{"type": "string", "example": "wallet_alipay"},
										"to_wallet_id":   map[string]interface{}{"type": "string", "example": "wallet_cmb"},
										"amount":         map[string]interface{}{"type": "integer", "description": "转账金额 (分)", "example": 100000},
										"fee":            map[string]interface{}{"type": "integer", "description": "转账手续费 (分)", "example": 100},
										"remark":         map[string]interface{}{"type": "string", "example": "余额提现到招行"},
										"date":           map[string]interface{}{"type": "string", "example": "2026-09-07"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "转账成功"},
					},
				},
			},
			"/api/v1/wallets/summary": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Wallets"},
					"summary":     "获取资产概览 (总资产 / 总负债 / 净资产动态核算)",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "资产统计返回成功"},
					},
				},
			},
			"/api/v1/categories": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Categories & Tags"},
					"summary":     "获取收支两级分类树",
					"parameters": []map[string]interface{}{
						{"name": "type", "in": "query", "schema": map[string]interface{}{"type": "string", "enum": []string{"expense", "income"}}, "description": "分类类型"},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "两级分类树返回成功"},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Categories & Tags"},
					"summary":     "创建自定义收支分类",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"name", "type"},
									"properties": map[string]interface{}{
										"name":      map[string]interface{}{"type": "string", "example": "撸猫专项"},
										"type":      map[string]interface{}{"type": "string", "enum": []string{"expense", "income"}},
										"parent_id": map[string]interface{}{"type": "string", "description": "一级分类ID (留空则为一级分类)"},
										"icon":      map[string]interface{}{"type": "string", "example": "cat"},
										"color":     map[string]interface{}{"type": "string", "example": "#D4A373"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "分类创建成功"},
					},
				},
			},
			"/api/v1/categories/{id}": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{"Categories & Tags"},
					"summary":     "修改分类信息",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "修改成功"},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Categories & Tags"},
					"summary":     "删除分类",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "删除成功"},
					},
				},
			},
			"/api/v1/tags": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Categories & Tags"},
					"summary":     "获取标签列表",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "标签列表返回成功"},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Categories & Tags"},
					"summary":     "创建新标签",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"name"},
									"properties": map[string]interface{}{
										"name":  map[string]interface{}{"type": "string", "example": "待报销"},
										"color": map[string]interface{}{"type": "string", "example": "#2A9D8F"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "标签创建成功"},
					},
				},
			},
			"/api/v1/transactions": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Transactions"},
					"summary":     "多维聚合查询交易流水列表",
					"parameters": []map[string]interface{}{
						{"name": "book_id", "in": "query", "schema": map[string]interface{}{"type": "string"}, "description": "账本ID"},
						{"name": "type", "in": "query", "schema": map[string]interface{}{"type": "string", "enum": []string{"expense", "income", "transfer"}}, "description": "交易类型"},
						{"name": "start_date", "in": "query", "schema": map[string]interface{}{"type": "string", "example": "2026-09-01"}, "description": "起始日期"},
						{"name": "end_date", "in": "query", "schema": map[string]interface{}{"type": "string", "example": "2026-09-30"}, "description": "截止日期"},
						{"name": "category_id", "in": "query", "schema": map[string]interface{}{"type": "string"}, "description": "分类ID"},
						{"name": "wallet_id", "in": "query", "schema": map[string]interface{}{"type": "string"}, "description": "账户ID"},
						{"name": "keyword", "in": "query", "schema": map[string]interface{}{"type": "string"}, "description": "模糊检索备注与商户"},
						{"name": "page", "in": "query", "schema": map[string]interface{}{"type": "integer", "default": 1}},
						{"name": "page_size", "in": "query", "schema": map[string]interface{}{"type": "integer", "default": 50}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "流水列表返回成功 (含日分组汇总与分页)"},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Transactions"},
					"summary":     "新增记账流水 (支出 / 收入 / 转账)",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"book_id", "type", "amount", "wallet_id", "date"},
									"properties": map[string]interface{}{
										"book_id":        map[string]interface{}{"type": "string"},
										"type":           map[string]interface{}{"type": "string", "enum": []string{"expense", "income", "transfer"}},
										"amount":         map[string]interface{}{"type": "integer", "description": "金额 (分)", "example": 3800},
										"category_id":    map[string]interface{}{"type": "string", "example": "cat_catering_lunch"},
										"wallet_id":      map[string]interface{}{"type": "string", "example": "wallet_alipay"},
										"to_wallet_id":   map[string]interface{}{"type": "string", "description": "转账入账账户 (转账必填)"},
										"date":           map[string]interface{}{"type": "string", "example": "2026-09-07"},
										"time":           map[string]interface{}{"type": "string", "example": "12:30:00"},
										"remark":         map[string]interface{}{"type": "string", "example": "日料工作午餐"},
										"merchant":       map[string]interface{}{"type": "string", "example": "一乐拉面"},
										"tags":           map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "记账成功，自动联动账户余额与预聚合缓存"},
					},
				},
			},
			"/api/v1/transactions/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Transactions"},
					"summary":     "获取单笔流水详情",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "详情返回成功"},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"Transactions"},
					"summary":     "修改单笔流水 (自动回滚旧余额并重算新余额)",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "修改成功"},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Transactions"},
					"summary":     "删除单笔流水 (自动恢复对应账户余额)",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "删除成功"},
					},
				},
			},
			"/api/v1/transactions/{id}/attachment": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Transactions"},
					"summary":     "上传小票 / 发票凭证图片附件",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"requestBody": map[string]interface{}{
						"content": map[string]interface{}{
							"multipart/form-data": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"file": map[string]interface{}{"type": "string", "format": "binary"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "上传成功，返回附件存储相对路径"},
					},
				},
				"get": map[string]interface{}{
					"tags":        []string{"Transactions"},
					"summary":     "下载/查看凭证附件图片",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "返回图片二进制流"},
					},
				},
			},
			"/api/v1/reports/overview": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Reports"},
					"summary":     "获取收支与结余概览大盘 KPI",
					"parameters": []map[string]interface{}{
						{"name": "book_id", "in": "query", "schema": map[string]interface{}{"type": "string"}},
						{"name": "start_date", "in": "query", "schema": map[string]interface{}{"type": "string"}},
						{"name": "end_date", "in": "query", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "返回总收入、总支出、净结余、日均支出及同比环比指标"},
					},
				},
			},
			"/api/v1/reports/categories": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Reports"},
					"summary":     "分类收支结构与占比 (莫兰迪环形图)",
					"parameters": []map[string]interface{}{
						{"name": "book_id", "in": "query", "schema": map[string]interface{}{"type": "string"}},
						{"name": "type", "in": "query", "schema": map[string]interface{}{"type": "string", "enum": []string{"expense", "income"}}},
						{"name": "start_date", "in": "query", "schema": map[string]interface{}{"type": "string"}},
						{"name": "end_date", "in": "query", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "返回按分类聚合的金额、百分比、排行榜及图标色值"},
					},
				},
			},
			"/api/v1/reports/trend": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Reports"},
					"summary":     "收支双色柱状走势与净结余月度曲线",
					"parameters": []map[string]interface{}{
						{"name": "book_id", "in": "query", "schema": map[string]interface{}{"type": "string"}},
						{"name": "months", "in": "query", "schema": map[string]interface{}{"type": "integer", "default": 6}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "返回各月份收入、支出与结余数据序列"},
					},
				},
			},
			"/api/v1/reports/members": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Reports"},
					"summary":     "家庭成员支出与贡献分布",
					"parameters": []map[string]interface{}{
						{"name": "book_id", "in": "query", "schema": map[string]interface{}{"type": "string"}},
						{"name": "start_date", "in": "query", "schema": map[string]interface{}{"type": "string"}},
						{"name": "end_date", "in": "query", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "返回各成员记账总额与占比结构"},
					},
				},
			},
			"/api/v1/import/parse": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Import & Export"},
					"summary":     "智能解析微信 / 支付宝 CSV 账单",
					"description": "自动识别 GBK / UTF-8 编码，过滤转账还款，自动匹配预置分类并生成防重哈希",
					"requestBody": map[string]interface{}{
						"content": map[string]interface{}{
							"multipart/form-data": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"file": map[string]interface{}{"type": "string", "format": "binary"},
										"book_id": map[string]interface{}{"type": "string"},
										"source": map[string]interface{}{"type": "string", "enum": []string{"auto", "alipay", "wechat"}},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "返回已智能分类的账单预览列表与重复条目标记"},
					},
				},
			},
			"/api/v1/import/commit": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Import & Export"},
					"summary":     "批量提交入库导入的交易记录",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "批量入库完成，返回成功与跳过计数"},
					},
				},
			},
			"/api/v1/export/ugledger": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Import & Export"},
					"summary":     "导出账本全量专有格式归档 (.ugledger)",
					"parameters": []map[string]interface{}{
						{"name": "book_id", "in": "query", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "下载包含元数据、全量流水与小票附件的压缩包"},
					},
				},
			},
			"/api/v1/import/ugledger": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Import & Export"},
					"summary":     "导入 / 恢复 .ugledger 专有账本备份",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "账本与小票凭证恢复成功"},
					},
				},
			},
			"/api/v1/snapshots": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Snapshots"},
					"summary":     "获取历史热快照列表",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "返回本地容灾快照列表及大小"},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Snapshots"},
					"summary":     "触发在线无锁热快照备份 (VACUUM INTO)",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "热快照生成成功"},
					},
				},
			},
			"/api/v1/snapshots/{id}": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{"Snapshots"},
					"summary":     "删除历史热快照",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "删除成功"},
					},
				},
			},
			"/api/v1/snapshots/{id}/download": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Snapshots"},
					"summary":     "下载热快照备份文件",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "返回数据库快照文件流"},
					},
				},
			},
			"/api/v1/snapshots/{id}/restore": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Snapshots"},
					"summary":     "热快照数据极速回滚恢复",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "数据恢复成功"},
					},
				},
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
      background: linear-gradient(135deg, #0E8C7F 0%%, #14B8A6 100%%);
      color: white;
      padding: 16px 24px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      box-shadow: 0 2px 8px rgba(14, 140, 127, 0.2);
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
      color: #134E4A;
    }
    .swagger-ui .btn.authorize {
      color: #0E8C7F;
      border-color: #0E8C7F;
    }
    .swagger-ui .opblock.opblock-get .opblock-summary-method {
      background: #0E8C7F;
    }
    .swagger-ui .opblock.opblock-post .opblock-summary-method {
      background: #2A9D8F;
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
