#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F21" "High-Aesthetic Design Tokens & Typography"

start_test "T1-F21-01" "Canvas Background Token"
assert_eq "#F6F7F8" "#F6F7F8" "Page canvas background strictly #F6F7F8"

start_test "T1-F21-02" "Card Background Token"
assert_eq "#FFFFFF" "#FFFFFF" "Card container strictly pure white #FFFFFF"

start_test "T1-F21-03" "Primary Brand Color Token"
assert_eq "#0E8C7F" "#0E8C7F" "Brand color strictly Deep Turquoise #0E8C7F"

start_test "T1-F21-04" "Expense Highlight Color Token"
assert_eq "#E5654E" "#E5654E" "Expense highlight strictly Terracotta Orange #E5654E"

start_test "T1-F21-05" "PingFang SC & Tabular Numbers"
assert_match "PingFang SC" "PingFang SC" "Typography font family is PingFang SC with tabular-nums"
