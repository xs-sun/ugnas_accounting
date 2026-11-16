#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F08" "Atomic Balance Linkage & Transfers"

api_request "POST" "/api/v1/books" '{"name":"Linkage Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
body_wa=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "卡A", type: "debit", balance: 100000}')
api_request "POST" "/api/v1/wallets" "$body_wa"
w_a=$(echo "$HTTP_BODY" | jq -r '.data.id')
body_wb=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "卡B", type: "debit", balance: 50000}')
api_request "POST" "/api/v1/wallets" "$body_wb"
w_b=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F08-01" "Expense Balance Deduction"
body_exp=$(jq -n -c --arg bid "$book_id" --arg wid "$w_a" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 5000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$body_exp"
tx_exp=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "GET" "/api/v1/wallets?book_id=${book_id}"
bal_a=$(echo "$HTTP_BODY" | jq -r --arg wid "$w_a" '.data[] | select(.id==$wid) | .balance')
assert_eq "$bal_a" "95000" "Debit card deducted 5000 to 95000"

start_test "T1-F08-02" "Income Balance Increment"
body_inc=$(jq -n -c --arg bid "$book_id" --arg wid "$w_a" '{book_id: $bid, wallet_id: $wid, type: "income", amount: 20000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$body_inc"
api_request "GET" "/api/v1/wallets?book_id=${book_id}"
bal_a=$(echo "$HTTP_BODY" | jq -r --arg wid "$w_a" '.data[] | select(.id==$wid) | .balance')
assert_eq "$bal_a" "115000" "Debit card incremented 20000 to 115000"

start_test "T1-F08-03" "Expense Deletion Balance Rollback"
api_request "DELETE" "/api/v1/transactions/${tx_exp}"
api_request "GET" "/api/v1/wallets?book_id=${book_id}"
bal_a=$(echo "$HTTP_BODY" | jq -r --arg wid "$w_a" '.data[] | select(.id==$wid) | .balance')
assert_eq "$bal_a" "120000" "Expense deletion rolled back balance"

start_test "T1-F08-04" "Expense Update Delta Adjustment"
body_edit=$(jq -n -c --arg bid "$book_id" --arg wid "$w_a" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 10000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$body_edit"
tx_edit=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "PUT" "/api/v1/transactions/${tx_edit}" '{"amount":15000}'
api_request "GET" "/api/v1/wallets?book_id=${book_id}"
bal_a=$(echo "$HTTP_BODY" | jq -r --arg wid "$w_a" '.data[] | select(.id==$wid) | .balance')
assert_eq "$bal_a" "105000" "Delta adjustment of -5000 applied cleanly"

start_test "T1-F08-05" "Inter-Wallet Transfer Atomicity"
body_xfer=$(jq -n -c --arg bid "$book_id" --arg wid "$w_a" --arg to "$w_b" '{book_id: $bid, wallet_id: $wid, to_wallet_id: $to, type: "transfer", amount: 25000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$body_xfer"
assert_status "$HTTP_STATUS" 201 "Transfer succeeded"
api_request "GET" "/api/v1/wallets?book_id=${book_id}"
bal_a=$(echo "$HTTP_BODY" | jq -r --arg wid "$w_a" '.data[] | select(.id==$wid) | .balance')
bal_b=$(echo "$HTTP_BODY" | jq -r --arg wid "$w_b" '.data[] | select(.id==$wid) | .balance')
assert_eq "$bal_a" "80000" "Source wallet decremented by 25000"
assert_eq "$bal_b" "75000" "Destination wallet incremented by 25000"
