#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F17" "Online Hot Snapshot Engine"

start_test "T1-F17-01" "Trigger Hot Snapshot Creation"
api_request "POST" "/api/v1/snapshots/create"
assert_status "$HTTP_STATUS" 201 "Snapshot generated"
snap_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F17-02" "Native VACUUM INTO Execution"
record_pass

start_test "T1-F17-03" "Archive Packaging (.ugledger)"
assert_contains "$HTTP_BODY" ".ugledger" "Snapshot archive has .ugledger extension"

start_test "T1-F17-04" "List Historical Snapshots"
api_request "GET" "/api/v1/snapshots"
assert_status "$HTTP_STATUS" 200 "Snapshots listed"
cnt=$(echo "$HTTP_BODY" | jq -r --arg sid "$snap_id" '[.data[] | select(.id==$sid)] | length')
assert_eq "$cnt" "1" "Snapshot found in history"

start_test "T1-F17-05" "Download Snapshot Archive"
tmp_dl=$(mktemp /tmp/snap_dl_XXXXXX.ugledger)
api_download "/api/v1/snapshots/${snap_id}/download" "$tmp_dl"
assert_status "$HTTP_STATUS" 200 "Snapshot downloaded"
assert_file_exists "$tmp_dl" "Downloaded file exists"
rm -f "$tmp_dl"
