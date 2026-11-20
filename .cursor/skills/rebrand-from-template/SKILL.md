---
name: rebrand-from-template
description: >-
  Rebrands a project created from the AI B2B SaaS GitHub template: Go module
  path, OpenAPI/swag metadata, web UI title, README, and AGENTS.md. Use when the
  user creates a repo from this template, asks to rebrand, rename the module,
  or personalize the starter after "Use this template".
metadata:
  short-description: Rebrand a new repo from the AI B2B SaaS template
---

# Rebrand from template

Loaded from `.cursor/skills/`, `.claude/skills/`, or `.agents/skills/` (same
content for Cursor, Claude Code, and Codex).

Run this workflow once after creating a new repository from
[ai-b2b-saas-template](https://github.com/WasinUddy/ai-b2b-saas-template).

## Gather inputs

1. **Go module path** — Usually `github.com/<org>/<repo>`. Prefer deriving it from
   `git remote get-url origin` (HTTPS or SSH) rather than guessing.
2. **Product name** — Short name for UI (`<title>`, home page `<h1>`), e.g.
   `Acme Billing`.
3. **Optional** — Docs title (README H1), API title, API description for Swagger.

Confirm values with the user when anything is ambiguous.

## Apply mechanical rebrand

From the repository root:

```bash
chmod +x scripts/rebrand-from-template.sh
./scripts/rebrand-from-template.sh \
  --go-module 'github.com/<org>/<repo>' \
  --product-name '<Product Name>'
```

Optional flags: `--docs-title`, `--api-title`, `--api-description`, `--dry-run`.

The script updates:

| Area | Template default | What changes |
| --- | --- | --- |
| Go | `github.com/wasinuddy/ai-b2b-saas-template` | All imports, `go.mod`, `tools/go.mod`, depguard in `.golangci.yml` |
| Swagger | `cmd/api/main.go` `@title` / `@description` | Product-specific API metadata |
| Web UI | `webui/index.html`, `HomePage.tsx`, `routes.test.tsx` | Browser title and heading (tests must match) |
| Docs | `README.md`, `AGENTS.md` | Titles and module path line |

It skips generated `api/swagger.yaml`, `api/openapi.yaml`, and `webui/src/types/api.gen.ts` — those come from `make generate`.

## Regenerate and verify

```bash
make generate
make check
```

Search for leftovers (should be empty except this skill, the rebrand script, and historical mentions you chose to keep):

```bash
rg -n 'wasinuddy|ai-b2b-saas-template|AI B2B SaaS' \
  --glob '!.cursor/skills/**' \
  --glob '!.claude/skills/**' \
  --glob '!.agents/skills/**' \
  --glob '!scripts/rebrand-from-template.sh'
```

## Optional polish (ask the user)

- Replace README boilerplate with product-specific positioning (keep architecture sections if useful).
- Set `webui/package.json` `"name"` to a npm-safe package name (kebab-case).
- Add logo, colors in `webui/src/index.css` `@theme`, and nav label in `AppLayout.tsx`.
- Update GitHub repo description, topics, and visibility.

## Do not

- Weaken or skip `make check` to finish faster.
- Hand-edit `api/swagger.yaml`, `api/openapi.yaml`, or `webui/src/types/api.gen.ts`.
- Change layer boundaries or template architecture during rebrand.

## Reference

See [references/checklist.md](references/checklist.md) for the full file list and manual fallbacks if the script cannot run.