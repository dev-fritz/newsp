---
title: newsp
hide:
  - navigation
---

# newsp

![newsp: Read the market. Ship the next idea.](assets/banner.svg#only-light)
![newsp: Read the market. Ship the next idea.](assets/banner-dark.svg#only-dark)

**newsp** is a self-hosted, multi-user, open source (AGPL-3.0) market radar. It collects news and
discussions from public sources, then uses AI models, local or hosted, to write a **daily market
report** for each user and **software project ideas that never repeat**: when a new idea is too
similar to an existing one, it becomes a **new version** of that idea instead of a duplicate.

## Why newsp

- **You own the data and the models.** Run everything with `docker compose`, point it at a local
  model (Ollama, llama.cpp, LM Studio) and nothing leaves your machine.
- **Reports you can check.** Every claim in a report cites numbered items you can open, and each
  report knows what the previous one said, so it highlights what changed.
- **Ideas that grow instead of repeating.** Embeddings find near-duplicates, an LLM judge confirms
  them, and the merge becomes a new version with history and a diff showing which news motivated
  the change.
- **Swappable AI.** One interface for OpenAI-compatible servers, OpenAI, Anthropic and Gemini.
  Pick a different model per task and per user.
- **Built for a small team.** Admin and member roles, invite links, per-user interests, time
  zone, report time and ideas per day. Invite and reset links work without an e-mail server;
  optional SMTP adds public signup with e-mail verification, "Forgot password?" and e-mailed
  invites.

## Highlights

| Area | What you get |
| --- | --- |
| Sources | Hacker News, TabNews, Reddit, YouTube channels (RSS), Lobsters, dev.to, Mastodon, Bluesky, new GitHub repositories, Product Hunt and any RSS or Atom feed, each with its own interval (5 to 1440 minutes) |
| Reports | Scheduled daily or on demand, Markdown with numbered citations, ranked by your interests, engagement and recency |
| Ideas | Why now, audience, how, stack, MVP plan, monetization, competitors, difficulty, effort and evidence |
| Versions | History, diff, "deepen" and manual edit; discarded ideas still block duplicates |
| AI | Local models, OpenAI, Anthropic and Gemini; API keys encrypted at rest |
| Web | pt-BR and English, light and dark themes, responsive |
| Operations | Postgres 17 with pgvector, a job queue in Postgres, retention, health checks, structured logs |

!!! note "Screenshots"
    Screenshots of the reports, ideas and admin screens will be added here once the first
    release is tagged. Until then, [Usage](usage.md) describes each screen.

## Quick start

You need Docker with the Compose plugin.

```bash
git clone https://github.com/dev-fritz/newsp.git
cd newsp
./scripts/init-env.sh          # creates .env with random secrets
docker compose up -d --build
```

Open <http://localhost:3000>, then:

1. Create the first admin account on the setup screen.
2. Add an AI provider and choose the chat and embedding models in the wizard.
3. Enable a few sources.
4. Open your profile, set your interests and ask for a report.

Want local models? Start the bundled Ollama too:

```bash
docker compose --profile ollama up -d
```

Continue with the [installation guide](installation.md), the [configuration guide](configuration.md)
or [how it works](how-it-works.md).

## Project links

- Source code and issues: [github.com/dev-fritz/newsp](https://github.com/dev-fritz/newsp)
- [Contributing](https://github.com/dev-fritz/newsp/blob/main/CONTRIBUTING.md) and the
  [development guide](development.md)
- [Security policy](https://github.com/dev-fritz/newsp/blob/main/SECURITY.md)
- [Changelog](https://github.com/dev-fritz/newsp/blob/main/CHANGELOG.md)
