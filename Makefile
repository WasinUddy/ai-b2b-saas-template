export GOTOOLCHAIN := local

SPECTRAL := npx --yes @stoplight/spectral-cli@6.15.0
SWAG := go tool -modfile=tools/go.mod swag
GOLANGCI := go tool -modfile=tools/go.mod golangci-lint
COVERAGE := go tool -modfile=tools/go.mod go-test-coverage

.PHONY: generate api-check api-lint api-lint-selftest lint apilint test hooks-selftest check hooks webui-check dev migrate

generate:
	$(SWAG) init -q -g api/main.go -d cmd,internal --parseInternal --requiredByDefault --outputTypes yaml -o api
	go -C tools run ./cmd/openapi3 -in "$(CURDIR)/api/swagger.yaml" -out "$(CURDIR)/api/openapi.yaml"
	@if [ ! -d webui/node_modules ]; then pnpm --dir webui install --frozen-lockfile; fi
	pnpm --dir webui exec openapi-typescript ../api/openapi.yaml -o src/types/api.gen.ts

api-check:
	bash scripts/api-check.sh

api-lint:
	$(SPECTRAL) lint api/swagger.yaml --ruleset .spectral.yaml --fail-severity=warn

api-lint-selftest:
	bash scripts/spectral-selftest.sh

lint:
	$(GOLANGCI) run ./...

apilint:
	mkdir -p bin
	go build -C tools -o "$(CURDIR)/bin/apilint" ./cmd/apilint
	go vet -vettool="$(CURDIR)/bin/apilint" ./...

test:
	go test -coverprofile=coverage.out ./...
	$(COVERAGE) --config=.testcoverage.yml
	go test -C tools ./...

hooks-selftest:
	bash scripts/ai-hooks/selftest.sh

webui-check:
	@if [ ! -d webui/node_modules ]; then pnpm --dir webui install --frozen-lockfile; fi
	pnpm --dir webui check
	pnpm --dir webui build

check: api-check api-lint api-lint-selftest lint apilint test webui-check hooks-selftest

hooks:
	git config core.hooksPath .githooks

migrate:
	go run ./cmd/migrate

dev:
	bash scripts/dev.sh
