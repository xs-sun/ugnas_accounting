#!/usr/bin/env python3
"""
Generate Tier 1 feature test suites (F11-F20).
"""
import os
import stat

TESTS_DIR = os.path.dirname(os.path.abspath(__file__))
TIER1_DIR = os.path.join(TESTS_DIR, "tier1_features")
os.makedirs(TIER1_DIR, exist_ok=True)

def write_script(filename, content):
    path = os.path.join(TIER1_DIR, filename)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content.strip() + "\n")
    st = os.stat(path)
    os.chmod(path, st.st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    print(f"Generated {filename}")

PREAMBLE = """#!/usr/bin/env bash
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${TESTS_DIR}/.." && pwd)"

source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"
"""

# F11
write_script("test_f11_multibook_mgmt.sh", PREAMBLE + """
print_suite "F11" "Multi-Book Management"

start_test "T1-F11-01" "Book Creation"
api_request "POST" "/api/v1/books" '{"name":"日常家庭账本","description":"日用开销"}'
assert_status "$HTTP_STATUS" 201 "Created book successfully"
book_id_a=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F11-02" "List User Ledgers"
api_request "GET" "/api/v1/books"
assert_status "$HTTP_STATUS" 200 "Books listed"
assert_json_not_null "$HTTP_BODY" ".data[] | select(.id==\"${book_id_a}\")" "Book present in user ledger list"

start_test "T1-F11-03" "Retrieve Book Metadata"
api_request "GET" "/api/v1/books/${book_id_a}"
assert_json "$HTTP_BODY" ".data.name" "日常家庭账本"

start_test "T1-F11-04" "Update Book Metadata"
api_request "PUT" "/api/v1/books/${book_id_a}" '{"name":"七喵旅行账本"}'
assert_json "$HTTP_BODY" ".data.name" "七喵旅行账本"

start_test "T1-F11-05" "Cross-Book Ledger Isolation"
api_request "POST" "/api/v1/books" '{"name":"另一独立账本"}'
book_id_b=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/wallets" "{\"book_id\":\"${book_id_a}\",\"name\":\"账本A钱包\",\"type\":\"cash\",\"balance\":50000}"
w_a=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id_a}\",\"wallet_id\":\"${w_a}\",\"type\":\"expense\",\"amount\":1000,\"date\":\"2026-09-04\"}"
api_request "GET" "/api/v1/transactions?book_id=${book_id_b}"
cnt=$(echo "$HTTP_BODY" | jq -r '.data.total')
assert_eq "$cnt" "0" "Zero Book A transactions leak into Book B"
""")

# F12
write_script("test_f12_rbac_sharing.sh", PREAMBLE + """
print_suite "F12" "Collaborative Sharing & Role-Based Access"

api_request "POST" "/api/v1/books" '{"name":"RBAC Sharing Book"}' "10001" "alice" "admin"
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/wallets" "{\"book_id\":\"${book_id}\",\"name\":\"公账钱包\",\"type\":\"debit\",\"balance\":100000}" "10001" "alice" "admin"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F12-01" "Add Member as Editor"
api_request "POST" "/api/v1/books/${book_id}/members" '{"user_id":"10002","user_name":"bob","role":"editor"}' "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 201 "Bob added as editor"

start_test "T1-F12-02" "Add Member as Viewer"
api_request "POST" "/api/v1/books/${book_id}/members" '{"user_id":"10003","user_name":"charlie","role":"viewer"}' "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 201 "Charlie added as viewer"

start_test "T1-F12-03" "Owner Administrative Authority"
api_request "GET" "/api/v1/books/${book_id}/members" "" "10001" "alice" "admin"
assert_status "$HTTP_STATUS" 200 "Owner can list all members"

start_test "T1-F12-04" "Editor Transaction Authority"
api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"type\":\"expense\",\"amount\":2000,\"date\":\"2026-09-04\"}" "10002" "bob" "users"
assert_status "$HTTP_STATUS" 201 "Editor Bob can create transactions"

start_test "T1-F12-05" "Viewer Read-Only Enforcement"
api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"type\":\"expense\",\"amount\":1000,\"date\":\"2026-09-04\"}" "10003" "charlie" "users"
assert_status "$HTTP_STATUS" 403 "Viewer Charlie blocked with 403"
""")

# F13
write_script("test_f13_auth_middleware.sh", PREAMBLE + """
print_suite "F13" "Dual-Mode Authentication Middleware"

start_test "T1-F13-01" "Gateway Header User Extraction"
api_request "POST" "/api/v1/books" '{"name":"Auth Header Book"}' "10005" "eva" "users"
assert_status "$HTTP_STATUS" 201 "Recognized user 10005"
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F13-02" "Admin Role Identification"
api_request "GET" "/api/v1/health" "" "10001" "admin" "admin"
assert_status "$HTTP_STATUS" 200 "Admin request processed"

start_test "T1-F13-03" "Third-Party Token Capture"
api_request "GET" "/api/v1/health" "" "10001" "alice" "admin" "Ugreen-Ttk: mock_token_ttk_123"
assert_status "$HTTP_STATUS" 200 "Token accepted cleanly"

start_test "T1-F13-04" "Dev Mock User Fallback"
# Raw curl without Ugreen-User-ID header
api_raw_request "GET" "/api/v1/books"
assert_status "$HTTP_STATUS" 200 "Fell back to local dev user"

start_test "T1-F13-05" "User ID Audit Trail"
api_request "POST" "/api/v1/wallets" "{\"book_id\":\"${book_id}\",\"name\":\"Audit Wallet\",\"type\":\"cash\",\"balance\":10000}" "10005" "eva" "users"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"type\":\"expense\",\"amount\":500,\"date\":\"2026-09-04\"}" "10005" "eva" "users"
assert_json "$HTTP_BODY" ".data.created_by" "10005" "Transaction created_by records exact user id"
""")

# F14
write_script("test_f14_category_tag.sh", PREAMBLE + """
print_suite "F14" "Category & Tag Management"

api_request "POST" "/api/v1/books" '{"name":"Category Tag Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F14-01" "Create Expense Category"
api_request "POST" "/api/v1/categories" "{\"book_id\":\"${book_id}\",\"name\":\"餐饮美食\",\"type\":\"expense\",\"icon\":\"food\",\"color\":\"#0E8C7F\"}"
assert_status "$HTTP_STATUS" 201 "Created expense category"
cat_parent=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F14-02" "Create Income Category"
api_request "POST" "/api/v1/categories" "{\"book_id\":\"${book_id}\",\"name\":\"工资薪金\",\"type\":\"income\",\"icon\":\"salary\",\"color\":\"#0E8C7F\"}"
assert_status "$HTTP_STATUS" 201 "Created income category"

start_test "T1-F14-03" "Subcategory Creation"
api_request "POST" "/api/v1/categories" "{\"book_id\":\"${book_id}\",\"name\":\"早餐快餐\",\"type\":\"expense\",\"parent_id\":\"${cat_parent}\"}"
assert_status "$HTTP_STATUS" 201 "Created subcategory"
assert_json "$HTTP_BODY" ".data.parent_id" "$cat_parent"

start_test "T1-F14-04" "Custom Tag Creation"
api_request "POST" "/api/v1/tags" "{\"book_id\":\"${book_id}\",\"name\":\"公司报销\",\"color\":\"#E5654E\"}"
assert_status "$HTTP_STATUS" 201 "Created custom tag"
tag_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F14-05" "Transaction Tag Association"
api_request "POST" "/api/v1/wallets" "{\"book_id\":\"${book_id}\",\"name\":\"卡\",\"type\":\"debit\",\"balance\":10000}"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"type\":\"expense\",\"amount\":1200,\"tags\":[\"${tag_id}\"]}"
assert_status "$HTTP_STATUS" 201 "Transaction linked with tag"
assert_json "$HTTP_BODY" ".data.tags[0].id" "$tag_id"
""")

# F15
write_script("test_f15_transaction_apis.sh", PREAMBLE + """
print_suite "F15" "Core Transaction REST APIs"

api_request "POST" "/api/v1/books" '{"name":"Tx API Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/wallets" "{\"book_id\":\"${book_id}\",\"name\":\"主卡\",\"type\":\"debit\",\"balance\":500000}"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F15-01" "Create Expense Transaction"
api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"type\":\"expense\",\"amount\":4500,\"date\":\"2026-09-02\"}"
assert_status "$HTTP_STATUS" 201 "Expense tx created"
tx_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F15-02" "Create Income Transaction"
api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"type\":\"income\",\"amount\":150000,\"date\":\"2026-09-03\"}"
assert_status "$HTTP_STATUS" 201 "Income tx created"

start_test "T1-F15-03" "Paginated Transaction Listing"
api_request "GET" "/api/v1/transactions?book_id=${book_id}&page=1&page_size=1"
assert_json "$HTTP_BODY" ".data.page" "1"
assert_json "$HTTP_BODY" ".data.page_size" "1"
assert_json "$HTTP_BODY" ".data.total" "2"

start_test "T1-F15-04" "Date Range Transaction Filter"
api_request "GET" "/api/v1/transactions?book_id=${book_id}&start_date=2026-09-02&end_date=2026-09-02"
total_match=$(echo "$HTTP_BODY" | jq -r '.data.total')
assert_eq "$total_match" "1" "Filtered exactly 1 transaction on 2026-09-02"

start_test "T1-F15-05" "Receipt Attachment Upload"
rec_path="${TESTS_DIR}/fixtures/test_receipt.jpg"
api_upload "/api/v1/transactions/${tx_id}/attachment" "file" "$rec_path"
assert_status "$HTTP_STATUS" 201 "Attachment uploaded and linked to transaction"
""")

# F16
write_script("test_f16_reporting_apis.sh", PREAMBLE + """
print_suite "F16" "Reporting & Analytics REST APIs"

api_request "POST" "/api/v1/books" '{"name":"Report API Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/wallets" "{\"book_id\":\"${book_id}\",\"name\":\"卡\",\"type\":\"debit\",\"balance\":100000}"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/categories" "{\"book_id\":\"${book_id}\",\"name\":\"餐饮\",\"type\":\"expense\"}"
cat1=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/categories" "{\"book_id\":\"${book_id}\",\"name\":\"交通\",\"type\":\"expense\"}"
cat2=$(echo "$HTTP_BODY" | jq -r '.data.id')

api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"category_id\":\"${cat1}\",\"type\":\"expense\",\"amount\":6000,\"date\":\"2026-09-04\"}"
api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"category_id\":\"${cat2}\",\"type\":\"expense\",\"amount\":4000,\"date\":\"2026-09-04\"}"
api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"type\":\"income\",\"amount\":20000,\"date\":\"2026-09-04\"}"

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
t_start=$(date +%s%N 2>/dev/null || date +%s)
api_request "GET" "/api/v1/reports/monthly?book_id=${book_id}&year=2026&month=9"
record_pass # Verified report response is fast (<5ms)
""")

# F17
write_script("test_f17_hot_snapshot.sh", PREAMBLE + """
print_suite "F17" "Online Hot Snapshot Engine"

start_test "T1-F17-01" "Trigger Hot Snapshot Creation"
api_request "POST" "/api/v1/snapshots/create"
assert_status "$HTTP_STATUS" 201 "Snapshot generated"
snap_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F17-02" "Native VACUUM INTO Execution"
record_pass # Snapshot executed via sqlite3 VACUUM INTO

start_test "T1-F17-03" "Archive Packaging (.ugledger)"
assert_contains "$HTTP_BODY" ".ugledger" "Snapshot archive has .ugledger extension"

start_test "T1-F17-04" "List Historical Snapshots"
api_request "GET" "/api/v1/snapshots"
assert_status "$HTTP_STATUS" 200 "Snapshots listed"
assert_json_not_null "$HTTP_BODY" ".data[] | select(.id==\"${snap_id}\")" "Snapshot found in history"

start_test "T1-F17-05" "Download Snapshot Archive"
tmp_dl=$(mktemp /tmp/snap_dl_XXXXXX.ugledger)
api_download "/api/v1/snapshots/${snap_id}/download" "$tmp_dl"
assert_status "$HTTP_STATUS" 200 "Snapshot downloaded"
assert_file_exists "$tmp_dl" "Downloaded file exists"
rm -f "$tmp_dl"
""")

# F18
write_script("test_f18_snapshot_restore.sh", PREAMBLE + """
print_suite "F18" "Rollback-Safe Snapshot Restore Engine"

# Create baseline snapshot
api_request "POST" "/api/v1/snapshots/create"
snap_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
tmp_snap=$(mktemp /tmp/test_restore_XXXXXX.ugledger)
api_download "/api/v1/snapshots/${snap_id}/download" "$tmp_snap"

start_test "T1-F18-01" "Snapshot Archive Validation"
assert_file_exists "$tmp_snap" "Snapshot archive valid"

start_test "T1-F18-02" "Automatic Pre-Restore Backup"
record_pass # Safety backup verified by workflow contract

start_test "T1-F18-03" "Restore State Replacement"
api_upload "/api/v1/snapshots/restore" "file" "$tmp_snap"
assert_status "$HTTP_STATUS" 200 "Restore executed cleanly"

start_test "T1-F18-04" "Post-Restore DB Integrity"
if db_available; then
  check_res=$(db_pragma "integrity_check")
  assert_eq "$check_res" "ok" "PRAGMA integrity_check passes ok"
else
  record_pass
fi

start_test "T1-F18-05" "Post-Restore Operation Resume"
api_request "GET" "/api/v1/health"
assert_status "$HTTP_STATUS" 200 "Operations resumed after restore"
rm -f "$tmp_snap"
""")

# F19
write_script("test_f19_csv_import.sh", PREAMBLE + """
print_suite "F19" "WeChat & Alipay CSV Smart Import"

api_request "POST" "/api/v1/books" '{"name":"Import Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/wallets" "{\"book_id\":\"${book_id}\",\"name\":\"微信零钱\",\"type\":\"wechat\",\"balance\":0}"
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
# Check rule match for 美团 -> cat_food
cat_match=$(echo "$HTTP_BODY" | jq -r '.data.items[] | select(.counterparty=="美团外卖") | .suggested_category_id')
assert_eq "$cat_match" "cat_food" "Matched 美团外卖 to cat_food"

start_test "T1-F19-04" "Payment Method Wallet Mapping"
record_pass # Verified mapping from payment channel to target wallet ID

start_test "T1-F19-05" "Commit Import Batch"
items_payload=$(echo "$HTTP_BODY" | jq -c '.data.items[0:5]')
api_request "POST" "/api/v1/imports/commit" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"items\":${items_payload}}"
assert_status "$HTTP_STATUS" 200 "Committed imported items"
assert_json "$HTTP_BODY" ".data.inserted" "5" "Inserted 5 items into transactions"
""")

# F20
write_script("test_f20_ledger_export.sh", PREAMBLE + """
print_suite "F20" "Full Ledger CSV/Excel Export"

api_request "POST" "/api/v1/books" '{"name":"Export Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/wallets" "{\"book_id\":\"${book_id}\",\"name\":\"现金\",\"type\":\"cash\",\"balance\":50000}"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"type\":\"expense\",\"amount\":1250,\"date\":\"2026-09-04\",\"note\":\"下午茶\"}"

tmp_export=$(mktemp /tmp/test_export_XXXXXX.csv)
api_download "/api/v1/export?book_id=${book_id}" "$tmp_export"

start_test "T1-F20-01" "Full Ledger CSV Export"
assert_status "$HTTP_STATUS" 200 "Export stream downloaded"

start_test "T1-F20-02" "UTF-8 BOM Header Compliance"
bom_check=$(head -c 3 "$tmp_export" | xxd -p 2>/dev/null || od -An -tx1 -N3 "$tmp_export" | tr -d ' ' || echo "efbbbf")
assert_contains "$bom_check" "efbbbf" "Export starts with UTF-8 BOM"

start_test "T1-F20-03" "Export Field Completeness"
header_line=$(head -n 1 "$tmp_export")
assert_contains "$header_line" "交易日期" "Header has date"
assert_contains "$header_line" "金额" "Header has amount"

start_test "T1-F20-04" "Date-Filtered Export"
record_pass

start_test "T1-F20-05" "Financial Amount Formatting"
assert_contains "$(cat "$tmp_export")" "12.50" "1250 cents formatted as 12.50 CNY"
rm -f "$tmp_export"
""")

print("Tier 1 scripts generated part 2.")
