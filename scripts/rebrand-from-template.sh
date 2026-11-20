#!/usr/bin/env bash
# Rebrand a repo created from github.com/WasinUddy/ai-b2b-saas-template.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

DEFAULT_GO_MODULE='github.com/wasinuddy/ai-b2b-saas-template'
DEFAULT_PRODUCT_NAME='AI B2B SaaS'
DEFAULT_DOCS_TITLE='AI B2B SaaS Template'
DEFAULT_API_TITLE='AI B2B SaaS API'
DEFAULT_API_DESCRIPTION='HTTP API for the AI B2B SaaS template.'

GO_MODULE=''
PRODUCT_NAME=''
DOCS_TITLE=''
API_TITLE=''
API_DESCRIPTION=''
DRY_RUN=false

usage() {
  cat <<'EOF'
Usage: scripts/rebrand-from-template.sh --go-module MODULE --product-name NAME [options]

Required:
  --go-module MODULE     New Go module path (e.g. github.com/acme/billing-api)
  --product-name NAME    Short product name shown in the web UI and browser title

Optional:
  --docs-title TITLE     README / AGENTS.md title (default: product name)
  --api-title TITLE      OpenAPI title (default: "<product name> API")
  --api-description TEXT Swagger @description (default: "HTTP API for <product name>.")
  --dry-run              Print planned edits without writing files

After a real run, execute: make generate && make check
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --go-module)
      GO_MODULE=$2
      shift 2
      ;;
    --product-name)
      PRODUCT_NAME=$2
      shift 2
      ;;
    --docs-title)
      DOCS_TITLE=$2
      shift 2
      ;;
    --api-title)
      API_TITLE=$2
      shift 2
      ;;
    --api-description)
      API_DESCRIPTION=$2
      shift 2
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$GO_MODULE" || -z "$PRODUCT_NAME" ]]; then
  echo "error: --go-module and --product-name are required" >&2
  usage >&2
  exit 1
fi

if [[ -z "$DOCS_TITLE" ]]; then
  DOCS_TITLE="$PRODUCT_NAME"
fi
if [[ -z "$API_TITLE" ]]; then
  API_TITLE="${PRODUCT_NAME} API"
fi
if [[ -z "$API_DESCRIPTION" ]]; then
  API_DESCRIPTION="HTTP API for ${PRODUCT_NAME}."
fi

if [[ "$GO_MODULE" == "$DEFAULT_GO_MODULE" && "$PRODUCT_NAME" == "$DEFAULT_PRODUCT_NAME" ]]; then
  echo "error: nothing to change; pick a new module path and product name" >&2
  exit 1
fi

replace_in_file() {
  local file=$1
  if [[ ! -f "$file" ]]; then
    return 0
  fi
  if $DRY_RUN; then
    if grep -qE "$DEFAULT_GO_MODULE|${DEFAULT_PRODUCT_NAME}|${DEFAULT_DOCS_TITLE}|${DEFAULT_API_TITLE}|${DEFAULT_API_DESCRIPTION}" "$file" 2>/dev/null; then
      echo "would update: $file"
    fi
    return 0
  fi
  perl -i -pe "
    s/\Q${DEFAULT_GO_MODULE}\E/${GO_MODULE}/g;
    s/\Q${DEFAULT_DOCS_TITLE}\E/${DOCS_TITLE}/g;
    s/\Q${DEFAULT_API_TITLE}\E/${API_TITLE}/g;
    s/\Q${DEFAULT_API_DESCRIPTION}\E/${API_DESCRIPTION}/g;
    s/\Q${DEFAULT_PRODUCT_NAME}\E/${PRODUCT_NAME}/g;
  " "$file"
}

found=0
while IFS= read -r file; do
  [[ -z "$file" ]] && continue
  found=1
  replace_in_file "$file"
done < <(
  rg -l --hidden \
    -g '!.git' \
    -g '!node_modules' \
    -g '!webui/dist' \
    -g '!bin' \
    -g '!.cache' \
    -g '!coverage.out' \
    -g '!*.db' \
    -g '!pnpm-lock.yaml' \
    -g '!scripts/rebrand-from-template.sh' \
    -g '!.cursor/skills/rebrand-from-template/**' \
    -g '!.claude/skills/**' \
    -g '!.agents/skills/**' \
    -g '!api/swagger.yaml' \
    -g '!api/openapi.yaml' \
    -g '!webui/src/types/api.gen.ts' \
    "${DEFAULT_GO_MODULE}|${DEFAULT_PRODUCT_NAME}|${DEFAULT_DOCS_TITLE}|${DEFAULT_API_TITLE}" \
    "$ROOT" 2>/dev/null || true
)

if [[ "$found" -eq 0 ]]; then
  echo "No template branding markers found; already rebranded?" >&2
  exit 0
fi

if $DRY_RUN; then
  echo "Dry run complete. Re-run without --dry-run to apply, then: make generate && make check"
  exit 0
fi

echo "Rebrand applied."
echo "Next: make generate && make check"
echo "Then search for leftovers: rg 'wasinuddy|ai-b2b-saas-template|AI B2B SaaS'"