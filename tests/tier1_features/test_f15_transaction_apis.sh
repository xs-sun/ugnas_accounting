#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F15" "Core Transaction REST APIs"

api_request "POST" "/api/v1/books" '{"name":"Tx API Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "主卡", type: "debit", balance: 500000}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F15-01" "Create Expense Transaction"
btx1=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 4500, date: "2026-09-02"}')
api_request "POST" "/api/v1/transactions" "$btx1"
assert_status "$HTTP_STATUS" 201 "Expense tx created"
tx_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F15-02" "Create Income Transaction"
btx2=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "income", amount: 150000, date: "2026-09-03"}')
api_request "POST" "/api/v1/transactions" "$btx2"
assert_status "$HTTP_STATUS" 201 "Income tx created"

start_test "T1-F15-03" "Paginated Transaction Listing"
api_request "GET" "/api/v1/transactions?book_id=${book_id}&page=1&page_size=1"
assert_json "$HTTP_BODY" ".data.page" "1"
assert_json "$HTTP_BODY" ".data.page_size" "1"
assert_json "$HTTP_BODY" ".data.total" "2"

start_test "T1-F15-04" "Date Range Transaction Filter"
api_request "GET" "/api/v1/transactions?book_id=${book_id}&start_date=2026-09-02&end_date=2026-09-02"
total_match=$(echo "$HTTP_BODY" | jq -r '.data.total')
assert_eq "$total_match" "1" "Filtered exactly 1 transaction on 2026-09-02"

start_test "T1-F15-05" "Receipt Attachment Upload"
rec_path="${TESTS_DIR}/fixtures/test_receipt.jpg"
api_upload "/api/v1/transactions/${tx_id}/attachment" "file" "$rec_path"
assert_status "$HTTP_STATUS" 201 "Attachment uploaded and linked to transaction"
