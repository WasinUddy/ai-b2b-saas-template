# Rebrand checklist

Template defaults to replace:

| Token | Example |
| --- | --- |
| Go module | `github.com/wasinuddy/ai-b2b-saas-template` |
| Product name | `AI B2B SaaS` |
| Docs title | `AI B2B SaaS Template` |
| API title | `AI B2B SaaS API` |
| API description | `HTTP API for the AI B2B SaaS template.` |

## Files touched by `scripts/rebrand-from-template.sh`

Any tracked file under the repo that still contains the markers above (except
`pnpm-lock.yaml`, build artifacts, SQLite files, and agent skill directories
`.cursor/skills/`, `.claude/skills/`, `.agents/skills/`).

Common locations:

- `go.mod`, `tools/go.mod`, `tools/cmd/apilint/main.go`, `tools/apilint/apilint_test.go`
- All `*.go` under `cmd/`, `internal/`, `tools/`
- `.golangci.yml` (depguard package prefixes)
- `cmd/api/main.go` (swag comments — then `make generate`)
- `README.md`, `AGENTS.md`
- `webui/index.html`, `webui/src/pages/HomePage.tsx`, `webui/src/app/routes.test.tsx`

## Generated (run `make generate`, do not hand-edit)

- `api/swagger.yaml`
- `api/openapi.yaml`
- `webui/src/types/api.gen.ts`

## Usually unchanged

- `configs/app.env.example` (no product name)
- `Dockerfile`, `.github/workflows/ci.yml` (no template module path)
- `migrations/` (no branding)
- `webui/package.json` `"name": "webui"` unless the user wants an npm package name

## Manual `go mod` fallback

If imports are already partially edited:

```bash
# example
go mod edit -module 'github.com/acme/my-app'
find . -name '*.go' -not -path './webui/*' -print0 | xargs -0 perl -pi -e 's#github.com/wasinuddy/ai-b2b-saas-template#github.com/acme/my-app#g'
(cd tools && go mod edit -module 'github.com/acme/my-app/tools')
```

Then `make generate && make check`.