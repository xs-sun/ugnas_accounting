#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F12-B" "Collaborative Sharing & RBAC Boundaries"

api_request "POST" "/api/v1/books" '{"name":"RBAC Boundary Book"}' "10001" "alice" "admin"
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F12-01" "Sole Owner Self-Removal Rejection"
api_request "DELETE" "/api/v1/books/${book_id}/members/10001" "" "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 400 "Sole owner cannot remove self"

start_test "T2-F12-02" "Duplicate Member Invitation"
api_request "POST" "/api/v1/books/${book_id}/members" '{"user_id":"10002","role":"editor"}' "10001" "alice" "admin"
api_request "POST" "/api/v1/books/${book_id}/members" '{"user_id":"10002","role":"editor"}' "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 409 "Duplicate member invitation rejected with 409"

start_test "T2-F12-03" "Editor Privilege Escalation Blocked"
api_request "POST" "/api/v1/books/${book_id}/members" '{"user_id":"10004","role":"editor"}' "10002" "bob" "users"
assert_status "$HTTP_STATUS" 403 "Editor cannot invite new members"

start_test "T2-F12-04" "Viewer Write Blockade"
api_request "POST" "/api/v1/books/${book_id}/members" '{"user_id":"10003","role":"viewer"}' "10001" "alice" "admin"
bw_view=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "Viewer Wallet", type: "cash", balance: 1000}')
api_request "POST" "/api/v1/wallets" "$bw_view" "10003" "charlie" "users"
assert_status "$HTTP_STATUS" 403 "Viewer cannot create wallet"

start_test "T2-F12-05" "Instant Revocation Enforcement"
api_request "DELETE" "/api/v1/books/${book_id}/members/10002" "" "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 200 "Bob membership revoked"
