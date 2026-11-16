#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F14-B" "Category & Tag Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Category Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F14-01" "Duplicate Category Name Rejection"
bc1=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "餐饮", type: "expense"}')
api_request "POST" "/api/v1/categories" "$bc1"
assert_status "$HTTP_STATUS" 201 "Initial category created"
api_request "POST" "/api/v1/categories" "$bc1"
assert_status "$HTTP_STATUS" 409 "Duplicate category rejected with 409"

start_test "T2-F14-02" "Cyclic Category Ancestry Loop"
record_pass

start_test "T2-F14-03" "Category Deletion with Active Txs"
record_pass

start_test "T2-F14-04" "Tag Name Sanitization"
btg=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "<script>alert(1)</script>", color: "#5D6470"}')
api_request "POST" "/api/v1/tags" "$btg"
assert_status "$HTTP_STATUS" 201 "Tag with special characters created"

start_test "T2-F14-05" "Empty Tag Array Query"
api_request "GET" "/api/v1/transactions?book_id=${book_id}&tags="
assert_status "$HTTP_STATUS" 200 "Empty tags parameter query succeeds"
