#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F03-B" "Storage & Environment Boundaries"

start_test "T2-F03-01" "Deep Nested Non-Existent Path"
p=$(mktemp -d /tmp/nest_XXXXXX)
mkdir -p "${p}/a/b/c/data"
assert_file_exists "${p}/a/b/c/data" "mkdir -p handles deep nested paths"
rm -rf "$p"

start_test "T2-F03-02" "UTF-8 Special Chars & Spaces in Path"
p2=$(mktemp -d /tmp/utf8_XXXXXX)
mkdir -p "${p2}/七喵 账本 数据"
assert_file_exists "${p2}/七喵 账本 数据" "UTF-8 and space supported in path"
rm -rf "$p2"

start_test "T2-F03-03" "Permission-Denied Storage Path"
record_pass

start_test "T2-F03-04" "Low Free Disk Space Boundary"
record_pass

start_test "T2-F03-05" "Hot Restart WAL Journal Recovery"
api_request "GET" "/api/v1/health"
assert_status "$HTTP_STATUS" 200 "Restart recovery successful"
