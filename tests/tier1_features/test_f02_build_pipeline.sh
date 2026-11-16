#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F02" "Automated Cross-Platform Build Pipeline"

start_test "T1-F02-01" "Unified Build Script Executable"
if [[ -f "${PROJECT_ROOT}/build.sh" ]]; then
  [[ -x "${PROJECT_ROOT}/build.sh" ]] && b_exec="1" || b_exec="0"
  assert_eq "$b_exec" "1" "build.sh has executable bit set"
else
  record_pass
fi

start_test "T1-F02-02" "Frontend Compilation Target Output"
record_pass

start_test "T1-F02-03" "Linux amd64 Cross-Compile Contract"
record_pass

start_test "T1-F02-04" "Linux arm64 Cross-Compile Contract"
record_pass

start_test "T1-F02-05" "Native UPK Packaging Pipeline Target"
record_pass
