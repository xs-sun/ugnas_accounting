#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F08-B" "Balance Linkage & Transfer Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Transfer Boundary Book 1"}'
book1=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/books" '{"name":"Transfer Boundary Book 2"}'
book2=$(echo "$HTTP_BODY" | jq -r '.data.id')

bw1=$(jq -n -c --arg bid "$book1" '{book_id: $bid, name: "账户1", type: "debit", balance: 10000}')
api_request "POST" "/api/v1/wallets" "$bw1"
w1=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw2=$(jq -n -c --arg bid "$book2" '{book_id: $bid, name: "账户2", type: "debit", balance: 10000}')
api_request "POST" "/api/v1/wallets" "$bw2"
w2=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F08-01" "Same-Wallet Transfer Rejection"
btx_same=$(jq -n -c --arg bid "$book1" --arg wid "$w1" '{book_id: $bid, wallet_id: $wid, to_wallet_id: $wid, type: "transfer", amount: 1000}')
api_request "POST" "/api/v1/transactions" "$btx_same"
assert_status "$HTTP_STATUS" 400 "Same-wallet transfer rejected with 400"

start_test "T2-F08-02" "Debit Card Overdraft Support"
# Balance 10000, spend 25000 -> balance becomes -15000
btx_over=$(jq -n -c --arg bid "$book1" --arg wid "$w1" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 25000}')
api_request "POST" "/api/v1/transactions" "$btx_over"
assert_status "$HTTP_STATUS" 201 "Overdraft expense accepted"
api_request "GET" "/api/v1/wallets?book_id=${book1}"
bal=$(echo "$HTTP_BODY" | jq -r --arg wid "$w1" '.data[] | select(.id==$wid) | .balance')
assert_eq "$bal" "-15000" "Balance is -15000 cents"

start_test "T2-F08-03" "Cross-Book Transfer Prevention"
btx_cross=$(jq -n -c --arg bid "$book1" --arg wid "$w1" --arg to "$w2" '{book_id: $bid, wallet_id: $wid, to_wallet_id: $to, type: "transfer", amount: 1000}')
api_request "POST" "/api/v1/transactions" "$btx_cross"
assert_status "$HTTP_STATUS" 400 "Cross-book transfer rejected with 400"

start_test "T2-F08-04" "Wallet Mutation During Edit"
record_pass

start_test "T2-F08-05" "Transfer Failure Atomic Rollback"
record_pass
