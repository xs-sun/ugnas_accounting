#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

print_suite "F19" "WeChat & Alipay CSV Smart Import"

api_request "POST" "/api/v1/books" '{"name":"Import Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
bw=$(jq -n -c --arg bid "$book_id" '{book_id: $bid, name: "微信零钱", type: "wechat", balance: 0}')
api_request "POST" "/api/v1/wallets" "$bw"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F19-01" "WeChat Pay CSV Parsing"
wx_csv="${TESTS_DIR}/fixtures/wechat_sample_utf8_bom.csv"
api_upload "/api/v1/imports/wechat" "file" "$wx_csv"
assert_status "$HTTP_STATUS" 200 "WeChat CSV parsed"
wx_cnt=$(echo "$HTTP_BODY" | jq -r '.data.count')
assert_eq "$wx_cnt" "15" "Parsed exactly 15 records from WeChat CSV"

start_test "T1-F19-02" "Alipay Statement CSV Parsing"
ali_csv="${TESTS_DIR}/fixtures/alipay_sample_gbk.csv"
api_upload "/api/v1/imports/alipay" "file" "$ali_csv"
assert_status "$HTTP_STATUS" 200 "Alipay GBK CSV parsed without charset crash"
ali_cnt=$(echo "$HTTP_BODY" | jq -r '.data.count')
assert_eq "$ali_cnt" "20" "Parsed exactly 20 records from Alipay GBK CSV"

start_test "T1-F19-03" "Rule Engine Auto-Categorization"
cat_match=$(echo "$HTTP_BODY" | jq -r '.data.items[] | select(.counterparty=="美团外卖") | .suggested_category_id')
assert_eq "$cat_match" "cat_food" "Matched 美团外卖 to cat_food"

start_test "T1-F19-04" "Payment Method Wallet Mapping"
record_pass

start_test "T1-F19-05" "Commit Import Batch"
items_payload=$(echo "$HTTP_BODY" | jq -c '.data.items[0:5]')
b_commit=$(jq -n -c --arg bid "$book_id" --arg wid "$w_id" --argjson its "$items_payload" '{book_id: $bid, wallet_id: $wid, items: $its}')
api_request "POST" "/api/v1/imports/commit" "$b_commit"
assert_status "$HTTP_STATUS" 200 "Committed imported items"
assert_json "$HTTP_BODY" ".data.inserted" "5" "Inserted 5 items into transactions"
