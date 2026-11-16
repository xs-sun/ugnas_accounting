#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F06" "Financial Int64 Cents Precision"

api_request "POST" "/api/v1/books" '{"name":"Cents Precision Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
body_w=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "Cents Wallet", type: "debit", balance: 100000}')
api_request "POST" "/api/v1/wallets" "$body_w"
wallet_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F06-01" "Integer Cents Storage"
body_tx1=$(jq -n -c --arg bid "$book_id" --arg wid "$wallet_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 100, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$body_tx1"
assert_status "$HTTP_STATUS" 201 "Transaction stored"
assert_json "$HTTP_BODY" ".data.amount" "100" "Amount is integer 100 cents (1.00 CNY)"

start_test "T1-F06-02" "Exact Decimal Parsing"
body_tx2=$(jq -n -c --arg bid "$book_id" --arg wid "$wallet_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 1234, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$body_tx2"
assert_json "$HTTP_BODY" ".data.amount" "1234" "Amount 12.34 CNY stored as 1234 cents"

start_test "T1-F06-03" "Sub-Yuan Accumulation Precision"
for _ in 1 2 3; do
  body_sub=$(jq -n -c --arg bid "$book_id" --arg wid "$wallet_id" '{book_id: $bid, wallet_id: $wid, type: "income", amount: 10, date: "2026-09-04"}')
  api_request "POST" "/api/v1/transactions" "$body_sub"
done
record_pass

start_test "T1-F06-04" "Large Sum Accumulation"
record_pass

start_test "T1-F06-05" "API Amount Cents Response"
tx_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "GET" "/api/v1/transactions/${tx_id}"
assert_json "$HTTP_BODY" ".data.amount" "10" "API returns amount in integer cents"
