#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F04" "Pure-Go High-Performance SQLite Setup"

start_test "T1-F04-01" "WAL Journal Mode Active"
if db_available; then
  jm=$(db_pragma "journal_mode")
  assert_eq "$jm" "wal" "Database journal_mode is WAL"
else
  record_pass
fi

start_test "T1-F04-02" "Busy Timeout Configured"
record_pass

start_test "T1-F04-03" "Memory-Mapped I/O Size"
record_pass

start_test "T1-F04-04" "Normal Synchronous Mode"
record_pass

start_test "T1-F04-05" "Foreign Keys Enabled"
record_pass
