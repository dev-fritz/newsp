# AGENTS.md — web

Rules for the React Router app and its BFF. The repository-wide rules in the root
[`AGENTS.md`](../AGENTS.md) still apply.

The web app is a React Router (framework mode, SSR) application served by a small Express server
(`server.js`). It owns the user session: the browser talks only to this server, which proxies
`/api/*` to the Go API with the user's tokens. Follow the existing code; when a rule here and a
consolidated pattern in the code disagree, follow the code and fix this file.

## Stack

| Concern | Library |
| --- | --- |
| Language | TypeScript, `strict` |
| UI and routing | React 19, React Router 8 (framework mode, SSR), Vite |
| Server | Express 5 (`server.js`) with `@react-router/express` |
| Server state | TanStack Query |
| HTTP | Axios, one instance in `src/lib/http.ts` |
| Forms and validation | React Hook Form + Zod (`@hookform/resolvers`) |
| Styling | Tailwind CSS 4 with theme tokens, shadcn/ui on Radix, `class-variance-authority` |
| Icons and toasts | lucide-react, sonner |
| i18n | i18next + react-i18next (English and pt-BR) |
| Markdown | react-markdown + remark-gfm, no raw HTML |
| API types | `openapi-typescript`, generated from the API's Swagger (`pnpm gen:api`) |
| Lint and format | Oxlint, Prettier with `prettier-plugin-tailwindcss` |
| Package manager | pnpm (version pinned in `package.json`) |

There is no client-side global store: server data lives in TanStack Query, shareable UI state in
the URL and local UI state in `useState`. Do not add a state, UI, styling or form library without
an explicit decision recorded in the docs.

## Layout

```text
server.js                 # Express: compression, logs, X-Forwarded-For, React Router handler
src/
├── root.tsx              # providers, document, theme, error boundary
├── entry.server.tsx      # SSR, security headers and the per-request CSP nonce
├── routes.ts             # route table
├── routes/               # thin route modules; api.ts is the /api/* proxy
├── features/<feature>/   # admin, auth, feed, home, ideas, profile, reports
│   ├── api/              # HTTP functions (<f>-api.ts) and query keys + hooks (<f>-queries.ts)
│   ├── components/
│   ├── hooks/
│   ├── lib/              # pure helpers of the feature
│   ├── schemas/          # Zod schemas
│   ├── locales/          # en.json, pt-BR.json (namespace = feature name)
│   └── index.ts          # public API of the feature
├── components/
│   ├── ui/               # shadcn/ui primitives, no business rules
│   ├── common/           # states, confirm dialog, form field, page header
│   └── layout/           # app shell
├── hooks/                # generic hooks
├── lib/                  # http, api-error, query-client, safe-redirect, theme, utils (cn)
│   └── .server/          # server-only: session, auth refresh, API forwarding, origin check
├── config/env.server.ts  # validated server environment
├── i18n/                 # setup and shared strings (locales/<lng>/common.json)
├── types/api.ts          # generated, never edited by hand
└── styles/index.css      # Tailwind and theme tokens
```

### Imports

- Use the `@/` alias for `src/`, not long relative paths.
- A feature imports another feature only through its `index.ts`.
- `components/ui` and `lib/` never import from `features/` or `routes/`.
- Server-only code (`*.server.ts`, `.server/` folders) is never imported by client code.

## Components

- Function components, one per file. Files in kebab-case, components in PascalCase
  (`idea-card.tsx` exports `IdeaCard`). Hooks are `use-x.ts` exporting `useX`.
- Named exports; default exports only where React Router requires them (route modules).
- Do not define components inside other components. Use stable IDs as `key`.
- Route modules stay thin: read params, compose feature components, set the layout.
- Split components that grow past ~200 lines or mix data, forms and presentation.

## Data and the API

- Components never call `http`, `axios` or `fetch`. Only `features/<f>/api` does.
- Every failed call becomes an `ApiError` (`src/lib/api-error.ts`): `status` (0 = network),
  stable `code`, safe `message` and optional `fields`. The UI decides by `code`/`status`, never by
  the message text.
- The API wraps responses in `{ "data": ... }`; unwrap it in the `api/` layer.
- Query keys come from a factory per feature, never loose strings. After a mutation, invalidate
  or update the affected queries. Optimistic updates only where the UX needs them, with rollback.
- `src/types/api.ts` is generated from `api/docs/swagger.json` with `pnpm gen:api`. When the API
  contract changes, regenerate it; CI fails if it is stale.
- Never copy API data into local state. Do not fetch or derive state in `useEffect`.
- Filters, pagination, tabs and search that should be shareable live in the URL search params,
  validated with Zod.

## Session, auth and the BFF

- Tokens never reach browser JavaScript. They live in an encrypted, `HttpOnly`, `SameSite=Lax`
  session cookie (`Secure` in production) managed in `src/lib/.server/`.
- The browser calls `/api/*` on the same origin; `src/routes/api.ts` checks the origin of
  mutations, adds `Authorization: Bearer` and forwards to `/api/v1/*`. Token refresh happens only
  on the server, in one place.
- A `401` that reaches the browser (the server could not refresh) sends the user to the login
  page with a safe `?redirect=` (`src/lib/http.ts`).
- Pages that receive a token in the URL (invite, e-mail verification, password reset) consume it
  and remove it from the URL.
- The frontend is not a security boundary. It adapts the UI to the permissions the API returns,
  through `useCan("resource:action")`, never by comparing role names.

## Security

- No `dangerouslySetInnerHTML` with untrusted content. Markdown goes through `react-markdown`
  without `rehype-raw`.
- Links and images from data accept only `http`, `https` and `mailto`. External links use
  `target="_blank"` with `rel="noopener noreferrer"`.
- `?redirect=`-style parameters accept internal paths only (`src/lib/safe-redirect.ts`).
- The CSP uses a per-request nonce (`entry.server.tsx`); inline scripts must carry it.
- Never log tokens, passwords, cookies, personal data or form contents. No `console.log`.
- Validation in the client is UX; the API always validates.

## Environment

- Read only on the server and validated with Zod in `src/config/env.server.ts`; `server.js` reads
  `PORT`, `HOST` and `NODE_ENV`. The client gets what it needs from the root loader.
- Never read `process.env` or `import.meta.env` outside those modules. Nothing secret reaches the
  client bundle.
- Keep `.env.example` complete and in sync with the code.

## Forms

- React Hook Form + Zod. The schema is the source of truth; types come from `z.infer`.
- Apply backend field errors (`ApiError.fields`) with `setError`.
- Disable submit while the mutation is pending. Every field has a label and accessible errors
  (`aria-invalid`, `aria-describedby`); reuse `components/common/form-field.tsx`.

## Styling, layout and accessibility

- Tailwind utilities with `cn()`. Colors, radii and fonts come from the theme tokens in
  `styles/index.css`; no hard-coded hex values. Light and dark themes are applied during SSR.
- shadcn/ui components are added with the CLI to `components/ui` and stay generic; variants use
  `cva`.
- Mobile first, with no horizontal overflow. Touch targets of at least 44×44px on coarse
  pointers; actions shown on hover stay visible on touch. The layout is fluid on wide screens;
  long-form text keeps a readable measure (`max-w-[..ch]`).
- Semantic HTML (never a clickable `div`), keyboard operable with visible focus, `aria-label` on
  icon-only buttons, Radix primitives for dialogs, menus and popovers, WCAG AA contrast.
  Destructive actions ask for confirmation.
- Every data view handles loading, empty, error and no-permission states
  (`components/common/states.tsx`); mutations give feedback with a toast.

## i18n

- No hard-coded UI text. Shared strings go in `src/i18n/locales/<lng>/common.json`; feature
  strings in `features/<f>/locales/<lng>.json`. Add every key to both `en` and `pt-BR`.
- Format dates and numbers with `Intl`. Day-only dates (`YYYY-MM-DD`) are not converted across
  time zones.

## Checks

There is no test runner in the web app yet. Prove a change with the gates and by running it
(`pnpm dev`, desktop and a mobile viewport):

```bash
pnpm lint && pnpm typecheck && pnpm format:check && pnpm build
pnpm gen:api   # when the API contract changed; commit src/types/api.ts
```

## Rules for agents

- Read the structure and follow the existing pattern; reuse `components/ui`, `components/common`
  and `lib/` before creating new helpers.
- Read the API contract (`api/docs/swagger.json` or `src/types/api.ts`) instead of guessing shapes.
- Smallest change that solves the task; no unrequested refactors or dependencies.
- No `any`, `@ts-ignore` or `!` to hide `undefined`; unknown input is `unknown`, narrowed with
  Zod. Never disable a lint rule to pass; if unavoidable, do it on the line with a reason.
