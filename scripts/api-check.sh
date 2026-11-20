#!/usr/bin/env bash
# Fail when swag comments, OpenAPI specs, or generated frontend API types drift.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
export GOTOOLCHAIN=local

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/fmt" "$tmp/spec"
cp go.mod go.sum "$tmp/fmt/"
cp -R cmd internal "$tmp/fmt/"
(
  cd "$tmp/fmt"
  go tool -modfile="$root/tools/go.mod" swag fmt -g api/main.go -d cmd,internal
)
diff -ru "$root/cmd" "$tmp/fmt/cmd"
diff -ru "$root/internal" "$tmp/fmt/internal"

go tool -modfile=tools/go.mod swag init -q \
  -g api/main.go \
  -d cmd,internal \
  --parseInternal \
  --requiredByDefault \
  --outputTypes yaml \
  -o "$tmp/spec"
diff -u api/swagger.yaml "$tmp/spec/swagger.yaml"

go -C tools run ./cmd/openapi3 -in "$tmp/spec/swagger.yaml" -out "$tmp/spec/openapi.yaml"
diff -u api/openapi.yaml "$tmp/spec/openapi.yaml"

if [[ ! -d webui/node_modules ]]; then
  pnpm --dir webui install --frozen-lockfile
fi
pnpm --dir webui exec openapi-typescript "$tmp/spec/openapi.yaml" -o "$tmp/spec/api.gen.ts" >/dev/null
diff -u webui/src/types/api.gen.ts "$tmp/spec/api.gen.ts"
