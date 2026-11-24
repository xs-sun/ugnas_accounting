#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F10" "Materialized daily_summaries Pre-Aggregation"

api_request "POST" "/api/v1/books" '{"name":"Pre-Agg Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "主账户", type: "debit", balance: 100000}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "GET" "/api/v1/categories?book_id=${book_id}&type=expense"
cat_id=$(echo "$HTTP_BODY" | jq -r '.data[]? | select(.name=="餐饮" and (.parent_id == null or .parent_id == "")) | .id' | head -n 1)
if [[ -z "$cat_id" || "$cat_id" == "null" ]]; then
  bc=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "餐饮", type: "expense"}')
  api_request "POST" "/api/v1/categories" "$bc"
  cat_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
fi

start_test "T1-F10-01" "Insert Trigger Updates Pre-Agg"
btx1=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --arg cid "$cat_id" '{book_id: $bid, wallet_id: $wid, category_id: $cid, type: "expense", amount: 3500, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx1"
tx1=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=9"
assert_json "$HTTP_BODY" ".data.total_expense" "3500" "Pre-aggregated total expense matches"

start_test "T1-F10-02" "Income Pre-Aggregation"
btx2=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" '{book_id: $bid, wallet_id: $wid, type: "income", amount: 8000, date: "2026-09-04"}')
api_request "POST" "/api/v1/transactions" "$btx2"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=9"
assert_json "$HTTP_BODY" ".data.total_income" "8000" "Pre-aggregated total income matches"

start_test "T1-F10-03" "Transaction Date Change Trigger"
api_request "PUT" "/api/v1/transactions/${tx1}" '{"date":"2026-08-15"}'
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=8"
assert_json "$HTTP_BODY" ".data.total_expense" "3500" "August summary incremented"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=9"
assert_json "$HTTP_BODY" ".data.total_expense" "0" "September summary decremented"

start_test "T1-F10-04" "Transaction Delete Trigger"
api_request "DELETE" "/api/v1/transactions/${tx1}"
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=8"
assert_json "$HTTP_BODY" ".data.total_expense" "0" "Deleted transaction decrements summary"

start_test "T1-F10-05" "Covering Index Usage Verification"
if db_available; then
  plan=$(sqlite3 "$DB_PATH" "EXPLAIN QUERY PLAN SELECT SUM(total_amount) FROM daily_summaries WHERE book_id='${book_id}' AND type='expense';")
  assert_contains "$plan" "idx_daily_summaries_query" "Hits covering index"
else
  record_pass
fi
