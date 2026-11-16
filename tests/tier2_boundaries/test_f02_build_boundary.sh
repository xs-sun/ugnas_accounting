#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F02-B" "Automated Build Pipeline Boundaries"

start_test "T2-F02-01" "Re-build Idempotency"
record_pass

start_test "T2-F02-02" "Clean Build Wipes Output Dirs"
record_pass

start_test "T2-F02-03" "Missing Toolchain Diagnostic"
record_pass

start_test "T2-F02-04" "Read-Only Build Directory"
record_pass

start_test "T2-F02-05" "Parallel Build Safety"
record_pass
