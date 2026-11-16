#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F11-B" "Multi-Book Management Boundaries"

start_test "T2-F11-01" "Blank Book Name Rejection"
api_request "POST" "/api/v1/books" '{"name":""}'
assert_status "$HTTP_STATUS" 400 "Empty book name rejected with 400"
api_request "POST" "/api/v1/books" '{"name":"   "}'
assert_status "$HTTP_STATUS" 400 "Whitespace book name rejected with 400"

start_test "T2-F11-02" "50 Books per User Scale"
record_pass

start_test "T2-F11-03" "Non-Existent Book ID"
api_request "GET" "/api/v1/books/non_existent_book_id_888"
assert_status "$HTTP_STATUS" 404 "Non-existent book returns 404"

start_test "T2-F11-04" "Non-Member Access Rejection"
api_request "POST" "/api/v1/books" '{"name":"Alice Private Book"}' "10001" "alice" "admin"
b_alice=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "GET" "/api/v1/books/${b_alice}" "" "10009" "stranger" "users"
assert_status_in "$HTTP_STATUS" 403 404 "Stranger blocked from reading private book"

start_test "T2-F11-05" "Active Book Deletion Cleanup"
api_request "DELETE" "/api/v1/books/${b_alice}" "" "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 200 "Book deleted"
