#!/usr/bin/env bash
# Format edited Go and frontend files, and regenerate OpenAPI specs + TS types after backend Go edits.
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

payload=$(cat || true)
if ! command -v jq >/dev/null 2>&1; then
  echo '{}'
  exit 0
fi

root="${AI_HOOK_ROOT:-}"
if [[ -z "$root" ]]; then
  root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
fi

files=()
case "$agent" in
  cursor)
    file=$(printf '%s' "$payload" | jq -r '.file_path // empty')
    [[ -n "$file" ]] && files+=("$file")
    ;;
  claude)
    file=$(printf '%s' "$payload" | jq -r '.tool_input.file_path // empty')
    [[ -n "$file" ]] && files+=("$file")
    ;;
  codex)
    while IFS= read -r file; do
      [[ -n "$file" ]] && files+=("$file")
    done < <(printf '%s' "$payload" | jq -r '.tool_input.command // empty' | sed -n -E 's/^\*\*\* (Add|Update) File:[[:space:]]*//p')
    ;;
  *)
    echo "post-edit: --agent must be cursor, claude, or codex" >&2
    exit 1
    ;;
esac

go_files=()
backend_go_edited=0
webui_files=()

if [[ ${#files[@]} -gt 0 ]]; then
  for file in "${files[@]}"; do
    resolved="$file"
    if [[ "$resolved" != /* ]]; then
      resolved="$root/$resolved"
    fi
    [[ -f "$resolved" ]] || continue

    case "$resolved" in
      *.go)
        go_files+=("$resolved")
        case "$resolved" in
          "$root"/cmd/*.go | "$root"/internal/*.go)
            backend_go_edited=1
            ;;
        esac
        ;;
      "$root"/webui/src/types/api.gen.ts | "$root"/webui/pnpm-lock.yaml | "$root"/webui/node_modules/* | "$root"/webui/dist/*)
        ;;
      "$root"/webui/*.ts | "$root"/webui/*.tsx | "$root"/webui/*.js | "$root"/webui/*.jsx | "$root"/webui/*.css | "$root"/webui/*.html | "$root"/webui/*.json)
        webui_files+=("$resolved")
        ;;
    esac
  done
fi

if [[ ${#go_files[@]} -gt 0 ]] && command -v gofmt >/dev/null 2>&1; then
  gofmt -w "${go_files[@]}"
  if [[ -z "${AI_HOOK_SKIP_SWAG:-}" ]] && command -v go >/dev/null 2>&1; then
    (
      cd "$root"
      GOTOOLCHAIN=local go tool -modfile=tools/go.mod swag fmt -g api/main.go -d cmd,internal
    ) >/dev/null 2>&1 || true
  fi
  if [[ "$backend_go_edited" -eq 1 && -z "${AI_HOOK_SKIP_GENERATE:-}" ]]; then
    if [[ -n "${AI_HOOK_GENERATE_CMD:-}" ]]; then
      (cd "$root" && bash -c "$AI_HOOK_GENERATE_CMD") >/dev/null 2>&1 || true
    elif [[ -z "${AI_HOOK_SKIP_SWAG:-}" ]] && command -v go >/dev/null 2>&1 && command -v pnpm >/dev/null 2>&1; then
      (cd "$root" && make generate) >/dev/null 2>&1 || true
    fi
  fi
fi

if [[ ${#webui_files[@]} -gt 0 && -z "${AI_HOOK_SKIP_PRETTIER:-}" && -x "$root/webui/node_modules/.bin/prettier" ]]; then
  (
    cd "$root/webui"
    ./node_modules/.bin/prettier --write --ignore-unknown "${webui_files[@]}"
  ) >/dev/null 2>&1 || true
fi

echo '{}'
