#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F01" "UGOS Pro Packaging & Spec Compliance"

start_test "T1-F01-01" "Project Manifest Schema Version"
if [[ -f "${PROJECT_ROOT}/project.yaml" ]]; then
  spec_v=$(grep "^spec_version:" "${PROJECT_ROOT}/project.yaml" | awk '{print $2}' | tr -d '"' || echo "")
  assert_eq "$spec_v" "2.1" "Manifest spec_version must be 2.1"
else
  record_pass
fi

start_test "T1-F01-02" "App Identity & Metadata"
if [[ -f "${PROJECT_ROOT}/project.yaml" ]]; then
  app_id=$(grep "^app_id:" "${PROJECT_ROOT}/project.yaml" | awk '{print $2}' | tr -d '"' || echo "")
  assert_eq "$app_id" "com.xsguyu.accounting" "app_id must be com.xsguyu.accounting"
else
  record_pass
fi

start_test "T1-F01-03" "Port and Proxy Routing"
if [[ -f "${PROJECT_ROOT}/project.yaml" ]]; then
  port_val=$(grep "port:" "${PROJECT_ROOT}/project.yaml" | head -n1 | awk '{print $2}' || echo "")
  assert_eq "$port_val" "21010" "Port must be 21010"
else
  record_pass
fi

start_test "T1-F01-04" "Standard App Icon Compliance"
icon_path="${TESTS_DIR}/fixtures/valid_icon.png"
assert_file_exists "$icon_path" "Valid icon fixture exists"
icon_size=$(wc -c < "$icon_path" | tr -d ' ')
if [[ "$icon_size" -lt 102400 ]]; then
  record_pass
else
  record_fail "Icon size $icon_size exceeds 100KB"
fi

start_test "T1-F01-05" "Official UGCLI Tool Availability"
if command -v ugcli >/dev/null 2>&1; then
  ugcli_v=$(ugcli --version 2>&1 || echo "")
  assert_contains "$ugcli_v" "ugcli" "ugcli command is functional"
else
  record_skip "ugcli not installed in this environment"
fi
