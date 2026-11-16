#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

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
