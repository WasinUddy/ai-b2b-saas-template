# AI B2B SaaS Template — Agent Guide

Fullstack single-binary template: **Go (Fiber v2 + GORM + SQLite)** backend with an embedded **Vite + React 19 + TypeScript + Tailwind v4** frontend (`webui/`).

Every architectural, API, OpenAPI, and frontend rule is mechanically enforced. **Run `make check` before finishing.** Do not weaken linters, tests, coverage thresholds, or Spectral rules to make a change pass.

## Quick Commands

| Command | Purpose |
| --- | --- |
| `make dev` | Run Go API (`:8080`) + Vite dev server (`:5173`, proxying `/api`) together |
| `make generate` | Regenerate `api/swagger.yaml`, `api/openapi.yaml`, and `webui/src/types/api.gen.ts` |
| `make migrate` | Apply embedded SQLite migrations in `migrations/*.sql` |
| `make check` | Run all checks: OpenAPI/TS drift, Spectral + self-test, golangci-lint, `apilint`, Go tests + 85% service coverage, `webui` typecheck/lint/format/test/build, and hook self-tests |

## Backend Architecture (`cmd/`, `internal/`, `migrations/`)

Layer direction is strictly enforced by `depguard` in `.golangci.yml`:

```text
cmd/api  -->  handler / middleware  -->  service  -->  repository  <--  platform (gormrepo, database, config)
                                            |               |                     |
                                            +---------------+---------------------+-->  model
```

- `internal/model`: pure domain structs and sentinel errors (`ErrX`). **No `json` or GORM tags** (`apilint` `model-no-wire-tags`).
- `internal/repository`: data-access interfaces only.
- `internal/service`: business logic depending only on `repository` and `model`. No Fiber, GORM, SQL, or `log`. Must keep **>= 85% statement coverage** (unit test with fake repositories).
- `internal/platform/gormrepo`: GORM implementations using private DB record structs; include `var _ repository.X = (*X)(nil)`.
- `internal/handler`: thin Fiber adapters, request/response DTOs, swag annotations, and route wiring.
  - Bind inputs only via `bindJSON` / `bindQuery`; write JSON only via `respond` / `noContent` (`forbidigo` bans `c.JSON`, `c.BodyParser`, `c.QueryParser`, `fiber.Map`, `fmt.Print*`).
  - Request DTO fields must have explicit `json`/`query` and `validate` (`required` or `omitempty`) tags matching in `json` and `validate` (`apilint` `dto-explicit-fields`).
  - Every exported handler method must have `@Router`, `@Success`, and `@Failure ... ErrorResponse` annotations.
- `internal/middleware`: Fiber middleware (`Logger`, `RequireAuth`).
- **Errors & logging**: Never log inside `model`, `repository`, `service`, or `platform`. Return wrapped errors (`fmt.Errorf("context: %w", err)`). Never drop `err` inside `if err != nil` (`apilint` `error-cause`). `handler.NewErrorHandler` logs every failure once at the HTTP edge.
- **Complexity caps**: max 60 lines / 40 statements per function (`funlen`), cyclomatic & cognitive complexity <= 15, nesting <= 4, max 5 args, max 3 return values. Any `//nolint:<linter> // reason` must name the linter and explain why.

## API Contract, Streaming (SSE), and Webhooks

Full HTTP rules are in [docs/api-design.md](docs/api-design.md) and enforced by `.spectral.yaml`:

- Public routes live under `/api/vN/...` with lowercase kebab-case plural nouns (`/api/v1/bookings/{bookingId}`). Only `/healthz` and `/readyz` stay at root.
- JSON fields and params are camelCase; errors always use `ErrorResponse` (`{"error":{"code","message","details"}}`).
- List endpoints use cursor pagination (`pageSize <= 100`, `pageToken`, `{ items, nextPageToken }`).
- **SSE / Real-time streaming**:
  - Annotate with `@Produce text/event-stream`, `@Success 200 {string} string "SSE event stream"`, and `@Failure` with `ErrorResponse`.
  - Both `GET` (e.g. `/api/v1/events`) and `POST` (e.g. `/api/v1/chats/{chatId}/stream`) are supported.
  - Validate auth and DTOs *before* calling `streamSSE` (`internal/handler/sse.go`) so failures return JSON `ErrorResponse`.
  - WebSocket upgrade routes may declare `101` when marked with `x-realtime: websocket`.
- **Webhooks**: Mount under `POST /api/v1/webhooks/{provider}` returning `202` (or `200`/`204`), read raw bytes with `c.Body()` for signature checks.

## OpenAPI-First Frontend (`webui/`)

- **All local backend calls MUST go through `@/services/apiClient`** (`apiClient`, `unwrapApiResponse`, `streamApiEvents`, `setAuthToken`), typed from the auto-generated `@/types/api.gen.ts`.
- ESLint (`webui/eslint.config.js`) forbids calling `fetch` or `EventSource` directly outside `webui/src/services/apiClient.ts` and test files.
- Never hand-edit `api/swagger.yaml`, `api/openapi.yaml`, or `webui/src/types/api.gen.ts`. Always run `make generate` (also triggered automatically by the post-edit hook after backend Go edits).
- **Frontend layer direction** (enforced by ESLint):
  `types < services < hooks < components < features < pages < app`
- Import features only through their barrel (`@/features/<name>`). No `any`, no circular imports.

## Database Migrations & Docker

- Add numbered SQLite SQL files to `migrations/` (e.g. `migrations/0002_create_projects.sql`). `gorm.AutoMigrate` is never used.
- `migrations/embed.go` embeds `*.sql` into the Go binaries.
- `cmd/api` automatically applies pending migrations on startup (`DB_AUTO_MIGRATE=true` by default), so Docker users simply mount `-v mydata:/data` and run.
- `Dockerfile` verifies all migrations in its `migrate-verify` stage during build and exposes a standalone `migrate` target (`cmd/migrate`) for init-container deployments (`DB_AUTO_MIGRATE=false`).

## Optional Auth Middleware

- `internal/middleware/auth.go` provides a generic `RequireAuth(verifier, enabled)` middleware and `AuthClaims(c)` helper backed by `model.AuthClaims` and `model.ErrInvalidToken`.
- It is **disabled by default** (`AUTH_ENABLED=false`), passing requests through unchanged until a project needs authentication.
- No opinionated user/password schema or auth endpoints are pre-installed. When a project needs auth, implement `middleware.TokenVerifier` (user/password JWT, sessions, OAuth/OIDC, or API keys), wire it in `cmd/api/main.go`, mount protected route groups with `deps.AuthGuard` in `internal/handler/router.go`, and set `AUTH_ENABLED=true`.

## Rebrand after “Use this template”

New repos from [ai-b2b-saas-template](https://github.com/WasinUddy/ai-b2b-saas-template)
should run the project skill **rebrand-from-template**
(`.cursor/skills/`, `.claude/skills/`, or `.agents/skills/rebrand-from-template/SKILL.md`) or
`scripts/rebrand-from-template.sh` before feature work. Finish with `make generate`
and `make check`.

## Adding a Fullstack Feature (Checklist)

1. Add domain types in `internal/model` (no `json` tags) and repository interface in `internal/repository`.
2. Add a numbered SQL migration in `migrations/` and GORM implementation in `internal/platform/gormrepo`.
3. Add the use case in `internal/service` with fake-repo unit tests (>= 85% coverage).
4. Add request/response DTOs, swag annotations, and Fiber handlers in `internal/handler`; wire routes in `internal/handler/router.go` and dependencies in `cmd/api/main.go`.
5. Run `make generate` to update `api/swagger.yaml`, `api/openapi.yaml`, and `webui/src/types/api.gen.ts`.
6. Add contract test cases in `internal/handler/contract_test.go`.
7. Build the frontend UI in `webui/src/` calling `apiClient` / `streamApiEvents` from `@/services/apiClient`, respecting layer boundaries, and add Vitest tests.
8. Run `make check` and fix violations that come from your changes before stopping. If a failure belongs to another agent editing the repo at the same time, leave it for that agent.

## Agent Hooks

Cursor (`.cursor/hooks.json`), Claude Code (`.claude/settings.json`), and Codex (`.codex/hooks.json`) all run:
- **Post-edit hook** (`scripts/ai-hooks/post-edit.sh`): formats edited Go (`gofmt`, `swag fmt`) and `webui/` files (Prettier), and regenerates OpenAPI specs + `webui/src/types/api.gen.ts` after backend Go edits.
- **Stop hook** (`scripts/ai-hooks/stop-check.sh`): fingerprints Go, OpenAPI, migrations, Dockerfile, and `webui/` files, runs `make check`, and sends a failure back to the agent that owns it (capped at 3 retries). A failure in another live agent's files is not handed back. The follow-up tells the agent to fix its own files and leave the rest.
