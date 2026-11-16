#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F14" "Category & Tag Management"

api_request "POST" "/api/v1/books" '{"name":"Category Tag Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F14-01" "Create Expense Category"
bc1=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "餐饮美食", type: "expense", icon: "food", color: "#0E8C7F"}')
api_request "POST" "/api/v1/categories" "$bc1"
assert_status "$HTTP_STATUS" 201 "Created expense category"
cat_parent=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F14-02" "Create Income Category"
bc2=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "工资薪金", type: "income", icon: "salary", color: "#0E8C7F"}')
api_request "POST" "/api/v1/categories" "$bc2"
assert_status "$HTTP_STATUS" 201 "Created income category"

start_test "T1-F14-03" "Subcategory Creation"
bc3=$(jq -n -c --arg bid "$book_id" --arg pid "$cat_parent" '{book_id: $bid, name: "早餐快餐", type: "expense", parent_id: $pid}')
api_request "POST" "/api/v1/categories" "$bc3"
assert_status "$HTTP_STATUS" 201 "Created subcategory"
assert_json "$HTTP_BODY" ".data.parent_id" "$cat_parent"

start_test "T1-F14-04" "Custom Tag Creation"
btg=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "公司报销", color: "#E5654E"}')
api_request "POST" "/api/v1/tags" "$btg"
assert_status "$HTTP_STATUS" 201 "Created custom tag"
tag_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F14-05" "Transaction Tag Association"
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "卡", type: "debit", balance: 10000}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
btx=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --arg tid "$tag_id" '{book_id: $bid, wallet_id: $wid, type: "expense", amount: 1200, tags: [$tid]}')
api_request "POST" "/api/v1/transactions" "$btx"
assert_status "$HTTP_STATUS" 201 "Transaction linked with tag"
assert_json "$HTTP_BODY" ".data.tags[0].id" "$tag_id"
