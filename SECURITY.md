# Security Policy

## Supported versions

newsp is young and moves fast. Only the **latest release** (and `main`) receives security
fixes. Please upgrade before reporting, and include the version or commit you tested.

## Reporting a vulnerability

**Do not open a public issue for a vulnerability.** Report it privately through GitHub
Security Advisories:

<https://github.com/dev-fritz/newsp/security/advisories/new>

Please include:

- what the problem is and which component it affects (`api`, `worker`, `web`, Docker setup);
- steps to reproduce or a proof of concept;
- the impact you expect (data exposure, privilege escalation, SSRF, ...);
- the version or commit you tested.

You will get an acknowledgement as soon as the maintainer can read the report, and updates as
the fix progresses. When a fix ships, the advisory is published and credits the reporter unless
you prefer to stay anonymous. Please keep the details private until then.

## Scope and security model

newsp is **self-hosted**: whoever runs it is the operator and owns its deployment. What the
project guarantees, and what it leaves to you:

| Topic | Behavior |
| --- | --- |
| TLS and reverse proxy | Operators provide TLS (see the Caddy example in the [installation guide](docs/installation.md)). The stack itself speaks plain HTTP. Production mode refuses an `http://` `APP_BASE_URL` unless it is `localhost`. |
| Provider API keys | Encrypted at rest with AES-256-GCM using `ENCRYPTION_KEY`, never returned by the API (only a short hint). Losing or changing the key makes stored keys unreadable. Back it up with your database. |
| Sessions | The web app is a BFF: tokens live in an encrypted, `HttpOnly` cookie on the web server and never reach browser JavaScript. Refresh tokens rotate and are stored hashed. |
| Passwords and links | Passwords use bcrypt. Links are single use and stored as SHA-256 hashes: e-mail verification expires in 24 hours, self-service password reset in 1 hour, admin invite and reset in 7 days. Links sent by e-mail stay encrypted in the job queue and are deleted after sending. |
| Fetching sources | Feeds and article pages are fetched with an SSRF-safe client that refuses private, loopback, link-local and metadata addresses, also on redirects. AI provider base URLs may be private on purpose (local models on your LAN) because only admins can set them. |
| Rate limiting | Unauthenticated auth endpoints are limited per client IP, and logins per account. The web server appends each connection's peer address to `X-Forwarded-For`, and the API takes the rightmost hop outside `TRUSTED_PROXIES`, so clients cannot choose the address they are limited by. Signup is limited to 5 per hour per IP, verification resend to 3 per hour, and password reset requests to 5 per hour per IP and 3 per hour per e-mail. Hops on networks listed in `TRUSTED_PROXIES` are trusted, so keep that list to the networks of your proxy and containers. |
| E-mail | SMTP is optional. When configured in production it must use STARTTLS or TLS (`SMTP_TLS=none` is refused). `SMTP_PASSWORD` is a secret. A password reset request stores the typed address in the job queue, which is purged after 7 days. |
| Secrets | `.env` is never committed. Production mode refuses the documented development defaults for `JWT_SECRET`, `ENCRYPTION_KEY` and `SESSION_SECRET`. Generate real ones with `./scripts/init-env.sh`. |

Reports about the following are usually out of scope: missing TLS on a deployment you did not
put behind a proxy, issues that need an already-compromised admin account or host, denial of
service from an admin pointing a source at a slow site, and findings in third-party services.

## Hardening checklist for operators

- Create the first admin right after starting the stack: until then, anyone who reaches the
  web port can complete the setup.
- Serve the web app over HTTPS and keep the API and database off the public internet. Only the
  web port (3000 in the default compose file) needs to be reachable.
- Use unique random values for `JWT_SECRET`, `SESSION_SECRET` and `ENCRYPTION_KEY`.
- Set `TRUSTED_PROXIES` to the networks of your reverse proxy and the web container only. For the
  compose stack, set it in the root `.env` (default: the private ranges).
- Leave public signup off if you do not need it, and set up SPF and DKIM for the `MAIL_FROM` domain.
- Back up the database regularly and test the restore.
- Keep the images up to date (see the upgrade section of the installation guide).
