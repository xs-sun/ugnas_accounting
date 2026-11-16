#!/usr/bin/env bash
# tests/e2e_runner.sh — Master E2E Test Suite Runner for 「七喵记账」
set -uo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TESTS_DIR="${PROJECT_ROOT}/tests"

# Source common utilities
source "${TESTS_DIR}/common/colors.sh"
source "${TESTS_DIR}/common/assert.sh"
source "${TESTS_DIR}/common/api_client.sh"
source "${TESTS_DIR}/common/db_helper.sh"

# Default configuration
TIER_FILTER="all"
FEATURE_FILTER=""
BAIL=0
VERBOSE=0
EPHEMERAL=0
TARGET_HOST="127.0.0.1"
TARGET_PORT="21010"
TARGET_URL=""

# Parse command line options
while [[ $# -gt 0 ]]; do
  case "$1" in
    --tier)
      TIER_FILTER="$2"
      shift 2
      ;;
    --feature)
      FEATURE_FILTER="$2"
      shift 2
      ;;
    --bail)
      BAIL=1
      export BAIL
      shift
      ;;
    --verbose)
      VERBOSE=1
      export VERBOSE
      shift
      ;;
    --ephemeral)
      EPHEMERAL=1
      shift
      ;;
    --target-url)
      TARGET_URL="$2"
      shift 2
      ;;
    --host)
      TARGET_HOST="$2"
      shift 2
      ;;
    --port)
      TARGET_PORT="$2"
      shift 2
      ;;
    --help|-h)
      echo "七喵记账 (com.xsguyu.accounting) — E2E Test Runner"
      echo ""
      echo "Usage: ./tests/e2e_runner.sh [OPTIONS]"
      echo ""
      echo "Options:"
      echo "  --tier <1|2|3|4|all>   Run specific test tier (default: all)"
      echo "  --feature <1-28>       Run test scripts for a specific feature number"
      echo "  --bail                 Halt execution immediately on first failure"
      echo "  --verbose              Print detailed request/response payloads and debug logs"
      echo "  --target-url <URL>     Target backend base URL (default: http://127.0.0.1:21010)"
      echo "  --host <HOST>          Target host (default: 127.0.0.1)"
      echo "  --port <PORT>          Target port (default: 21010)"
      echo "  --ephemeral            Spin up an isolated ephemeral SQLite test backend"
      echo "  --help, -h             Display this help message"
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      echo "Use --help for usage instructions." >&2
      exit 1
      ;;
  esac
done

if [[ -z "$TARGET_URL" ]]; then
  TARGET_URL="http://${TARGET_HOST}:${TARGET_PORT}"
fi
export TARGET_URL

EPHEMERAL_PID=""
TMP_DATA_DIR=""
TMP_LOG_DIR=""

cleanup() {
  if [[ -n "$EPHEMERAL_PID" ]]; then
    kill "$EPHEMERAL_PID" 2>/dev/null || true
    wait "$EPHEMERAL_PID" 2>/dev/null || true
  fi
  if [[ -n "$TMP_DATA_DIR" && -d "$TMP_DATA_DIR" ]]; then
    rm -rf "$TMP_DATA_DIR"
  fi
  if [[ -n "$TMP_LOG_DIR" && -d "$TMP_LOG_DIR" ]]; then
    rm -rf "$TMP_LOG_DIR"
  fi
}
trap cleanup EXIT INT TERM

# Check if target server is running
SERVER_ONLINE=0
if curl -s --connect-timeout 1 "${TARGET_URL}/api/v1/health" >/dev/null 2>&1; then
  SERVER_ONLINE=1
fi

if [[ "$EPHEMERAL" -eq 1 || "$SERVER_ONLINE" -eq 0 ]]; then
  # Pick an ephemeral port if needed
  if [[ "$SERVER_ONLINE" -eq 0 && "$EPHEMERAL" -eq 0 ]]; then
    print_info "Target server at ${TARGET_URL} not detected. Launching ephemeral test backend..."
  fi
  EPHEMERAL_PORT=$((21020 + (RANDOM % 80)))
  TMP_DATA_DIR=$(mktemp -d /tmp/ugnas_test_data_XXXXXX)
  TMP_LOG_DIR=$(mktemp -d /tmp/ugnas_test_log_XXXXXX)
  export UGAPP_DATA_DIR="$TMP_DATA_DIR"
  export UGAPP_LOG_DIR="$TMP_LOG_DIR"
  export DB_PATH="${TMP_DATA_DIR}/accounting.db"
  TARGET_URL="http://127.0.0.1:${EPHEMERAL_PORT}"
  export TARGET_URL

  python3 "${TESTS_DIR}/ephemeral_server.py" --port "$EPHEMERAL_PORT" --data-dir "$TMP_DATA_DIR" --log-dir "$TMP_LOG_DIR" >/dev/null 2>&1 &
  EPHEMERAL_PID=$!

  # Wait for server readiness
  READY=0
  for _ in {1..30}; do
    if curl -s --connect-timeout 1 "${TARGET_URL}/api/v1/health" >/dev/null 2>&1; then
      READY=1
      break
    fi
    sleep 0.1
  done

  if [[ "$READY" -ne 1 ]]; then
    echo -e "${CLR_RED}Failed to start ephemeral backend server on ${TARGET_URL}${CLR_RESET}" >&2
    exit 1
  fi
  print_info "Ephemeral backend online at ${TARGET_URL} (DB: ${DB_PATH})"
else
  print_info "Connected to existing backend at ${TARGET_URL}"
  # Check if local data dir exists
  if [[ -f "${PROJECT_ROOT}/data/accounting.db" ]]; then
    export DB_PATH="${PROJECT_ROOT}/data/accounting.db"
  fi
fi

# Track overall metrics
OVERALL_RUN=0
OVERALL_PASS=0
OVERALL_FAIL=0
OVERALL_SKIP=0
START_TIME=$(date +%s)

run_test_script() {
  local script_path="$1"
  if [[ ! -x "$script_path" ]]; then
    chmod +x "$script_path" 2>/dev/null || true
  fi

  # Execute in subshell to isolate environment variables
  local out
  local code=0
  out=$("$script_path" 2>&1) || code=$?
  echo "$out"

  # Extract pass/fail counts from script output
  local p_count f_count s_count r_count
  p_count=$(echo "$out" | grep -c "\[PASS\]" || true)
  f_count=$(echo "$out" | grep -c "\[FAIL\]" || true)
  s_count=$(echo "$out" | grep -c "\[SKIP\]" || true)
  r_count=$((p_count + f_count + s_count))

  OVERALL_RUN=$((OVERALL_RUN + r_count))
  OVERALL_PASS=$((OVERALL_PASS + p_count))
  OVERALL_FAIL=$((OVERALL_FAIL + f_count))
  OVERALL_SKIP=$((OVERALL_SKIP + s_count))

  if [[ "$code" -ne 0 && "$BAIL" -eq 1 ]]; then
    echo -e "\n${CLR_RED}Execution aborted due to failure (--bail enabled).${CLR_RESET}"
    exit 1
  fi
}

print_header "七喵记账 (com.xsguyu.accounting) E2E Test Suite"
echo -e "${CLR_DIM}Target: ${TARGET_URL} | Tier: ${TIER_FILTER} | Feature: ${FEATURE_FILTER:-all}${CLR_RESET}"

# Tier 1
if [[ "$TIER_FILTER" == "all" || "$TIER_FILTER" == "1" ]]; then
  print_header "Tier 1: Feature Coverage Suites (F01–F28)"
  for script in "${TESTS_DIR}"/tier1_features/test_f*.sh; do
    [[ -f "$script" ]] || continue
    if [[ -n "$FEATURE_FILTER" ]]; then
      feat_padded=$(printf "%02d" "$FEATURE_FILTER")
      if [[ "$script" != *"test_f${feat_padded}_"* ]]; then
        continue
      fi
    fi
    run_test_script "$script"
  done
fi

# Tier 2
if [[ "$TIER_FILTER" == "all" || "$TIER_FILTER" == "2" ]]; then
  print_header "Tier 2: Boundary & Corner Case Suites (F01–F28)"
  for script in "${TESTS_DIR}"/tier2_boundaries/test_f*.sh; do
    [[ -f "$script" ]] || continue
    if [[ -n "$FEATURE_FILTER" ]]; then
      feat_padded=$(printf "%02d" "$FEATURE_FILTER")
      if [[ "$script" != *"test_f${feat_padded}_"* ]]; then
        continue
      fi
    fi
    run_test_script "$script"
  done
fi

# Tier 3
if [[ -z "$FEATURE_FILTER" ]] && [[ "$TIER_FILTER" == "all" || "$TIER_FILTER" == "3" ]]; then
  print_header "Tier 3: Pairwise Subsystem Interaction Suites"
  for script in "${TESTS_DIR}"/tier3_interactions/test_t3_*.sh; do
    [[ -f "$script" ]] || continue
    run_test_script "$script"
  done
fi

# Tier 4
if [[ -z "$FEATURE_FILTER" ]] && [[ "$TIER_FILTER" == "all" || "$TIER_FILTER" == "4" ]]; then
  print_header "Tier 4: Realistic Real-World Workflow Scenarios"
  for script in "${TESTS_DIR}"/tier4_real_world/scenario_*.sh; do
    [[ -f "$script" ]] || continue
    run_test_script "$script"
  done
fi

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

# Print Summary Table
echo ""
echo -e "${CLR_BOLD}================================================================${CLR_RESET}"
echo -e "${CLR_BOLD}                  E2E TEST EXECUTION SUMMARY                    ${CLR_RESET}"
echo -e "${CLR_BOLD}================================================================${CLR_RESET}"
printf "  %-20s : %d\n" "Total Test Assertions" "$OVERALL_RUN"
printf "  %-20s : ${CLR_GREEN}%d${CLR_RESET}\n" "Passed" "$OVERALL_PASS"
printf "  %-20s : ${CLR_RED}%d${CLR_RESET}\n" "Failed" "$OVERALL_FAIL"
printf "  %-20s : ${CLR_YELLOW}%d${CLR_RESET}\n" "Skipped" "$OVERALL_SKIP"
printf "  %-20s : %ds\n" "Duration" "$ELAPSED"
echo -e "${CLR_BOLD}================================================================${CLR_RESET}"

if [[ "$OVERALL_FAIL" -eq 0 && "$OVERALL_RUN" -gt 0 ]]; then
  echo -e "${CLR_BG_GREEN}${CLR_WHITE}${CLR_BOLD}  ALL TESTS PASSED SUCCESSFULLY  ${CLR_RESET}\n"
  exit 0
else
  echo -e "${CLR_BG_RED}${CLR_WHITE}${CLR_BOLD}  TEST SUITE COMPLETED WITH FAILURES  ${CLR_RESET}\n"
  exit 1
fi
