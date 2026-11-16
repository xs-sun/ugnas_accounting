#!/usr/bin/env bash
# tests/common/api_client.sh — HTTP REST API client with UGOS gateway headers

TARGET_URL="${TARGET_URL:-http://127.0.0.1:21010}"
VERBOSE="${VERBOSE:-0}"

HTTP_STATUS=0
HTTP_BODY=""

api_request() {
  local method="$1"
  local endpoint="$2"
  local body="${3:-}"
  local user_id="${4:-10001}"
  local user_name="${5:-alice}"
  local user_type="${6:-admin}"
  local extra_headers=()
  if [[ $# -gt 6 ]]; then
    extra_headers=("${@:7}")
  fi

  local url="${TARGET_URL%/}${endpoint}"
  local tmp_resp
  tmp_resp=$(mktemp)

  local curl_args=(
    -s -S
    -X "$method"
    -w "%{http_code}"
    -o "$tmp_resp"
    -H "Content-Type: application/json"
    -H "Ugreen-User-ID: ${user_id}"
    -H "Ugreen-User-Name: ${user_name}"
    -H "Ugreen-User-Type: ${user_type}"
    -H "Ugreen-Ttk: ttk_${user_id}"
  )

  if [[ ${#extra_headers[@]} -gt 0 ]]; then
    for hdr in "${extra_headers[@]}"; do
      curl_args+=(-H "$hdr")
    done
  fi

  if [[ -n "$body" ]]; then
    curl_args+=(-d "$body")
  fi

  if [[ "$VERBOSE" -eq 1 ]]; then
    echo -e "${CLR_GRAY:-}[API REQ] ${method} ${url} (User: ${user_id})${CLR_RESET:-}" >&2
    if [[ -n "$body" ]]; then
      echo -e "${CLR_GRAY:-}[REQ BODY] ${body}${CLR_RESET:-}" >&2
    fi
  fi

  HTTP_STATUS=$(curl "${curl_args[@]}" "$url" 2>/dev/null || echo "000")
  HTTP_BODY=$(cat "$tmp_resp" 2>/dev/null || echo "")
  rm -f "$tmp_resp"

  if [[ "$VERBOSE" -eq 1 ]]; then
    echo -e "${CLR_GRAY:-}[API RES] HTTP ${HTTP_STATUS}: ${HTTP_BODY}${CLR_RESET:-}" >&2
  fi

  return 0
}

api_raw_request() {
  local method="$1"
  local endpoint="$2"
  local body="${3:-}"
  shift 3 2>/dev/null || true
  local custom_headers=()
  if [[ $# -gt 0 ]]; then
    custom_headers=("$@")
  fi

  local url="${TARGET_URL%/}${endpoint}"
  local tmp_resp
  tmp_resp=$(mktemp)

  local curl_args=(
    -s -S
    -X "$method"
    -w "%{http_code}"
    -o "$tmp_resp"
  )

  for hdr in "${custom_headers[@]}"; do
    curl_args+=(-H "$hdr")
  done

  if [[ -n "$body" ]]; then
    curl_args+=(-d "$body")
  fi

  HTTP_STATUS=$(curl "${curl_args[@]}" "$url" 2>/dev/null || echo "000")
  HTTP_BODY=$(cat "$tmp_resp" 2>/dev/null || echo "")
  rm -f "$tmp_resp"

  return 0
}

api_upload() {
  local endpoint="$1"
  local field_name="$2"
  local file_path="$3"
  local user_id="${4:-10001}"
  local user_name="${5:-alice}"
  local user_type="${6:-admin}"
  local extra_form_fields=()
  if [[ $# -gt 6 ]]; then
    extra_form_fields=("${@:7}")
  fi

  local url="${TARGET_URL%/}${endpoint}"
  local tmp_resp
  tmp_resp=$(mktemp)

  local curl_args=(
    -s -S
    -X POST
    -w "%{http_code}"
    -o "$tmp_resp"
    -H "Ugreen-User-ID: ${user_id}"
    -H "Ugreen-User-Name: ${user_name}"
    -H "Ugreen-User-Type: ${user_type}"
    -H "Ugreen-Ttk: ttk_${user_id}"
    -F "${field_name}=@${file_path}"
  )

  if [[ ${#extra_form_fields[@]} -gt 0 ]]; then
    for f in "${extra_form_fields[@]}"; do
      curl_args+=(-F "$f")
    done
  fi

  HTTP_STATUS=$(curl "${curl_args[@]}" "$url" 2>/dev/null || echo "000")
  HTTP_BODY=$(cat "$tmp_resp" 2>/dev/null || echo "")
  rm -f "$tmp_resp"

  return 0
}

api_download() {
  local endpoint="$1"
  local output_file="$2"
  local user_id="${3:-10001}"
  local user_name="${4:-alice}"
  local user_type="${5:-admin}"

  local url="${TARGET_URL%/}${endpoint}"

  HTTP_STATUS=$(curl -s -S -w "%{http_code}" -o "$output_file" \
    -H "Ugreen-User-ID: ${user_id}" \
    -H "Ugreen-User-Name: ${user_name}" \
    -H "Ugreen-User-Type: ${user_type}" \
    -H "Ugreen-Ttk: ttk_${user_id}" \
    "$url" 2>/dev/null || echo "000")

  return 0
}
