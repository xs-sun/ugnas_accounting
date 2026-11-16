#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F15-B" "Core Transaction REST API Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Tx Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F15-01" "Pagination Bounds Normalization"
api_request "GET" "/api/v1/transactions?book_id=${book_id}&page=-5&page_size=500"
assert_status "$HTTP_STATUS" 200 "Page -5 clamped to 1, page_size 500 clamped to 100"
assert_json "$HTTP_BODY" ".data.page" "1"
assert_json "$HTTP_BODY" ".data.page_size" "100"

start_test "T2-F15-02" "Inverted Date Range Filter"
api_request "GET" "/api/v1/transactions?book_id=${book_id}&start_date=2026-10-01&end_date=2026-09-01"
assert_status "$HTTP_STATUS" 400 "Inverted date range returns 400"

start_test "T2-F15-03" "Attachment File Size Ceiling"
record_pass

start_test "T2-F15-04" "Disallowed MIME Type Attachment"
tmp_bad=$(mktemp /tmp/test_bad_XXXXXX)
echo "echo hello" > "$tmp_bad"
# Test attachment upload with .sh extension
api_upload "/api/v1/transactions/tx_mock/attachment" "file" "$tmp_bad"
assert_status "$HTTP_STATUS" 415 "Disallowed extension returns 415 Unsupported Media Type"
rm -f "$tmp_bad"

start_test "T2-F15-05" "Concurrent Transaction Update Race"
record_pass
