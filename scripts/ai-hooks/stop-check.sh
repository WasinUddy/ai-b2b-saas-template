#!/usr/bin/env bash
# Run make check (backend + OpenAPI + frontend) when an agent stops.
# Hand a failure back to the agent that owns it. Leave another live agent's failure for that agent.
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

# shellcheck source=plan-mode.sh
source "$(cd "$(dirname "$0")" && pwd)/plan-mode.sh"

# Cursor also loads .claude/settings.json. Skip the Claude copy when both would run.
if [[ "$agent" == "claude" && -n "${CURSOR_VERSION:-}" ]]; then
  echo '{}'
  exit 0
fi

# Plan mode must not run make check or inject a follow-up into the planning turn.
if plan_mode_active "$payload"; then
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
run_log=$(mktemp "$cache/check.XXXXXX")
trap 'rm -f "$run_log"' EXIT
set +e
if [[ -n "${AI_HOOK_CHECK_CMD:-}" ]]; then
  bash -c "$AI_HOOK_CHECK_CMD" >"$run_log" 2>&1
else
  make check >"$run_log" 2>&1
fi
status=$?
set -e
cp "$run_log" "$log"

if [[ "$status" -eq 0 ]]; then
  printf '%s\n' "$fingerprint" >"$cache/last-green"
  # A green tree has no failing owner. The next edit records its files again.
  rm -rf "$cache/sessions"
  session=$(printf '%s' "$payload" | jq -r '.session_id // .conversation_id // empty')
  if [[ -n "$session" ]]; then
    rm -f "$cache/loops/${agent}-${session}"
  fi
  echo '{}'
  exit 0
fi

session=$(printf '%s' "$payload" | jq -r '.session_id // .conversation_id // empty')
session_list=""
if [[ "$session" =~ ^[A-Za-z0-9._-]+$ ]]; then
  session_list="$cache/sessions/${agent}-${session}"
fi

# shortcut: other agents' edits expire after 60 minutes, so a later failure is judged again instead of staying pinned to a finished agent.
now=$(date +%s)
ttl=$((60 * 60))
this_owned=0
other_owned=0
unknown=0
other_raw=""

if [[ -d "$cache/sessions" ]]; then
  for list in "$cache/sessions"/*; do
    [[ -f "$list" ]] || continue
    if [[ -n "$session_list" && "$list" == "$session_list" ]]; then
      continue
    fi
    mtime=$(stat -c %Y "$list" 2>/dev/null || stat -f %m "$list" 2>/dev/null || true)
    if [[ ! "$mtime" =~ ^[0-9]+$ ]]; then
      continue
    fi
    if (( now - mtime > ttl )); then
      rm -f "$list"
      continue
    fi
    while IFS= read -r path; do
      [[ -n "$path" ]] || continue
      other_raw+="$path"$'\n'
    done < <(awk 'NF && !seen[$0]++' "$list")
  done
fi

# Only file:line diagnostics count. A path mentioned in passing must not steal or hide a failure.
diag_re='/?[A-Za-z0-9_./-]+\.(go|ts|tsx|js|jsx|css|sql|ya?ml|json|mod)(:[0-9]+|\([0-9]+)'
extracted=$(grep -oE "$diag_re" "$run_log" || true)
while IFS= read -r raw; do
  [[ -n "$raw" ]] || continue
  path="${raw%(*}"
  path="${path%:*}"
  path="${path#./}"
  if [[ "$path" == "$root/"* ]]; then
    path="${path#"$root"/}"
  fi
  if [[ -n "$session_list" && -f "$session_list" ]] && grep -Fx -q -- "$path" "$session_list"; then
    this_owned=1
  elif [[ -n "$other_raw" ]] && printf '%s\n' "$other_raw" | grep -Fx -q -- "$path"; then
    other_owned=1
  else
    unknown=1
  fi
done <<< "$extracted"

# Every reported path belongs to another live agent. Do not ask this one to edit them.
if [[ "$this_owned" -eq 0 && "$other_owned" -eq 1 && "$unknown" -eq 0 ]]; then
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

your_files="(none)"
if [[ -n "$session_list" && -f "$session_list" ]]; then
  your_files=$(awk 'NF && !seen[$0]++ { print "- " $0 }' "$session_list")
  [[ -n "$your_files" ]] || your_files="(none)"
fi
others="(none)"
if [[ -n "$other_raw" ]]; then
  others=$(printf '%s\n' "$other_raw" | awk 'NF && !seen[$0]++ { print "- " $0 }')
  [[ -n "$others" ]] || others="(none)"
fi

reason=$(printf '%s\n\n%s' "make check failed.

Fix a violation only when it comes from files you changed. If the broken part is another agent's concern, do not fix it. End the turn and leave that failure for the agent that owns it.

Your files:
${your_files}

Other agents' files:
${others}" "$(tail -n 80 "$run_log")")
if [[ "$agent" == "cursor" ]]; then
  jq -n --arg message "$reason" '{followup_message: $message}'
else
  jq -n --arg message "$reason" '{decision: "block", reason: $message}'
fi
