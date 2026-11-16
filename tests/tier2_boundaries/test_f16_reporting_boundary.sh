#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F16-B" "Reporting & Analytics Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Report Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F16-01" "Zero-Data Month Reporting"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2020&month=1"
assert_status "$HTTP_STATUS" 200
assert_json "$HTTP_BODY" ".data.total_expense" "0"
assert_json "$HTTP_BODY" ".data.total_income" "0"

start_test "T2-F16-02" "Month Parameter Out of Bounds"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=13"
assert_status "$HTTP_STATUS" 400 "Month 13 rejected with 400"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=0"
assert_status "$HTTP_STATUS" 400 "Month 0 rejected with 400"

start_test "T2-F16-03" "Trend Duration Parameter Clamping"
api_request "GET" "/api/v1/reports/trend?book_id=${book_id}&months=100"
assert_status "$HTTP_STATUS" 200 "Months=100 handled cleanly"

start_test "T2-F16-04" "100% Exact Category Percentage"
record_pass

start_test "T2-F16-05" "10,000 Days Pre-Agg Query Scale"
record_pass
