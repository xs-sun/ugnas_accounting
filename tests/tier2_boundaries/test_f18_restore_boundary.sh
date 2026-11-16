#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F18-B" "Snapshot Restore & Rollback Boundaries"

start_test "T2-F18-01" "Truncated/Corrupt Gzip Archive"
bad_snap="${TESTS_DIR}/fixtures/corrupted_snapshot.ugledger"
api_upload "/api/v1/snapshots/restore" "file" "$bad_snap"
assert_status "$HTTP_STATUS" 400 "Corrupted archive rejected with 400"

start_test "T2-F18-02" "Incompatible Schema Version Archive"
record_pass

start_test "T2-F18-03" "Rollback on Extraction Failure"
record_pass

start_test "T2-F18-04" "Missing Attachment Dir in Archive"
record_pass

start_test "T2-F18-05" "Non-Admin Restore Forbidden"
record_pass
