# FAQ

## Do I need an API key or a GPU?

No key is needed if you run a local model through Ollama, llama.cpp or LM Studio. A GPU helps a
lot for larger chat models; embeddings with `nomic-embed-text` run fine on a CPU. You can also use
OpenAI, Anthropic or Gemini with an API key.

## Which models should I start with?

Any instruction-following chat model that can produce JSON, plus a 768-dimension embedding model.
For a first local setup, `qwen3:14b` (chat) and `nomic-embed-text` (embeddings) through Ollama is
a reasonable pair. Try your own, since quality depends on your hardware and topics.

## Why does the embedding model have to return 768 dimensions?

Vectors are stored in a `vector(768)` column with an HNSW index. `nomic-embed-text` is natively
768, and newsp asks OpenAI and Gemini for 768 dimensions. A model with another size is rejected.
See [Configuration](configuration.md#the-768-dimension-embedding-requirement).

## Can I change the embedding model later?

Yes. Saving a different embedding model queues a reindex job that re-embeds items, ideas and user
topics. It can take a while on large databases.

## Does newsp send my data to third parties?

Only to what you configure. Sources are fetched from their public endpoints, and prompts go to the
AI provider you choose. With a local model nothing leaves your network. Keys are encrypted at rest.

## Why did my report fail with "no items"?

No collected item matched the window and your filters. Check **Admin → Sources** for errors, use
**Collect now**, and look at the **Feed**. Widen the report window or remove excluded sources.

## Why does my Reddit source fail?

When Reddit refuses the JSON listing (HTTP 403), the connector falls back to the subreddit's
Atom feed by itself, without scores. An HTTP 429 means Reddit is rate limiting your address:
raise the source interval (60 minutes or more) and the next run usually works. Datacenter
addresses are blocked more often than residential ones.

## Why did a generated idea become "version 2" of another idea?

That is the dedup working: the draft was close to an existing idea in embedding space (above the
similarity threshold) and the LLM judge agreed it was the same idea, so it was merged as a new
version with a change note. Discarded ideas count too. Tune the threshold in
[Configuration](configuration.md#instance-settings) if you want more or fewer merges.

## Can I use it without e-mail?

Yes. E-mail is optional. Without SMTP in production, admins share copied invite and reset links,
but there is no public signup and no "Forgot password?". In development the e-mails (with their
links) are written to the API log instead. See
[Usage](usage.md#users-invites-and-password-resets) and
[E-mail (SMTP)](installation.md#e-mail-smtp).

## I lost my admin password

If e-mail is configured, use **Forgot password?** on the login page. Otherwise, ask another admin
for a reset link. If you are the only admin and have no e-mail, you will need database access to
promote another user or create a reset token; back up regularly and keep a second admin account.

## How do I back up and upgrade?

See [Installation](installation.md#backups) and [Installation](installation.md#upgrading).

## Where do I report bugs or vulnerabilities?

Bugs and ideas: [GitHub issues](https://github.com/dev-fritz/newsp/issues). Vulnerabilities:
[private advisory form](https://github.com/dev-fritz/newsp/security/advisories/new), never a
public issue.

## Is there a Portuguese version?

The app is available in pt-BR and English, and the repository README has a Portuguese translation
(`README.pt-BR.md`). This documentation site is in English.
