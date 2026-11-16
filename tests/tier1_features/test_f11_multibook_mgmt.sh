#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

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
