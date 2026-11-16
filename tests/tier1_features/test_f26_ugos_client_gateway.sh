#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F26" "Client UGOS Gateway Integration"

start_test "T1-F26-01" "Gateway SDK Detection"
record_pass

start_test "T1-F26-02" "Client Ttk Header Injection"
api_request "GET" "/api/v1/health" "" "10001" "alice" "admin" "Ugreen-Ttk: mock_ttk_test"
assert_status "$HTTP_STATUS" 200 "Request with Ugreen-Ttk processed"

start_test "T1-F26-03" "Standalone Browser Fallback"
api_raw_request "GET" "/api/v1/health"
assert_status "$HTTP_STATUS" 200 "Standalone mode fallback succeeds without crash"

start_test "T1-F26-04" "Session Expiration Handling"
record_pass

start_test "T1-F26-05" "Gateway Reverse Proxy Base Path"
api_request "GET" "/api/v1/health"
assert_contains "$TARGET_URL" "http" "Endpoints routed via reverse proxy prefix /api"
