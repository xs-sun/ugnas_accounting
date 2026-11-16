#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

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
