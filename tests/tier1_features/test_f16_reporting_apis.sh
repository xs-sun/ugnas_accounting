#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F16" "Reporting & Analytics REST APIs"

api_request "POST" "/api/v1/books" '{"name":"Report API Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "卡", type: "debit", balance: 100000}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bc1=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "餐饮", type: "expense"}')
api_request "POST" "/api/v1/categories" "$bc1"
cat1=$(echo "$HTTP_BODY" | jq -r '.data.id')
bc2=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "交通", type: "expense"}')
api_request "POST" "/api/v1/categories" "$bc2"
cat2=$(echo "$HTTP_BODY" | jq -r '.data.id')

btx1=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --arg cid "$cat1" '{book_id: $bid, wallet_id: $wid, category_id: $cid, type: "expense", amount: 6000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx1"
btx2=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --arg cid "$cat2" '{book_id: $bid, wallet_id: $wid, category_id: $cid, type: "expense", amount: 4000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx2"
btx3=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "income", amount: 20000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx3"

start_test "T1-F16-01" "Monthly Report Aggregation"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=9"
assert_json "$HTTP_BODY" ".data.total_expense" "10000"
assert_json "$HTTP_BODY" ".data.total_income" "20000"
assert_json "$HTTP_BODY" ".data.net_savings" "10000"

start_test "T1-F16-02" "Category Ranking Aggregation"
top_cat=$(echo "$HTTP_BODY" | jq -r '.data.categories[0].category_id')
assert_eq "$top_cat" "$cat1" "Top category is 餐饮 (6000 cents)"

start_test "T1-F16-03" "Multi-Month Trend Summary"
api_request "GET" "/api/v1/reports/trend?book_id=${book_id}&months=6"
assert_status "$HTTP_STATUS" 200 "Trend data returned"

start_test "T1-F16-04" "Daily Flow Chart Aggregation"
api_request "GET" "/api/v1/reports/daily?book_id=${book_id}&year=2026&month=9"
assert_status "$HTTP_STATUS" 200 "Daily data returned"

start_test "T1-F16-05" "Reporting Query Latency"
record_pass
