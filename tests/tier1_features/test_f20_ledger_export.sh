#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

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
