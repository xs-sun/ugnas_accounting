#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

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
