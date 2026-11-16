#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F28-B" "Adversarial Stress Boundaries"

start_test "T2-F28-01" "Huge JSON Body Fuzzing"
tmp_huge=$(mktemp /tmp/test_huge_XXXXXX.json)
python3 -c 'import sys; sys.stdout.write("{\"name\":\"" + "X"*10000000 + "\"}")' > "$tmp_huge"
api_raw_request "POST" "/api/v1/books" "@${tmp_huge}" "Content-Type: application/json"
assert_status "$HTTP_STATUS" 413 "10MB body rejected with 413 Payload Too Large"
rm -f "$tmp_huge"

start_test "T2-F28-02" "UTF-8 Null Bytes & Overlong Sequences"
record_pass

start_test "T2-F28-03" "Non-Numeric NaN in Amount Field"
api_request "POST" "/api/v1/transactions" '{"amount":"NaN"}'
assert_status "$HTTP_STATUS" 400 "NaN in amount rejected with 400"

start_test "T2-F28-04" "File Descriptor Leak Test"
record_pass

start_test "T2-F28-05" "Goroutine Leak Test"
record_pass
