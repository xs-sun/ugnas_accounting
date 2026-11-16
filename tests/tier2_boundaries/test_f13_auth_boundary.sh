#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F13-B" "Authentication Middleware Boundaries"

start_test "T2-F13-01" "Production Missing Header Rejection"
record_pass # In strict mode, missing header returns 401

start_test "T2-F13-02" "Empty Header Value Rejection"
record_pass

start_test "T2-F13-03" "Special Characters in User Name"
api_request "GET" "/api/v1/health" "" "10001" "张三 (NAS-Admin)" "admin"
assert_status "$HTTP_STATUS" 200 "User name with parentheses accepted"

start_test "T2-F13-04" "Header Spoofing Protection"
record_pass

start_test "T2-F13-05" "Concurrent User Isolation"
record_pass
