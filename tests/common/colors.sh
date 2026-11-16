#!/usr/bin/env bash
# tests/common/colors.sh — ANSI color formatting and presentation utilities

if [[ -z "${NO_COLOR:-}" ]]; then
  CLR_RESET="\033[0m"
  CLR_BOLD="\033[1m"
  CLR_DIM="\033[2m"
  CLR_RED="\033[31m"
  CLR_GREEN="\033[32m"
  CLR_YELLOW="\033[33m"
  CLR_BLUE="\033[34m"
  CLR_MAGENTA="\033[35m"
  CLR_CYAN="\033[36m"
  CLR_WHITE="\033[37m"
  CLR_GRAY="\033[90m"
  CLR_BG_RED="\033[41m"
  CLR_BG_GREEN="\033[42m"
else
  CLR_RESET=""
  CLR_BOLD=""
  CLR_DIM=""
  CLR_RED=""
  CLR_GREEN=""
  CLR_YELLOW=""
  CLR_BLUE=""
  CLR_MAGENTA=""
  CLR_CYAN=""
  CLR_WHITE=""
  CLR_GRAY=""
  CLR_BG_RED=""
  CLR_BG_GREEN=""
fi

print_header() {
  local title="$1"
  echo -e "\n${CLR_BOLD}${CLR_CYAN}=== ${title} ===${CLR_RESET}"
}

print_suite() {
  local suite_id="$1"
  local suite_name="$2"
  echo -e "${CLR_BOLD}${CLR_BLUE}▶ [${suite_id}] ${suite_name}${CLR_RESET}"
}

print_pass() {
  local case_id="$1"
  local case_name="$2"
  local duration="${3:-}"
  if [[ -n "$duration" ]]; then
    echo -e "  ${CLR_GREEN}✓ [PASS]${CLR_RESET} ${case_id}: ${case_name} ${CLR_GRAY}(${duration})${CLR_RESET}"
  else
    echo -e "  ${CLR_GREEN}✓ [PASS]${CLR_RESET} ${case_id}: ${case_name}"
  fi
}

print_fail() {
  local case_id="$1"
  local case_name="$2"
  local reason="${3:-}"
  echo -e "  ${CLR_RED}✗ [FAIL]${CLR_RESET} ${case_id}: ${case_name}"
  if [[ -n "$reason" ]]; then
    echo -e "    ${CLR_RED}Reason: ${reason}${CLR_RESET}"
  fi
}

print_skip() {
  local case_id="$1"
  local case_name="$2"
  local reason="${3:-}"
  echo -e "  ${CLR_YELLOW}○ [SKIP]${CLR_RESET} ${case_id}: ${case_name} ${CLR_GRAY}(${reason})${CLR_RESET}"
}

print_info() {
  local msg="$1"
  echo -e "${CLR_GRAY}  ℹ ${msg}${CLR_RESET}"
}
