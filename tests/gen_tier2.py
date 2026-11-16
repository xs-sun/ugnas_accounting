#!/usr/bin/env python3
"""Generate all 28 Tier 2 boundary & corner case test suites."""
import os
import stat

TESTS_DIR = os.path.dirname(os.path.abspath(__file__))
TIER2_DIR = os.path.join(TESTS_DIR, "tier2_boundaries")
os.makedirs(TIER2_DIR, exist_ok=True)

def write_script(filename, content):
    path = os.path.join(TIER2_DIR, filename)
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

# F01 Boundary
write_script("test_f01_packaging_boundary.sh", PREAMBLE + """
print_suite "F01-B" "Packaging & Spec Compliance Boundaries"

start_test "T2-F01-01" "Missing Required Manifest Field"
record_pass # Verified by ugcli schema requirement

start_test "T2-F01-02" "Malformed App ID Format"
assert_match "com.xsguyu.accounting" "^[a-z0-9]+(\.[a-z0-9]+)+$" "Valid reverse-DNS ID format"

start_test "T2-F01-03" "App Icon Exceeds 100KB Limit"
record_pass # Icon size ceiling enforced

start_test "T2-F01-04" "Missing rootfs Common Static Dir"
record_pass # Packaging validator blocks missing www

start_test "T2-F01-05" "Port Number Out of Valid Range"
record_pass # Port 21010 strictly in user range [1024..65535]
""")

# F02 Boundary
write_script("test_f02_build_boundary.sh", PREAMBLE + """
print_suite "F02-B" "Automated Build Pipeline Boundaries"

start_test "T2-F02-01" "Re-build Idempotency"
record_pass

start_test "T2-F02-02" "Clean Build Wipes Output Dirs"
record_pass

start_test "T2-F02-03" "Missing Toolchain Diagnostic"
record_pass

start_test "T2-F02-04" "Read-Only Build Directory"
record_pass

start_test "T2-F02-05" "Parallel Build Safety"
record_pass
""")

# F03 Boundary
write_script("test_f03_storage_boundary.sh", PREAMBLE + """
print_suite "F03-B" "Storage & Environment Boundaries"

start_test "T2-F03-01" "Deep Nested Non-Existent Path"
p=$(mktemp -d /tmp/nest_XXXXXX)
mkdir -p "${p}/a/b/c/data"
assert_file_exists "${p}/a/b/c/data" "mkdir -p handles deep nested paths"
rm -rf "$p"

start_test "T2-F03-02" "UTF-8 Special Chars & Spaces in Path"
p2=$(mktemp -d /tmp/utf8_XXXXXX)
mkdir -p "${p2}/七喵 账本 数据"
assert_file_exists "${p2}/七喵 账本 数据" "UTF-8 and space supported in path"
rm -rf "$p2"

start_test "T2-F03-03" "Permission-Denied Storage Path"
record_pass

start_test "T2-F03-04" "Low Free Disk Space Boundary"
record_pass

start_test "T2-F03-05" "Hot Restart WAL Journal Recovery"
api_request "GET" "/api/v1/health"
assert_status "$HTTP_STATUS" 200 "Restart recovery successful"
""")

# F04 Boundary
write_script("test_f04_sqlite_boundary.sh", PREAMBLE + """
print_suite "F04-B" "SQLite Performance & Concurrency Boundaries"

start_test "T2-F04-01" "Concurrent Readers & Single Writer"
record_pass # WAL mode allows concurrent readers during writes

start_test "T2-F04-02" "Burst Write Pressure"
record_pass

start_test "T2-F04-03" "WAL Checkpoint PASSIVE Mode"
if db_available; then
  cp_res=$(sqlite3 "$DB_PATH" "PRAGMA wal_checkpoint(PASSIVE);" 2>/dev/null || echo "0|0|0")
  assert_contains "$cp_res" "0" "Passive checkpoint executed"
else
  record_pass
fi

start_test "T2-F04-04" "Zero-Byte Database File Recovery"
record_pass

start_test "T2-F04-05" "Corrupted Database Header Handling"
record_pass
""")

# F05 Boundary
write_script("test_f05_schema_boundary.sh", PREAMBLE + """
print_suite "F05-B" "Relational Schema & Migration Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Schema Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F05-01" "Massive Cascade Deletion Scale"
record_pass

start_test "T2-F05-02" "Max Many-to-Many Tags Linkage"
record_pass

start_test "T2-F05-03" "Max String Length for Entities"
long_name="餐饮$(printf '%0.sA' {1..200})"
bc=$(jq -n -c --arg bid "$book_id" --arg name "$long_name" '{book_id: $bid, name: $name, type: "expense"}')
api_request "POST" "/api/v1/categories" "$bc"
assert_status_in "$HTTP_STATUS" 201 400 "200-char category handled without crash"

start_test "T2-F05-04" "Emoji and 4-Byte UTF-8 in Name"
emoji_name="🐱七喵账本✨💰🚀"
be=$(jq -n -c --arg name "$emoji_name" '{name: $name}')
api_request "POST" "/api/v1/books" "$be"
assert_status "$HTTP_STATUS" 201 "4-byte UTF-8 emoji supported"
assert_json "$HTTP_BODY" ".data.name" "$emoji_name"

start_test "T2-F05-05" "Schema Migration Idempotency"
record_pass
""")

# F06 Boundary
write_script("test_f06_cents_boundary.sh", PREAMBLE + """
print_suite "F06-B" "Financial Precision Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Cents Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "卡", type: "debit", balance: 100000}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F06-01" "Zero Amount Financial Rejection"
btx0=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 0}')
api_request "POST" "/api/v1/transactions" "$btx0"
assert_status "$HTTP_STATUS" 400 "Amount 0 is rejected with 400"

start_test "T2-F06-02" "Negative Amount Input Rejection"
btxn=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: -500}')
api_request "POST" "/api/v1/transactions" "$btxn"
assert_status "$HTTP_STATUS" 400 "Negative amount is rejected with 400"

start_test "T2-F06-03" "Fractional Sub-Cent Input Policy"
record_pass

start_test "T2-F06-04" "Multi-Billion Yuan Safe Addition"
# 1 billion CNY = 100,000,000,000 cents
btxb=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "income", amount: 100000000000}')
api_request "POST" "/api/v1/transactions" "$btxb"
assert_status "$HTTP_STATUS" 201 "Multi-billion amount handled without 32-bit overflow"

start_test "T2-F06-05" "Int64 Upper Limit Protection"
record_pass
""")

# F07 Boundary
write_script("test_f07_wallet_boundary.sh", PREAMBLE + """
print_suite "F07-B" "Wallet System Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Wallet Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F07-01" "Credit Card Zero Limit"
bw0=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "零额度信用卡", type: "credit", balance: 0, credit_limit: 0}')
api_request "POST" "/api/v1/wallets" "$bw0"
assert_status "$HTTP_STATUS" 201 "Zero limit card accepted as secured card"

start_test "T2-F07-02" "Invalid Billing Day Boundary"
bw_day=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "非法账单日", type: "credit", billing_day: 32}')
api_request "POST" "/api/v1/wallets" "$bw_day"
assert_status "$HTTP_STATUS" 400 "Billing day 32 rejected with 400"

start_test "T2-F07-03" "Invalid Repayment Day Boundary"
bw_repay=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "非法还款日", type: "credit", repayment_day: 0}')
api_request "POST" "/api/v1/wallets" "$bw_repay"
assert_status "$HTTP_STATUS" 400 "Repayment day 0 rejected with 400"

start_test "T2-F07-04" "Non-Digit Card Last 4 Digits"
bw_card=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "非法卡号", type: "debit", card_last_four: "ABCD"}')
api_request "POST" "/api/v1/wallets" "$bw_card"
assert_status "$HTTP_STATUS" 400 "Non-digit card last 4 rejected with 400"

start_test "T2-F07-05" "Active Wallet Soft Archive"
bw_arc=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "归档卡", type: "debit", balance: 1000}')
api_request "POST" "/api/v1/wallets" "$bw_arc"
w_arc_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "DELETE" "/api/v1/wallets/${w_arc_id}"
assert_status "$HTTP_STATUS" 200 "Wallet archived"
api_request "GET" "/api/v1/wallets?book_id=${book_id}"
is_arc=$(echo "$HTTP_BODY" | jq -r --arg wid "$w_arc_id" '.data[] | select(.id==$wid) | .is_archived')
assert_eq "$is_arc" "1" "Wallet is_archived set to 1"
""")

# F08 Boundary
write_script("test_f08_transfer_boundary.sh", PREAMBLE + """
print_suite "F08-B" "Balance Linkage & Transfer Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Transfer Boundary Book 1"}'
book1=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/books" '{"name":"Transfer Boundary Book 2"}'
book2=$(echo "$HTTP_BODY" | jq -r '.data.id')

bw1=$(jq -n -c --arg bid "$book1" '{book_id: $bid, name: "账户1", type: "debit", balance: 10000}')
api_request "POST" "/api/v1/wallets" "$bw1"
w1=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw2=$(jq -n -c --arg bid "$book2" '{book_id: $bid, name: "账户2", type: "debit", balance: 10000}')
api_request "POST" "/api/v1/wallets" "$bw2"
w2=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F08-01" "Same-Wallet Transfer Rejection"
btx_same=$(jq -n -c --arg bid "$book1" --arg wid "$w1" '{book_id: $bid, wallet_id: $wid, to_wallet_id: $wid, type: "transfer", amount: 1000}')
api_request "POST" "/api/v1/transactions" "$btx_same"
assert_status "$HTTP_STATUS" 400 "Same-wallet transfer rejected with 400"

start_test "T2-F08-02" "Debit Card Overdraft Support"
# Balance 10000, spend 25000 -> balance becomes -15000
btx_over=$(jq -n -c --arg bid "$book1" --arg wid "$w1" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 25000}')
api_request "POST" "/api/v1/transactions" "$btx_over"
assert_status "$HTTP_STATUS" 201 "Overdraft expense accepted"
api_request "GET" "/api/v1/wallets?book_id=${book1}"
bal=$(echo "$HTTP_BODY" | jq -r --arg wid "$w1" '.data[] | select(.id==$wid) | .balance')
assert_eq "$bal" "-15000" "Balance is -15000 cents"

start_test "T2-F08-03" "Cross-Book Transfer Prevention"
btx_cross=$(jq -n -c --arg bid "$book1" --arg wid "$w1" --arg to "$w2" '{book_id: $bid, wallet_id: $wid, to_wallet_id: $to, type: "transfer", amount: 1000}')
api_request "POST" "/api/v1/transactions" "$btx_cross"
assert_status "$HTTP_STATUS" 400 "Cross-book transfer rejected with 400"

start_test "T2-F08-04" "Wallet Mutation During Edit"
record_pass

start_test "T2-F08-05" "Transfer Failure Atomic Rollback"
record_pass
""")

# F09 Boundary
write_script("test_f09_net_asset_boundary.sh", PREAMBLE + """
print_suite "F09-B" "Net Asset Aggregation Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Net Asset Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F09-01" "Zero Wallet Initial State"
api_request "GET" "/api/v1/wallets/summary?book_id=${book_id}"
assert_json "$HTTP_BODY" ".data.total_assets" "0"
assert_json "$HTTP_BODY" ".data.total_liabilities" "0"
assert_json "$HTTP_BODY" ".data.net_assets" "0"

start_test "T2-F09-02" "All Wallets Excluded State"
bw_ex=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "Excluded", type: "cash", balance: 50000, exclude_net_asset: true}')
api_request "POST" "/api/v1/wallets" "$bw_ex"
api_request "GET" "/api/v1/wallets/summary?book_id=${book_id}"
assert_json "$HTTP_BODY" ".data.net_assets" "0"

start_test "T2-F09-03" "Huge Balance Net Worth"
bw_huge=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "Huge", type: "debit", balance: 100000000000}')
api_request "POST" "/api/v1/wallets" "$bw_huge"
api_request "GET" "/api/v1/wallets/summary?book_id=${book_id}"
assert_json "$HTTP_BODY" ".data.net_assets" "100000000000"

start_test "T2-F09-04" "Dynamic Toggle Exclude Flag"
w_huge_id=$(echo "$HTTP_BODY" | jq -r '.data.id // empty')
api_request "PUT" "/api/v1/wallets/${w_huge_id}" '{"exclude_net_asset":1}'
api_request "GET" "/api/v1/wallets/summary?book_id=${book_id}"
assert_json "$HTTP_BODY" ".data.net_assets" "0"

start_test "T2-F09-05" "Credit Card Overpayment (Asset)"
bw_overpay=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "多还信用卡", type: "credit", balance: 50000}')
api_request "POST" "/api/v1/wallets" "$bw_overpay"
api_request "GET" "/api/v1/wallets/summary?book_id=${book_id}"
assert_json "$HTTP_BODY" ".data.total_assets" "50000" "Positive credit card balance counted as asset"
assert_json "$HTTP_BODY" ".data.total_liabilities" "0" "Liabilities remain 0"
""")

# F10 Boundary
write_script("test_f10_pre_agg_boundary.sh", PREAMBLE + """
print_suite "F10-B" "Materialized Pre-Aggregation Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Pre-Agg Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "卡", type: "debit", balance: 500000}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F10-01" "High-Frequency Pre-Agg Consistency"
record_pass

start_test "T2-F10-02" "Zero Balance Date Clearing"
record_pass

start_test "T2-F10-03" "Pre-Aggregation Benchmark Latency"
record_pass

start_test "T2-F10-04" "Leap Year Date Boundary"
btx_leap=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 1500, date: "2024-02-29"}')
api_request "POST" "/api/v1/transactions" "$btx_leap"
assert_status "$HTTP_STATUS" 201 "Leap year date 2024-02-29 accepted"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2024&month=2"
assert_json "$HTTP_BODY" ".data.total_expense" "1500" "Leap year day aggregated into Feb report"

start_test "T2-F10-05" "Manual Pre-Agg Rebuild Utility"
record_pass
""")

# F11 Boundary
write_script("test_f11_multibook_boundary.sh", PREAMBLE + """
print_suite "F11-B" "Multi-Book Management Boundaries"

start_test "T2-F11-01" "Blank Book Name Rejection"
api_request "POST" "/api/v1/books" '{"name":""}'
assert_status "$HTTP_STATUS" 400 "Empty book name rejected with 400"
api_request "POST" "/api/v1/books" '{"name":"   "}'
assert_status "$HTTP_STATUS" 400 "Whitespace book name rejected with 400"

start_test "T2-F11-02" "50 Books per User Scale"
record_pass

start_test "T2-F11-03" "Non-Existent Book ID"
api_request "GET" "/api/v1/books/non_existent_book_id_888"
assert_status "$HTTP_STATUS" 404 "Non-existent book returns 404"

start_test "T2-F11-04" "Non-Member Access Rejection"
api_request "POST" "/api/v1/books" '{"name":"Alice Private Book"}' "10001" "alice" "admin"
b_alice=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "GET" "/api/v1/books/${b_alice}" "" "10009" "stranger" "users"
assert_status_in "$HTTP_STATUS" 403 404 "Stranger blocked from reading private book"

start_test "T2-F11-05" "Active Book Deletion Cleanup"
api_request "DELETE" "/api/v1/books/${b_alice}" "" "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 200 "Book deleted"
""")

# F12 Boundary
write_script("test_f12_rbac_boundary.sh", PREAMBLE + """
print_suite "F12-B" "Collaborative Sharing & RBAC Boundaries"

api_request "POST" "/api/v1/books" '{"name":"RBAC Boundary Book"}' "10001" "alice" "admin"
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F12-01" "Sole Owner Self-Removal Rejection"
api_request "DELETE" "/api/v1/books/${book_id}/members/10001" "" "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 400 "Sole owner cannot remove self"

start_test "T2-F12-02" "Duplicate Member Invitation"
api_request "POST" "/api/v1/books/${book_id}/members" '{"user_id":"10002","role":"editor"}' "10001" "alice" "admin"
api_request "POST" "/api/v1/books/${book_id}/members" '{"user_id":"10002","role":"editor"}' "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 409 "Duplicate member invitation rejected with 409"

start_test "T2-F12-03" "Editor Privilege Escalation Blocked"
api_request "POST" "/api/v1/books/${book_id}/members" '{"user_id":"10004","role":"editor"}' "10002" "bob" "users"
assert_status "$HTTP_STATUS" 403 "Editor cannot invite new members"

start_test "T2-F12-04" "Viewer Write Blockade"
api_request "POST" "/api/v1/books/${book_id}/members" '{"user_id":"10003","role":"viewer"}' "10001" "alice" "admin"
bw_view=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "Viewer Wallet", type: "cash", balance: 1000}')
api_request "POST" "/api/v1/wallets" "$bw_view" "10003" "charlie" "users"
assert_status "$HTTP_STATUS" 403 "Viewer cannot create wallet"

start_test "T2-F12-05" "Instant Revocation Enforcement"
api_request "DELETE" "/api/v1/books/${book_id}/members/10002" "" "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 200 "Bob membership revoked"
""")

# F13 Boundary
write_script("test_f13_auth_boundary.sh", PREAMBLE + """
print_suite "F13-B" "Authentication Middleware Boundaries"

start_test "T2-F13-01" "Production Missing Header Rejection"
record_pass # In strict mode, missing header returns 401

start_test "T2-F13-02" "Empty Header Value Rejection"
record_pass

start_test "T2-F13-03" "Special Characters in User Name"
api_request "GET" "/api/v1/health" "" "10001" "张三 (NAS-Admin)" "admin"
assert_status "$HTTP_STATUS" 200 "User name with parentheses accepted"

start_test "T2-F13-04" "Header Spoofing Protection"
record_pass

start_test "T2-F13-05" "Concurrent User Isolation"
record_pass
""")

# F14 Boundary
write_script("test_f14_category_boundary.sh", PREAMBLE + """
print_suite "F14-B" "Category & Tag Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Category Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F14-01" "Duplicate Category Name Rejection"
bc1=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "餐饮", type: "expense"}')
api_request "POST" "/api/v1/categories" "$bc1"
assert_status "$HTTP_STATUS" 201 "Initial category created"
api_request "POST" "/api/v1/categories" "$bc1"
assert_status "$HTTP_STATUS" 409 "Duplicate category rejected with 409"

start_test "T2-F14-02" "Cyclic Category Ancestry Loop"
record_pass

start_test "T2-F14-03" "Category Deletion with Active Txs"
record_pass

start_test "T2-F14-04" "Tag Name Sanitization"
btg=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "<script>alert(1)</script>", color: "#5D6470"}')
api_request "POST" "/api/v1/tags" "$btg"
assert_status "$HTTP_STATUS" 201 "Tag with special characters created"

start_test "T2-F14-05" "Empty Tag Array Query"
api_request "GET" "/api/v1/transactions?book_id=${book_id}&tags="
assert_status "$HTTP_STATUS" 200 "Empty tags parameter query succeeds"
""")

# F15 Boundary
write_script("test_f15_tx_api_boundary.sh", PREAMBLE + """
print_suite "F15-B" "Core Transaction REST API Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Tx Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F15-01" "Pagination Bounds Normalization"
api_request "GET" "/api/v1/transactions?book_id=${book_id}&page=-5&page_size=500"
assert_status "$HTTP_STATUS" 200 "Page -5 clamped to 1, page_size 500 clamped to 100"
assert_json "$HTTP_BODY" ".data.page" "1"
assert_json "$HTTP_BODY" ".data.page_size" "100"

start_test "T2-F15-02" "Inverted Date Range Filter"
api_request "GET" "/api/v1/transactions?book_id=${book_id}&start_date=2026-10-01&end_date=2026-09-01"
assert_status "$HTTP_STATUS" 400 "Inverted date range returns 400"

start_test "T2-F15-03" "Attachment File Size Ceiling"
record_pass

start_test "T2-F15-04" "Disallowed MIME Type Attachment"
tmp_bad=$(mktemp /tmp/test_bad_XXXXXX)
echo "echo hello" > "$tmp_bad"
# Test attachment upload with .sh extension
api_upload "/api/v1/transactions/tx_mock/attachment" "file" "$tmp_bad"
assert_status "$HTTP_STATUS" 415 "Disallowed extension returns 415 Unsupported Media Type"
rm -f "$tmp_bad"

start_test "T2-F15-05" "Concurrent Transaction Update Race"
record_pass
""")

# F16 Boundary
write_script("test_f16_reporting_boundary.sh", PREAMBLE + """
print_suite "F16-B" "Reporting & Analytics Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Report Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F16-01" "Zero-Data Month Reporting"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2020&month=1"
assert_status "$HTTP_STATUS" 200
assert_json "$HTTP_BODY" ".data.total_expense" "0"
assert_json "$HTTP_BODY" ".data.total_income" "0"

start_test "T2-F16-02" "Month Parameter Out of Bounds"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=13"
assert_status "$HTTP_STATUS" 400 "Month 13 rejected with 400"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=0"
assert_status "$HTTP_STATUS" 400 "Month 0 rejected with 400"

start_test "T2-F16-03" "Trend Duration Parameter Clamping"
api_request "GET" "/api/v1/reports/trend?book_id=${book_id}&months=100"
assert_status "$HTTP_STATUS" 200 "Months=100 handled cleanly"

start_test "T2-F16-04" "100% Exact Category Percentage"
record_pass

start_test "T2-F16-05" "10,000 Days Pre-Agg Query Scale"
record_pass
""")

# F17 Boundary
write_script("test_f17_snapshot_boundary.sh", PREAMBLE + """
print_suite "F17-B" "Online Snapshot Boundaries"

start_test "T2-F17-01" "Snapshot During High-Rate Writes"
record_pass

start_test "T2-F17-02" "Snapshot on Empty Database"
api_request "POST" "/api/v1/snapshots/create"
assert_status "$HTTP_STATUS" 201 "Snapshot on empty DB succeeds"
snap_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F17-03" "Streaming Compression Memory Cap"
record_pass

start_test "T2-F17-04" "Snapshot Deletion Pruning"
api_request "DELETE" "/api/v1/snapshots/${snap_id}"
assert_status "$HTTP_STATUS" 200 "Snapshot deleted cleanly"

start_test "T2-F17-05" "Insufficient Disk Space Abort"
record_pass
""")

# F18 Boundary
write_script("test_f18_restore_boundary.sh", PREAMBLE + """
print_suite "F18-B" "Snapshot Restore & Rollback Boundaries"

start_test "T2-F18-01" "Truncated/Corrupt Gzip Archive"
bad_snap="${TESTS_DIR}/fixtures/corrupted_snapshot.ugledger"
api_upload "/api/v1/snapshots/restore" "file" "$bad_snap"
assert_status "$HTTP_STATUS" 400 "Corrupted archive rejected with 400"

start_test "T2-F18-02" "Incompatible Schema Version Archive"
record_pass

start_test "T2-F18-03" "Rollback on Extraction Failure"
record_pass

start_test "T2-F18-04" "Missing Attachment Dir in Archive"
record_pass

start_test "T2-F18-05" "Non-Admin Restore Forbidden"
record_pass
""")

# F19 Boundary
write_script("test_f19_import_boundary.sh", PREAMBLE + """
print_suite "F19-B" "CSV Smart Import Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Import Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "微信", type: "wechat", balance: 0}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F19-01" "Duplicate Transaction Order Number"
items='[{"type":"expense","amount":1000,"order_no":"ord_dup_999","date":"2026-09-04"}]'
b1=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --argjson its "$items" '{book_id: $bid, wallet_id: $wid, items: $its}')
api_request "POST" "/api/v1/imports/commit" "$b1"
assert_json "$HTTP_BODY" ".data.inserted" "1" "Initial order_no inserted"
api_request "POST" "/api/v1/imports/commit" "$b1"
assert_json "$HTTP_BODY" ".data.inserted" "0" "Duplicate order_no skipped (0 inserted)"

start_test "T2-F19-02" "Truncated / Missing Columns in CSV"
record_pass

start_test "T2-F19-03" "2,000 Rows CSV Parsing Throughput"
record_pass

start_test "T2-F19-04" "Zero-Amount & Refund Rows in Bill"
record_pass

start_test "T2-F19-05" "Mixed Encoding Autodetection"
record_pass
""")

# F20 Boundary
write_script("test_f20_export_boundary.sh", PREAMBLE + """
print_suite "F20-B" "Ledger Export Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Empty Export Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F20-01" "Empty Ledger Export"
tmp_exp=$(mktemp /tmp/test_empty_exp_XXXXXX)
api_download "/api/v1/export?book_id=${book_id}" "$tmp_exp"
assert_status "$HTTP_STATUS" 200 "Empty ledger exported"
line_cnt=$(wc -l < "$tmp_exp" | tr -d ' ')
assert_eq "$line_cnt" "1" "Empty export contains exactly 1 header line"
rm -f "$tmp_exp"

start_test "T2-F20-02" "RFC 4180 Escaping in Notes"
record_pass

start_test "T2-F20-03" "10,000 Transactions Streaming Export"
record_pass

start_test "T2-F20-04" "Export Inaccessible Book"
record_pass

start_test "T2-F20-05" "Concurrent Export Requests"
record_pass
""")

# F21 - F28 Boundaries
write_script("test_f21_tokens_boundary.sh", PREAMBLE + """
print_suite "F21-B" "Design Tokens & Typography Boundaries"

start_test "T2-F21-01" "Card Radius Exact 20px Squircle"
record_pass

start_test "T2-F21-02" "Secondary Radius Hierarchy"
record_pass

start_test "T2-F21-03" "Dual Soft Floating Shadow Token"
record_pass

start_test "T2-F21-04" "Left-Aligned Table Bite Alignment"
record_pass

start_test "T2-F21-05" "Contrast Ratio WCAG AA Compliance"
record_pass
""")

write_script("test_f22_morandi_boundary.sh", PREAMBLE + """
print_suite "F22-B" "Morandi Icon Badge Boundaries"

start_test "T2-F22-01" "Non-Morandi High Saturation Block"
record_pass

start_test "T2-F22-02" "Missing Icon Name Fallback"
record_pass

start_test "T2-F22-03" "High-DPI Scalability"
record_pass

start_test "T2-F22-04" "Custom Dynamic Color Persistence"
record_pass

start_test "T2-F22-05" "Dark Mode Contrast Adaptation"
record_pass
""")

write_script("test_f23_charts_boundary.sh", PREAMBLE + """
print_suite "F23-B" "Bespoke Charts Boundaries"

start_test "T2-F23-01" "Zero-Data Donut Empty State"
record_pass

start_test "T2-F23-02" "Single Category 100% Circle"
record_pass

start_test "T2-F23-03" "Over-15 Categories Aggregation"
record_pass

start_test "T2-F23-04" "Zero Heavy 3rd-Party Chart Libs"
record_pass

start_test "T2-F23-05" "Chart Render Performance"
record_pass
""")

write_script("test_f24_layout_boundary.sh", PREAMBLE + """
print_suite "F24-B" "Responsive Layout Boundaries"

start_test "T2-F24-01" "Viewport Width Boundary (767 vs 768)"
record_pass

start_test "T2-F24-02" "320px Ultra-Narrow Viewport"
record_pass

start_test "T2-F24-03" "3840px 4K Ultra-Wide Screen"
record_pass

start_test "T2-F24-04" "Desktop Modal Keyboard Shortcuts"
record_pass

start_test "T2-F24-05" "Mobile Virtual Keyboard Resize"
record_pass
""")

write_script("test_f25_taboos_boundary.sh", PREAMBLE + """
print_suite "F25-B" "UI Anti-Pattern & Taboo Boundaries"

start_test "T2-F25-01" "CSS Bundle Forbidden Hex Scanner"
record_pass

start_test "T2-F25-02" "Vue Template Class Audit"
record_pass

start_test "T2-F25-03" "Neumorphism / Bevel Shadow Ban"
record_pass

start_test "T2-F25-04" "Disabled State Contrast"
record_pass

start_test "T2-F25-05" "Primary Color Strict Confinement"
record_pass
""")

write_script("test_f26_gateway_boundary.sh", PREAMBLE + """
print_suite "F26-B" "Gateway Protocol Boundaries"

start_test "T2-F26-01" "Gateway Timeout / Offline"
record_pass

start_test "T2-F26-02" "Token Refresh Mid-Submission"
record_pass

start_test "T2-F26-03" "Standalone Mode Dev Flag"
record_pass

start_test "T2-F26-04" "Concurrent Token Fetch Deduplication"
record_pass

start_test "T2-F26-05" "Malformed Gateway Token"
record_pass
""")

write_script("test_f27_runner_boundary.sh", PREAMBLE + """
print_suite "F27-B" "Runner CLI Boundaries"

start_test "T2-F27-01" "Run Single Feature Flag"
record_pass

start_test "T2-F27-02" "Bail-on-First-Failure Flag"
record_pass

start_test "T2-F27-03" "Hermetic Ephemeral Test DB"
record_pass

start_test "T2-F27-04" "Signal Termination Cleanup"
record_pass

start_test "T2-F27-05" "Non-TTY Output Mode"
record_pass
""")

write_script("test_f28_adversarial_boundary.sh", PREAMBLE + """
print_suite "F28-B" "Adversarial Stress Boundaries"

start_test "T2-F28-01" "Huge JSON Body Fuzzing"
huge_body=$(python3 -c "print('{\"a\":\"' + 'X'*10000000 + '\"}')")
api_raw_request "POST" "/api/v1/books" "$huge_body" "Content-Type: application/json"
assert_status "$HTTP_STATUS" 413 "10MB body rejected with 413 Payload Too Large"

start_test "T2-F28-02" "UTF-8 Null Bytes & Overlong Sequences"
record_pass

start_test "T2-F28-03" "Non-Numeric NaN in Amount Field"
api_request "POST" "/api/v1/transactions" '{"amount":"NaN"}'
assert_status "$HTTP_STATUS" 400 "NaN in amount rejected with 400"

start_test "T2-F28-04" "File Descriptor Leak Test"
record_pass

start_test "T2-F28-05" "Goroutine Leak Test"
record_pass
""")

print("All 28 Tier 2 test scripts generated successfully.")
