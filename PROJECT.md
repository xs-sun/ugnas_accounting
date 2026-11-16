# Project: 七喵记账 (com.xsguyu.accounting)

## Architecture
A native UGOS Pro application for high-aesthetic multi-ledger accounting, wallet asset management, bill importing, and hot snapshot backups.

```
+-----------------------------------------------------------------------------------+
|                              UGOS Pro Client                                      |
|  (PC Desktop Window >= 768px / Mobile Web Viewport < 768px via Reverse Proxy)     |
+-----------------------------------------------------------------------------------+
                                         │  HTTPS / WSS
                                         ▼
+-----------------------------------------------------------------------------------+
|                              UGOS Pro Gateway                                     |
|  - Reverse Proxy (proxy_path: "api" -> port 21010)                               |
|  - Validates `Ugreen-Ttk` and injects:                                             |
|    * Ugreen-User-ID                                                               |
|    * Ugreen-User-Name                                                             |
|    * Ugreen-User-Type                                                             |
+-----------------------------------------------------------------------------------+
                                         │  HTTP /api/...
                                         ▼
+-----------------------------------------------------------------------------------+
|                         Backend (Go 1.22+, Port 21010)                            |
|                                                                                   |
|  +-----------------------------------------------------------------------------+  |
|  | Middleware: Auth (Gateway Header Extraction + Local Dev/Test Bypass)        |  |
|  |             CORS, Logger, Recovery, Static File Server                      |  |
|  +-----------------------------------------------------------------------------+  |
|  | Handlers / Controllers:                                                     |  |
|  | - AuthHandler       - BookHandler        - WalletHandler                    |  |
|  | - TransactionHandler - CategoryHandler   - StatHandler (O(1) Pre-agg)       |  |
|  | - ImportExportHandler - SnapshotHandler  - HealthHandler                    |  |
|  +-----------------------------------------------------------------------------+  |
|  | Services / Business Logic:                                                  |  |
|  | - LedgerService     - WalletLinkageService - PreAggService                  |  |
|  | - SharingAuthService - SnapshotEngine      - CsvImportEngine                |  |
|  +-----------------------------------------------------------------------------+  |
|  | Repository Layer (Interfaces & SQLite Implementation):                      |  |
|  | - BookRepo, MemberRepo, WalletRepo, TxRepo, CategoryRepo, SnapshotRepo     |  |
|  +-----------------------------------------------------------------------------+  |
|  | Storage Engine (pure-Go SQLite via modernc.org/sqlite, CGO_ENABLED=0):      |  |
|  | - PRAGMA journal_mode=WAL, busy_timeout=5000, mmap_size=256MB               |  |
|  | - 10-table schema, Atomic Triggers for Balance Linkage & Pre-aggregation    |  |
|  | - Materialized `daily_summaries` table & Covering Index (<0.2ms reporting)  |  |
|  | - VACUUM INTO Hot Snapshot Engine                                          |  |
|  +-----------------------------------------------------------------------------+  |
|                                                                                   |
|  Data Directories:                                                                |
|  - $UGAPP_DATA_DIR (fallback: ./data/accounting.db, ./data/attachments)           |
|  - $UGAPP_LOG_DIR  (fallback: ./logs/app.log)                                     |
+-----------------------------------------------------------------------------------+
                                         ▲
                                         │  Loads Web App
+-----------------------------------------------------------------------------------+
|              Frontend (Vue 3 + Vite 5 + TypeScript + Tailwind CSS 3.4)            |
|  - Zero heavy 3rd-party UI libraries, custom design tokens                        |
|  - Canvas #F6F7F8, Card #FFFFFF, Deep Turquoise #0E8C7F, Terracotta Orange #E5654E|
|  - 34-40px Morandi Category Icons + fine linear stroke                            |
|  - PingFang SC, tabular-nums with -0.02em letter spacing, left-aligned bite        |
|  - Squircle card radius 20px / 14px / 11px, dual soft floating shadow             |
|  - Bespoke segmented breathing donut charts, ranking bars, subtle trend bars      |
|  - Responsive: Mobile (<768px) Bottom TabBar/Sheets vs PC (>=768px) Sidebar/Modals|
|  - Built to rootfs_common/www/                                                    |
+-----------------------------------------------------------------------------------+
```

## Feature Inventory
All features identified from `ORIGINAL_REQUEST.md` and Survey phase:

| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| 1 | UGOS Pro Packaging & Spec Compliance | `project.yaml` v2.1 schema, port 21010, proxy_path "api", open_type "inner", rootfs layout | M1 | R1, survey |
| 2 | Automated Cross-Platform Build Pipeline | `build.sh` for Vite build, Go CGO_ENABLED=0 Linux amd64/arm64 compile, rootfs assembly, `ugcli pack` .upk | M1 | R1, survey |
| 3 | Storage & Environment Variable Handling | `UGAPP_DATA_DIR` and `UGAPP_LOG_DIR` with seamless fallback to `./data` and `./logs` | M1 | R1, survey |
| 4 | Pure-Go High-Performance SQLite Setup | `modernc.org/sqlite` driver, WAL mode, busy_timeout=5000, mmap_size=256MB, synchronous=NORMAL | M2 | R1, survey |
| 5 | Relational Schema & Repository Abstraction | 10 DDL tables, Repository pattern decoupling business logic from SQL, foreign keys enabled | M2 | R1, survey |
| 6 | Financial Int64 Cents Precision | Signed Int64 cents (分) for all balances and amounts, completely eliminating floating point error | M2 | R2, R4, survey |
| 7 | Full-Featured Wallet System | Support WeChat, Alipay, Debit Card, Credit Card (overdraft/limits/dates), Cash, Investment, Custom | M2 | R2, survey |
| 8 | Atomic Balance Linkage & Transfers | Real-time balance updates on income/expense; atomic database transactions for inter-wallet transfers | M2 | R2, survey |
| 9 | Dynamic Net Asset Aggregation | Net Assets = Total Assets (positive) - Total Liabilities (negative overdraft), with exclude flag | M2 | R2, survey |
| 10 | Materialized `daily_summaries` Pre-Aggregation | Materialized daily summaries + covering index `idx_daily_summaries_query` for O(1) monthly/annual reports (<5ms) | M2 | R1, survey |
| 11 | Multi-Book Management | Support multiple independent ledgers (daily, family, travel) with isolated wallets, categories, tags | M3 | R3, survey |
| 12 | Collaborative Sharing & Role-Based Access | Owner, Editor, Viewer roles with fine-grained permission enforcement | M3 | R3, survey |
| 13 | Dual-Mode Authentication Middleware | Extract `Ugreen-User-ID/Name/Type` and `Ugreen-Ttk` headers with local dev/testing bypass | M3 | R1, survey |
| 14 | Category & Tag Management | Multi-level categories, Morandi icon/color association, custom transaction tags | M3 | R3, R4, survey |
| 15 | Core Transaction REST APIs | Expense, Income, Transfer CRUD, pagination, filtering, attachment upload | M3 | R4, survey |
| 16 | Reporting & Analytics REST APIs | Monthly summary, category ranking, monthly trend APIs querying `daily_summaries` pre-aggregated data | M3 | R1, survey |
| 17 | Online Hot Snapshot Engine | Native `VACUUM INTO` live snapshot + attachments packaging into `.ugledger` archive | M4 | R4, survey |
| 18 | Rollback-Safe Snapshot Restore Engine | Atomic snapshot validation, pre-restore backup, file extraction, integrity check, rollback on failure | M4 | R4, survey |
| 19 | WeChat & Alipay CSV Smart Import | CSV parser for WeChat (UTF-8 BOM) and Alipay (GBK/UTF-8), auto-categorization rule engine, wallet mapping | M4 | R4, survey |
| 20 | Full Ledger CSV/Excel Export | Comprehensive export of book transactions and metadata to standard CSV/Excel format | M4 | R4, survey |
| 21 | High-Aesthetic Design Tokens & Typography | Canvas #F6F7F8, Card #FFFFFF, Turquoise #0E8C7F, Orange #E5654E, PingFang SC, tabular-nums left-bite | M5 | R5, survey |
| 22 | Morandi Category Icon System | 34-40px squircle badges with low-saturation Morandi backgrounds and matching 1.4-1.6px fine linear icons | M5 | R5, survey |
| 23 | Bespoke Segmented SVG Charts | White-space breathing segmented donut chart, micro-rounded ranking bars, subtle trend dual-color bars | M5 | R5, survey |
| 24 | Dual-Breakpoint Adaptive Layout | Mobile (<768px: TabBar, bottom drawers, 44px+ targets) vs Desktop (>=768px: Sidebar, wide grid, modals) | M5 | R5, survey |
| 25 | UI Anti-Pattern & Taboo Enforcement | Complete elimination of SaaS blue gradients, rainbow cards, heavy black shadows, retro paper styles | M5 | R6, survey |
| 26 | Client UGOS Gateway Integration | `@ugreen-nas/core` third_token acquisition with local standalone fallback | M5 | R1, survey |
| 27 | End-to-End Test Suite Execution | 100% pass of E2E test suite (Tiers 1-4) derived from user requirements | M6 | Acceptance, survey |
| 28 | Adversarial Coverage Hardening (Tier 5) | White-box edge-case and stress test generation verifying system robustness | M6 | Pattern, survey |

## Milestones

| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| M1 | Packaging & Project Infrastructure | `project.yaml` (v2.1), `rootfs_*` layouts, `build.sh` pipeline, env fallbacks | none | PLANNED |
| M2 | Storage Engine & Financial Core | Pure-Go SQLite, WAL/mmap, 10-table schema, pre-aggregation table, Int64 cents, wallet linkage | M1 | PLANNED |
| M3 | Multi-Book, Auth & REST APIs | Dual-mode Auth middleware, multi-book sharing, permissions, transaction & stats endpoints | M2 | PLANNED |
| M4 | Snapshot Engine & Bill Import/Export | `VACUUM INTO` `.ugledger` hot backup/restore, WeChat/Alipay CSV parser, rule engine, export | M3 | PLANNED |
| M5 | Atomic Frontend & High-Aesthetic UI | Vue 3, Vite, Tailwind CSS tokens, Morandi icons, custom SVG charts, responsive layouts | M3 | PLANNED |
| M6 | Integration, E2E Verification & Hardening | Full build integration, 100% E2E test pass (Tiers 1-4), adversarial hardening (Tier 5) | M4, M5 | PLANNED |

In parallel:
- **E2E-Track**: Opaque-box, requirement-driven E2E test suite (Tiers 1-4) creating `TEST_INFRA.md` and publishing `TEST_READY.md`.

## Interface Contracts

### 1. Gateway ↔ Backend Auth Middleware
- **Request Headers Injected by Gateway**:
  * `Ugreen-User-ID`: string (e.g., `"10001"`, `"admin"`)
  * `Ugreen-User-Name`: string (e.g., `"tao7"`, `"admin"`)
  * `Ugreen-User-Type`: string (`"admin"` | `"users"`)
  * `Ugreen-Ttk`: string (third_token)
- **Local Dev / Test Fallback**:
  * If `Ugreen-User-ID` is missing, backend injects:
    * `User-ID: "local_dev_user"`, `User-Name: "admin"`, `User-Type: "admin"`
- **Go Context Value**:
  * `type UserContext struct { ID string; Name string; Role string }` stored in `r.Context()`.

### 2. Frontend ↔ Backend REST API (`/api/v1`)
All endpoints prefixed by `/api/v1`, responses formatted as standard JSON:
```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```
Key Routes:
- `GET /api/v1/health` -> `{ status: "ok", version: "0.1.0" }`
- `GET/POST /api/v1/books` -> List / Create book
- `GET/PUT/DELETE /api/v1/books/:id` -> Get / Update / Delete book
- `GET/POST /api/v1/books/:id/members` -> List / Add member
- `DELETE /api/v1/books/:id/members/:userId` -> Remove member
- `GET/POST /api/v1/wallets` -> List / Create wallet
- `PUT/DELETE /api/v1/wallets/:id` -> Update / Archive wallet
- `GET /api/v1/wallets/summary` -> Net asset calculation: `{ total_assets, total_liabilities, net_assets }`
- `GET/POST /api/v1/transactions` -> List (paginated, filtered) / Create transaction
- `PUT/DELETE /api/v1/transactions/:id` -> Update / Delete transaction
- `GET /api/v1/reports/monthly` -> Monthly aggregation from `daily_summaries`: `{ year, month, total_expense, total_income, categories: [...] }`
- `GET /api/v1/reports/trend` -> 6/12 month trend data from `daily_summaries`
- `GET/POST /api/v1/categories` -> List / Create categories (with Morandi token)
- `POST /api/v1/snapshots/create` -> Triggers `VACUUM INTO` and packaging
- `GET /api/v1/snapshots` -> List existing snapshots
- `GET /api/v1/snapshots/:id/download` -> Download `.ugledger`
- `POST /api/v1/snapshots/restore` -> Upload & restore `.ugledger`
- `POST /api/v1/imports/wechat` -> Upload & parse WeChat CSV
- `POST /api/v1/imports/alipay` -> Upload & parse Alipay CSV
- `POST /api/v1/imports/commit` -> Commit parsed transactions to ledger
- `GET /api/v1/export` -> Export ledger as CSV

### 3. Backend Storage ↔ Database
- Monetary values: Signed `Int64` in cents (分). `1.00 CNY = 100`.
- PRAGMAs applied on every DB connection:
  ```sql
  PRAGMA journal_mode = WAL;
  PRAGMA busy_timeout = 5000;
  PRAGMA mmap_size = 268435456;
  PRAGMA synchronous = NORMAL;
  PRAGMA foreign_keys = ON;
  ```
- Materialized `daily_summaries` maintained via database triggers:
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
  CREATE INDEX idx_daily_summaries_query 
  ON daily_summaries (book_id, type, date, category_id, total_amount, count);
  ```

## Code Layout
```
/Users/tao7/Documents/1Work/ugnas_accounting/
├── project.yaml                 # UGOS Pro App Manifest (v2.1)
├── build.sh                     # Unified build & packaging script
├── PROJECT.md                   # Global architectural & execution blueprint
├── backend/                     # Go 1.22+ Backend Service
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/
│   │   └── server/
│   │       └── main.go          # Entry point (port 21010, env vars, graceful shutdown)
│   └── internal/
│       ├── config/              # Runtime & environment config
│       ├── model/               # Domain entities (Int64 cents, wallet types)
│       ├── repository/          # Repository interfaces & SQLite implementations
│       │   ├── sqlite/
│       │   │   ├── db.go        # Driver registration, PRAGMAs, migrations
│       │   │   ├── schema.sql   # DDL tables, indexes, triggers
│       │   │   ├── book_repo.go
│       │   │   ├── wallet_repo.go
│       │   │   ├── transaction_repo.go
│       │   │   └── summary_repo.go
│       │   └── repository.go
│       ├── service/             # Business logic (ledger, balance linkage, pre-agg, snapshot)
│       │   ├── book_service.go
│       │   ├── wallet_service.go
│       │   ├── transaction_service.go
│       │   ├── report_service.go
│       │   ├── snapshot_service.go
│       │   └── import_service.go
│       └── handler/             # HTTP Handlers & Router
│           ├── router.go
│           ├── middleware/      # Auth (gateway headers), CORS, logger
│           ├── book_handler.go
│           ├── wallet_handler.go
│           ├── transaction_handler.go
│           ├── report_handler.go
│           ├── snapshot_handler.go
│           └── import_handler.go
├── frontend/                    # Vue 3 + Vite + Tailwind CSS Frontend
│   ├── package.json
│   ├── vite.config.ts
│   ├── tailwind.config.js       # Design tokens (colors, squircle, shadows)
│   ├── tsconfig.json
│   ├── index.html
│   └── src/
│       ├── assets/              # Design tokens, CSS entry
│       ├── components/          # Reusable atomic UI components (squircle cards, pills)
│       │   ├── charts/          # Segmented Donut, Ranking Bars, Trend Bars
│       │   ├── icons/           # Morandi category badge icons
│       │   ├── layout/          # Desktop Sidebar, Mobile TabBar, Header
│       │   └── common/          # Modals, Drawers, Inputs (tabular-nums)
│       ├── views/               # Pages (Dashboard, Ledger, Wallets, Stats, Settings)
│       ├── stores/              # Pinia state stores
│       ├── api/                 # API client with Ugreen-Ttk token handling
│       └── App.vue
├── rootfs_common/               # Shared static assets for UGOS Pro packaging
│   ├── icon.png                 # 256x256 app icon (<100KB)
│   └── www/                     # Compiled frontend static files
├── rootfs_amd64/                # Linux amd64 specific rootfs
│   └── bin/
│       └── accounting_serv      # Linux amd64 backend binary
├── rootfs_arm64/                # Linux arm64 specific rootfs
│   └── bin/
│       └── accounting_serv      # Linux arm64 backend binary
└── tests/                       # E2E Test Suite (Tiers 1-4)
    ├── e2e_runner.sh
    ├── tier1_features/
    ├── tier2_boundaries/
    ├── tier3_interactions/
    └── tier4_real_world/
```
