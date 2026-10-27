# AGENTS.md — api

Rules for the Go backend: the HTTP API, the worker and the migrations. The repository-wide rules
in the root [`AGENTS.md`](../AGENTS.md) still apply.

Write idiomatic, simple Go that is easy to test. Follow the existing code; when a rule here and a
consolidated pattern in the code disagree, follow the code and fix this file.

## Stack

| Concern | Library |
| --- | --- |
| Language | Go (the version in `go.mod`) |
| HTTP | `go-chi/chi/v5`, `go-chi/cors` |
| Database | PostgreSQL 17 with pgvector, `database/sql` + `jackc/pgx/v5/stdlib`, `pgvector-go` |
| Data access | Hand-written SQL. No ORM or query builder |
| Migrations | `golang-migrate/migrate/v4` |
| IDs | `google/uuid` |
| Validation | `go-playground/validator/v10` |
| Auth and crypto | `golang-jwt/jwt/v5`, `golang.org/x/crypto` (bcrypt) |
| Feeds and articles | `mmcdole/gofeed`, `go-readability` |
| API docs | `swaggo/swag` + `http-swagger/v2` |
| Logs | `log/slog` (JSON in production) |
| Tests | `testing` + Testify, `go.uber.org/mock`, `testcontainers-go` (Postgres) |
| Lint | `golangci-lint` (config in `.golangci.yml`) |

Forbidden without an explicit decision recorded in the docs: other HTTP frameworks (Gin, Echo,
Fiber), ORMs, `AutoMigrate`, third-party loggers, DI containers (wire, fx, dig), and any new
dependency the standard library or an existing one already covers.

## Layout

```text
cmd/
├── api/        # HTTP server
├── worker/     # scheduler and job runner
├── migrate/    # up | down | version
└── reindex/    # enqueues a full embedding reindex
internal/
├── bootstrap/  # container.go (DI), routes.go, worker.go (jobs and scheduled tasks)
├── config/     # environment loading and validation
├── database/   # connection, migrations, TxManager
├── apperrors/  # semantic errors with stable codes
├── auth/       # JWT, password hashing, tokens, permissions
├── dtos/       # request and response contracts
├── handlers/   # HTTP layer (decode, respond, pagination helpers)
├── middlewares/
├── models/
├── services/   # business rules; ports.go holds the interfaces services depend on
├── repositories/
└── workers/    # job queue and scheduler
pkg/            # reusable infrastructure without domain rules
├── ai/         # provider-agnostic AI layer (OpenAI-compatible, OpenAI, Anthropic, Gemini)
├── connectors/ # source connectors (one file per source, fixtures in testdata/)
├── extract/    # article text extraction
├── mail/       # Mailer: SMTP adapter and a log mailer for development
├── safehttp/   # SSRF-safe HTTP client
├── secretbox/  # AES-GCM encryption at rest
└── ratelimit/
migrations/     # 0000NN_<name>.up.sql / .down.sql, embedded
docs/           # generated Swagger (committed)
tests/          # integration tests, apitest helpers, testdb, generated mocks
```

`main.go` stays small: load config, build the container, start, shut down gracefully.

## Layers

```text
HTTP → middlewares → handler → service → repository → PostgreSQL
                                  └──→ ports (AI, connectors, mailer, queue, clock)
```

| Layer | Does | Does not |
| --- | --- | --- |
| Handler | decode and validate the shape, call the service, map errors, write the response | business rules, SQL |
| Service | business rules, authorization, transactions, orchestration | SQL, `http.Request` |
| Repository | SQL, scanning, persistence | business rules, e-mail, tokens |
| Middleware | auth, CORS, rate limit, client IP, logs, recover, security headers | business rules |

- Dependency injection by constructor, wired in `internal/bootstrap/container.go`. No globals or
  singletons.
- Small interfaces defined by the consumer (`internal/services/ports.go`), only where they buy
  decoupling or testability.
- Services never depend on a vendor API shape: AI goes through `AIGateway` and ports such as
  `Embedder` and `GenerationAI`; providers implement `ai.Provider` in `pkg/ai`. Local models must
  keep working.
- DTOs are separate from models. Clients never send backend-controlled fields (`id`,
  `created_at`, `role`).
- Time comes from an injected `now func() time.Time`, never `time.Now()` inside services.

## Database and SQL

- Parameterized queries only (`$1`, `$2`). Dynamic parts that cannot be parameters (sort column,
  direction) come from an allowlist in code.
- List the columns; no `SELECT *`. Use the `...Context` methods with the request `ctx`. Close
  `rows` and check `rows.Err()`.
- Avoid N+1: joins, `= ANY($1)` or batches. Lists are paginated with a server-side cap.
- Atomic multi-write operations use `TxManager.WithinTx` in the service; the transaction travels
  in the context.
- Counters and limits rely on constraints, `FOR UPDATE` or advisory locks, not read-check-write.
- Constraints in the database are the last line of defense; services translate violations
  (unique → `409`).
- UUID primary keys (`uuid.UUID` in Go), `TIMESTAMPTZ` in UTC.

## Migrations

- Every migration has an `up` and a `down` file. They run through `cmd/migrate` (`make migrate`),
  never on API startup.
- Never edit a migration that was already released; add a new one.
- Keep schema changes compatible with the previous app version (expand, migrate, contract).

## Configuration

- All configuration comes from environment variables, loaded once in `internal/config` and
  injected. No `os.Getenv` outside that package.
- Invalid or missing required values stop the process with a clear message. Production refuses
  development defaults (secrets, `sslmode=disable` without `DATABASE_ALLOW_PLAINTEXT`, CORS `*`,
  `SMTP_TLS=none`).
- `.env` is never committed. `.env.example` and the docs' variable tables change with the code.

## HTTP

- Routes live in `internal/bootstrap/routes.go` under `/api/v1`. Breaking changes inside a version
  need an explicit decision. `/healthz` and `/readyz` stay unauthenticated.
- Use the helpers in `internal/handlers/respond.go`: `decodeJSON` (body limit, unknown fields
  refused, validation), `respondJSON` and `respondError`. Do not reinvent them per endpoint.
- Responses: `{ "data": ... }` or `{ "error": { "code", "message", "fields" } }`. Codes are
  stable `UPPER_SNAKE_CASE`; the web app decides by code.
- Status codes: `400` invalid input, `401`, `403`, `404`, `409` conflict, `422` business rule,
  `429` rate limit, `500` internal, `502`/`503` dependency. Never `200` with an error body.

## Errors, context and concurrency

- Handle every error; wrap with context (`fmt.Errorf("create source: %w", err)`) and compare with
  `errors.Is`/`errors.As`. Semantic errors come from `internal/apperrors`.
- Never return stack traces, SQL, Postgres messages or SDK errors to the client; log them.
- Every I/O function takes `ctx` first and propagates it. Every outbound call has a timeout; use a
  configured `http.Client`, never `http.DefaultClient`.
- Goroutines have an owner and a way to stop. Shared state is protected. Tests run with `-race`.
- Slow work is a job in the Postgres queue (`internal/workers`), handled by the worker. Handlers
  are idempotent: a job can run more than once.

## Security

- No secrets in code, fixtures, images or logs. Compare secrets in constant time.
- Third-party secrets stored in the database (provider API keys, source credentials) are
  encrypted with `pkg/secretbox`.
- One-time tokens (invites, verification, password reset, refresh tokens) come from
  `crypto/rand`, are stored only as SHA-256 hashes, expire and are single use.
- Passwords use bcrypt with a length cap; login and reset answers never reveal whether an e-mail
  exists.
- JWT: fixed algorithm, `exp` checked, nothing sensitive in claims. Identity comes from the
  context set by the auth middleware, never from the request body.
- Authorization is checked in the service for every resource, including lists and helpers;
  permissions are centralized in `internal/auth/permissions.go`. Resources the user may not see
  answer `404`.
- Outbound requests to URLs that come from data (feeds, articles) use `pkg/safehttp`, which
  blocks private, loopback, link-local and metadata addresses after DNS resolution. AI provider
  base URLs are the exception (admins set them, and local models live on private networks).
- Rate limits protect auth, signup, reset and e-mail endpoints. The client IP comes from
  `X-Forwarded-For` only through `TRUSTED_PROXIES`.
- E-mail HTML comes from `html/template` (`email_templates.go`), never from string
  concatenation.
- Never log passwords, tokens, `Authorization`, cookies, API keys or URLs with credentials.

## API docs

- Every endpoint has swag annotations above the handler (`@Summary`, `@Tags`, `@Param`,
  `@Success`, every `@Failure`, `@Security`, `@Router`).
- Regenerate with `make swag` and commit `docs/`. The web types come from it (`pnpm gen:api` in
  `web/`). CI fails when either is stale.

## Tests

| Kind | Tooling | Where |
| --- | --- | --- |
| Unit | `testing` + Testify, table-driven | next to the code |
| Mocks | `go.uber.org/mock`, `//go:generate` (`make mocks`) | `tests/mocks/` |
| Repositories and API | real Postgres via testcontainers, `httptest` against the real router | `tests/integration/` |
| Connectors | recorded synthetic fixtures | `pkg/connectors/testdata/` |

- Services cover success, invalid input, not found, conflict, forbidden and dependency failure.
  Endpoints cover status, body, auth and another user's access.
- Repositories are tested against real Postgres, never a mocked `database/sql`.
- Deterministic tests: no real network, no real clock, no order dependence. `-short` skips the
  integration tests that need Docker.
- Every bug fix comes with a test that fails without it. Mocks are generated, not hand-written.
- Changes to idea generation keep the dedup tests green.

## Checks

```bash
make check        # go vet, golangci-lint, go test -race, build, gofmt
make swag         # when handlers or DTOs changed
make mocks        # when a mocked interface changed
```

Local loop: `make infra-up migrate run` and `make worker` in another terminal.

## Rules for agents

- Read the structure and follow the existing pattern; reuse the helpers for decoding, responses,
  transactions, errors and pagination.
- Smallest change that solves the task; no unrequested refactors or dependencies.
- Do not break public contracts (routes, payloads, error codes) without an explicit request.
- Never edit a released migration, disable a test or lint rule to pass, or commit secrets.
