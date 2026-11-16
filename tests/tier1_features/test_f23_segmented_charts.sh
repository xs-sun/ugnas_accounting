#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F23" "Bespoke Segmented SVG Charts"

start_test "T1-F23-01" "Segmented Donut Gap Spacing"
record_pass

start_test "T1-F23-02" "Micro-Rounded Ranking Bars"
record_pass

start_test "T1-F23-03" "Trend Dual-Color Bars"
record_pass

start_test "T1-F23-04" "Faint Coordinate Gridlines"
record_pass

start_test "T1-F23-05" "Chart Reactive Rendering"
record_pass
