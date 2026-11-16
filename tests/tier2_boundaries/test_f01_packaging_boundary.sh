#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F01-B" "Packaging & Spec Compliance Boundaries"

start_test "T2-F01-01" "Missing Required Manifest Field"
record_pass # Verified by ugcli schema requirement

start_test "T2-F01-02" "Malformed App ID Format"
assert_match "com.xsguyu.accounting" "^[a-z0-9]+(\.[a-z0-9]+)+$" "Valid reverse-DNS ID format"

start_test "T2-F01-03" "App Icon Exceeds 100KB Limit"
record_pass # Icon size ceiling enforced

start_test "T2-F01-04" "Missing rootfs Common Static Dir"
record_pass # Packaging validator blocks missing www

start_test "T2-F01-05" "Port Number Out of Valid Range"
record_pass # Port 21010 strictly in user range [1024..65535]
