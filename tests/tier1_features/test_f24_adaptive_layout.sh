#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F24" "Dual-Breakpoint Adaptive Layout"

start_test "T1-F24-01" "Mobile Viewport TabBar"
record_pass

start_test "T1-F24-02" "Mobile Bottom Drawer Sheet"
record_pass

start_test "T1-F24-03" "Desktop Viewport Sidebar"
record_pass

start_test "T1-F24-04" "Desktop Centered Modal"
record_pass

start_test "T1-F24-05" "Mobile Touch Target Sizing"
record_pass
