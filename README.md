<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/banner-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="docs/assets/banner.svg">
    <img alt="newsp: Read the market. Ship the next idea." src="docs/assets/banner.svg" width="100%">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/dev-fritz/newsp/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/dev-fritz/newsp/ci.yml?branch=main&label=CI"></a>
  <a href="https://github.com/dev-fritz/newsp/actions/workflows/codeql.yml"><img alt="CodeQL" src="https://img.shields.io/github/actions/workflow/status/dev-fritz/newsp/codeql.yml?branch=main&label=CodeQL"></a>
  <a href="https://dev-fritz.github.io/newsp/"><img alt="Docs" src="https://img.shields.io/github/actions/workflow/status/dev-fritz/newsp/deploy-docs.yml?branch=main&label=docs"></a>
  <a href="LICENSE"><img alt="License: AGPL-3.0" src="https://img.shields.io/badge/license-AGPL--3.0-blue"></a>
  <img alt="Go" src="https://img.shields.io/github/go-mod/go-version/dev-fritz/newsp?filename=api%2Fgo.mod&label=Go">
  <img alt="Node" src="https://img.shields.io/badge/node-%E2%89%A524-339933?logo=node.js&logoColor=white">
</p>

<p align="center">
  <a href="https://dev-fritz.github.io/newsp/">Docs</a> ·
  <a href="https://dev-fritz.github.io/newsp/installation/">Installation</a> ·
  <a href="https://dev-fritz.github.io/newsp/configuration/">Configuration</a> ·
  <a href="https://dev-fritz.github.io/newsp/how-it-works/">How it works</a> ·
  <a href="https://dev-fritz.github.io/newsp/faq/">FAQ</a> ·
  <a href="CONTRIBUTING.md">Contributing</a> ·
  <a href="README.pt-BR.md">Português</a>
</p>

**newsp** is a self-hosted, multi-user market radar. It collects news and discussions from
Hacker News, TabNews, Reddit, YouTube, Lobsters, dev.to, Mastodon, Bluesky, GitHub, Product Hunt
and any RSS feed, then uses AI models (local or hosted)
to write a **daily market report** for each user and **software project ideas that never
repeat**: when a new idea is too close to one you already have, it becomes a new version of
that idea instead of a duplicate.

## Features

- **Collection** on a schedule per source (Hacker News, TabNews, Reddit, YouTube channels,
  Lobsters, dev.to, Mastodon, Bluesky, new GitHub repositories, Product Hunt launches, RSS),
  with full-text search over everything collected.
- **Daily report** per user, ranked by your interests, with numbered citations to the original
  items and a "what changed since the last report" section. Generate one on demand for any
  window.
- **Project ideas** that explain why now, for whom and how: audience, stack and architecture,
  MVP plan, monetization, competitors and the evidence behind them.
- **No duplicates**: embeddings plus an LLM judge merge near-duplicates into a new version, with
  version history, word-level diff, "deepen with AI" and manual edits.
- **Any model**: Ollama, llama.cpp, LM Studio or any OpenAI-compatible server, plus OpenAI,
  Anthropic and Gemini. Pick the model per task; API keys are encrypted at rest.
- **Multi-user**: the admin invites people by link or e-mail, with optional public signup and
  e-mail verification (SMTP e-mail is optional); each user has their own interests, schedule,
  language (English or Portuguese) and theme.

## Quick start

Requirements: Docker with Compose and `openssl`.

```bash
git clone https://github.com/dev-fritz/newsp.git
cd newsp
./scripts/init-env.sh          # creates .env with random secrets
docker compose up -d --build
```

Open <http://localhost:3000>, create the admin account and follow the setup: add an AI provider
and its models, then a few sources. The first report and ideas show up after the first
collection.

Local models: `docker compose --profile ollama up -d` starts Ollama next to the stack; add an
"OpenAI-compatible" provider with base URL `http://ollama:11434/v1`. To serve newsp on a domain,
put it behind a TLS reverse proxy; see the [installation guide](https://dev-fritz.github.io/newsp/installation/).

## Development

```bash
cd api && cp .env.example .env && make infra-up migrate
make run                       # API on :8080 (another terminal: make worker)
cd ../web && cp .env.example .env && pnpm install
pnpm dev                       # http://localhost:5173
```

Checks before a PR: `cd api && make check` and
`cd web && pnpm lint && pnpm typecheck && pnpm format:check && pnpm build`.
See [CONTRIBUTING.md](CONTRIBUTING.md) and the [development guide](https://dev-fritz.github.io/newsp/development/).

## How it is built

| Part | Stack |
| --- | --- |
| `api/` | Go: HTTP API and a worker that runs collection and generation jobs from a queue in Postgres |
| `web/` | React Router (SSR) with a backend-for-frontend that keeps tokens out of the browser, Tailwind and shadcn/ui |
| Data | PostgreSQL 17 with pgvector |
| `docs/` | This documentation, published with MkDocs Material |

More in [How it works](https://dev-fritz.github.io/newsp/how-it-works/) and
[Architecture](https://dev-fritz.github.io/newsp/architecture/).

## Contributing and security

Issues and pull requests are welcome; read [CONTRIBUTING.md](CONTRIBUTING.md) and the
[Code of Conduct](CODE_OF_CONDUCT.md). Report vulnerabilities privately as described in
[SECURITY.md](SECURITY.md).

## License

[GNU AGPL-3.0](LICENSE). If you run a modified newsp for other people over a network, you must
offer them its source code.
