# TEST_INFRA.md — E2E Test Infrastructure Specification

**Application**: 七喵记账 (`com.xsguyu.accounting`)  
**Scope**: Opaque-Box, Requirement-Driven End-to-End Test Suite (Tiers 1–4)  
**Standard**: UGOS Pro Native Application Specifications  

---

## 1. Overview

The test infrastructure for **七喵记账** provides a hermetic, automated, and requirement-traceable verification harness. It tests the application through external interfaces (HTTP REST APIs at `/api/v1/*`, SQLite database files, UGOS Pro manifests, and CLI build scripts) without coupling to internal struct implementations.

The suite comprises **300+ test cases** structured into 4 distinct tiers:
- **Tier 1 (Feature Coverage)**: 28 features $\times$ 5 test cases = **140 tests** covering happy paths.
- **Tier 2 (Boundaries & Edge Cases)**: 28 features $\times$ 5 test cases = **140 tests** stress-testing numerical bounds, overdrafts, limits, injection defenses, and empty/corrupt states.
- **Tier 3 (Pairwise Interactions)**: **7 comprehensive interaction suites** validating cross-subsystem ripple effects (transactions $\leftrightarrow$ triggers $\leftrightarrow$ pre-aggregation $\leftrightarrow$ wallet balances $\leftrightarrow$ net assets $\leftrightarrow$ multi-book RBAC).
- **Tier 4 (Real-World Workloads)**: **5 multi-step authentic scenarios** replicating month-long family accounting, multi-statement CSV smart imports, live hot snapshot backups with disaster rollback, multi-tenant permission isolation, and visual design token/taboo compliance audits.

---

## 2. Prerequisites & Environment

The test runner and test cases require only standard POSIX tools available on modern macOS and Linux systems:

| Dependency | Required Version / Path | Purpose |
|---|---|---|
| `bash` | 3.2+ (`/bin/bash` or `/usr/bin/env bash`) | Test runner orchestration and execution scripts |
| `curl` | 7.68+ (`/usr/bin/curl`) | HTTP REST API requests, multipart file uploads, header injection |
| `jq` | 1.6+ (`/usr/local/bin/jq` or `/usr/bin/jq`) | JSON response parsing and assertion validation |
| `sqlite3` | 3.30+ (`/usr/bin/sqlite3`) | Direct SQLite PRAGMA verification and table consistency inspection |
| `python3` | 3.8+ (`/usr/bin/python3` or `/usr/local/bin/python3`) | Ephemeral server runner and GBK/BOM test fixture generators |

---

## 3. Directory Layout

The test suite resides under `/tests` in the project repository:

```
tests/
├── e2e_runner.sh                   # Master test runner CLI (chmod +x)
├── ephemeral_server.py             # Reference/ephemeral SQLite-backed backend server
├── common/                         # Shared testing utilities
│   ├── colors.sh                   # ANSI colors and status symbols
│   ├── assert.sh                   # Assertion library (assert_eq, assert_status, assert_json, etc.)
│   ├── api_client.sh               # Curl client injecting UGOS gateway headers
│   └── db_helper.sh                # Direct SQLite inspection utilities
├── fixtures/                       # Deterministic test fixtures
│   ├── wechat_sample_utf8_bom.csv  # Standard WeChat Pay statement (UTF-8 with BOM)
│   ├── alipay_sample_gbk.csv       # Standard Alipay statement (GBK encoded)
│   ├── corrupted_snapshot.ugledger # Corrupted tar.gz archive for rollback tests
│   ├── valid_icon.png              # 256x256 valid app icon (<100KB)
│   └── test_receipt.jpg            # Sample image attachment
├── tier1_features/                 # 28 feature test scripts (T1-F01 to T1-F28, 140 tests)
├── tier2_boundaries/               # 28 boundary test scripts (T2-F01 to T2-F28, 140 tests)
├── tier3_interactions/             # 7 cross-feature interaction suites
└── tier4_real_world/               # 5 authentic end-to-end user workflows
```

---

## 4. Gateway Header Conventions

All HTTP requests dispatched by the test suite simulate the UGOS Pro reverse gateway proxy by injecting authentication headers:

| Header Name | Type | Example Value | Description |
|---|---|---|---|
| `Ugreen-User-ID` | String | `10001` (Alice), `10002` (Bob), `10003` (Charlie) | Unique NAS user identifier |
| `Ugreen-User-Name` | String | `alice`, `bob`, `charlie` | NAS login username |
| `Ugreen-User-Type` | String | `admin`, `users` | NAS role hierarchy |
| `Ugreen-Ttk` | String | `ttk_test_token_alice_10001` | Third-party service session token |

When these headers are omitted in development mode, backend endpoints automatically fall back to `local_dev_user` (`admin`).

---

## 5. Monetary & Financial Precision Rules

1. **Int64 Cents Storage**: All monetary fields (`amount`, `balance`, `credit_limit`, `total_assets`, `total_liabilities`, `net_assets`) are represented as signed 64-bit integers in **cents** (`分`).
   - $1.00\text{ CNY} = 100\text{ cents}$.
   - $12.34\text{ CNY} = 1234\text{ cents}$.
2. **Zero Floating-Point Error**: All internal calculations, pre-aggregations in `daily_summaries`, and test assertions evaluate exact integer arithmetic. No IEEE-754 floating-point numbers are accepted in financial storage or assertions.
3. **Net Asset Equation**:
   $$\text{Net Assets} = \sum \text{Assets} (\text{positive balances}) - \sum \text{Liabilities} (\text{credit card overdrafts})$$
   Wallets flagged with `exclude_net_asset: true` are strictly excluded from both total assets and liabilities.

---

## 6. How to Run the Test Suite

### 6.1 Basic Invocation
```bash
# Run the complete test suite against local server (default: http://127.0.0.1:21010)
./tests/e2e_runner.sh

# Run with an automatic isolated ephemeral server (uses random port & temp DB)
./tests/e2e_runner.sh --ephemeral

# Run against a custom target URL
./tests/e2e_runner.sh --target-url http://127.0.0.1:21010
```

### 6.2 Filter by Tier
```bash
# Run only Tier 1 (140 Feature Coverage tests)
./tests/e2e_runner.sh --tier 1

# Run only Tier 2 (140 Boundary & Edge Case tests)
./tests/e2e_runner.sh --tier 2

# Run only Tier 3 (7 Cross-Feature Interaction suites)
./tests/e2e_runner.sh --tier 3

# Run only Tier 4 (5 Real-World Workflow Scenarios)
./tests/e2e_runner.sh --tier 4
```

### 6.3 Filter by Feature
```bash
# Run Tier 1 and Tier 2 tests for Feature 8 (Atomic Balance Linkage & Transfers)
./tests/e2e_runner.sh --feature 8

# Run tests for Feature 10 (Materialized Pre-Aggregation)
./tests/e2e_runner.sh --feature 10
```

### 6.4 Diagnostic Flags
```bash
# Stop immediately upon first failed test
./tests/e2e_runner.sh --bail

# Verbose mode (prints request/response payloads)
./tests/e2e_runner.sh --verbose

# Display help and usage information
./tests/e2e_runner.sh --help
```

---

## 7. Exit Code Semantics

- **`0`**: All executed test cases passed successfully.
- **`1`**: One or more test cases failed or runtime error occurred.

---

## 8. Ephemeral Reference Server Architecture

When testing in environments where the Go backend binary is not yet running or during early development iterations, the runner supports `--ephemeral`. The ephemeral server (`tests/ephemeral_server.py`):
1. Allocates an ephemeral port (`21011`–`21099`).
2. Creates an isolated temporary directory for `UGAPP_DATA_DIR` and `UGAPP_LOG_DIR`.
3. Initializes a pure SQLite database with WAL mode, `busy_timeout=5000`, full 10-table relational schema, and materialized pre-aggregation triggers.
4. Serves `/api/v1/*` endpoints complying with `PROJECT.md` API contracts.
5. Automatically terminates and wipes temporary directories when the test runner exits (`trap cleanup EXIT INT TERM`).
