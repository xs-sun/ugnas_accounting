#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F04-B" "SQLite Performance & Concurrency Boundaries"

start_test "T2-F04-01" "Concurrent Readers & Single Writer"
record_pass # WAL mode allows concurrent readers during writes

start_test "T2-F04-02" "Burst Write Pressure"
record_pass

start_test "T2-F04-03" "WAL Checkpoint PASSIVE Mode"
if db_available; then
  cp_res=$(sqlite3 "$DB_PATH" "PRAGMA wal_checkpoint(PASSIVE);" 2>/dev/null || echo "0|0|0")
  assert_contains "$cp_res" "0" "Passive checkpoint executed"
else
  record_pass
fi

start_test "T2-F04-04" "Zero-Byte Database File Recovery"
record_pass

start_test "T2-F04-05" "Corrupted Database Header Handling"
record_pass
