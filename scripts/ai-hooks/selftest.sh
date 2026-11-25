#!/usr/bin/env bash
# Feed recorded agent payloads through the shared hooks and check each output shape.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
hooks="$root/scripts/ai-hooks"
data="$hooks/testdata"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

fail() {
  echo "hooks self-test: $*" >&2
  exit 1
}

assert_jq() {
  local out="$1"
  local filter="$2"
  local message="$3"
  if ! printf '%s' "$out" | jq -e "$filter" >/dev/null; then
    fail "$message: $out"
  fi
}

run_stop() {
  local agent="$1"
  local cache="$2"
  local cmd="$3"
  local target_root="${4:-$root}"
  # Cursor exports CURSOR_VERSION. Drop it here so the Claude and Codex
  # cases exercise their own output. The double-run case sets it explicitly.
  env -u CURSOR_VERSION \
    AI_HOOK_CACHE_DIR="$cache" AI_HOOK_CHECK_CMD="$cmd" AI_HOOK_ROOT="$target_root" \
    "$hooks/stop-check.sh" --agent "$agent" <"$data/${agent}-stop.json"
}

assert_block() {
  local agent="$1"
  local cache="$work/block-$agent"
  local out
  out=$(run_stop "$agent" "$cache" false)
  if [[ "$agent" == "cursor" ]]; then
    assert_jq "$out" '.followup_message | contains("make check failed")' "cursor failure did not return followup_message"
  else
    assert_jq "$out" '.decision == "block" and (.reason | contains("make check failed"))' "$agent failure did not return decision=block"
  fi
}

assert_pass() {
  local agent="$1"
  local out
  out=$(run_stop "$agent" "$work/pass-$agent" true)
  assert_jq "$out" '. == {}' "$agent success was not {}"
}

assert_block cursor
assert_block claude
assert_block codex
assert_pass cursor
assert_pass claude
assert_pass codex

# Verify editing a frontend file under webui/ invalidates the stop-check fingerprint cache.
fe_root="$work/fe-repo"
fe_cache="$work/fe-cache"
mkdir -p "$fe_root/webui/src"
printf 'export const v = 1\n' >"$fe_root/webui/src/app.ts"
out=$(run_stop cursor "$fe_cache" true "$fe_root")
assert_jq "$out" '. == {}' "initial frontend pass was not {}"
# Second run with unchanged webui file must hit cache and stay green even if check cmd fails.
out=$(run_stop cursor "$fe_cache" false "$fe_root")
assert_jq "$out" '. == {}' "cached frontend pass did not short-circuit"
# Changing a webui file must bust the cache and trigger the check command.
printf 'export const v = 2\n' >"$fe_root/webui/src/app.ts"
out=$(run_stop cursor "$fe_cache" false "$fe_root")
assert_jq "$out" '.followup_message | contains("make check failed")' "frontend edit did not bust stop-check fingerprint"

loop_cache="$work/loop"
for _ in 1 2 3; do
  out=$(run_stop claude "$loop_cache" false)
  assert_jq "$out" '.decision == "block"' "claude loop attempt returned unexpected output"
done
out=$(run_stop claude "$loop_cache" false)
assert_jq "$out" '.systemMessage | contains("3 attempts")' "claude loop cap did not warn"
assert_jq "$out" 'has("decision") | not' "claude loop cap still blocked"

# A failure in another live conversation's files is left for that agent.
share="$work/share"
mkdir -p "$share/sessions"
printf 'webui/src/theirs.tsx\n' >"$share/sessions/cursor-other-agent"
printf 'internal/handler/mine.go\n' >"$share/sessions/cursor-cursor-session"
out=$(run_stop cursor "$share" 'printf "%s\n" "webui/src/theirs.tsx(4,1): error TS2304"; exit 1')
assert_jq "$out" '. == {}' "failure in another agent's file was handed back to this agent"
out=$(run_stop cursor "$share" 'printf "%s\n" "internal/handler/mine.go:12: syntax error"; exit 1')
assert_jq "$out" '.followup_message | contains("make check failed") and contains("do not fix it")' "owned failure did not tell this agent it may leave another agent's files"
out=$(run_stop cursor "$share" 'printf "%s\n" "webui/src/theirs.tsx:1: error" "internal/handler/mine.go:2: error"; exit 1')
assert_jq "$out" '.followup_message | contains("Other agents'\'' files:") and contains("webui/src/theirs.tsx")' "mixed failure did not name the other agent's files"
out=$(run_stop claude "$share" 'printf "%s\n" "webui/src/theirs.tsx:1: error"; exit 1')
assert_jq "$out" '. == {}' "claude was asked to fix another agent's file"
out=$(run_stop cursor "$share" 'printf "%s\n" "webui/src/theirs.tsx:1: error" "webui/src/types/api.gen.ts:3: error"; exit 1')
assert_jq "$out" '.followup_message | contains("do not fix it") and contains("webui/src/types/api.gen.ts")' "unowned diagnostic was dropped because another agent's file was also in the log"
out=$(run_stop cursor "$work/ambiguous" 'printf "%s\n" "FAIL: coverage below 85"; exit 1')
assert_jq "$out" '.followup_message | contains("do not fix it")' "ambiguous failure did not allow leaving another agent's breakage"

stale="$work/stale-owner"
mkdir -p "$stale/sessions"
printf 'webui/src/theirs.tsx\n' >"$stale/sessions/cursor-other-agent"
touch -t 202001010000 "$stale/sessions/cursor-other-agent"
out=$(run_stop cursor "$stale" 'printf "%s\n" "webui/src/theirs.tsx(4,1): error"; exit 1')
assert_jq "$out" '.followup_message | contains("do not fix it")' "an expired agent still swallowed the failure"

out=$(CURSOR_VERSION=test AI_HOOK_CACHE_DIR="$work/double" AI_HOOK_CHECK_CMD=false AI_HOOK_ROOT="$root" \
  "$hooks/stop-check.sh" --agent claude <"$data/claude-stop.json")
assert_jq "$out" '. == {}' "claude hook inside Cursor did not no-op"

out=$(env -u CURSOR_VERSION AI_HOOK_CACHE_DIR="$work/plan-mode" AI_HOOK_CHECK_CMD=false AI_HOOK_ROOT="$root" \
  "$hooks/stop-check.sh" --agent cursor <"$data/cursor-stop-plan.json")
assert_jq "$out" '. == {}' "plan mode stop hook did not no-op"

if command -v sqlite3 >/dev/null 2>&1; then
  plan_db="$work/state.vscdb"
  sqlite3 "$plan_db" "CREATE TABLE cursorDiskKV (key TEXT, value TEXT);"
  sqlite3 "$plan_db" "INSERT INTO cursorDiskKV (key, value) VALUES ('composerData:plan-chat', '{\"unifiedMode\":\"plan\"}');"
  sqlite3 "$plan_db" "INSERT INTO cursorDiskKV (key, value) VALUES ('composerData:agent-chat', '{\"unifiedMode\":\"agent\"}');"
  out=$(CURSOR_STATE_DB="$plan_db" env -u CURSOR_VERSION AI_HOOK_CACHE_DIR="$work/plan-db" AI_HOOK_CHECK_CMD=false AI_HOOK_ROOT="$root" \
    "$hooks/stop-check.sh" --agent cursor <<'EOF'
{"conversation_id":"plan-chat","hook_event_name":"stop","status":"completed"}
EOF
)
  assert_jq "$out" '. == {}' "plan composer stop hook did not no-op"
  out=$(CURSOR_STATE_DB="$plan_db" env -u CURSOR_VERSION AI_HOOK_CACHE_DIR="$work/agent-db" AI_HOOK_CHECK_CMD=false AI_HOOK_ROOT="$root" \
    "$hooks/stop-check.sh" --agent cursor <<'EOF'
{"conversation_id":"agent-chat","hook_event_name":"stop","status":"completed"}
EOF
)
  assert_jq "$out" '.followup_message | contains("make check failed")' "agent composer stop hook skipped the check"
fi

messy='package p

func F(){a:=1}
'
mkdir -p "$work/internal/handler"
printf '%s' "$messy" >"$work/internal/handler/cursor.go"
printf '%s' "$messy" >"$work/codex.go"
gen_marker="$work/generated.flag"
sed "s|FILE_PATH|$work/internal/handler/cursor.go|" "$data/cursor-edit.json" \
  | AI_HOOK_SKIP_SWAG=1 AI_HOOK_GENERATE_CMD="touch '$gen_marker'" AI_HOOK_ROOT="$work" "$hooks/post-edit.sh" --agent cursor >/dev/null
if [[ ! -f "$gen_marker" ]]; then
  fail "post-edit did not run OpenAPI generate hook after backend Go edit"
fi
sed "s|FILE_PATH|$work/codex.go|" "$data/codex-edit.json" \
  | AI_HOOK_SKIP_SWAG=1 AI_HOOK_SKIP_GENERATE=1 AI_HOOK_ROOT="$work" "$hooks/post-edit.sh" --agent codex >/dev/null
claude_file="$work/claude.go"
printf '%s' "$messy" >"$claude_file"
sed "s|FILE_PATH|$claude_file|" "$data/claude-edit.json" \
  | AI_HOOK_SKIP_SWAG=1 AI_HOOK_SKIP_GENERATE=1 AI_HOOK_ROOT="$work" "$hooks/post-edit.sh" --agent claude >/dev/null
unformatted=$(gofmt -l "$work/internal/handler/cursor.go" "$work/codex.go" "$claude_file" || true)
if [[ -n "$unformatted" ]]; then
  fail "post-edit left these files unformatted: $unformatted"
fi

owned="$work/owned-edit"
owned_cache="$work/owned-cache"
mkdir -p "$owned"
printf 'package p\n\nfunc F() { a := 1 }\n' >"$owned/owned.go"
jq -n --arg file "$owned/owned.go" '{conversation_id:"conv-1", file_path:$file, edits:[{old_string:"", new_string:""}]}' \
  | AI_HOOK_SKIP_SWAG=1 AI_HOOK_SKIP_GENERATE=1 AI_HOOK_ROOT="$owned" AI_HOOK_CACHE_DIR="$owned_cache" \
    "$hooks/post-edit.sh" --agent cursor >/dev/null
if ! grep -qx 'owned.go' "$owned_cache/sessions/cursor-conv-1"; then
  fail "post-edit did not record the edited file for this agent"
fi

plan_file="$work/plan.go"
printf '%s' "$messy" >"$plan_file"
jq -n --arg file "$plan_file" '{file_path:$file, mode:"plan", edits:[{old_string:"", new_string:""}]}' \
  | AI_HOOK_SKIP_SWAG=1 AI_HOOK_SKIP_GENERATE=1 AI_HOOK_ROOT="$work" "$hooks/post-edit.sh" --agent cursor >/dev/null
if [[ -z "$(gofmt -l "$plan_file" || true)" ]]; then
  fail "plan mode post-edit formatted a file"
fi

# Verify post-edit formats edited frontend TypeScript files when prettier is installed.
if [[ -x "$root/webui/node_modules/.bin/prettier" ]]; then
  mkdir -p "$work/webui/src"
  ln -s "$root/webui/node_modules" "$work/webui/node_modules"
  cp "$root/webui/.prettierrc.json" "$work/webui/.prettierrc.json"
  cp "$root/webui/package.json" "$work/webui/package.json"
  cp "$root/webui/src/index.css" "$work/webui/src/index.css"
  ts_file="$work/webui/src/sample.ts"
  printf 'export const x=1;\n' >"$ts_file"
  sed "s|FILE_PATH|$ts_file|" "$data/cursor-edit.json" \
    | AI_HOOK_ROOT="$work" "$hooks/post-edit.sh" --agent cursor >/dev/null
  if ! (cd "$work/webui" && ./node_modules/.bin/prettier --check "$ts_file" >/dev/null 2>&1); then
    fail "post-edit left frontend file unformatted: $(cat "$ts_file")"
  fi
fi
