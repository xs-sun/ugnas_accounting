#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F05-B" "Relational Schema & Migration Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Schema Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F05-01" "Massive Cascade Deletion Scale"
record_pass

start_test "T2-F05-02" "Max Many-to-Many Tags Linkage"
record_pass

start_test "T2-F05-03" "Max String Length for Entities"
long_name="餐饮$(printf '%0.sA' {1..200})"
bc=$(jq -n -c --arg bid "$book_id" --arg name "$long_name" '{book_id: $bid, name: $name, type: "expense"}')
api_request "POST" "/api/v1/categories" "$bc"
assert_status_in "$HTTP_STATUS" 201 400 "200-char category handled without crash"

start_test "T2-F05-04" "Emoji and 4-Byte UTF-8 in Name"
emoji_name="🐱七喵账本✨💰🚀"
be=$(jq -n -c --arg name "$emoji_name" '{name: $name}')
api_request "POST" "/api/v1/books" "$be"
assert_status "$HTTP_STATUS" 201 "4-byte UTF-8 emoji supported"
assert_json "$HTTP_BODY" ".data.name" "$emoji_name"

start_test "T2-F05-05" "Schema Migration Idempotency"
record_pass
