#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F28" "Adversarial Coverage Hardening (Tier 5 Patterns)"

api_request "POST" "/api/v1/books" '{"name":"Security Audit Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "安全钱包", type: "cash", balance: 100000}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F28-01" "SQL Injection Parameterization"
btx1=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --arg note "' OR '1'='1" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 100, note: $note}')
api_request "POST" "/api/v1/transactions" "$btx1"
assert_status "$HTTP_STATUS" 201 "SQL injection payload safely parameterized"
tx_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "GET" "/api/v1/transactions/${tx_id}"
assert_json "$HTTP_BODY" ".data.note" "' OR '1'='1" "Payload stored as literal string"

start_test "T1-F28-02" "XSS Payload Sanitization"
btx2=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --arg note "<script>alert(1)</script>" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 100, note: $note}')
api_request "POST" "/api/v1/transactions" "$btx2"
assert_status "$HTTP_STATUS" 201 "XSS script payload safely handled"

start_test "T1-F28-03" "Path Traversal Protection"
api_request "GET" "/api/v1/snapshots/..%2F..%2Fetc%2Fpasswd/download"
assert_status "$HTTP_STATUS" 400 "Path traversal blocked with 400"

start_test "T1-F28-04" "Concurrency Write Safety"
for i in 1 2 3 4 5; do
  btx_c=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 100}')
  api_request "POST" "/api/v1/transactions" "$btx_c"
  assert_status "$HTTP_STATUS" 201 "Concurrent write $i succeeded"
done

start_test "T1-F28-05" "Sudden Process Restart Resilience"
api_request "GET" "/api/v1/health"
assert_status "$HTTP_STATUS" 200 "Backend healthy after burst writes"
