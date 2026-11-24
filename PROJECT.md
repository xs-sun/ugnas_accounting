# Project: 七喵记账 (com.xsguyu.accounting) — v0.2.0 工程架构蓝图

## 系统概述
**七喵记账**（`com.xsguyu.accounting`）是一款面向绿联云 **UGOS Pro** 私有部署与独立 Web/移动端运行的 AI 轻盈协同记账与家庭财务管理系统。系统采用 Go（纯 Go `modernc.org/sqlite`，`CGO_ENABLED=0`）+ Vue 3 / TypeScript / Tailwind CSS 单体精简架构，实现**数据 100% 本地私有归属**、**金融级 Int64 分位精度**与 **APP/PC 双端重构高颜值交互体验**。

---

## 系统架构图

```
+---------------------------------------------------------------------------------------------------+
|                                   七喵记账 客户端双端自适应层                                        |
|  • APP 移动端 (< 768px): 浮动胶囊顶栏 + 底部 4 Tab 胶囊(明细/日历/统计/我的) + 右侧琥珀 FAB 浮钮      |
|                          6 大手势抽屉/全屏(AI聊着记⇄手动记一笔 / 账本 / 成员邀请 / 子页面 / 认证流)   |
|  • PC 桌面端 (>= 768px): 256px 左侧导航栏 + 8 视图工作台 + 4 模块轻量管理中台(Admin 专属)             |
|                          居中双模记账弹窗 + 左右分栏认证页 + 金额隐私遮罩一键切换                      |
+---------------------------------------------------------------------------------------------------+
                                               │  HTTPS / HTTP (/api/v1)
                                               ▼
+---------------------------------------------------------------------------------------------------+
|                                      混合认证与网关接入层                                            |
|  1. UGOS Pro 网关免登: 透传 Ugreen-User-ID / Ugreen-User-Name / Ugreen-User-Type / Ugreen-Ttk       |
|  2. 独立邮箱账号体系:  邮箱密码登录 / 验证码注册与找回密码 / 邮箱绑定 (Bearer Token 会话)             |
|  3. 本地开发模式:      APP_ENV=development 下支持本地访客模式回退联调                                |
+---------------------------------------------------------------------------------------------------+
                                               │
                                               ▼
+---------------------------------------------------------------------------------------------------+
|                               Backend Service (Go 1.22+, Port 21010)                              |
|                                                                                                   |
|  +---------------------------------------------------------------------------------------------+  |
|  | HTTP 路由与中间件: HybridAuth / AdminOnly / CORS / Logger / Recovery / Static WWW / Avatars   |  |
|  +---------------------------------------------------------------------------------------------+  |
|  | Handlers (REST /api/v1):                                                                    |  |
|  | - AuthHandler (send-code, register, login, forgot-password)   - UserHandler (profile, bind) |  |
|  | - BookHandler (books, members, invite-email, virtual-members) - WalletHandler (CRUD, summary)|  |
|  | - CategoryTagHandler (categories, tags)                       - TransactionHandler (+batch) |  |
|  | - BudgetHandler (total, category)                             - RecurringHandler (+toggle)  |  |
|  | - ReportHandler (stats, monthly, trend, daily)                - AIHandler (chat, ocr, test) |  |
|  | - AdminHandler (overview, ai-config/usage, reg-config, users, system-config, smtp-test)     |  |
|  | - ImportExportHandler (wechat, alipay, commit, export)        - SnapshotHandler (hot backup)|  |
|  +---------------------------------------------------------------------------------------------+  |
|  | Services / 后台调度器:                                                                      |  |
|  | - AuthService (密码哈希/验证码/Token)     - Ledger & WalletLinkageService (Int64 原子联动)   |  |
|  | - Budget & RecurringScheduler (周期触发)  - AIService (Chat/OCR/Insights + Token 计量)      |  |
|  | - AdminPrivacyGuard (宏观零隐私聚合)      - SmtpMailer & HotBackupScheduler                 |  |
|  | - CsvImportEngine (微信/支付宝规则引擎)   - SnapshotEngine (VACUUM INTO + 原子回滚)         |  |
|  +---------------------------------------------------------------------------------------------+  |
|  | Storage Engine (pure-Go SQLite via modernc.org/sqlite, CGO_ENABLED=0):                      |  |
|  | - PRAGMA journal_mode=WAL, busy_timeout=5000, mmap_size=256MB, synchronous=NORMAL, FK=ON    |  |
|  | - Materialized `daily_summaries` 表与覆盖索引 `idx_daily_summaries_query` (O(1) 极速报表)    |  |
|  | - 数据目录: $UGAPP_DATA_DIR (回退 ./data/) | 日志目录: $UGAPP_LOG_DIR (回退 ./logs/)           |
|  +---------------------------------------------------------------------------------------------+  |
+---------------------------------------------------------------------------------------------------+
                                               ▲
                                               │  加载静态包 rootfs_common/www/
+---------------------------------------------------------------------------------------------------+
|                   Frontend (Vue 3 + Vite 5 + TypeScript + Tailwind CSS 3.4)                       |
|  • 视觉 Token: 暖琥珀金主色 #DE8A1E (Hover #C67612 / 浅底 #FFF6E8,#FFF3DC / 渐变按钮)              |
|               支出红陶 #D6553E | 收入森绿 #3E8E63 | 画布 #F5F6F8/#F4F5F8 | 纯白卡片 #FFFFFF       |
|  • 排版圆角: Inter + PingFang SC + tabular-nums | APP 22px 圆角 / PC 20px 圆角                     |
|  • 视觉资产: 30+ 双层双色几何 SVG 图标 | 6 款 3D 毛绒小猫头像库 + Canvas 圆形上传裁剪器            |
|  • 网关集成: @ugreen-nas/core third_token 自动获取与独立 Web 会话双兼容                            |
+---------------------------------------------------------------------------------------------------+
```

---

## Feature Inventory（功能特性清单）

| # | 特性模块 | 核心能力描述 | 里程碑 | 状态 |
|---|---------|-------------|--------|------|
| 1 | UGOS Pro 打包规范合规 | `project.yaml` v2.1 清单（`0.2.0`），端口 `21010`，`proxy_path: "api"`，`open_type: "inner"` | M1 | 已完成 |
| 2 | 跨平台自动化构建与打包管线 | `build.sh` 支持 Vite 构建、Go 跨平台编译、3 级 launcher 生成（zig -> Docker -> POSIX `#!/bin/sh`）、`ugcli check/pack` | M1/M7 | 已升级 |
| 3 | 存储与日志环境适配 | 支持 `UGAPP_DATA_DIR` 与 `UGAPP_LOG_DIR`，本地缺失时无缝回退 `./data` 与 `./logs` | M1 | 已完成 |
| 4 | Pure-Go 高性能 SQLite 底座 | `modernc.org/sqlite` 无 CGO 驱动，WAL 模式、`busy_timeout=5000`、`mmap_size=256MB`、`synchronous=NORMAL` | M2 | 已完成 |
| 5 | 金融级 Int64 分位精度 | 全系统余额、交易额、信用额度与预算上限采用 Signed Int64 分位（`1.00 元 = 100`），零浮点误差 | M2 | 已完成 |
| 6 | 资产账户与原子转账联动 | 现金/微信/支付宝/储蓄卡/信用卡/理财/应收应付管理，跨账户原子转账（含手续费）与动态净资产聚合 | M2 | 已完成 |
| 7 | `daily_summaries` 物化预聚合 | 触发器维护每日分类收支汇总，覆盖索引支撑 O(1) 月度/年度/热力日历极速查询 | M2 | 已完成 |
| 8 | 多账本与虚拟成员协作 | 支持多账本隔离（个人/家庭/旅行等）、Owner/Editor/Viewer 权限、口令/邮件邀请与无账号虚拟成员 | M3/M7 | 已升级 |
| 9 | 混合认证架构 | 兼顾 UGOS Pro 网关头免登与独立邮箱注册/登录/验证码找回/邮箱绑定安全认证 | M3/M7 | 已升级 |
| 10 | 一/二级分类与正交标签体系 | 预置两级收支分类树、自定义编辑与排序、跨分类正交标签、30+ 双层双色几何 SVG 图标库 | M3/M7 | 已升级 |
| 11 | 流水管理与批量操作 | 分页多维筛选查询、单笔详情编辑、凭证附件上传、PC 批量多选删除及原子余额回滚 | M3/M7 | 已升级 |
| 12 | 预算脉搏与分分类控制 | 月/年总预算 + 分分类子预算、预算水位进度条、APP 首页「今日建议日均可用额度」实时测算 | M7 | 新增 |
| 13 | 周期性定时自动记账 | 支持日/工作日/周/月/年定时自动入账规则配置与一键启停调度 | M7 | 新增 |
| 14 | 热力日历与深度统计报表 | 3 档红陶支出热力 + 收入森绿标识日历（含选中日钻取）、周/月/年呼吸环形图、趋势柱状图与 Top 排行 | M5/M7 | 已升级 |
| 15 | AI 聊着记、OCR 与财务洞察 | 自然语言聊着记、语音波形转写、小票拍照 OCR、内联微调预览确认卡；统计页消费洞察与 PC 3 卡片诊断 | M7 | 新增 |
| 16 | 3D 小猫头像与 Canvas 裁剪器 | 6 款 3D 毛绒小猫预设头像（小银灰/大橘宝/小三花/小布偶/奶牛猫/招财黑猫）+ 交互式圆形上传裁剪 | M7 | 新增 |
| 17 | 轻量管理中台（零隐私运维） | 宏观匿名统计（严禁访问用户流水）、AI 配置与 Token 统计、公开注册/日限额开关、SMTP 测试与定时备份 | M7 | 新增 |
| 18 | 微信/支付宝导入与热快照 | UTF-8 BOM/GBK CSV 智能解析、指纹去重与规则分类；`VACUUM INTO` 无锁热快照与原子回滚恢复 | M4 | 已完成 |
| 19 | 暖琥珀金双端 UI/UX 重构 | APP 4 Tab + 6 手势抽屉与 PC 8 视图工作台 + 4 模块后台；`#DE8A1E` / `#D6553E` / `#3E8E63` / `Inter`+`PingFang SC` | M5/M7 | 已升级 |
| 20 | 自动化 E2E 测试体系 | Tiers 1–5 测试套件，覆盖打包、SQLite WAL、财务精度、API 契约与新设计 Token 校验 | M6/M7 | 持续演进 |

---

## Milestones（里程碑规划）

| # | 名称 | 范围 | 依赖 | 状态 |
|---|------|------|------|------|
| M1 | 工程与打包基建 | `project.yaml` v2.1、`rootfs_*` 目录、环境变量回退、`build.sh` | none | COMPLETED |
| M2 | 存储与金融核心引擎 | Pure-Go SQLite WAL、Int64 分位、原子转账、`daily_summaries` 预聚合 | M1 | COMPLETED |
| M3 | 多账本与基础 REST API | 网关鉴权中间件、多账本权限、账户/分类/流水/报表接口 | M2 | COMPLETED |
| M4 | 热快照与账单导入导出 | `VACUUM INTO` `.ugledger` 热备份恢复、微信/支付宝 CSV 解析导入与导出 | M3 | COMPLETED |
| M5 | 第一代莫兰迪双端界面 | 初版 Vue 3 + Tailwind 界面、基础图表与移动端适配 | M3 | COMPLETED |
| M6 | 首轮集成与 E2E 验证 | 端到端测试套件（Tiers 1–5）构建与验证 | M4, M5 | COMPLETED |
| M7 | **v0.2.0 全面重构升级** | 暖琥珀金设计语言、混合邮箱认证、APP 4Tab+6抽屉 / PC 8视图+4中台、AI 聊着记/OCR/洞察、预算脉搏、定时记账、3D 小猫头像+裁剪、`build.sh` zig/POSIX 启动器升级 | M6 | IN PROGRESS |

---

## Interface Contracts（系统接口契约）

### 1. 混合认证契约（Gateway ↔ Standalone Auth ↔ Backend Middleware）
* **UGOS Pro 网关透传头**：
  * `Ugreen-User-ID`: string（例如 `"10001"`）
  * `Ugreen-User-Name`: string（例如 `"tao7"`）
  * `Ugreen-User-Type`: string（`"admin"` | `"users"`）
  * `Ugreen-Ttk`: string（third_token）
* **独立邮箱会话头**：
  * `Authorization: Bearer <token>`（由 `/api/v1/auth/login` 或 `/api/v1/auth/register` 签发），经后端验证后还原对应用户身份与角色（`admin` / `member`）。
* **本地开发回退**：
  * 当 `APP_ENV=development` 且无网关头与 Bearer Token 时，允许回退到默认开发调试用户；生产环境未认证请求返回 `401 Unauthorized`。
* **管理中台零隐私约束**：
  * `/api/v1/admin/*` 路由强制校验 `Role == "admin"`；且管理员接口不得查询或返回普通用户个人账本与交易明细内容。

### 2. Frontend ↔ Backend REST API (`/api/v1`)
统一 JSON 响应格式：`{ "code": 0, "message": "success", "data": ... }`。

* **Health**
  * `GET /api/v1/health`
* **Auth（独立认证）**
  * `POST /api/v1/auth/send-code`
  * `POST /api/v1/auth/register`
  * `POST /api/v1/auth/login`
  * `POST /api/v1/auth/forgot-password`
* **User（个人资料与头像）**
  * `GET /api/v1/user/profile`、`PUT /api/v1/user/profile`
  * `POST /api/v1/user/bind-email`
  * `GET /api/v1/avatars/:filename`
* **Books（多账本与成员/虚拟成员）**
  * `GET /api/v1/books`、`POST /api/v1/books`
  * `GET /api/v1/books/:id`、`PUT /api/v1/books/:id`、`DELETE /api/v1/books/:id`
  * `GET /api/v1/books/:id/members`、`POST /api/v1/books/:id/members`、`PUT /api/v1/books/:id/members/:userId`、`DELETE /api/v1/books/:id/members/:userId`
  * `POST /api/v1/books/:id/invite-email`
  * `POST /api/v1/books/:id/virtual-members`
  * `POST /api/v1/books/join`
* **Wallets（资产账户）**
  * `GET /api/v1/wallets`、`POST /api/v1/wallets`
  * `PUT /api/v1/wallets/:id`、`DELETE /api/v1/wallets/:id`
  * `GET /api/v1/wallets/summary`
* **Categories & Tags（分类与标签）**
  * `GET /api/v1/categories`、`POST /api/v1/categories`、`PUT /api/v1/categories/:id`、`DELETE /api/v1/categories/:id`
  * `GET /api/v1/tags`、`POST /api/v1/tags`、`DELETE /api/v1/tags/:id`
* **Transactions（交易流水与批量处理）**
  * `GET /api/v1/transactions`、`POST /api/v1/transactions`
  * `GET /api/v1/transactions/:id`、`PUT /api/v1/transactions/:id`、`DELETE /api/v1/transactions/:id`
  * `POST /api/v1/transactions/batch-delete`
  * `POST /api/v1/transactions/:id/attachment`
* **Budgets（总预算与分类预算）**
  * `GET /api/v1/budgets`
  * `PUT /api/v1/budgets/total`
  * `PUT /api/v1/budgets/category`
* **Recurring（周期性定时记账）**
  * `GET /api/v1/recurring`、`POST /api/v1/recurring`
  * `PUT /api/v1/recurring/:id`、`DELETE /api/v1/recurring/:id`
  * `POST /api/v1/recurring/:id/toggle`
* **Reports（统计报表与热力日历）**
  * `GET /api/v1/reports/stats`
  * `GET /api/v1/reports/monthly`
  * `GET /api/v1/reports/trend`
  * `GET /api/v1/reports/daily`
* **AI（聊着记、小票 OCR、消费洞察与连通测试）**
  * `POST /api/v1/ai/chat`
  * `POST /api/v1/ai/ocr`
  * `GET /api/v1/ai/insights`
  * `POST /api/v1/ai/test`
* **Admin（轻量管理中台）**
  * `GET /api/v1/admin/overview`
  * `GET /api/v1/admin/ai-config`、`PUT /api/v1/admin/ai-config`
  * `GET /api/v1/admin/ai-usage`
  * `GET /api/v1/admin/reg-config`、`PUT /api/v1/admin/reg-config`
  * `GET /api/v1/admin/users`、`PUT /api/v1/admin/users/:id`
  * `GET /api/v1/admin/system-config`、`PUT /api/v1/admin/system-config`
  * `POST /api/v1/admin/smtp-test`
* **Import/Export & Snapshots（导入导出与热快照）**
  * `GET /api/v1/export`
  * `POST /api/v1/imports/wechat`、`POST /api/v1/imports/alipay`、`POST /api/v1/imports/commit`
  * `GET /api/v1/snapshots`、`POST /api/v1/snapshots/create`、`POST /api/v1/snapshots/restore`
  * `GET /api/v1/snapshots/:id/download`、`DELETE /api/v1/snapshots/:id`

### 3. 存储引擎与数据库契约
* 所有金额单位为整型 `INTEGER`（分，Cents）。
* 每次连接强制初始化 PRAGMA：
  ```sql
  PRAGMA journal_mode = WAL;
  PRAGMA busy_timeout = 5000;
  PRAGMA mmap_size = 268435456;
  PRAGMA synchronous = NORMAL;
  PRAGMA foreign_keys = ON;
  ```
* 物化汇总表 `daily_summaries`（配合 `idx_daily_summaries_query` 覆盖索引）实时响应统计与热力日历：
  ```sql
  CREATE TABLE daily_summaries (
      date TEXT NOT NULL,
      book_id TEXT NOT NULL,
      type TEXT NOT NULL,
      category_id TEXT NOT NULL,
      total_amount INTEGER NOT NULL DEFAULT 0,
      count INTEGER NOT NULL DEFAULT 0,
      PRIMARY KEY (date, book_id, type, category_id)
  );
  ```
* 核心持久化扩展覆盖：用户独立账号与邮箱验证表、账本与虚拟成员表、分分类预算表、定时自动记账规则表、AI 配置与分用户 Token 消耗流水表、系统设置（注册策略/SMTP/自动备份）表。

---

## Build & Packaging Pipeline（构建与打包流水线）

`build.sh` 实现全流程自动化构建与 UGOS Pro `.upk` 打包：
1. **工具链智能探测**：
   * Go 探测优先遍历 `$HOME/.local/bin/go`、`$HOME/.local/go/bin/go` 及标准系统路径（如均缺失且 Docker 运行中则自动回退容器编译）；
   * `ugcli` 探测支持 `PATH`、`$HOME/.local/bin/ugcli` 与 `/usr/local/bin/ugcli`。
2. **前端构建**：使用 `pnpm`（`pnpm install --prefer-offline || pnpm install`）或 `npm` 编译 Vue 3 产物至 `rootfs_common/www/`。
3. **后端跨平台编译与 3 级 Launcher 启动器生成**：
   * 使用 `CGO_ENABLED=0` 分别编译 `linux/amd64` 与 `linux/arm64` 至 `rootfs_*/bin/accounting_core`；
   * `compile_native_launchers()` 按 3 级优先级生成 `rootfs_amd64/bin/accounting_serv` 与 `rootfs_arm64/bin/accounting_serv`（用于清除 `GODEBUG` 并执行 `accounting_core`）：
     1. 检测到 `zig`（`PATH` 或 `~/.local/bin/zig`）时，使用 `zig cc -target x86_64-linux-musl -static -O2 -s` 与 `aarch64-linux-musl` 编译 `backend/cmd/launcher/main.c`；
     2. 否则若 Docker 守护进程运行中，使用 `debian:bookworm-slim` 容器内交叉 gcc 静态编译；
     3. 否则（如无 zig 且无 Docker 的 macOS 打包环境），自动生成零依赖 POSIX `#!/bin/sh` 启动脚本并赋予可执行权限。
   * 本地 `--run` 模式同步支持 `clang` / `gcc` 编译或 POSIX `#!/bin/sh` 启动器回退。
4. **结构校验与打包**：运行 `ugcli check` 校验包清单与目录结构，`--pack` 时调用 `ugcli pack --build <num> --arch <arch>` 生成 `.upk`。

---

## Code Layout（项目目录结构）

```
ugnas_accounting/
├── project.yaml                 # UGOS Pro 应用清单 (v2.1, version 0.2.0)
├── build.sh                     # 统一跨平台构建、3级 launcher 生成与 ugcli 打包脚本
├── PROJECT.md                   # 系统工程架构蓝图 (本文档)
├── docs/
│   └── 功能清单.md               # v0.2.0 全部功能清单与产品需求规范
├── .design_ref/                 # 视觉与交互设计参考基准稿
│   ├── ai_app.html              # APP 移动端交互设计稿 (4 Tab + 6 手势抽屉)
│   └── pc.html                  # PC 桌面端交互设计稿 (8 视图 + 4 模块管理中台)
├── backend/                     # Go 1.22+ 后端服务
│   ├── go.mod
│   ├── go.sum
│   └── cmd/
│       ├── server/main.go       # 服务入口 (port 21010, env 适配, 定时调度, 优雅退出)
│       └── launcher/main.c      # 原生静态启动器源码 (清除 GODEBUG 并 exec accounting_core)
│   └── internal/
│       ├── config/              # 运行时环境配置与存储路径解析
│       ├── model/               # 领域模型 (Int64 分位、账本/虚拟成员、预算、定时、AI、Admin)
│       ├── repository/          # 仓储接口与 pure-Go SQLite 实现 (DDL、迁移、触发器、汇总查询)
│       ├── service/             # 核心业务逻辑 (认证、账本、资产联动、预算、定时、AI/OCR、Admin、快照、导入)
│       └── handler/             # HTTP 路由、混合鉴权中间件与全量 /api/v1 Handlers
├── frontend/                    # Vue 3 + Vite 5 + TypeScript + Tailwind CSS 3.4 前端工程
│   ├── package.json             # version 0.2.0
│   ├── vite.config.ts
│   ├── tailwind.config.js       # 暖琥珀金设计 Token (#DE8A1E, #D6553E, #3E8E63, #F5F6F8, Inter)
│   └── src/
│       ├── style.css            # 全局 CSS 变量、tabular-nums 与基础样式
│       ├── assets/              # 品牌 Logo、3D 小猫头像资产
│       ├── components/          # 通用组件、双色 SVG 图标库、呼吸环形图/趋势图、双模记账、裁剪器
│       ├── views/               # APP 移动端视图与 PC 8 视图工作台 + 4 模块管理中台 + 认证页
│       ├── stores/              # 状态管理
│       ├── types/               # TypeScript 类型定义
│       └── api/                 # API 客户端 (UGOS third_token + Bearer 认证兼容)
├── rootfs_common/               # UGOS Pro 公共资源 (icon.png + www/)
├── rootfs_amd64/bin/            # Linux amd64 产物 (accounting_core + accounting_serv)
├── rootfs_arm64/bin/            # Linux arm64 产物 (accounting_core + accounting_serv)
└── tests/                       # 自动化 E2E 测试套件 (Tiers 1–5)
    ├── e2e_runner.sh
    ├── common/
    ├── tier1_features/          # 包含 test_f21_design_tokens.sh 等特性验证
    ├── tier2_boundaries/
    ├── tier3_interactions/
    └── tier4_real_world/
```
