#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

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
