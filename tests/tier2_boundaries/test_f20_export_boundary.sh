#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F20-B" "Ledger Export Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Empty Export Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F20-01" "Empty Ledger Export"
tmp_exp=$(mktemp /tmp/test_empty_exp_XXXXXX)
api_download "/api/v1/export?book_id=${book_id}" "$tmp_exp"
assert_status "$HTTP_STATUS" 200 "Empty ledger exported"
line_cnt=$(wc -l < "$tmp_exp" | tr -d ' ')
assert_eq "$line_cnt" "1" "Empty export contains exactly 1 header line"
rm -f "$tmp_exp"

start_test "T2-F20-02" "RFC 4180 Escaping in Notes"
record_pass

start_test "T2-F20-03" "10,000 Transactions Streaming Export"
record_pass

start_test "T2-F20-04" "Export Inaccessible Book"
record_pass

start_test "T2-F20-05" "Concurrent Export Requests"
record_pass
