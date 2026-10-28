# Usage

This page walks through the web app from the first admin to daily use.

## First admin

On a fresh install, any visit to the app opens the **setup** screen. Create the first account with
your name, e-mail and a password (12 to 72 characters). It becomes an **admin**, and the setup
screen disappears for good.

A short wizard follows: add an [AI provider and models](configuration.md#ai-providers), then
[enable sources](configuration.md#sources). You can return to both later under **Admin**.

## Users, invites and password resets

- **Invite a user:** **Admin → Users → Invite**, pick the role (**member** or **admin**) and,
  optionally, an e-mail. With an e-mail, the invite is sent to that address and the account must
  use it. The copyable link (`<APP_BASE_URL>/invite/<token>`) is always available. Without an
  e-mail the invite works as a plain link: the person chooses name, e-mail and password.
- **Public signup:** off by default. An admin turns it on in **Admin → Settings** (needs e-mail
  configured). The login page then shows **Create account** (`/signup`). The account is a
  **member** and signs in right away, but the e-mail is unverified. A verification link, valid for
  24 hours and single use, is sent; it can be resent from the banner in the app (3 per hour). Until
  verified, the account gets no daily report or ideas and cannot generate manually. Limit: 5
  signups per hour per IP.
- **Forgot password:** **Forgot password?** on the login page (`/forgot-password`). Enter the
  e-mail and, if an active account exists, a reset link valid for 1 hour is sent. The answer is
  always the same, so it does not reveal whether the e-mail exists. Limits: 5 requests per hour per
  IP and 3 per hour per e-mail.
- **Reset by an admin:** on the user's row, **Reset password** creates a link
  (`<APP_BASE_URL>/reset-password/<token>`) that replaces older ones.
- Admin links are **single use**, expire after **7 days** and are shown only once. Pending invites
  can be revoked.
- Admins can disable and re-enable users. A disabled user cannot sign in, and needs to be
  reactivated before a reset link can be made.

!!! note
    Without an e-mail server everything still works through copied links: invites and admin
    resets. In production without SMTP, public signup, "Forgot password?" and e-mailed invites
    are not available. In development the e-mails, with their links, are written to the API log,
    so those flows work there.

!!! note
    Roles are **admin** (manages users, sources, AI providers and instance settings) and
    **member** (reads, generates and manages their own reports and ideas). Reports and ideas are
    private to each user; collected items are shared by everyone.

## Profile and interests

Under **Profile**, each user sets:

| Setting | Effect |
| --- | --- |
| **Topics** (up to 20) | Your interests. They are embedded and used to rank items, so reports and ideas favor them |
| **Output language** (`pt-BR` or `en`) | Language of reports and ideas |
| **Time zone** and **report time** | When the daily report runs, for example `08:00` |
| **Daily report** on/off | Turns the scheduled report off while keeping manual ones |
| **Ideas per day** (0 to 10) | Ideas generated after each scheduled report; 0 disables daily ideas |
| **Report model** and **idea model** | Your own choice among enabled chat models; empty uses the instance default |
| **Excluded sources** | Sources you never want in your reports and ideas |
| **Theme** and **interface language** | Light, dark or system; pt-BR or English |

### My AI providers

If the admin enables BYOK, **Profile → My AI providers** lets you use your own key. Add one
provider per kind (OpenAI, Anthropic or Gemini), paste the key and run the connection test, then
register the chat models you want from the list it returns. Keys are encrypted and never shown
again. Your models appear under **Models**, marked *yours*, and you can pick them for reports and
ideas. Usage of the last 7 days is shown per model. Provider costs are billed by the provider.

!!! tip
    Write topics as short phrases ("developer tools for small teams", "LLM evaluation"). Ranking by
    topics needs an embedding model; without one, the profile shows that topics are not used.

## Feed

**Feed** lists everything collected, filterable by source, search text and period. It is the quickest
way to check that a source works and to see what the reports draw from.

## Reports

Open **Reports**.

- **Scheduled:** a report is generated every day at your report time, for the last 24 hours.
- **On demand:** choose a window (1 hour to 30 days) and generate it now. Generation runs as a job,
  and the page follows its progress. Local models can take minutes.
- A report is Markdown with numbered citations such as `[3]`. Each number links to the original
  item. The report knows what your previous report said and focuses on what changed.
- If nothing was collected in the window, or no chat model is configured, the report fails with a
  clear message instead of inventing content.

## Ideas

Open **Ideas**.

**Generating.** After each scheduled report, newsp generates your "ideas per day". You can also
**generate now**, choosing 1 to 5 ideas, an optional **focus** (for example "tools for developers")
and the window of news to consider (1 to 30 days, default 7).

**What an idea contains.** Title and summary, why now, audience segments, how it works, a
differentiator, stack, architecture steps, an MVP plan, monetization, competitors, difficulty
(beginner, intermediate, advanced), estimated effort in weeks, and the **evidence**: the collected
items that support it.

**Status.** Every idea is **new**, **saved** or **discarded**. Triage with the status buttons.
Discarded ideas still count for dedup, so they do not come back as new ideas.

**Versions.** If a generated idea is too similar to one you already have, it is not duplicated: it
becomes a **new version** of the existing idea, with a change note explaining what is new and the
news that motivated it. Each version shows its origin: *generated*, *merged*, *deepened* or *manual*.
Open any version, and use **Compare** to see a diff between two versions.

**Deepen.** Ask for a deeper version on chosen aspects (architecture, MVP plan, market,
monetization, audience, competitors) and add an instruction of up to 500 characters. The result is
saved as a new version.

**Edit manually.** Edit any field yourself. Saving creates a new *manual* version, so the history
is never lost.

## Admin

- **Sources:** add, edit, enable or disable, **Collect now**, and check last status and errors.
- **AI:** providers, models, connection tests and token usage of the last 7 days per model.
- **Users:** invites, resets, roles and access.
- **Settings:** embedding model, default models, retention, public signup, pausing inactive
  accounts, similarity threshold and the BYOK switch (**Allow users' own AI keys**). Admins never
  see users' own providers or keys.

See [Configuration](configuration.md) for the options.
