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

out=$(CURSOR_VERSION=test AI_HOOK_CACHE_DIR="$work/double" AI_HOOK_CHECK_CMD=false AI_HOOK_ROOT="$root" \
  "$hooks/stop-check.sh" --agent claude <"$data/claude-stop.json")
assert_jq "$out" '. == {}' "claude hook inside Cursor did not no-op"

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
