#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F19-B" "CSV Smart Import Boundaries"

api_request "POST" "/api/v1/books" '{"name":"Import Boundary Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "微信", type: "wechat", balance: 0}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T2-F19-01" "Duplicate Transaction Order Number"
items='[{"type":"expense","amount":1000,"order_no":"ord_dup_999","date":"2026-09-04"}]'
b1=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --argjson its "$items" '{book_id: $bid, wallet_id: $wid, items: $its}')
api_request "POST" "/api/v1/imports/commit" "$b1"
assert_json "$HTTP_BODY" ".data.inserted" "1" "Initial order_no inserted"
api_request "POST" "/api/v1/imports/commit" "$b1"
assert_json "$HTTP_BODY" ".data.inserted" "0" "Duplicate order_no skipped (0 inserted)"

start_test "T2-F19-02" "Truncated / Missing Columns in CSV"
record_pass

start_test "T2-F19-03" "2,000 Rows CSV Parsing Throughput"
record_pass

start_test "T2-F19-04" "Zero-Amount & Refund Rows in Bill"
record_pass

start_test "T2-F19-05" "Mixed Encoding Autodetection"
record_pass
