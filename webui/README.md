# webui

Vite, React, TypeScript, React Router, and Tailwind CSS, managed with pnpm. Requires Node 24 and pnpm 11 (pinned in `package.json` as `packageManager`).

## Commands

Run these from `webui/`.

```bash
pnpm install          # install dependencies
pnpm dev              # start the dev server
pnpm lint             # ESLint, including import-boundary and OpenAPI client rules
pnpm typecheck        # tsc -b
pnpm format:check     # Prettier check (use `pnpm format` to fix)
pnpm test             # Vitest, single run (`pnpm test:watch` to watch)
pnpm check            # typecheck, lint, format:check, test
pnpm build            # tsc -b, then vite build into dist/
```

From the repository root, `make dev` runs the Go API and Vite dev server together, and `make check` runs all backend checks plus `make webui-check`.

## Calling the Go API (OpenAPI-typed)

All local backend calls **must** go through `@/services/apiClient`, which wraps `openapi-fetch` with the generated types from `@/types/api.gen.ts` (produced from `api/openapi.yaml` by `make generate`).

```ts
import { apiClient, setAuthToken, unwrapApiResponse } from '@/services/apiClient'

// Optionally attach a Bearer token when auth middleware is enabled:
setAuthToken(token)

// Paths, request payloads, and responses are strictly typed from api/openapi.yaml:
const health = unwrapApiResponse(await apiClient.GET('/healthz'))
```

For Server-Sent Events (SSE) endpoints, use `streamApiEvents` from `@/services/apiClient`. ESLint blocks direct `fetch()` and `EventSource` calls outside `src/services/apiClient.ts` and test files.

Never edit `src/types/api.gen.ts` by hand; run `make generate` from the repository root after editing Go handlers or DTOs. `make api-check` fails if generated types drift.

### Development proxy

`vite.config.ts` proxies `/api` to the Go API without rewriting the path. The default target is `http://localhost:8080`, which is the backend's default `HTTP_ADDR` (`:8080`, from `internal/platform/config`). If the API listens elsewhere, set `API_PROXY_TARGET` (shell variable or `webui/.env.local`):

```bash
API_PROXY_TARGET=http://localhost:9000 pnpm dev
```

`/healthz` and `/readyz` are not proxied. Frontend routes must not start with `/api`.

### Production

The Go API serves `dist/` itself from `WEBUI_DIR` (default `webui/dist` relative to where the API runs). Build with `pnpm build`, then start the API from the repository root. Unmatched `/api/*` paths return the API's JSON 404 instead of `index.html`. The root `Dockerfile` does both steps and ships only the API binary, migrator binary, and `dist/`. See the root README, "Frontend".

## Structure

```text
src/
  index.css   Tailwind CSS entry point and theme configuration
  app/        app setup: router, layout, providers
  pages/      route-level components
  services/   OpenAPI-typed API client and external services
  types/      generated OpenAPI types (api.gen.ts) and shared domain types
  test/       Vitest setup
```

Add `components/`, `features/`, and `hooks/` when there is something to put in them. Create them only when needed. Feature code is imported through its `index.ts` only (`@/features/<name>`).

## Styling

Tailwind CSS v4 is wired through `@tailwindcss/vite` in `vite.config.ts` and loaded from `src/index.css` (`@import 'tailwindcss';`). Add custom design tokens with `@theme` in `src/index.css`. Prettier sorts Tailwind utility classes via `prettier-plugin-tailwindcss`.

## Layering

Lower layers must not import higher ones. ESLint enforces this (`import-x/no-restricted-paths`), along with `import-x/no-cycle` and the no-deep-feature-import rule. Config is in `eslint.config.js`.

```text
types  <  services  <  hooks  <  components  <  features  <  pages  <  app
```

Use the `@/` alias for imports from `src/`.
