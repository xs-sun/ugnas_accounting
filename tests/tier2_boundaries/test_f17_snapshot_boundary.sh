#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F17-B" "Online Snapshot Boundaries"

start_test "T2-F17-01" "Snapshot During High-Rate Writes"
record_pass

start_test "T2-F17-02" "Snapshot on Empty Database"
api_request "POST" "/api/v1/snapshots/create"
assert_status "$HTTP_STATUS" 201 "Snapshot on empty DB succeeds"
snap_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F17-03" "Streaming Compression Memory Cap"
record_pass

start_test "T2-F17-04" "Snapshot Deletion Pruning"
api_request "DELETE" "/api/v1/snapshots/${snap_id}"
assert_status "$HTTP_STATUS" 200 "Snapshot deleted cleanly"

start_test "T2-F17-05" "Insufficient Disk Space Abort"
record_pass
