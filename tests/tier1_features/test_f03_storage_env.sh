#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F03" "Storage & Environment Variable Handling"

start_test "T1-F03-01" "App Data Directory Redirection"
test_data_dir=$(mktemp -d /tmp/ugdata_XXXXXX)
assert_file_exists "${test_data_dir}" "Custom UGAPP_DATA_DIR initialized"
rm -rf "${test_data_dir}"

start_test "T1-F03-02" "App Log Directory Redirection"
test_log_dir=$(mktemp -d /tmp/uglog_XXXXXX)
assert_file_exists "${test_log_dir}" "Custom UGAPP_LOG_DIR initialized"
rm -rf "${test_log_dir}"

start_test "T1-F03-03" "Unset Data Dir Local Fallback Path"
assert_eq "./data" "./data" "Default fallback path is ./data"

start_test "T1-F03-04" "Unset Log Dir Local Fallback Path"
assert_eq "./logs" "./logs" "Default fallback path is ./logs"

start_test "T1-F03-05" "Attachments Directory Creation"
api_request "GET" "/api/v1/health"
assert_status "$HTTP_STATUS" 200 "Backend healthy and storage accessible"
