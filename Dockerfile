# syntax=docker/dockerfile:1

# 1. Build the frontend.
FROM node:24-alpine AS webui
WORKDIR /webui
RUN corepack enable
COPY webui/package.json webui/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile
COPY webui/ ./
RUN pnpm build

# 2. Build the API and standalone migrator binaries (migrations are embedded via embed.FS).
FROM golang:1.26.2-alpine AS api
ENV GOTOOLCHAIN=local CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY migrations/ migrations/
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate \
    && mkdir /out/data

# 3. Migration verification stage: run embedded SQL migrations against a scratch SQLite DB.
# Fails the image build if any migration SQL is invalid, without touching the final /data volume.
FROM alpine:3.21 AS migrate-verify
COPY --from=api /out/migrate /out/migrate
RUN DB_DSN="file:/tmp/verify.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)" \
    /out/migrate \
    && rm -f /tmp/verify.db /tmp/verify.db-wal /tmp/verify.db-shm

# 4. Standalone migration target (docker build --target migrate) for init-container workflows.
FROM gcr.io/distroless/static-debian12:nonroot AS migrate
WORKDIR /app
COPY --from=migrate-verify /out/migrate /app/migrate
COPY --from=api --chown=nonroot:nonroot /out/data /data
ENV DB_DSN="file:/data/app.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
VOLUME /data
ENTRYPOINT ["/app/migrate"]

# 5. Runtime: the API binary, built frontend, and empty /data mount point, nothing else.
# Mounting /data automatically initializes and migrates /data/app.db on startup (DB_AUTO_MIGRATE=true).
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=api /out/api /app/api
COPY --from=migrate-verify /out/migrate /app/migrate
COPY --from=webui /webui/dist /app/webui/dist
# SQLite needs a directory the nonroot user can write. Mount a volume at /data to keep it.
COPY --from=api --chown=nonroot:nonroot /out/data /data
ENV HTTP_ADDR=":8080" WEBUI_DIR="/app/webui/dist" DB_AUTO_MIGRATE="true" \
    DB_DSN="file:/data/app.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/app/api"]
