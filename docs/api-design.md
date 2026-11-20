# API design

These conventions apply to every public HTTP endpoint. `make check` enforces them locally. Do not add an endpoint that fails that command.

## Decisions

- JSON fields and parameter names are camelCase.
- Successful responses return the resource directly. There is no success envelope.
- List responses are `{ "items": [...], "nextPageToken": "..." }`.
- Pagination is cursor based: `pageSize` and `pageToken`. `pageSize` is an integer whose maximum is at most 100.
- Errors use the shape below. Do not return stack traces, SQL errors, or secrets.
- Nullable JSON values are pointer fields. An optional field has `omitempty` in both its `json` and `validate` tags. A required field has `required` in `validate` and no `omitempty` in `json`. Swag reads the first, the validator reads the second, and `apilint` fails when they disagree.
- `/healthz` and `/readyz` are unversioned operational probes served at the root. Every other path is versioned and starts with `/api/vN`, for example `/api/v1/bookings`.

## Paths and resources

- Version public APIs, for example `/api/v1/bookings`.
- Use nouns, not action names: `GET /api/v1/bookings`, `POST /api/v1/bookings`, `GET /api/v1/bookings/{bookingId}`.
- Use plural resource names: `/users`, `/bookings`.
- Use lowercase paths. Use hyphens for multiword names: `/payment-methods`.
- Do not add a trailing slash.
- Nest a resource only when the relationship is meaningful and the path stays readable: `/users/{userId}/bookings`. Nest at most two resources.
- Do not encode filters or actions as path segments. Use query parameters or the HTTP method.

## HTTP methods and status codes

- GET reads and must not change server state or take a body.
- POST creates a resource or runs a non-idempotent operation.
- PUT replaces a resource. PATCH partially updates it.
- DELETE deletes a resource and must not take a body.
- Use status codes consistently:
  - 200 OK for a successful read or update that returns a body
  - 201 Created when a resource is created
  - 202 Accepted when work is accepted for asynchronous processing
  - 204 No Content for success with no body
  - 400 Bad Request for a malformed or invalid request
  - 401 Unauthorized for missing or invalid authentication
  - 403 Forbidden when the caller is authenticated but not permitted
  - 404 Not Found when the resource does not exist or is not accessible
  - 409 Conflict for a state conflict
  - 422 Unprocessable Content for a syntactically valid but semantically invalid request
  - 412 Precondition Failed when an ETag or version precondition does not match
  - 413 Content Too Large when the request body exceeds the limit
  - 429 Too Many Requests when the caller is rate limited
  - 500 Internal Server Error and 503 Service Unavailable for server failures
- POST declares 200, 201, or 202. PUT and PATCH declare 200 or 204. DELETE declares 204 or 202.
- An operation with a body declares 400. An operation with a path parameter declares 404. Other operations declare at least one 4xx, except `/healthz` and `/readyz`.

## Parameters and JSON

- Identify one resource with a path parameter: `/bookings/{bookingId}`.
- Use query parameters for filtering, sorting, pagination, and optional selection: `?status=confirmed`, `?sortBy=createdAt`, `?pageSize=20&pageToken=...`.
- `sortBy` declares an enum of the fields the server accepts. Reject unsupported values.
- Use a request DTO for each endpoint. Do not bind a request body to a database or domain model.
- Declare required, optional, nullable, and defaulted fields in the DTO tags.
- Validate input at the boundary and return field-specific errors.
- Give every exported request field a `json` or `query` tag and a `validate` tag that includes `required` or `omitempty`.
- Give every exported response field a `json` tag.

```json
{
  "checkInDate": "2026-12-01",
  "guestCount": 2
}
```

## Responses and errors

Keep response shapes consistent. Do not expose database models, internal IDs, or implementation details unless they are part of the contract.

```json
{
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "The request is invalid.",
    "details": [
      {
        "field": "checkInDate",
        "reason": "must be a valid date"
      }
    ]
  }
}
```

Stable codes are `INVALID_ARGUMENT`, `UNAUTHENTICATED`, `PERMISSION_DENIED`, `NOT_FOUND`, `CONFLICT`, `UNPROCESSABLE`, `FAILED_PRECONDITION`, `PAYLOAD_TOO_LARGE`, `RESOURCE_EXHAUSTED`, `UNAVAILABLE`, and `INTERNAL`.

### Logging errors

Every error is logged once, at the HTTP edge, by `NewErrorHandler` in `internal/handler/errors.go`. A 5xx logs at error level and anything else at warn. Each line includes the full cause chain, the status, the code, the method, and the path. `recover` turns panics into errors, so they reach the same handler.

- `model`, `repository`, `service`, and `platform` never log. They return the error, wrapped with `fmt.Errorf("context: %w", err)`. Only `cmd`, `handler`, and `middleware` receive a `*slog.Logger` through dependency injection.
- A handler that maps an error to a client response keeps the original error as the cause: `newAPIError(status, message, err)`. The cause is logged and never sent to the client.
- Do not log and then return the same error. That logs it twice.

Write JSON with `respond`. Return an empty success with `noContent`. Parse JSON with `bindJSON` and query parameters with `bindQuery`. Do not call `c.JSON`, `c.BodyParser`, `c.QueryParser`, or build a `fiber.Map` from a handler.

## Collections and retries

- Define pagination on list endpoints. Prefer a cursor. Set a maximum page size.
- Document filter and sort fields. Reject unsupported values.
- A POST that returns 201 accepts an `Idempotency-Key` header when a retry could create a duplicate, such as a payment or an order. No check enforces this and the template has no idempotency middleware, so implement the header server-side before documenting it.
- Use an ETag or a version field when clients need to detect concurrent updates. No current endpoint requires one, so this is a convention rather than a generated check.

## Real-time streaming (SSE, WebSockets) and webhooks

Standard REST JSON endpoints use `bindJSON`/`bindQuery` and `respond`/`noContent`. Real-time and webhook endpoints have explicit, narrow conventions that keep error handling and spec generation consistent:

- **Server-Sent Events (SSE):**
  - Use noun paths under `/api/vN/...`, such as `GET /api/v1/events` or `POST /api/v1/chats/{chatId}/stream` (`POST` is allowed when streaming a response to a request body such as an LLM prompt).
  - Annotate the handler with `@Produce text/event-stream`, `@Success 200 {string} string "SSE event stream"`, and `@Failure` responses referencing `ErrorResponse`.
  - Validate request DTOs (`bindJSON`/`bindQuery`), auth, and resource existence **before** opening the stream so failures still return standard JSON `ErrorResponse`.
  - Stream events using `streamSSE` and `SSEWriter` in `internal/handler/sse.go`.
  - Spectral exempts `text/event-stream` operations from collection pagination rules and enforces `sse-stream-response` (`200` required).
- **WebSockets:**
  - Mount under `/api/vN/...` (for example `/api/v1/rooms/{roomId}/socket`), mark the OpenAPI operation with `x-realtime: websocket` if declaring `101` Switching Protocols, and validate auth/params before upgrading.
- **Webhooks:**
  - Mount under `POST /api/v1/webhooks/{provider}` returning `202` (or `200`/`204` where required by the provider) and `ErrorResponse` for `400`/`404`.
  - Read raw bytes with `c.Body()` for HMAC signature verification before decoding payload structs.

## Add an endpoint

1. Add the domain type in `internal/model` without a `json` tag.
2. Add the repository interface, the SQL migration, and the GORM implementation.
3. Add the use case in `internal/service` and a unit test with a fake repository.
4. Add a handler request DTO and a response DTO in `internal/handler`.
5. Annotate the handler with `@Summary`, `@Description`, `@Tags`, `@ID`, `@Param`, `@Success`, `@Failure` referencing `ErrorResponse`, and `@Router`.
6. Call `bindJSON` or `bindQuery`, then `respond` or `noContent`.
7. Run `make generate` and commit `api/swagger.yaml`.
8. Add a row to the contract test in `internal/handler/contract_test.go`.
9. Run `make check`.

## What enforces each rule

| Rule | Check |
| --- | --- |
| Prefixed (`/api/vN`), kebab-case, plural, shallow, noun paths | Spectral `path-versioned` (`/api/vN/` prefix), `path-kebab-case`, `path-collection-plural`, `path-max-depth`, `path-no-verbs` |
| camelCase path, query, and JSON names | Spectral `path-param-camel-case`, `query-param-camel-case`, `property-camel-case`; golangci-lint `tagliatelle` |
| Allowed methods and status codes | Spectral `get-no-body`, `delete-no-body`, `post-success-status`, `put-patch-success-status`, `delete-success-status`, `status-code-allowed`, `operation-has-4xx`, `path-param-has-404`, `body-has-400` |
| Shared error schema | Spectral `error-response-ref`; `ErrorResponse` in `internal/handler/errors.go` |
| Cursor pagination and `sortBy` enum | Spectral `collection-pagination-params`, `collection-pagination-response`, `sort-by-enum` |
| Handler DTOs, not domain or database types | `apilint` `dto-only-responses`, `dto-only-requests`, `model-no-wire-tags` |
| Explicit required and optional fields | `apilint` `dto-explicit-fields`; swag `--requiredByDefault` |
| Swag annotations on handlers | `apilint` `handler-annotated` |
| No `fiber.Map`, direct `JSON`, `BodyParser`, or `QueryParser` | golangci-lint `forbidigo` |
| JSON tags on encoded structs | golangci-lint `musttag` |
| Spec matches the annotations | `make api-check` |
| Routes, status codes, and bodies match the spec | `internal/handler/contract_test.go` |
| Go complexity and layer boundaries | golangci-lint, including `depguard` |
| Logging only at the edge | golangci-lint `depguard` `logging-only-at-edge`; `forbidigo` bans `fmt.Print*` and `println` |
| Error branches keep the cause | `apilint` `error-cause` in every `internal` package |
| Every error and panic is logged | `TestErrorHandlerLogsEveryError`, `TestPanicIsLoggedAsInternalError` |

`api/testdata/bad.yaml` breaks every custom Spectral rule on purpose. `make api-lint-selftest` fails if a rule stops firing.
