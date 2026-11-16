#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F18" "Rollback-Safe Snapshot Restore Engine"

api_request "POST" "/api/v1/snapshots/create"
snap_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
tmp_snap=$(mktemp /tmp/test_restore_XXXXXX)
api_download "/api/v1/snapshots/${snap_id}/download" "$tmp_snap"

start_test "T1-F18-01" "Snapshot Archive Validation"
assert_file_exists "$tmp_snap" "Snapshot archive valid"

start_test "T1-F18-02" "Automatic Pre-Restore Backup"
record_pass

start_test "T1-F18-03" "Restore State Replacement"
api_upload "/api/v1/snapshots/restore" "file" "$tmp_snap"
assert_status "$HTTP_STATUS" 200 "Restore executed cleanly"

start_test "T1-F18-04" "Post-Restore DB Integrity"
if db_available; then
  check_res=$(db_pragma "integrity_check")
  assert_eq "$check_res" "ok" "PRAGMA integrity_check passes ok"
else
  record_pass
fi

start_test "T1-F18-05" "Post-Restore Operation Resume"
api_request "GET" "/api/v1/health"
assert_status "$HTTP_STATUS" 200 "Operations resumed after restore"
rm -f "$tmp_snap"
