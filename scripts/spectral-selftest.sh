#!/usr/bin/env bash
# Every custom Spectral rule must fire on the bad fixture.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

out=$(mktemp)
trap 'rm -f "$out"' EXIT
npx --yes @stoplight/spectral-cli@6.15.0 lint api/testdata/bad.yaml --ruleset .spectral.yaml -f json >"$out" || true

rules=$(awk '
  /^  [A-Za-z0-9-]+:/ {
    key = $1
    sub(/:.*/, "", key)
  }
  /function: apiRules/ { print key }
' .spectral.yaml)

if [[ -z "$rules" ]]; then
  echo "spectral self-test: no custom rules found in .spectral.yaml" >&2
  exit 1
fi

missing=0
while IFS= read -r rule; do
  [[ -z "$rule" ]] && continue
  if ! jq -e --arg code "$rule" 'map(.code) | index($code) != null' "$out" >/dev/null; then
    echo "spectral self-test: $rule did not fire on api/testdata/bad.yaml" >&2
    missing=1
  fi
done <<<"$rules"

if ! good=$(npx --yes @stoplight/spectral-cli@6.15.0 lint api/testdata/good.yaml --ruleset .spectral.yaml --fail-severity=warn 2>&1); then
  echo "spectral self-test: api/testdata/good.yaml must lint clean" >&2
  echo "$good" >&2
  missing=1
fi

exit "$missing"
