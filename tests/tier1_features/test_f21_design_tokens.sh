#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

STYLE_CSS="${PROJECT_ROOT}/frontend/src/style.css"
TAILWIND_CFG="${PROJECT_ROOT}/frontend/tailwind.config.js"

assert_token_in_design_files() {
  local token="$1"
  local desc="$2"
  if grep -qiF "$token" "$STYLE_CSS" "$TAILWIND_CFG" 2>/dev/null; then
    return 0
  else
    record_fail "${desc} (Token '${token}' not found in style.css or tailwind.config.js)"
    return 1
  fi
}

print_suite "F21" "Warm Amber Design Tokens & Typography"

start_test "T1-F21-01" "Design Token Source Files Exist"
assert_file_exists "$STYLE_CSS" "frontend/src/style.css should exist"
assert_file_exists "$TAILWIND_CFG" "frontend/tailwind.config.js should exist"

start_test "T1-F21-02" "Canvas Background Token (#F5F6F8)"
assert_token_in_design_files "#F5F6F8" "Canvas background token #F5F6F8 must be defined"

start_test "T1-F21-03" "Card Surface Token (#FFFFFF)"
assert_token_in_design_files "#FFFFFF" "Card container surface token #FFFFFF must be defined"

start_test "T1-F21-04" "Primary Amber Gold Brand Token (#DE8A1E)"
assert_token_in_design_files "#DE8A1E" "Primary brand color Warm Amber Gold #DE8A1E must be defined"

start_test "T1-F21-05" "Expense Terracotta Color Token (#D6553E)"
assert_token_in_design_files "#D6553E" "Expense highlight color Terracotta #D6553E must be defined"

start_test "T1-F21-06" "Income Forest Green Color Token (#3E8E63)"
assert_token_in_design_files "#3E8E63" "Income highlight color Forest Green #3E8E63 must be defined"

start_test "T1-F21-07" "Typography Stack (Inter & PingFang SC)"
assert_token_in_design_files "Inter" "Typography stack must include Inter"
assert_token_in_design_files "PingFang SC" "Typography stack must include PingFang SC"

start_test "T1-F21-08" "Tabular Numerals Enforcement (tabular-nums)"
assert_token_in_design_files "tabular-nums" "Numeric display must enforce tabular-nums"

finish_suite
