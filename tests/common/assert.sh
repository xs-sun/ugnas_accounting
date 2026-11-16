#!/usr/bin/env bash
# tests/common/assert.sh — Assertion functions and test status tracking

# Initialize counters
SUITE_RUN=0
SUITE_PASS=0
SUITE_FAIL=0
SUITE_SKIP=0

CURRENT_TEST_ID=""
CURRENT_TEST_NAME=""
CURRENT_TEST_START=0
CURRENT_TEST_FINISHED=1
CURRENT_TEST_FAILED=0

_test_duration() {
  if command -v perl >/dev/null 2>&1; then
    local now
    now=$(perl -MTime::HiRes=time -e 'printf "%.4f\n", time')
    local diff
    diff=$(awk -v n="$now" -v s="$CURRENT_TEST_START" 'BEGIN { printf "%.1fms", (n - s) * 1000 }')
    echo "$diff"
  else
    local now
    now=$(date +%s)
    echo "$((now - CURRENT_TEST_START))s"
  fi
}

_finish_previous_test() {
  if [[ -n "$CURRENT_TEST_ID" && "$CURRENT_TEST_FINISHED" -eq 0 ]]; then
    CURRENT_TEST_FINISHED=1
    if [[ "$CURRENT_TEST_FAILED" -eq 0 ]]; then
      record_pass
    fi
  fi
}

start_test() {
  _finish_previous_test
  CURRENT_TEST_ID="$1"
  CURRENT_TEST_NAME="$2"
  CURRENT_TEST_FINISHED=0
  CURRENT_TEST_FAILED=0
  SUITE_RUN=$((SUITE_RUN + 1))
  if command -v perl >/dev/null 2>&1; then
    CURRENT_TEST_START=$(perl -MTime::HiRes=time -e 'printf "%.4f\n", time')
  else
    CURRENT_TEST_START=$(date +%s)
  fi
}

record_pass() {
  CURRENT_TEST_FINISHED=1
  SUITE_PASS=$((SUITE_PASS + 1))
  local dur
  dur=$(_test_duration)
  print_pass "$CURRENT_TEST_ID" "$CURRENT_TEST_NAME" "$dur"
}

record_fail() {
  CURRENT_TEST_FAILED=1
  CURRENT_TEST_FINISHED=1
  local reason="$1"
  SUITE_FAIL=$((SUITE_FAIL + 1))
  print_fail "$CURRENT_TEST_ID" "$CURRENT_TEST_NAME" "$reason"
  if [[ "${BAIL:-0}" -eq 1 ]]; then
    echo -e "${CLR_RED}Bailing out due to failure (--bail enabled).${CLR_RESET}" >&2
    exit 1
  fi
}

record_skip() {
  CURRENT_TEST_FINISHED=1
  local reason="$1"
  SUITE_SKIP=$((SUITE_SKIP + 1))
  print_skip "$CURRENT_TEST_ID" "$CURRENT_TEST_NAME" "$reason"
}

finish_suite() {
  _finish_previous_test
  if [[ "$SUITE_FAIL" -gt 0 ]]; then
    return 1
  fi
  return 0
}

trap finish_suite EXIT

assert_eq() {
  local actual="$1"
  local expected="$2"
  local desc="${3:-Values should be equal}"
  if [[ "$actual" == "$expected" ]]; then
    return 0
  else
    record_fail "${desc} (Expected: '${expected}', Got: '${actual}')"
    return 1
  fi
}

assert_ne() {
  local actual="$1"
  local unexpected="$2"
  local desc="${3:-Values should not be equal}"
  if [[ "$actual" != "$unexpected" ]]; then
    return 0
  else
    record_fail "${desc} (Expected not: '${unexpected}', Got: '${actual}')"
    return 1
  fi
}

assert_status() {
  local actual="$1"
  local expected="$2"
  local desc="${3:-HTTP status code check}"
  if [[ "$actual" -eq "$expected" ]]; then
    return 0
  else
    record_fail "${desc} (Expected HTTP: ${expected}, Got: ${actual})"
    return 1
  fi
}

assert_status_in() {
  local actual="$1"
  shift
  local expected_list=("$@")
  for exp in "${expected_list[@]}"; do
    if [[ "$actual" -eq "$exp" ]]; then
      return 0
    fi
  done
  record_fail "HTTP status ${actual} not in expected list (${expected_list[*]})"
  return 1
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local desc="${3:-Should contain substring}"
  if [[ "$haystack" == *"$needle"* ]]; then
    return 0
  else
    record_fail "${desc} (Substring '${needle}' not found in target)"
    return 1
  fi
}

assert_match() {
  local text="$1"
  local regex="$2"
  local desc="${3:-Should match regular expression}"
  if [[ "$text" =~ $regex ]]; then
    return 0
  else
    record_fail "${desc} (Pattern '${regex}' did not match: '${text}')"
    return 1
  fi
}

assert_json() {
  local json_str="$1"
  local jq_expr="$2"
  local expected="$3"
  local desc="${4:-JSON field verification}"
  local actual
  actual=$(echo "$json_str" | jq -r "$jq_expr" 2>/dev/null || echo "__JQ_PARSE_ERROR__")
  if [[ "$actual" == "$expected" ]]; then
    return 0
  else
    record_fail "${desc} [jq: ${jq_expr}] (Expected: '${expected}', Got: '${actual}')"
    return 1
  fi
}

assert_json_num_eq() {
  local json_str="$1"
  local jq_expr="$2"
  local expected="$3"
  local desc="${4:-JSON numeric field verification}"
  local actual
  actual=$(echo "$json_str" | jq -r "$jq_expr" 2>/dev/null || echo "")
  if [[ "$actual" =~ ^-?[0-9]+$ ]] && [[ "$expected" =~ ^-?[0-9]+$ ]] && [[ "$actual" -eq "$expected" ]]; then
    return 0
  else
    record_fail "${desc} [jq: ${jq_expr}] (Expected numeric: ${expected}, Got: '${actual}')"
    return 1
  fi
}

assert_json_not_null() {
  local json_str="$1"
  local jq_expr="$2"
  local desc="${3:-JSON field should exist and not be null}"
  local val
  val=$(echo "$json_str" | jq -r "$jq_expr" 2>/dev/null || echo "null")
  if [[ "$val" != "null" ]] && [[ -n "$val" ]] && [[ "$val" != "__JQ_PARSE_ERROR__" ]]; then
    return 0
  else
    record_fail "${desc} [jq: ${jq_expr}] (Got null or missing)"
    return 1
  fi
}

assert_file_exists() {
  local path="$1"
  local desc="${2:-File should exist}"
  if [[ -e "$path" ]]; then
    return 0
  else
    record_fail "${desc} (File not found at: '${path}')"
    return 1
  fi
}

assert_exit_code() {
  local code="$1"
  local expected="${2:-0}"
  local desc="${3:-Command exit code check}"
  if [[ "$code" -eq "$expected" ]]; then
    return 0
  else
    record_fail "${desc} (Expected exit code: ${expected}, Got: ${code})"
    return 1
  fi
}

assert_gt() {
  local val="$1"
  local min="$2"
  local desc="${3:-Value should be greater than minimum}"
  if [[ "$val" -gt "$min" ]]; then
    return 0
  else
    record_fail "${desc} (${val} not > ${min})"
    return 1
  fi
}
