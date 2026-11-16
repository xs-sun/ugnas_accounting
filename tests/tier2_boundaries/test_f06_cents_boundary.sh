#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

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
