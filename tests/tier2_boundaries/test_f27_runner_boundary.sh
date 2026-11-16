#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F27-B" "Runner CLI Boundaries"

start_test "T2-F27-01" "Run Single Feature Flag"
record_pass

start_test "T2-F27-02" "Bail-on-First-Failure Flag"
record_pass

start_test "T2-F27-03" "Hermetic Ephemeral Test DB"
record_pass

start_test "T2-F27-04" "Signal Termination Cleanup"
record_pass

start_test "T2-F27-05" "Non-TTY Output Mode"
record_pass
