# panel-ui

Web panel in TypeScript + React (Vite, React Router, TanStack Query).
Components of our own in `src/shared/ui`, built on Radix UI and Tailwind
CSS. Talks only to panel-api, through the client generated from
[`schema/api/panel-api.yaml`](../schema/api/panel-api.yaml).

## What it does today

| Page | Who |
| --- | --- |
| Log in | everyone |
| Tenants (add a tenant) | platform users |
| Protected sites (list, protect a site) | tenant users; adding: administrators |
| Site → Map: versions with their approval path, YAML editor with syntax check, send for review, approve, reject, publish | editors write; approvers and administrators decide (never on their own version) |
| Site → Connectors: add (the token and the `docker run` command shown once), rotate, remove | administrators |
| People (add a person with a role) | administrators |
| Activity (audit log in plain sentences) | administrators, approvers, auditors; platform users |

## Layout

```text
src/
  app/        App (providers), router, Shell and Sidebar
  shared/     ui/ (Button, fields, Dialog, Notice, Table, TabNav, Toast…),
              api/ (generated schema.d.ts, client, error messages)
  features/   auth/ tenants/ sites/ connectors/ maps/ users/ audit/
              each with components/, hooks/, pages/ and index.ts
```

Rules, checked by `npm run lint:boundaries` in CI: `shared/` never imports
from `features/` or `app/`; a feature uses another one only through its
`index.ts`; no file over 200 lines. Data only through hooks (TanStack
Query) over the generated client.

## Design

- Palette: ink `#1C2633`, paper `#F4F6F8`, rules `#CBD3DC`, signal blue
  `#1F4FB8`; a colour per map state (draft grey, in review amber, approved
  blue, published green, rejected red, retired grey). Tokens in
  `src/styles.css`.
- Type: Public Sans for the interface, JetBrains Mono only for maps and
  commands (bundled, no external requests).
- The one distinctive element is the approval rail of each version: four
  steps, filled in the colour of the version's state, with who did each
  step. Everything else stays quiet: rules instead of cards, no shadows.

## Development

```bash
npm ci
npm run gen:api        # after a change to schema/api/panel-api.yaml
EQ_PANEL_API=http://127.0.0.1:8080 npm run dev   # with panel-api running locally
npm test && npm run build && npm run lint:boundaries
```

In Compose, the `panel-ui` image (Caddy) serves the build and passes `/v1`
to panel-api on the same origin; security headers (CSP, frame denial) are
in `Caddyfile`. panel-api can also serve the build itself with
`EQ_UI_DIR`.
