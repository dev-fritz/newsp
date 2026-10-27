# Configuration

Everything below is configured from the web app by an admin, under **Admin**. Deployment settings
(secrets, URLs, ports) are environment variables and are covered in the
[installation guide](installation.md#environment-variables).

## AI providers

Go to **Admin → AI** and add a provider. A provider has a name, a kind, a base URL and, for
hosted services, an API key. Keys are encrypted at rest with AES-256-GCM and never shown again:
the app only displays a short hint such as `••••a9Fk`. After saving, use **Test connection**: it
checks the credentials and lists the models the server offers.

| Kind | Use it for | Base URL |
| --- | --- | --- |
| `openai_compatible` | Ollama, llama.cpp server, LM Studio, vLLM and any server that speaks the OpenAI API | Required, for example `http://ollama:11434/v1` |
| `openai` | OpenAI | Optional, defaults to `https://api.openai.com/v1` |
| `anthropic` | Anthropic (Claude) | Optional, defaults to `https://api.anthropic.com` |
| `gemini` | Google Gemini | Optional, defaults to `https://generativelanguage.googleapis.com` |

=== "Ollama in Compose"

    ```bash
    docker compose --profile ollama up -d
    docker compose exec ollama ollama pull qwen3:14b
    docker compose exec ollama ollama pull nomic-embed-text
    ```

    Kind `openai_compatible`, base URL `http://ollama:11434/v1`, no API key. The compose file
    already raises the context window (`OLLAMA_CONTEXT_LENGTH`, 16384 by default).

=== "Ollama on the host"

    Kind `openai_compatible`, base URL `http://host.docker.internal:11434/v1`, no API key.
    Make sure Ollama listens on an address the containers can reach (`OLLAMA_HOST=0.0.0.0`) and
    start it with a larger context: `OLLAMA_CONTEXT_LENGTH=16384 ollama serve`.

=== "llama.cpp or LM Studio"

    Kind `openai_compatible`, base URL of the server's `/v1` endpoint, for example
    `http://host.docker.internal:1234/v1` for LM Studio. Use the model name the server reports,
    and load the model with a context of at least 16k tokens (`llama-server -c 16384`, or the
    context length setting in LM Studio).

!!! warning "Give local models a context window of at least 16k tokens"
    Reports and ideas send prompts of 5k to 12k tokens. Ollama defaults to 4096 tokens and, through
    its OpenAI-compatible API, silently drops the beginning of longer prompts, which removes the
    instructions; the result is answers without citations that newsp rejects as
    `AI_INVALID_OUTPUT`. The failed job's `result.detail` shows the reason.

=== "OpenAI, Anthropic, Gemini"

    Pick the kind, paste the API key and leave the base URL empty to use the default.

!!! info "Private addresses are allowed for providers"
    Provider base URLs may point to `localhost` or a LAN address on purpose, because local model
    servers live there and only admins can set them. Source fetching, in contrast, is
    SSRF-protected and refuses private addresses.

### Chat models and embedding models

Register the models you want to use under each provider, and mark each as **chat** or
**embedding** (the model name must match what the server reports, for example `qwen3:14b`):

- **Chat models** write reports and ideas, judge duplicates and merge versions. Choose a model that
  follows instructions and can return JSON: newsp asks for structured output using each provider's
  JSON schema mode, validates it, and retries once with the error as feedback. Small local
  models may fail the schema more often. Reports and ideas need a context window large enough for
  the selected items, so a larger window helps.
- **Embedding models** turn items, ideas and your interests into vectors. There is **one embedding
  model for the whole instance**.

You then select, in **Admin → Settings**:

- the instance **embedding model**;
- the **default report model** and **default idea model**, used when a user has not picked one.

Users can choose their own report and idea models in their profile, among the enabled chat models.

### The 768-dimension embedding requirement

Vectors are stored as `vector(768)` with an HNSW index, so the embedding model **must return 768
dimensions**. Known good choices:

| Model | How to get 768 |
| --- | --- |
| `nomic-embed-text` (Ollama) | Native 768 |
| `text-embedding-3-small` (OpenAI) | newsp requests `dimensions: 768` |
| Gemini embedding models | newsp requests 768 output dimensions |

A model that returns another size is rejected with an error instead of being stored. Changing the
instance embedding model queues a **reindex** job: the worker re-embeds all items, ideas and user
topics so that vectors from different models are not mixed. Do it at a quiet time: the job can take a while on large databases.

### Without an embedding model

newsp still works, with lower quality: reports rank by engagement and recency only (your topics
are not used for ranking), and idea dedup can only detect an exact title match. Set an embedding
model before you rely on ideas.

## Instance settings

In **Admin → Settings**:

| Setting | Default | Range | Meaning |
| --- | --- | --- | --- |
| Embedding model | none | any enabled embedding model | Used for items, ideas and topics |
| Default report model | none | any enabled chat model | Fallback for users without a choice |
| Default idea model | none | any enabled chat model | Fallback for users without a choice |
| Retention (days) | 90 | 7 to 3650 | Items older than this are deleted every 6 hours, **unless** a report or an idea cites them |
| Public signup | off | on or off | Shows **Create account** on the login page. Can only be turned on when e-mail is configured (otherwise the API answers 422 `MAILER_NOT_CONFIGURED`) |
| Pause inactive accounts after (days) | 30 | 0 to 3650 | Accounts with no access (login or app use) for longer than this stop getting scheduled reports and ideas. 0 turns it off |
| Idea similarity threshold | 0.80 | 0.50 to 0.99 | Minimum embedding similarity for a draft to be sent to the LLM judge as a possible duplicate |

**AI cost and verification.** Scheduled reports and ideas cost AI calls for every account. The pause
setting applies to all accounts and ends by itself on the next access. Accounts created by public
signup start with an unverified e-mail: until it is verified they get no daily report or ideas and
cannot generate manually. Existing accounts and accounts created by invite count as verified.

**Choosing the threshold.** A lower value sends more drafts to the judge (more merges, more
LLM calls). A higher value lets more near-duplicates through as new ideas. The judge always has the
last word, so a threshold that is slightly too low is the safer error. Adjust it based on what you
see in the ideas list and measure before and after.

## Sources

Add them in **Admin → Sources**. Every source has a name, a kind, a config, an interval and an
on/off switch. The interval is **5 to 1440 minutes** (default 30). **Collect now** queues an
immediate run, and the list shows the last run, last success, last error and items collected in
the last 24 hours. After repeated failures the interval backs off, up to 6 hours.

Config fields are validated when you save. Omitted optional fields take the defaults below.

| Kind | Field | Values | Default |
| --- | --- | --- | --- |
| `hackernews` | `list` | `top`, `best`, `new` | `top` |
| | `limit` | 1 to 100 | 30 |
| `tabnews` | `strategy` | `relevant`, `new`, `old` | `relevant` |
| | `limit` | 1 to 100 | 30 |
| `reddit` | `subreddit` | name such as `SaaS` or `r/SaaS` (2 to 21 letters, digits, underscores), required | none |
| | `sort` | `top`, `hot`, `new` | `top` |
| | `time` | `hour`, `day`, `week` (only used with `top`) | `day` |
| | `limit` | 1 to 100 | 25 |
| `youtube` | `channel_id` | channel ID starting with `UC` (24 characters), required | none |
| | `limit` | 1 to 50 | 15 |
| `rss` | `url` | absolute `http(s)` URL of an RSS or Atom feed, required | none |
| | `limit` | 1 to 200 | 50 |
| `lobsters` | `list` | `hottest`, `newest` (ignored when `tag` is set) | `hottest` |
| | `tag` | Lobsters tag such as `ai` or `go` | none |
| | `limit` | 1 to 25 | 25 |
| `devto` | `tag` | dev.to tag such as `saas` | none |
| | `top` | `0` (newest), or most popular in the last `1`, `7`, `30` days | `0` |
| | `limit` | 1 to 100 | 30 |
| `mastodon` | `instance` | host name such as `mastodon.social`, required | none |
| | `hashtag` | hashtag without `#` (set this or `account`) | none |
| | `account` | `user` or `user@other.host` (set this or `hashtag`) | none |
| | `limit` | 1 to 40 | 20 |
| `bluesky` | `mode` | `author`, `feed`, `search` | `author` |
| | `actor` | handle or DID, required with `author` | none |
| | `feed` | `at://…/app.bsky.feed.generator/…` or `https://bsky.app/profile/…/feed/…`, required with `feed` | none |
| | `query` | search terms, required with `search` | none |
| | `sort` | `top`, `latest` (only used with `search`) | `top` |
| | `identifier` | handle or e-mail that logs in, required with `search` | none |
| | `limit` | 1 to 100 | 30 |
| `github` | `query` | free search terms (no `created:`, `stars:` or `sort:`) | none |
| | `language` | language such as `Go` or `TypeScript` | none |
| | `topic` | topic such as `llm` | none |
| | `min_stars` | 1 to 1000000 | 10 |
| | `created_within_days` | 1 to 30 | 7 |
| | `limit` | 1 to 100 | 30 |
| `producthunt` | `order` | `ranking`, `votes`, `newest` | `ranking` |
| | `topic` | topic slug such as `developer-tools` | none |
| | `posted_within_days` | 1 to 7 | 1 |
| | `limit` | 1 to 50 | 20 |

Examples of the `config` JSON:

```json
{ "list": "best", "limit": 40 }
{ "strategy": "relevant", "limit": 30 }
{ "subreddit": "SaaS", "sort": "top", "time": "week", "limit": 25 }
{ "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx", "limit": 10 }
{ "url": "https://feeds.bbci.co.uk/news/technology/rss.xml", "limit": 30 }
{ "tag": "ai", "limit": 25 }
{ "tag": "saas", "top": 7, "limit": 30 }
{ "instance": "mastodon.social", "hashtag": "indiehackers", "limit": 40 }
{ "mode": "search", "query": "saas pricing", "identifier": "you.bsky.social" }
{ "language": "Go", "min_stars": 50, "created_within_days": 7 }
{ "order": "votes", "topic": "developer-tools" }
```

### Credentials

Some kinds take a credential, sent in the `secret` field of the source (the form shows a password
field). It is encrypted with `ENCRYPTION_KEY`, like provider keys, and never returned: the API
only says `has_secret`. Leave it blank when editing to keep it, or remove it explicitly.

| Kind | Credential | Needed | Cost and limits |
| --- | --- | --- | --- |
| `bluesky` | [app password](https://bsky.app/settings/app-passwords) of the `identifier` account | only for `search` | free; logins are cached for 90 minutes to stay under the login rate limit |
| `github` | [personal access token](https://github.com/settings/tokens) with no scopes | optional | free; search allows 10 requests per minute without a token, 30 with one |
| `producthunt` | developer token from [your API applications](https://www.producthunt.com/v2/oauth/applications) | always | free for non-commercial use; check the Product Hunt API terms before commercial use |

`lobsters`, `devto` and `mastodon` read public APIs and take no credential. If you change
`ENCRYPTION_KEY`, stored credentials can no longer be read: the source shows an error until you set
the credential again.

!!! warning "Reddit limits anonymous clients"
    Reddit is read through its public JSON listing, which carries score and comment counts. Reddit
    refuses that endpoint to many anonymous clients (HTTP 403); the connector then reads the
    subreddit's Atom feed instead, which has no engagement numbers, so Reddit items rank by
    relevance and recency only. Reddit also rate limits anonymous reads (HTTP 429): keep the
    interval at 60 minutes or more per subreddit.

!!! tip "Finding a YouTube channel ID"
    Open the channel page, view the page source and search for `channelId`. Handles such as
    `@name` are not accepted: the ID starts with `UC`.

All fetches go through an SSRF-safe client: feed URLs and Mastodon instances that resolve to
private, loopback or link-local addresses are refused, including after redirects.

LinkedIn, X, Threads and Instagram are not supported: they offer no free official API to read
public posts, and newsp does not scrape.

## Per-user settings

Users set their own interests, language, schedule and models in **Profile**; see
[Usage](usage.md#profile-and-interests).
