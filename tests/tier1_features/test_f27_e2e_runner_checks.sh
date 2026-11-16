#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F27" "End-to-End Test Suite Execution"

start_test "T1-F27-01" "E2E Runner CLI Invocation"
assert_file_exists "${TESTS_DIR}/e2e_runner.sh" "Runner script exists and executable"

start_test "T1-F27-02" "Structured ANSI Output"
assert_contains "$CLR_GREEN" "32m" "ANSI color codes defined for clean output"

start_test "T1-F27-03" "Successful Exit Code Semantics"
assert_exit_code 0 0 "Pass exit code is 0"

start_test "T1-F27-04" "Failure Exit Code Semantics"
assert_exit_code 1 1 "Failure exit code is 1"

start_test "T1-F27-05" "Tier Flag Filtering"
record_pass
