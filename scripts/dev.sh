#!/usr/bin/env bash
# Run the Go API and Vite frontend dev server concurrently with clean shutdown.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
export GOTOOLCHAIN=local

if [[ ! -d webui/node_modules ]]; then
  echo "[dev] installing webui dependencies..."
  pnpm --dir webui install --frozen-lockfile
fi

pids=()
cleanup() {
  local code=$?
  trap - INT TERM EXIT
  for pid in "${pids[@]:-}"; do
    if kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
    fi
  done
  wait 2>/dev/null || true
  exit "$code"
}
trap cleanup INT TERM EXIT

api_addr="${HTTP_ADDR:-:8080}"
echo "[dev] starting Go API on ${api_addr} (WEBUI_DIR disabled; Vite serves frontend)"
if command -v air >/dev/null 2>&1; then
  WEBUI_DIR="" air &
else
  WEBUI_DIR="" go run ./cmd/api &
fi
pids+=("$!")

echo "[dev] starting Vite dev server (proxying /api -> http://localhost${api_addr})"
pnpm --dir webui dev &
pids+=("$!")

wait
