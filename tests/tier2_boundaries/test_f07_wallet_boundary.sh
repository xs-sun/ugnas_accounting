#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

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
