# Development

## Prerequisites

- Go (the version in `api/go.mod`), `make`, Docker (Postgres and integration tests).
- Node 24 or newer and pnpm (the version in `web/package.json`).
- Optional: `golangci-lint` (`make tools` installs the pinned version), Python 3.13 for the docs.

Read [`AGENTS.md`](https://github.com/dev-fritz/newsp/blob/main/AGENTS.md) and the `AGENTS.md` of
the folder you work in before changing code.

## Run it locally

```bash
# 1. Backend
cd api
cp .env.example .env            # development defaults; never commit .env
make infra-up                   # starts Postgres (and pgvector) from the root compose file
make migrate                    # applies migrations
make run                        # API on http://localhost:8080
make worker                     # in another terminal: scheduler and jobs

# 2. Frontend
cd ../web
cp .env.example .env
pnpm install
pnpm dev                        # http://localhost:5173
```

Open <http://localhost:5173> and follow the setup screen. For a local model, run Ollama on your
machine and add an OpenAI-compatible provider with base URL `http://localhost:11434/v1`.

Swagger UI is served at <http://localhost:8080/swagger/index.html> outside production.

## Backend make targets (`api/`)

| Target | What it does |
| --- | --- |
| `make tools` | Install the pinned `golangci-lint` |
| `make infra-up` / `make infra-down` | Start Postgres from the root compose file / stop the stack |
| `make migrate` / `migrate-down` / `migrate-version` | Apply, revert or show migrations |
| `make run` | Run the API |
| `make worker` | Run the scheduler and job runner |
| `make reindex` | Enqueue a full embedding reindex (the worker does the work) |
| `make build` | Build the binaries into `bin/` |
| `make test` | `go test -race ./...` (integration tests need Docker) |
| `make test-short` | Same, skipping tests that need Docker |
| `make lint` / `make fmt` / `make vet` | golangci-lint, gofmt and formatters, `go vet` |
| `make swag` | Regenerate the Swagger spec in `api/docs` |
| `make mocks` | Regenerate GoMock mocks (`go generate`) |
| `make check` | `vet`, `lint`, `test`, `build` and a gofmt check: the gate for PRs |

## Frontend scripts (`web/`)

| Script | What it does |
| --- | --- |
| `pnpm dev` | Dev server with HMR on port 5173 |
| `pnpm build` | Production build |
| `pnpm start` | Serve the production build |
| `pnpm typecheck` | `react-router typegen` and `tsc` |
| `pnpm lint` | Oxlint |
| `pnpm format` / `pnpm format:check` | Prettier, write or check |
| `pnpm gen:api` | Regenerate `src/types/api.ts` from the Swagger spec |

The PR gate is `pnpm lint && pnpm typecheck && pnpm format:check && pnpm build`.

## Tests

Backend tests use Testify, GoMock and **testcontainers**. Integration tests start one
`pgvector/pgvector` container per test binary (the same image as the compose file), migrate a
template database once and clone it for every test, so tests are isolated and can run in parallel.
Docker must be running; use `make test-short` for a quick loop without it. Connectors are tested
against short synthetic fixtures, never real third-party content, and AI providers are faked with
`httptest` servers.

Frontend tests and end-to-end tests are planned after the MVP; for now the web gate is lint,
typecheck, format and build.

## Changing the API contract

1. Edit the handler, DTO and swag annotations.
2. Regenerate the spec and the TypeScript types:

    ```bash
    cd api && make swag
    cd ../web && pnpm gen:api
    ```

3. Commit the regenerated `api/docs/*` and `web/src/types/api.ts`. Never edit them by hand.
4. Keep `/api/v1` backward compatible unless the change explicitly authorizes a break.

## Database migrations

Migrations are SQL files in `api/migrations`, applied by `cmd/migrate` (`make migrate`). The API
never migrates on startup. Add a pair of `NNNNNN_name.up.sql` and `.down.sql` files and an
integration test when the schema changes.

## Docs

```bash
python3 -m venv .venv && . .venv/bin/activate
pip install -r docs/requirements.txt
mkdocs serve                    # http://127.0.0.1:8000
mkdocs build --strict           # what CI runs
```

## Commits and PRs

Conventional Commits 1.0.0 for commits and PR titles. See
[CONTRIBUTING.md](https://github.com/dev-fritz/newsp/blob/main/CONTRIBUTING.md).
