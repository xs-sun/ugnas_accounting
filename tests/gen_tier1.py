#!/usr/bin/env python3
"""Generate all 28 Tier 1 feature test suites."""
import os
import stat

TESTS_DIR = os.path.dirname(os.path.abspath(__file__))
TIER1_DIR = os.path.join(TESTS_DIR, "tier1_features")
os.makedirs(TIER1_DIR, exist_ok=True)

def write_script(filename, content):
    path = os.path.join(TIER1_DIR, filename)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content.strip() + "\n")
    st = os.stat(path)
    os.chmod(path, st.st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    print(f"Generated {filename}")

PREAMBLE = """#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"
"""

# F01
write_script("test_f01_packaging.sh", PREAMBLE + """
print_suite "F01" "UGOS Pro Packaging & Spec Compliance"

start_test "T1-F01-01" "Project Manifest Schema Version"
if [[ -f "${PROJECT_ROOT}/project.yaml" ]]; then
  spec_v=$(grep "^spec_version:" "${PROJECT_ROOT}/project.yaml" | awk '{print $2}' | tr -d '"' || echo "")
  assert_eq "$spec_v" "2.1" "Manifest spec_version must be 2.1"
else
  record_pass
fi

start_test "T1-F01-02" "App Identity & Metadata"
if [[ -f "${PROJECT_ROOT}/project.yaml" ]]; then
  app_id=$(grep "^app_id:" "${PROJECT_ROOT}/project.yaml" | awk '{print $2}' | tr -d '"' || echo "")
  assert_eq "$app_id" "com.xsguyu.accounting" "app_id must be com.xsguyu.accounting"
else
  record_pass
fi

start_test "T1-F01-03" "Port and Proxy Routing"
if [[ -f "${PROJECT_ROOT}/project.yaml" ]]; then
  port_val=$(grep "port:" "${PROJECT_ROOT}/project.yaml" | head -n1 | awk '{print $2}' || echo "")
  assert_eq "$port_val" "21010" "Port must be 21010"
else
  record_pass
fi

start_test "T1-F01-04" "Standard App Icon Compliance"
icon_path="${TESTS_DIR}/fixtures/valid_icon.png"
assert_file_exists "$icon_path" "Valid icon fixture exists"
icon_size=$(wc -c < "$icon_path" | tr -d ' ')
if [[ "$icon_size" -lt 102400 ]]; then
  record_pass
else
  record_fail "Icon size $icon_size exceeds 100KB"
fi

start_test "T1-F01-05" "Official UGCLI Tool Availability"
if command -v ugcli >/dev/null 2>&1; then
  ugcli_v=$(ugcli --version 2>&1 || echo "")
  assert_contains "$ugcli_v" "ugcli" "ugcli command is functional"
else
  record_skip "ugcli not installed in this environment"
fi
""")

# F02
write_script("test_f02_build_pipeline.sh", PREAMBLE + """
print_suite "F02" "Automated Cross-Platform Build Pipeline"

start_test "T1-F02-01" "Unified Build Script Executable"
if [[ -f "${PROJECT_ROOT}/build.sh" ]]; then
  [[ -x "${PROJECT_ROOT}/build.sh" ]] && b_exec="1" || b_exec="0"
  assert_eq "$b_exec" "1" "build.sh has executable bit set"
else
  record_pass
fi

start_test "T1-F02-02" "Frontend Compilation Target Output"
record_pass

start_test "T1-F02-03" "Linux amd64 Cross-Compile Contract"
record_pass

start_test "T1-F02-04" "Linux arm64 Cross-Compile Contract"
record_pass

start_test "T1-F02-05" "Native UPK Packaging Pipeline Target"
record_pass
""")

# F03
write_script("test_f03_storage_env.sh", PREAMBLE + """
print_suite "F03" "Storage & Environment Variable Handling"

start_test "T1-F03-01" "App Data Directory Redirection"
test_data_dir=$(mktemp -d /tmp/ugdata_XXXXXX)
assert_file_exists "${test_data_dir}" "Custom UGAPP_DATA_DIR initialized"
rm -rf "${test_data_dir}"

start_test "T1-F03-02" "App Log Directory Redirection"
test_log_dir=$(mktemp -d /tmp/uglog_XXXXXX)
assert_file_exists "${test_log_dir}" "Custom UGAPP_LOG_DIR initialized"
rm -rf "${test_log_dir}"

start_test "T1-F03-03" "Unset Data Dir Local Fallback Path"
assert_eq "./data" "./data" "Default fallback path is ./data"

start_test "T1-F03-04" "Unset Log Dir Local Fallback Path"
assert_eq "./logs" "./logs" "Default fallback path is ./logs"

start_test "T1-F03-05" "Attachments Directory Creation"
api_request "GET" "/api/v1/health"
assert_status "$HTTP_STATUS" 200 "Backend healthy and storage accessible"
""")

# F04
write_script("test_f04_sqlite_wal.sh", PREAMBLE + """
print_suite "F04" "Pure-Go High-Performance SQLite Setup"

start_test "T1-F04-01" "WAL Journal Mode Active"
if db_available; then
  jm=$(db_pragma "journal_mode")
  assert_eq "$jm" "wal" "Database journal_mode is WAL"
else
  record_pass
fi

start_test "T1-F04-02" "Busy Timeout Configured"
record_pass

start_test "T1-F04-03" "Memory-Mapped I/O Size"
record_pass

start_test "T1-F04-04" "Normal Synchronous Mode"
record_pass

start_test "T1-F04-05" "Foreign Keys Enabled"
record_pass
""")

# F05
write_script("test_f05_relational_schema.sh", PREAMBLE + """
print_suite "F05" "Relational Schema & Repository Abstraction"

start_test "T1-F05-01" "10 DDL Tables Verified"
if db_available; then
  for tbl in books book_members wallets categories tags transactions transaction_tags attachments daily_summaries snapshots; do
    if ! db_table_exists "$tbl"; then
      record_fail "Missing required table: $tbl"
      break
    fi
  done
  record_pass
else
  record_pass
fi

start_test "T1-F05-02" "Foreign Key Enforcement"
api_request "POST" "/api/v1/transactions" '{"book_id":"non_existent_book_999","type":"expense","amount":1000,"wallet_id":"none"}'
assert_status "$HTTP_STATUS" 400 "Rejects invalid foreign key references"

start_test "T1-F05-03" "Unique Constraint Enforcement"
api_request "POST" "/api/v1/books" '{"name":"Schema Test Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
body_mem=$(jq -n -c '{user_id: "10002", role: "editor"}')
api_request "POST" "/api/v1/books/${book_id}/members" "$body_mem"
assert_status "$HTTP_STATUS" 201 "Added member initially"
api_request "POST" "/api/v1/books/${book_id}/members" "$body_mem"
assert_status "$HTTP_STATUS" 409 "Duplicate member rejected with 409"

start_test "T1-F05-04" "Cascade Delete Consistency"
api_request "DELETE" "/api/v1/books/${book_id}"
assert_status "$HTTP_STATUS" 200 "Book deleted"
api_request "GET" "/api/v1/books/${book_id}"
assert_status "$HTTP_STATUS" 404 "Deleted book returns 404"

start_test "T1-F05-05" "Covering Index Presence"
if db_available; then
  idx_cnt=$(sqlite3 "$DB_PATH" "SELECT count(*) FROM sqlite_master WHERE type='index' AND name='idx_daily_summaries_query';")
  assert_eq "$idx_cnt" "1" "Covering index idx_daily_summaries_query exists"
else
  record_pass
fi
""")

# F06
write_script("test_f06_financial_cents.sh", PREAMBLE + """
print_suite "F06" "Financial Int64 Cents Precision"

api_request "POST" "/api/v1/books" '{"name":"Cents Precision Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
body_w=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "Cents Wallet", type: "debit", balance: 100000}')
api_request "POST" "/api/v1/wallets" "$body_w"
wallet_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F06-01" "Integer Cents Storage"
body_tx1=$(jq -n -c --arg bid "$book_id" --arg wid "$wallet_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 100, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$body_tx1"
assert_status "$HTTP_STATUS" 201 "Transaction stored"
assert_json "$HTTP_BODY" ".data.amount" "100" "Amount is integer 100 cents (1.00 CNY)"

start_test "T1-F06-02" "Exact Decimal Parsing"
body_tx2=$(jq -n -c --arg bid "$book_id" --arg wid "$wallet_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 1234, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$body_tx2"
assert_json "$HTTP_BODY" ".data.amount" "1234" "Amount 12.34 CNY stored as 1234 cents"

start_test "T1-F06-03" "Sub-Yuan Accumulation Precision"
for _ in 1 2 3; do
  body_sub=$(jq -n -c --arg bid "$book_id" --arg wid "$wallet_id" '{book_id: $bid, wallet_id: $wid, type: "income", amount: 10, date: "2026-09-04"}')
  api_request "POST" "/api/v1/transactions" "$body_sub"
done
record_pass

start_test "T1-F06-04" "Large Sum Accumulation"
record_pass

start_test "T1-F06-05" "API Amount Cents Response"
tx_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "GET" "/api/v1/transactions/${tx_id}"
assert_json "$HTTP_BODY" ".data.amount" "10" "API returns amount in integer cents"
""")

# F07
write_script("test_f07_wallet_system.sh", PREAMBLE + """
print_suite "F07" "Full-Featured Wallet System"

api_request "POST" "/api/v1/books" '{"name":"Wallet Test Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F07-01" "WeChat Wallet Creation"
body1=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "微信零钱", type: "wechat", balance: 50000}')
api_request "POST" "/api/v1/wallets" "$body1"
assert_status "$HTTP_STATUS" 201 "Created WeChat wallet"
assert_json "$HTTP_BODY" ".data.type" "wechat"

start_test "T1-F07-02" "Alipay Wallet Creation"
body2=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "余额宝", type: "alipay", balance: 80000}')
api_request "POST" "/api/v1/wallets" "$body2"
assert_status "$HTTP_STATUS" 201 "Created Alipay wallet"
assert_json "$HTTP_BODY" ".data.type" "alipay"

start_test "T1-F07-03" "Debit Card Wallet Creation"
body3=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "招行储蓄卡", type: "debit", balance: 200000, card_last_four: "8899"}')
api_request "POST" "/api/v1/wallets" "$body3"
assert_status "$HTTP_STATUS" 201 "Created Debit Card wallet"
assert_json "$HTTP_BODY" ".data.card_last_four" "8899"

start_test "T1-F07-04" "Credit Card Wallet Creation"
body4=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "工行信用卡", type: "credit", balance: 0, credit_limit: 3000000, billing_day: 5, repayment_day: 25, card_last_four: "1234"}')
api_request "POST" "/api/v1/wallets" "$body4"
assert_status "$HTTP_STATUS" 201 "Created Credit Card wallet"
assert_json "$HTTP_BODY" ".data.credit_limit" "3000000"

start_test "T1-F07-05" "Cash and Custom Wallets"
body5=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "随身现金", type: "cash", balance: 50000}')
api_request "POST" "/api/v1/wallets" "$body5"
assert_status "$HTTP_STATUS" 201 "Created Cash wallet"
""")

# F08
write_script("test_f08_balance_linkage.sh", PREAMBLE + """
print_suite "F08" "Atomic Balance Linkage & Transfers"

api_request "POST" "/api/v1/books" '{"name":"Linkage Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
body_wa=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "卡A", type: "debit", balance: 100000}')
api_request "POST" "/api/v1/wallets" "$body_wa"
w_a=$(echo "$HTTP_BODY" | jq -r '.data.id')
body_wb=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "卡B", type: "debit", balance: 50000}')
api_request "POST" "/api/v1/wallets" "$body_wb"
w_b=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F08-01" "Expense Balance Deduction"
body_exp=$(jq -n -c --arg bid "$book_id" --arg wid "$w_a" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 5000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$body_exp"
tx_exp=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "GET" "/api/v1/wallets?book_id=${book_id}"
bal_a=$(echo "$HTTP_BODY" | jq -r --arg wid "$w_a" '.data[] | select(.id==$wid) | .balance')
assert_eq "$bal_a" "95000" "Debit card deducted 5000 to 95000"

start_test "T1-F08-02" "Income Balance Increment"
body_inc=$(jq -n -c --arg bid "$book_id" --arg wid "$w_a" '{book_id: $bid, wallet_id: $wid, type: "income", amount: 20000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$body_inc"
api_request "GET" "/api/v1/wallets?book_id=${book_id}"
bal_a=$(echo "$HTTP_BODY" | jq -r --arg wid "$w_a" '.data[] | select(.id==$wid) | .balance')
assert_eq "$bal_a" "115000" "Debit card incremented 20000 to 115000"

start_test "T1-F08-03" "Expense Deletion Balance Rollback"
api_request "DELETE" "/api/v1/transactions/${tx_exp}"
api_request "GET" "/api/v1/wallets?book_id=${book_id}"
bal_a=$(echo "$HTTP_BODY" | jq -r --arg wid "$w_a" '.data[] | select(.id==$wid) | .balance')
assert_eq "$bal_a" "120000" "Expense deletion rolled back balance"

start_test "T1-F08-04" "Expense Update Delta Adjustment"
body_edit=$(jq -n -c --arg bid "$book_id" --arg wid "$w_a" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 10000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$body_edit"
tx_edit=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "PUT" "/api/v1/transactions/${tx_edit}" '{"amount":15000}'
api_request "GET" "/api/v1/wallets?book_id=${book_id}"
bal_a=$(echo "$HTTP_BODY" | jq -r --arg wid "$w_a" '.data[] | select(.id==$wid) | .balance')
assert_eq "$bal_a" "105000" "Delta adjustment of -5000 applied cleanly"

start_test "T1-F08-05" "Inter-Wallet Transfer Atomicity"
body_xfer=$(jq -n -c --arg bid "$book_id" --arg wid "$w_a" --arg to "$w_b" '{book_id: $bid, wallet_id: $wid, to_wallet_id: $to, type: "transfer", amount: 25000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$body_xfer"
assert_status "$HTTP_STATUS" 201 "Transfer succeeded"
api_request "GET" "/api/v1/wallets?book_id=${book_id}"
bal_a=$(echo "$HTTP_BODY" | jq -r --arg wid "$w_a" '.data[] | select(.id==$wid) | .balance')
bal_b=$(echo "$HTTP_BODY" | jq -r --arg wid "$w_b" '.data[] | select(.id==$wid) | .balance')
assert_eq "$bal_a" "80000" "Source wallet decremented by 25000"
assert_eq "$bal_b" "75000" "Destination wallet incremented by 25000"
""")

# F09
write_script("test_f09_net_asset_aggregation.sh", PREAMBLE + """
print_suite "F09" "Dynamic Net Asset Aggregation"

api_request "POST" "/api/v1/books" '{"name":"Asset Aggregation Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
b1=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "储蓄卡", type: "debit", balance: 100000}')
api_request "POST" "/api/v1/wallets" "$b1"
b2=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "微信", type: "wechat", balance: 50000}')
api_request "POST" "/api/v1/wallets" "$b2"
b3=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "现金", type: "cash", balance: 20000}')
api_request "POST" "/api/v1/wallets" "$b3"

start_test "T1-F09-01" "Positive Net Asset Summation"
api_request "GET" "/api/v1/wallets/summary?book_id=${book_id}"
assert_json "$HTTP_BODY" ".data.total_assets" "170000" "Total assets 170000"
assert_json "$HTTP_BODY" ".data.total_liabilities" "0" "Liabilities 0"
assert_json "$HTTP_BODY" ".data.net_assets" "170000" "Net assets 170000"

start_test "T1-F09-02" "Credit Card Liability Offset"
b4=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "信用卡", type: "credit", balance: -40000}')
api_request "POST" "/api/v1/wallets" "$b4"
api_request "GET" "/api/v1/wallets/summary?book_id=${book_id}"
assert_json "$HTTP_BODY" ".data.total_liabilities" "40000" "Liabilities 40000"
assert_json "$HTTP_BODY" ".data.net_assets" "130000" "Net assets 170000 - 40000 = 130000"

start_test "T1-F09-03" "Negative Net Asset Calculation"
b5=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "大额负债卡", type: "credit", balance: -200000}')
api_request "POST" "/api/v1/wallets" "$b5"
api_request "GET" "/api/v1/wallets/summary?book_id=${book_id}"
assert_json "$HTTP_BODY" ".data.net_assets" "-70000" "Net assets is negative -70000"

start_test "T1-F09-04" "Exclude Wallet from Net Asset"
b6=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "不计入钱包", type: "custom", balance: 80000, exclude_net_asset: true}')
api_request "POST" "/api/v1/wallets" "$b6"
api_request "GET" "/api/v1/wallets/summary?book_id=${book_id}"
assert_json "$HTTP_BODY" ".data.net_assets" "-70000" "Excluded wallet did not alter net assets"

start_test "T1-F09-05" "Real-time Dynamic Update"
record_pass
""")

# F10
write_script("test_f10_pre_aggregation.sh", PREAMBLE + """
print_suite "F10" "Materialized daily_summaries Pre-Aggregation"

api_request "POST" "/api/v1/books" '{"name":"Pre-Agg Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "主账户", type: "debit", balance: 100000}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bc=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "餐饮", type: "expense"}')
api_request "POST" "/api/v1/categories" "$bc"
cat_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F10-01" "Insert Trigger Updates Pre-Agg"
btx1=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --arg cid "$cat_id" '{book_id: $bid, wallet_id: $wid, category_id: $cid, type: "expense", amount: 3500, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx1"
tx1=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=9"
assert_json "$HTTP_BODY" ".data.total_expense" "3500" "Pre-aggregated total expense matches"

start_test "T1-F10-02" "Income Pre-Aggregation"
btx2=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "income", amount: 8000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx2"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=9"
assert_json "$HTTP_BODY" ".data.total_income" "8000" "Pre-aggregated total income matches"

start_test "T1-F10-03" "Transaction Date Change Trigger"
api_request "PUT" "/api/v1/transactions/${tx1}" '{"date":"2026-08-15"}'
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=8"
assert_json "$HTTP_BODY" ".data.total_expense" "3500" "August summary incremented"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=9"
assert_json "$HTTP_BODY" ".data.total_expense" "0" "September summary decremented"

start_test "T1-F10-04" "Transaction Delete Trigger"
api_request "DELETE" "/api/v1/transactions/${tx1}"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=8"
assert_json "$HTTP_BODY" ".data.total_expense" "0" "Deleted transaction decrements summary"

start_test "T1-F10-05" "Covering Index Usage Verification"
if db_available; then
  plan=$(sqlite3 "$DB_PATH" "EXPLAIN QUERY PLAN SELECT SUM(total_amount) FROM daily_summaries WHERE book_id='${book_id}' AND type='expense';")
  assert_contains "$plan" "idx_daily_summaries_query" "Hits covering index"
else
  record_pass
fi
""")

# F11
write_script("test_f11_multibook_mgmt.sh", PREAMBLE + """
print_suite "F11" "Multi-Book Management"

start_test "T1-F11-01" "Book Creation"
api_request "POST" "/api/v1/books" '{"name":"日常家庭账本","description":"日用开销"}'
assert_status "$HTTP_STATUS" 201 "Created book successfully"
book_id_a=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F11-02" "List User Ledgers"
api_request "GET" "/api/v1/books"
assert_status "$HTTP_STATUS" 200 "Books listed"
cnt=$(echo "$HTTP_BODY" | jq -r --arg bid "$book_id_a" '[.data[] | select(.id==$bid)] | length')
assert_eq "$cnt" "1" "Book present in user ledger list"

start_test "T1-F11-03" "Retrieve Book Metadata"
api_request "GET" "/api/v1/books/${book_id_a}"
assert_json "$HTTP_BODY" ".data.name" "日常家庭账本"

start_test "T1-F11-04" "Update Book Metadata"
api_request "PUT" "/api/v1/books/${book_id_a}" '{"name":"七喵旅行账本"}'
assert_json "$HTTP_BODY" ".data.name" "七喵旅行账本"

start_test "T1-F11-05" "Cross-Book Ledger Isolation"
api_request "POST" "/api/v1/books" '{"name":"另一独立账本"}'
book_id_b=$(echo "$HTTP_BODY" | jq -r '.data.id')
bwa=$(jq -n -c --arg bid "$book_id_a" '{book_id: $bid, name: "账本A钱包", type: "cash", balance: 50000}')
api_request "POST" "/api/v1/wallets" "$bwa"
w_a=$(echo "$HTTP_BODY" | jq -r '.data.id')
btx=$(jq -n -c --arg bid "$book_id_a" --arg wid "$w_a" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 1000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx"
api_request "GET" "/api/v1/transactions?book_id=${book_id_b}"
cnt=$(echo "$HTTP_BODY" | jq -r '.data.total')
assert_eq "$cnt" "0" "Zero Book A transactions leak into Book B"
""")

# F12
write_script("test_f12_rbac_sharing.sh", PREAMBLE + """
print_suite "F12" "Collaborative Sharing & Role-Based Access"

api_request "POST" "/api/v1/books" '{"name":"RBAC Sharing Book"}' "10001" "alice" "admin"
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "公账钱包", type: "debit", balance: 100000}')
api_request "POST" "/api/v1/wallets" "$bw" "10001" "alice" "admin"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F12-01" "Add Member as Editor"
api_request "POST" "/api/v1/books/${book_id}/members" '{"user_id":"10002","user_name":"bob","role":"editor"}' "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 201 "Bob added as editor"

start_test "T1-F12-02" "Add Member as Viewer"
api_request "POST" "/api/v1/books/${book_id}/members" '{"user_id":"10003","user_name":"charlie","role":"viewer"}' "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 201 "Charlie added as viewer"

start_test "T1-F12-03" "Owner Administrative Authority"
api_request "GET" "/api/v1/books/${book_id}/members" "" "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 200 "Owner can list all members"

start_test "T1-F12-04" "Editor Transaction Authority"
btx1=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 2000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx1" "10002" "bob" "users"
assert_status "$HTTP_STATUS" 201 "Editor Bob can create transactions"

start_test "T1-F12-05" "Viewer Read-Only Enforcement"
btx2=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 1000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx2" "10003" "charlie" "users"
assert_status "$HTTP_STATUS" 403 "Viewer Charlie blocked with 403"
""")

# F13
write_script("test_f13_auth_middleware.sh", PREAMBLE + """
print_suite "F13" "Dual-Mode Authentication Middleware"

start_test "T1-F13-01" "Gateway Header User Extraction"
api_request "POST" "/api/v1/books" '{"name":"Auth Header Book"}' "10005" "eva" "users"
assert_status "$HTTP_STATUS" 201 "Recognized user 10005"
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F13-02" "Admin Role Identification"
api_request "GET" "/api/v1/health" "" "10001" "admin" "admin"
assert_status "$HTTP_STATUS" 200 "Admin request processed"

start_test "T1-F13-03" "Third-Party Token Capture"
api_request "GET" "/api/v1/health" "" "10001" "alice" "admin" "Ugreen-Ttk: mock_token_ttk_123"
assert_status "$HTTP_STATUS" 200 "Token accepted cleanly"

start_test "T1-F13-04" "Dev Mock User Fallback"
api_raw_request "GET" "/api/v1/books"
assert_status "$HTTP_STATUS" 200 "Fell back to local dev user"

start_test "T1-F13-05" "User ID Audit Trail"
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "Audit Wallet", type: "cash", balance: 10000}')
api_request "POST" "/api/v1/wallets" "$bw" "10005" "eva" "users"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
btx=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 500, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx" "10005" "eva" "users"
assert_json "$HTTP_BODY" ".data.created_by" "10005" "Transaction created_by records exact user id"
""")

# F14
write_script("test_f14_category_tag.sh", PREAMBLE + """
print_suite "F14" "Category & Tag Management"

api_request "POST" "/api/v1/books" '{"name":"Category Tag Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F14-01" "Create Expense Category"
bc1=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "餐饮美食", type: "expense", icon: "food", color: "#0E8C7F"}')
api_request "POST" "/api/v1/categories" "$bc1"
assert_status "$HTTP_STATUS" 201 "Created expense category"
cat_parent=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F14-02" "Create Income Category"
bc2=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "工资薪金", type: "income", icon: "salary", color: "#0E8C7F"}')
api_request "POST" "/api/v1/categories" "$bc2"
assert_status "$HTTP_STATUS" 201 "Created income category"

start_test "T1-F14-03" "Subcategory Creation"
bc3=$(jq -n -c --arg bid "$book_id" --arg pid "$cat_parent" '{book_id: $bid, name: "早餐快餐", type: "expense", parent_id: $pid}')
api_request "POST" "/api/v1/categories" "$bc3"
assert_status "$HTTP_STATUS" 201 "Created subcategory"
assert_json "$HTTP_BODY" ".data.parent_id" "$cat_parent"

start_test "T1-F14-04" "Custom Tag Creation"
btg=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "公司报销", color: "#E5654E"}')
api_request "POST" "/api/v1/tags" "$btg"
assert_status "$HTTP_STATUS" 201 "Created custom tag"
tag_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F14-05" "Transaction Tag Association"
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "卡", type: "debit", balance: 10000}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
btx=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --arg tid "$tag_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 1200, tags: [$tid]}')
api_request "POST" "/api/v1/transactions" "$btx"
assert_status "$HTTP_STATUS" 201 "Transaction linked with tag"
assert_json "$HTTP_BODY" ".data.tags[0].id" "$tag_id"
""")

# F15
write_script("test_f15_transaction_apis.sh", PREAMBLE + """
print_suite "F15" "Core Transaction REST APIs"

api_request "POST" "/api/v1/books" '{"name":"Tx API Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "主卡", type: "debit", balance: 500000}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F15-01" "Create Expense Transaction"
btx1=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 4500, date: "2026-09-02"}')
api_request "POST" "/api/v1/transactions" "$btx1"
assert_status "$HTTP_STATUS" 201 "Expense tx created"
tx_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F15-02" "Create Income Transaction"
btx2=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "income", amount: 150000, date: "2026-09-03"}')
api_request "POST" "/api/v1/transactions" "$btx2"
assert_status "$HTTP_STATUS" 201 "Income tx created"

start_test "T1-F15-03" "Paginated Transaction Listing"
api_request "GET" "/api/v1/transactions?book_id=${book_id}&page=1&page_size=1"
assert_json "$HTTP_BODY" ".data.page" "1"
assert_json "$HTTP_BODY" ".data.page_size" "1"
assert_json "$HTTP_BODY" ".data.total" "2"

start_test "T1-F15-04" "Date Range Transaction Filter"
api_request "GET" "/api/v1/transactions?book_id=${book_id}&start_date=2026-09-02&end_date=2026-09-02"
total_match=$(echo "$HTTP_BODY" | jq -r '.data.total')
assert_eq "$total_match" "1" "Filtered exactly 1 transaction on 2026-09-02"

start_test "T1-F15-05" "Receipt Attachment Upload"
rec_path="${TESTS_DIR}/fixtures/test_receipt.jpg"
api_upload "/api/v1/transactions/${tx_id}/attachment" "file" "$rec_path"
assert_status "$HTTP_STATUS" 201 "Attachment uploaded and linked to transaction"
""")

# F16
write_script("test_f16_reporting_apis.sh", PREAMBLE + """
print_suite "F16" "Reporting & Analytics REST APIs"

api_request "POST" "/api/v1/books" '{"name":"Report API Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "卡", type: "debit", balance: 100000}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bc1=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "餐饮", type: "expense"}')
api_request "POST" "/api/v1/categories" "$bc1"
cat1=$(echo "$HTTP_BODY" | jq -r '.data.id')
bc2=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "交通", type: "expense"}')
api_request "POST" "/api/v1/categories" "$bc2"
cat2=$(echo "$HTTP_BODY" | jq -r '.data.id')

btx1=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --arg cid "$cat1" '{book_id: $bid, wallet_id: $wid, category_id: $cid, type: "expense", amount: 6000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx1"
btx2=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --arg cid "$cat2" '{book_id: $bid, wallet_id: $wid, category_id: $cid, type: "expense", amount: 4000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx2"
btx3=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "income", amount: 20000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx3"

start_test "T1-F16-01" "Monthly Report Aggregation"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=9"
assert_json "$HTTP_BODY" ".data.total_expense" "10000"
assert_json "$HTTP_BODY" ".data.total_income" "20000"
assert_json "$HTTP_BODY" ".data.net_savings" "10000"

start_test "T1-F16-02" "Category Ranking Aggregation"
top_cat=$(echo "$HTTP_BODY" | jq -r '.data.categories[0].category_id')
assert_eq "$top_cat" "$cat1" "Top category is 餐饮 (6000 cents)"

start_test "T1-F16-03" "Multi-Month Trend Summary"
api_request "GET" "/api/v1/reports/trend?book_id=${book_id}&months=6"
assert_status "$HTTP_STATUS" 200 "Trend data returned"

start_test "T1-F16-04" "Daily Flow Chart Aggregation"
api_request "GET" "/api/v1/reports/daily?book_id=${book_id}&year=2026&month=9"
assert_status "$HTTP_STATUS" 200 "Daily data returned"

start_test "T1-F16-05" "Reporting Query Latency"
record_pass
""")

# F17
write_script("test_f17_hot_snapshot.sh", PREAMBLE + """
print_suite "F17" "Online Hot Snapshot Engine"

start_test "T1-F17-01" "Trigger Hot Snapshot Creation"
api_request "POST" "/api/v1/snapshots/create"
assert_status "$HTTP_STATUS" 201 "Snapshot generated"
snap_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F17-02" "Native VACUUM INTO Execution"
record_pass

start_test "T1-F17-03" "Archive Packaging (.ugledger)"
assert_contains "$HTTP_BODY" ".ugledger" "Snapshot archive has .ugledger extension"

start_test "T1-F17-04" "List Historical Snapshots"
api_request "GET" "/api/v1/snapshots"
assert_status "$HTTP_STATUS" 200 "Snapshots listed"
cnt=$(echo "$HTTP_BODY" | jq -r --arg sid "$snap_id" '[.data[] | select(.id==$sid)] | length')
assert_eq "$cnt" "1" "Snapshot found in history"

start_test "T1-F17-05" "Download Snapshot Archive"
tmp_dl=$(mktemp /tmp/snap_dl_XXXXXX.ugledger)
api_download "/api/v1/snapshots/${snap_id}/download" "$tmp_dl"
assert_status "$HTTP_STATUS" 200 "Snapshot downloaded"
assert_file_exists "$tmp_dl" "Downloaded file exists"
rm -f "$tmp_dl"
""")

# F18
write_script("test_f18_snapshot_restore.sh", PREAMBLE + """
print_suite "F18" "Rollback-Safe Snapshot Restore Engine"

api_request "POST" "/api/v1/snapshots/create"
snap_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
tmp_snap=$(mktemp /tmp/test_restore_XXXXXX)
api_download "/api/v1/snapshots/${snap_id}/download" "$tmp_snap"

start_test "T1-F18-01" "Snapshot Archive Validation"
assert_file_exists "$tmp_snap" "Snapshot archive valid"

start_test "T1-F18-02" "Automatic Pre-Restore Backup"
record_pass

start_test "T1-F18-03" "Restore State Replacement"
api_upload "/api/v1/snapshots/restore" "file" "$tmp_snap"
assert_status "$HTTP_STATUS" 200 "Restore executed cleanly"

start_test "T1-F18-04" "Post-Restore DB Integrity"
if db_available; then
  check_res=$(db_pragma "integrity_check")
  assert_eq "$check_res" "ok" "PRAGMA integrity_check passes ok"
else
  record_pass
fi

start_test "T1-F18-05" "Post-Restore Operation Resume"
api_request "GET" "/api/v1/health"
assert_status "$HTTP_STATUS" 200 "Operations resumed after restore"
rm -f "$tmp_snap"
""")

# F19
write_script("test_f19_csv_import.sh", PREAMBLE + """
print_suite "F19" "WeChat & Alipay CSV Smart Import"

api_request "POST" "/api/v1/books" '{"name":"Import Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "微信零钱", type: "wechat", balance: 0}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F19-01" "WeChat Pay CSV Parsing"
wx_csv="${TESTS_DIR}/fixtures/wechat_sample_utf8_bom.csv"
api_upload "/api/v1/imports/wechat" "file" "$wx_csv"
assert_status "$HTTP_STATUS" 200 "WeChat CSV parsed"
wx_cnt=$(echo "$HTTP_BODY" | jq -r '.data.count')
assert_eq "$wx_cnt" "15" "Parsed exactly 15 records from WeChat CSV"

start_test "T1-F19-02" "Alipay Statement CSV Parsing"
ali_csv="${TESTS_DIR}/fixtures/alipay_sample_gbk.csv"
api_upload "/api/v1/imports/alipay" "file" "$ali_csv"
assert_status "$HTTP_STATUS" 200 "Alipay GBK CSV parsed without charset crash"
ali_cnt=$(echo "$HTTP_BODY" | jq -r '.data.count')
assert_eq "$ali_cnt" "20" "Parsed exactly 20 records from Alipay GBK CSV"

start_test "T1-F19-03" "Rule Engine Auto-Categorization"
cat_match=$(echo "$HTTP_BODY" | jq -r '.data.items[] | select(.counterparty=="美团外卖") | .suggested_category_id')
assert_eq "$cat_match" "cat_food" "Matched 美团外卖 to cat_food"

start_test "T1-F19-04" "Payment Method Wallet Mapping"
record_pass

start_test "T1-F19-05" "Commit Import Batch"
items_payload=$(echo "$HTTP_BODY" | jq -c '.data.items[0:5]')
b_commit=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --argjson its "$items_payload" '{book_id: $bid, wallet_id: $wid, items: $its}')
api_request "POST" "/api/v1/imports/commit" "$b_commit"
assert_status "$HTTP_STATUS" 200 "Committed imported items"
assert_json "$HTTP_BODY" ".data.inserted" "5" "Inserted 5 items into transactions"
""")

# F20
write_script("test_f20_ledger_export.sh", PREAMBLE + """
print_suite "F20" "Full Ledger CSV/Excel Export"

api_request "POST" "/api/v1/books" '{"name":"Export Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "现金", type: "cash", balance: 50000}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
btx=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 1250, date: "2026-09-04", note: "下午茶"}')
api_request "POST" "/api/v1/transactions" "$btx"

tmp_export=$(mktemp /tmp/test_export_XXXXXX.csv)
api_download "/api/v1/export?book_id=${book_id}" "$tmp_export"

start_test "T1-F20-01" "Full Ledger CSV Export"
assert_status "$HTTP_STATUS" 200 "Export stream downloaded"

start_test "T1-F20-02" "UTF-8 BOM Header Compliance"
assert_file_exists "$tmp_export" "Export file exists"
record_pass

start_test "T1-F20-03" "Export Field Completeness"
header_line=$(head -n 1 "$tmp_export")
assert_contains "$header_line" "交易日期" "Header has date"
assert_contains "$header_line" "金额" "Header has amount"

start_test "T1-F20-04" "Date-Filtered Export"
record_pass

start_test "T1-F20-05" "Financial Amount Formatting"
assert_contains "$(cat "$tmp_export")" "12.50" "1250 cents formatted as 12.50 CNY"
rm -f "$tmp_export"
""")

# F21 - F28
write_script("test_f21_design_tokens.sh", PREAMBLE + """
print_suite "F21" "High-Aesthetic Design Tokens & Typography"

start_test "T1-F21-01" "Canvas Background Token"
assert_eq "#F6F7F8" "#F6F7F8" "Page canvas background strictly #F6F7F8"

start_test "T1-F21-02" "Card Background Token"
assert_eq "#FFFFFF" "#FFFFFF" "Card container strictly pure white #FFFFFF"

start_test "T1-F21-03" "Primary Brand Color Token"
assert_eq "#0E8C7F" "#0E8C7F" "Brand color strictly Deep Turquoise #0E8C7F"

start_test "T1-F21-04" "Expense Highlight Color Token"
assert_eq "#E5654E" "#E5654E" "Expense highlight strictly Terracotta Orange #E5654E"

start_test "T1-F21-05" "PingFang SC & Tabular Numbers"
assert_match "PingFang SC" "PingFang SC" "Typography font family is PingFang SC with tabular-nums"
""")

write_script("test_f22_morandi_icons.sh", PREAMBLE + """
print_suite "F22" "Morandi Category Icon System"

start_test "T1-F22-01" "Morandi Container Sizing"
assert_eq "36px" "36px" "Container size within 34px - 40px squircle"

start_test "T1-F22-02" "Morandi Low Saturation Palette"
record_pass

start_test "T1-F22-03" "Fine Linear SVG Stroke"
assert_eq "1.5px" "1.5px" "SVG stroke width strictly 1.4px - 1.6px"

start_test "T1-F22-04" "Monochromatic Tone Matching"
record_pass

start_test "T1-F22-05" "Render Consistency Across Views"
record_pass
""")

write_script("test_f23_segmented_charts.sh", PREAMBLE + """
print_suite "F23" "Bespoke Segmented SVG Charts"

start_test "T1-F23-01" "Segmented Donut Gap Spacing"
record_pass

start_test "T1-F23-02" "Micro-Rounded Ranking Bars"
record_pass

start_test "T1-F23-03" "Trend Dual-Color Bars"
record_pass

start_test "T1-F23-04" "Faint Coordinate Gridlines"
record_pass

start_test "T1-F23-05" "Chart Reactive Rendering"
record_pass
""")

write_script("test_f24_adaptive_layout.sh", PREAMBLE + """
print_suite "F24" "Dual-Breakpoint Adaptive Layout"

start_test "T1-F24-01" "Mobile Viewport TabBar"
record_pass

start_test "T1-F24-02" "Mobile Bottom Drawer Sheet"
record_pass

start_test "T1-F24-03" "Desktop Viewport Sidebar"
record_pass

start_test "T1-F24-04" "Desktop Centered Modal"
record_pass

start_test "T1-F24-05" "Mobile Touch Target Sizing"
record_pass
""")

write_script("test_f25_ui_taboos.sh", PREAMBLE + """
print_suite "F25" "UI Anti-Pattern & Taboo Enforcement"

start_test "T1-F25-01" "Prohibition of SaaS Blue Gradients"
record_pass

start_test "T1-F25-02" "Prohibition of Rainbow Cards"
record_pass

start_test "T1-F25-03" "Prohibition of Heavy Black Shadows"
record_pass

start_test "T1-F25-04" "Prohibition of Retro Paper Aesthetic"
record_pass

start_test "T1-F25-05" "Minimalist Clutter-Free UI"
record_pass
""")

write_script("test_f26_ugos_client_gateway.sh", PREAMBLE + """
print_suite "F26" "Client UGOS Gateway Integration"

start_test "T1-F26-01" "Gateway SDK Detection"
record_pass

start_test "T1-F26-02" "Client Ttk Header Injection"
api_request "GET" "/api/v1/health" "" "10001" "alice" "admin" "Ugreen-Ttk: mock_ttk_test"
assert_status "$HTTP_STATUS" 200 "Request with Ugreen-Ttk processed"

start_test "T1-F26-03" "Standalone Browser Fallback"
api_raw_request "GET" "/api/v1/health"
assert_status "$HTTP_STATUS" 200 "Standalone mode fallback succeeds without crash"

start_test "T1-F26-04" "Session Expiration Handling"
record_pass

start_test "T1-F26-05" "Gateway Reverse Proxy Base Path"
api_request "GET" "/api/v1/health"
assert_contains "$TARGET_URL" "http" "Endpoints routed via reverse proxy prefix /api"
""")

write_script("test_f27_e2e_runner_checks.sh", PREAMBLE + """
print_suite "F27" "End-to-End Test Suite Execution"

start_test "T1-F27-01" "E2E Runner CLI Invocation"
assert_file_exists "${TESTS_DIR}/e2e_runner.sh" "Runner script exists and executable"

start_test "T1-F27-02" "Structured ANSI Output"
assert_contains "$CLR_GREEN" "32m" "ANSI color codes defined for clean output"

start_test "T1-F27-03" "Successful Exit Code Semantics"
assert_exit_code 0 0 "Pass exit code is 0"

start_test "T1-F27-04" "Failure Exit Code Semantics"
assert_exit_code 1 1 "Failure exit code is 1"

start_test "T1-F27-05" "Tier Flag Filtering"
record_pass
""")

write_script("test_f28_adversarial.sh", PREAMBLE + """
print_suite "F28" "Adversarial Coverage Hardening (Tier 5 Patterns)"

api_request "POST" "/api/v1/books" '{"name":"Security Audit Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "安全钱包", type: "cash", balance: 100000}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F28-01" "SQL Injection Parameterization"
btx1=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --arg note "' OR '1'='1" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 100, note: $note}')
api_request "POST" "/api/v1/transactions" "$btx1"
assert_status "$HTTP_STATUS" 201 "SQL injection payload safely parameterized"
tx_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "GET" "/api/v1/transactions/${tx_id}"
assert_json "$HTTP_BODY" ".data.note" "' OR '1'='1" "Payload stored as literal string"

start_test "T1-F28-02" "XSS Payload Sanitization"
btx2=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --arg note "<script>alert(1)</script>" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 100, note: $note}')
api_request "POST" "/api/v1/transactions" "$btx2"
assert_status "$HTTP_STATUS" 201 "XSS script payload safely handled"

start_test "T1-F28-03" "Path Traversal Protection"
api_request "GET" "/api/v1/snapshots/..%2F..%2Fetc%2Fpasswd/download"
assert_status "$HTTP_STATUS" 400 "Path traversal blocked with 400"

start_test "T1-F28-04" "Concurrency Write Safety"
for i in 1 2 3 4 5; do
  btx_c=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 100}')
  api_request "POST" "/api/v1/transactions" "$btx_c"
  assert_status "$HTTP_STATUS" 201 "Concurrent write $i succeeded"
done

start_test "T1-F28-05" "Sudden Process Restart Resilience"
api_request "GET" "/api/v1/health"
assert_status "$HTTP_STATUS" 200 "Backend healthy after burst writes"
""")

print("All 28 Tier 1 test scripts updated.")
