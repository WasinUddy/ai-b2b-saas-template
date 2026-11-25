#!/usr/bin/env bash
# True when this agent turn is plan mode. Cursor's stop payload does not include
# the composer mode, so a missing field falls back to the local composer record.

plan_mode_active() {
  local payload="$1"
  local mode cid db raw

  mode="$(printf '%s' "$payload" | jq -r '(.mode // .unified_mode // .unifiedMode // .composer_mode // .agent_mode // "") | ascii_downcase' 2>/dev/null || true)"
  if [[ "$mode" == "plan" ]]; then
    return 0
  fi

  cid="$(printf '%s' "$payload" | jq -r '.conversation_id // .session_id // empty' 2>/dev/null || true)"
  if [[ ! "$cid" =~ ^[A-Za-z0-9-]+$ ]]; then
    return 1
  fi

  db="${CURSOR_STATE_DB:-${HOME}/Library/Application Support/Cursor/User/globalStorage/state.vscdb}"
  if [[ ! -f "$db" ]] || ! command -v sqlite3 >/dev/null 2>&1; then
    return 1
  fi

  raw="$(sqlite3 "$db" "SELECT value FROM cursorDiskKV WHERE key = 'composerData:${cid}' LIMIT 1;" 2>/dev/null || true)"
  mode="$(printf '%s' "$raw" | jq -r '.unifiedMode // empty' 2>/dev/null || true)"
  [[ "$mode" == "plan" ]]
}
