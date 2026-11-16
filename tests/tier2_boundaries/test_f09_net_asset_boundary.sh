#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

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
w_huge_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "GET" "/api/v1/wallets/summary?book_id=${book_id}"
assert_json "$HTTP_BODY" ".data.net_assets" "100000000000"

start_test "T2-F09-04" "Dynamic Toggle Exclude Flag"
api_request "PUT" "/api/v1/wallets/${w_huge_id}" '{"exclude_net_asset":1}'
api_request "GET" "/api/v1/wallets/summary?book_id=${book_id}"
assert_json "$HTTP_BODY" ".data.net_assets" "0"

start_test "T2-F09-05" "Credit Card Overpayment (Asset)"
bw_overpay=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "多还信用卡", type: "credit", balance: 50000}')
api_request "POST" "/api/v1/wallets" "$bw_overpay"
api_request "GET" "/api/v1/wallets/summary?book_id=${book_id}"
assert_json "$HTTP_BODY" ".data.total_assets" "50000" "Positive credit card balance counted as asset"
assert_json "$HTTP_BODY" ".data.total_liabilities" "0" "Liabilities remain 0"
