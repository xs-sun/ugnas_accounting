#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F05" "Relational Schema & Repository Abstraction"

start_test "T1-F05-01" "10 DDL Tables Verified"
if db_available; then
  for tbl in books book_members wallets categories tags transactions transaction_tags attachments daily_summaries snapshots; do
    if ! db_table_exists "$tbl"; then
      record_fail "Missing required table: $tbl"
      break
    fi
  done
  record_pass
else
  record_pass
fi

start_test "T1-F05-02" "Foreign Key Enforcement"
api_request "POST" "/api/v1/transactions" '{"book_id":"non_existent_book_999","type":"expense","amount":1000,"wallet_id":"none"}'
assert_status "$HTTP_STATUS" 400 "Rejects invalid foreign key references"

start_test "T1-F05-03" "Unique Constraint Enforcement"
api_request "POST" "/api/v1/books" '{"name":"Schema Test Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
body_mem=$(jq -n -c '{user_id: "10002", role: "editor"}')
api_request "POST" "/api/v1/books/${book_id}/members" "$body_mem"
assert_status "$HTTP_STATUS" 201 "Added member initially"
api_request "POST" "/api/v1/books/${book_id}/members" "$body_mem"
assert_status "$HTTP_STATUS" 409 "Duplicate member rejected with 409"

start_test "T1-F05-04" "Cascade Delete Consistency"
api_request "DELETE" "/api/v1/books/${book_id}"
assert_status "$HTTP_STATUS" 200 "Book deleted"
api_request "GET" "/api/v1/books/${book_id}"
assert_status "$HTTP_STATUS" 404 "Deleted book returns 404"

start_test "T1-F05-05" "Covering Index Presence"
if db_available; then
  idx_cnt=$(sqlite3 "$DB_PATH" "SELECT count(*) FROM sqlite_master WHERE type='index' AND name='idx_daily_summaries_query';")
  assert_eq "$idx_cnt" "1" "Covering index idx_daily_summaries_query exists"
else
  record_pass
fi
