#!/usr/bin/env bash
# tests/common/db_helper.sh — Direct SQLite DB inspection utilities

DB_PATH="${DB_PATH:-}"

db_available() {
  if [[ -n "$DB_PATH" ]] && [[ -f "$DB_PATH" ]] && command -v sqlite3 >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

db_query() {
  local sql="$1"
  if db_available; then
    sqlite3 -batch "$DB_PATH" "$sql" 2>/dev/null
  else
    echo ""
  fi
}

db_pragma() {
  local pragma_name="$1"
  if db_available; then
    sqlite3 -batch "$DB_PATH" "PRAGMA ${pragma_name};" 2>/dev/null
  else
    echo ""
  fi
}

db_table_exists() {
  local table_name="$1"
  if db_available; then
    local count
    count=$(sqlite3 -batch "$DB_PATH" "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='${table_name}';" 2>/dev/null || echo "0")
    if [[ "$count" -ge 1 ]]; then
      return 0
    fi
  fi
  return 1
}

db_count_rows() {
  local table_name="$1"
  local where_clause="${2:-1=1}"
  if db_available; then
    sqlite3 -batch "$DB_PATH" "SELECT count(*) FROM ${table_name} WHERE ${where_clause};" 2>/dev/null || echo "0"
  else
    echo "0"
  fi
}
