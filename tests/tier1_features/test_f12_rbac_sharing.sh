#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

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
