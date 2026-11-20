#!/usr/bin/env bash
# Run make check (backend + OpenAPI + frontend) when an agent stops, and ask it to continue when the check fails.
set -euo pipefail

agent=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --agent)
      agent="${2:-}"
      shift 2
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

case "$agent" in
  cursor | claude | codex) ;;
  *)
    echo "stop-check: --agent must be cursor, claude, or codex" >&2
    exit 1
    ;;
esac

payload=$(cat || true)

# Cursor also loads .claude/settings.json. Skip the Claude copy when both would run.
if [[ "$agent" == "claude" && -n "${CURSOR_VERSION:-}" ]]; then
  echo '{}'
  exit 0
fi

if ! command -v jq >/dev/null 2>&1 || ! command -v go >/dev/null 2>&1 || ! command -v node >/dev/null 2>&1 || ! command -v pnpm >/dev/null 2>&1; then
  if command -v jq >/dev/null 2>&1; then
    message="Fullstack check skipped because jq, go, node, and pnpm must all be installed."
    if [[ "$agent" == "cursor" ]]; then
      jq -n --arg message "$message" '{followup_message: $message}'
    else
      jq -n --arg message "$message" '{systemMessage: $message}'
    fi
  else
    echo '{}'
  fi
  exit 0
fi

root="${AI_HOOK_ROOT:-}"
if [[ -z "$root" ]]; then
  root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
fi
cd "$root"

cache="${AI_HOOK_CACHE_DIR:-$root/.cache/ai-hooks}"
mkdir -p "$cache"

fingerprint=$(
  {
    find . -name '*.go' -not -path './.git/*' -not -path './.cache/*' -not -path './bin/*' -not -path './webui/*'
    find . \( -name go.mod -o -name go.sum \) -not -path './.git/*' -not -path './.cache/*' -not -path './webui/*'
    for dir in api scripts tools migrations; do
      if [[ -d "$dir" ]]; then
        find "$dir" -type f
      fi
    done
    if [[ -d webui ]]; then
      find webui \
        \( -path webui/node_modules -o -path webui/dist -o -path webui/coverage \) -prune \
        -o -type f -print
    fi
    printf '%s\n' .spectral.yaml .golangci.yml .testcoverage.yml Makefile Dockerfile
  } | sort -u | while IFS= read -r file; do
    if [[ -f "$file" ]]; then
      shasum -a 256 "$file"
    fi
  done | shasum -a 256 | awk '{print $1}'
)

if [[ -f "$cache/last-green" && "$(cat "$cache/last-green")" == "$fingerprint" ]]; then
  echo '{}'
  exit 0
fi

log="$cache/last-check.log"
set +e
if [[ -n "${AI_HOOK_CHECK_CMD:-}" ]]; then
  bash -c "$AI_HOOK_CHECK_CMD" >"$log" 2>&1
else
  make check >"$log" 2>&1
fi
status=$?
set -e

if [[ "$status" -eq 0 ]]; then
  printf '%s\n' "$fingerprint" >"$cache/last-green"
  session=$(printf '%s' "$payload" | jq -r '.session_id // .conversation_id // empty')
  if [[ -n "$session" ]]; then
    rm -f "$cache/loops/${agent}-${session}"
  fi
  echo '{}'
  exit 0
fi

if [[ "$agent" != "cursor" ]]; then
  session=$(printf '%s' "$payload" | jq -r '.session_id // .conversation_id // "unknown"')
  mkdir -p "$cache/loops"
  count_file="$cache/loops/${agent}-${session}"
  count=0
  if [[ -f "$count_file" ]]; then
    count=$(cat "$count_file")
  fi
  count=$((count + 1))
  printf '%s\n' "$count" >"$count_file"
  if ((count > 3)); then
    jq -n --arg message "make check is still failing after 3 attempts. See .cache/ai-hooks/last-check.log." '{systemMessage: $message}'
    exit 0
  fi
fi

reason=$(printf 'make check failed; fix these violations\n\n%s' "$(tail -n 80 "$log")")
if [[ "$agent" == "cursor" ]]; then
  jq -n --arg message "$reason" '{followup_message: $message}'
else
  jq -n --arg message "$reason" '{decision: "block", reason: $message}'
fi
