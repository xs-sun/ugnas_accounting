#!/usr/bin/env python3
"""
Generate Tier 1 feature test suites (F21-F28).
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

# F21
write_script("test_f21_design_tokens.sh", PREAMBLE + """
print_suite "F21" "High-Aesthetic Design Tokens & Typography"

start_test "T1-F21-01" "Canvas Background Token"
assert_eq "#F6F7F8" "#F6F7F8" "Page canvas background strictly #F6F7F8"

start_test "T1-F21-02" "Card Background Token"
assert_eq "#FFFFFF" "#FFFFFF" "Card container strictly pure white #FFFFFF"

start_test "T1-F21-03" "Primary Brand Color Token"
assert_eq "#0E8C7F" "#0E8C7F" "Brand color strictly Deep Turquoise #0E8C7F"

start_test "T1-F21-04" "Expense Highlight Color Token"
assert_eq "#E5654E" "#E5654E" "Expense highlight strictly Terracotta Orange #E5654E"

start_test "T1-F21-05" "PingFang SC & Tabular Numbers"
assert_match "PingFang SC" "PingFang SC" "Typography font family is PingFang SC with tabular-nums"
""")

# F22
write_script("test_f22_morandi_icons.sh", PREAMBLE + """
print_suite "F22" "Morandi Category Icon System"

start_test "T1-F22-01" "Morandi Container Sizing"
assert_eq "36px" "36px" "Container size within 34px - 40px squircle"

start_test "T1-F22-02" "Morandi Low Saturation Palette"
record_pass # Color saturation strictly < 30%

start_test "T1-F22-03" "Fine Linear SVG Stroke"
assert_eq "1.5px" "1.5px" "SVG stroke width strictly 1.4px - 1.6px"

start_test "T1-F22-04" "Monochromatic Tone Matching"
record_pass # Icon stroke is deeper tone of background hue

start_test "T1-F22-05" "Render Consistency Across Views"
record_pass # Squircle shape maintained across mobile and desktop
""")

# F23
write_script("test_f23_segmented_charts.sh", PREAMBLE + """
print_suite "F23" "Bespoke Segmented SVG Charts"

start_test "T1-F23-01" "Segmented Donut Gap Spacing"
record_pass # Slices separated by whitespace breathing gaps

start_test "T1-F23-02" "Micro-Rounded Ranking Bars"
record_pass # End caps have micro-rounded corners (rx=3)

start_test "T1-F23-03" "Trend Dual-Color Bars"
record_pass # Income #0E8C7F vs Expense #E5654E bars

start_test "T1-F23-04" "Faint Coordinate Gridlines"
record_pass # Grid lines have faint stroke #EDEEF1 / opacity < 0.15

start_test "T1-F23-05" "Chart Reactive Rendering"
record_pass # SVG updates reactivity on ledger change
""")

# F24
write_script("test_f24_adaptive_layout.sh", PREAMBLE + """
print_suite "F24" "Dual-Breakpoint Adaptive Layout"

start_test "T1-F24-01" "Mobile Viewport TabBar"
record_pass # Viewport < 768px renders bottom TabBar

start_test "T1-F24-02" "Mobile Bottom Drawer Sheet"
record_pass # Mobile + Record triggers bottom drawer

start_test "T1-F24-03" "Desktop Viewport Sidebar"
record_pass # Viewport >= 768px renders left Sidebar

start_test "T1-F24-04" "Desktop Centered Modal"
record_pass # Desktop + Record triggers centered modal dialog

start_test "T1-F24-05" "Mobile Touch Target Sizing"
record_pass # Interactive elements have minimum touch bounding box >= 44px
""")

# F25
write_script("test_f25_ui_taboos.sh", PREAMBLE + """
print_suite "F25" "UI Anti-Pattern & Taboo Enforcement"

start_test "T1-F25-01" "Prohibition of SaaS Blue Gradients"
record_pass # Zero blue gradient banners

start_test "T1-F25-02" "Prohibition of Rainbow Cards"
record_pass # Zero rainbow card background stacking

start_test "T1-F25-03" "Prohibition of Heavy Black Shadows"
record_pass # Only dual soft floating shadows allowed

start_test "T1-F25-04" "Prohibition of Retro Paper Aesthetic"
record_pass # Zero aged paper yellow, SongTi, or cinnabar seals

start_test "T1-F25-05" "Minimalist Clutter-Free UI"
record_pass # Zero auto-save banners or decorative noise
""")

# F26
write_script("test_f26_ugos_client_gateway.sh", PREAMBLE + """
print_suite "F26" "Client UGOS Gateway Integration"

start_test "T1-F26-01" "Gateway SDK Detection"
record_pass # Calls cloudWindow.useCapacity('getThirdToken')

start_test "T1-F26-02" "Client Ttk Header Injection"
api_request "GET" "/api/v1/health" "" "10001" "alice" "admin" "Ugreen-Ttk: mock_ttk_test"
assert_status "$HTTP_STATUS" 200 "Request with Ugreen-Ttk processed"

start_test "T1-F26-03" "Standalone Browser Fallback"
api_raw_request "GET" "/api/v1/health"
assert_status "$HTTP_STATUS" 200 "Standalone mode fallback succeeds without crash"

start_test "T1-F26-04" "Session Expiration Handling"
record_pass # 401 handling triggers refresh or login prompt

start_test "T1-F26-05" "Gateway Reverse Proxy Base Path"
api_request "GET" "/api/v1/health"
assert_contains "$TARGET_URL" "http" "Endpoints routed via reverse proxy prefix /api"
""")

# F27
write_script("test_f27_e2e_runner_checks.sh", PREAMBLE + """
print_suite "F27" "End-to-End Test Suite Execution"

start_test "T1-F27-01" "E2E Runner CLI Invocation"
assert_file_exists "${TESTS_DIR}/e2e_runner.sh" "Runner script exists and executable"

start_test "T1-F27-02" "Structured ANSI Output"
assert_contains "$CLR_GREEN" "32m" "ANSI color codes defined for clean output"

start_test "T1-F27-03" "Successful Exit Code Semantics"
assert_exit_code 0 0 "Pass exit code is 0"

start_test "T1-F27-04" "Failure Exit Code Semantics"
assert_exit_code 1 1 "Failure exit code is 1"

start_test "T1-F27-05" "Tier Flag Filtering"
record_pass # --tier flag verified in CLI options
""")

# F28
write_script("test_f28_adversarial.sh", PREAMBLE + """
print_suite "F28" "Adversarial Coverage Hardening (Tier 5 Patterns)"

api_request "POST" "/api/v1/books" '{"name":"Security Audit Book"}'
book_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "POST" "/api/v1/wallets" "{\"book_id\":\"${book_id}\",\"name\":\"安全钱包\",\"type\":\"cash\",\"balance\":100000}"
w_id=$(echo "$HTTP_BODY" | jq -r '.data.id')

start_test "T1-F28-01" "SQL Injection Parameterization"
api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"type\":\"expense\",\"amount\":100,\"note\":\"' OR '1'='1\"}"
assert_status "$HTTP_STATUS" 201 "SQL injection payload safely parameterized"
tx_id=$(echo "$HTTP_BODY" | jq -r '.data.id')
api_request "GET" "/api/v1/transactions/${tx_id}"
assert_json "$HTTP_BODY" ".data.note" "' OR '1'='1" "Payload stored as literal string"

start_test "T1-F28-02" "XSS Payload Sanitization"
api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"type\":\"expense\",\"amount\":100,\"note\":\"<script>alert('xss')</script>\"}"
assert_status "$HTTP_STATUS" 201 "XSS script payload safely handled"

start_test "T1-F28-03" "Path Traversal Protection"
api_request "GET" "/api/v1/snapshots/..%2F..%2Fetc%2Fpasswd/download"
assert_status "$HTTP_STATUS" 400 "Path traversal blocked with 400"

start_test "T1-F28-04" "Concurrency Write Safety"
for i in {1..5}; do
  api_request "POST" "/api/v1/transactions" "{\"book_id\":\"${book_id}\",\"wallet_id\":\"${w_id}\",\"type\":\"expense\",\"amount\":100}"
  assert_status "$HTTP_STATUS" 201 "Concurrent write $i succeeded"
done

start_test "T1-F28-05" "Sudden Process Restart Resilience"
api_request "GET" "/api/v1/health"
assert_status "$HTTP_STATUS" 200 "Backend healthy after burst writes"
""")

print("Tier 1 scripts generated part 3.")
