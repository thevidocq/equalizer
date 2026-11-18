# Changelog

A record of **every commit**, written so that anyone (a person or another
Claude session) can pick up the work with no prior context.

## How to write an entry

Add a section at the top of "Entries" for every commit:

```markdown
### YYYY-MM-DD — <commit title>

- **Branch:** <name>
- **What:** what changed, by file or area
- **Why:** reason for the change
- **Status:** what works, what is half-done, what is untested
- **Next steps:** ordered list of what comes next
```

## Current project status

| Area | Status |
| --- | --- |
| Contracts (`schema/`) | First version, validated in CI |
| Documentation | Architecture, map format, installation and manuals (planned features) |
| gateway | GET routes, assets and urlencoded POST forms in direct or connector mode, validation, faithful rendering (UTF-8, cleanup, form binding, URL rewriting, redirects); no file uploads or sessions yet |
| tunnel-relay | Token-authenticated tunnels, forwarding, admin endpoints; single replica |
| connector | Outbound tunnel, local allowlist, reconnection |
| orchestrator | Not implemented yet |
| panel-api | Not implemented yet |
| panel-ui | Not implemented yet |
| agent | Not implemented yet |
| deploy/compose | Caddy (gateway and relay domains), gateway (2 replicas), tunnel-relay, Vector, Loki, Grafana; statically validated |
| tools (eqtool) | Map signing keys, form key, relay token, connector registration, map signing |

## Entries

### 2026-11-18 — Add tunnel-relay and connector

- **Branch:** `feat/tunnel`
- **What:**
  - Contract: `schema/tunnel/tunnel.md` (`equalizer-tunnel/1`) replaces
    the gRPC draft `tunnel.proto`. The connector opens HTTPS on 443, the
    connection is upgraded and carries yamux streams with plain HTTP/1.1;
    the gateway → relay API on the control network is in the same document.
    Why not gRPC: its modules cannot be downloaded in the development
    environment, it would add a large dependency tree, and an upgraded
    HTTPS connection goes through Caddy on 443 with an automatic
    certificate (no extra port).
  - `tunnel-relay/` (new Go module): `connectors.json` registry (SHA-256
    of tokens, reloaded every 15 s and on SIGHUP; removed or rotated
    connectors lose their tunnels), upgrade and yamux server, session pool
    (max 8 per connector, round robin among sessions that declared the
    destination), gateway forwarding with a bearer token, errors in
    `X-Eq-Error`, admin `/healthz`, `/readyz`, `/v1/tunnels`, JSON logs,
    `connector.down` and `request.blocked` (`tunnel_unauthorized`) events.
  - `connector/` (new Go module): local allowlist `EQ_ALLOW` checked on the
    request and again in the dialer, executor without redirect following
    (30 s timeout), TLS options for legacy apps with self-signed
    certificates, reconnection with exponential backoff and jitter,
    `HTTPS_PROXY` support, Docker healthcheck while the tunnel is open.
  - Gateway: `transport.Relay` (destination checked against
    `legacy.reach.allow`, request id propagated, relay/connector errors
    mapped), `EQ_RELAY_URL` and `EQ_RELAY_TOKEN[_FILE]`, `site.BuildWith`
    with installation options.
  - `eqtool secret` (gateway ↔ relay token) and `eqtool connector`
    (registers a connector, prints its token once, `-rotate`).
  - Compose: `tunnel-relay` service (1 replica), Caddy site on
    `EQ_RELAY_DOMAIN` publishing only `/v1/tunnel`; port 8443 removed.
  - CI: Go matrix with the two new modules and their images;
    `publish.yml` pushes images to GHCR on every merge to `main`.
  - Docs: INSTALL (relay domain, connector registration and `docker run`),
    architecture, tenant manual, READMEs, CLAUDE.md decision.
- **Why:** the connector is what makes an already running vulnerable server
  protectable without exposing it: the legacy network needs only outbound
  HTTPS.
- **Status:** tests green with the race detector in all four modules. End
  to end with the real binaries (fake ISO-8859-1 legacy app, relay,
  connector, gateway in connector mode): 502 before the connector starts;
  then pages served, links rewritten, the form bound and submitted (the
  legacy app received `permitId=7&text=Caff%E8+ok`), 303 on success;
  relay restart → connector reconnects within seconds; a wrong connector
  token is refused; all 42 log and event lines validate against the
  schemas. **Not tested yet:** Caddy in front of the relay (upgrade through
  the real edge proxy) and the GHCR publication: both need the VPS and the
  first merge. Not done: mTLS enrolment from the panel, panel-signed
  allowlist, several relay replicas (tunnels are in memory), Prometheus
  metrics.
- **Next steps:**
  1. Test on the VPS: `docker compose up`, connector on the remote
     vulnerable server, map in connector mode (INSTALL steps 5–7).
  2. Pass-through login and sessions in Redis.
  3. File uploads with AV/CDR scanning; per-form rate limits.
  4. Scripts by hash with CSP, URLs inside CSS, `external_links`.
  5. Prometheus metrics (gateway, relay); panel-api.

### 2026-11-18 — Serve POST forms

- **Branch:** `feat/forms`
- **What:**
  - Contract: `hidden` placeholder namespace (legacy hidden inputs kept
    server-side) documented in the map schema; forms section in
    `docs/map-format.md`.
  - `internal/charset`: `FromUTF8` re-encodes values into ISO-8859-1,
    ISO-8859-15 or Windows-1252; unrepresentable characters are an error.
  - `internal/formtoken`: form state sealed with AES-256-GCM (form id,
    action parameters, kept hidden values, page charset, expiry, hash of the
    browser binding cookie). Stateless: works across replicas with a shared
    key.
  - `internal/formmap`: recognises a legacy `<form>` as a declared form by
    inverting its `legacy_request` (path, query, hidden inputs used as
    `{{params.x}}`), computes the public action and the input renames.
  - `internal/pipeline/forms`: new step before `rewrite`. Removes every
    hidden input of POST forms, sets the public action, renames inputs,
    drops `formaction`/`accept-charset`, adds the `_eq_token` input.
    Pipeline helpers `BaseURL`, `Attr`, `SetAttr`, `DelAttr`, `MarkFinal`.
  - `internal/legacyclient`: POST with an urlencoded body rebuilt from
    `legacy_request.form`.
  - `internal/site`: forms checked at load time (types, templates,
    `on_success.redirect`); forms with file fields marked unsupported (503).
  - `internal/server/form.go`: role, cross-site check (`Origin`,
    `Sec-Fetch-Site`), urlencoded only (415 otherwise), 1 MiB limit (413),
    token checks (missing, invalid, expired, other browser, other
    form/record → 403), field validation, re-encoding, success check
    (`status`, `body_contains`, `not_contains`, `redirect_to`), 303 to
    `on_success.redirect` or the legacy page rendered faithfully.
    `__Host-eq_form` cookie (Secure, HttpOnly, SameSite=Lax) set only on
    pages that contain a bound form.
  - Configuration: `EQ_FORM_KEY` / `EQ_FORM_KEY_FILE`; without it a random
    key and a warning (single replica only). `eqtool formkey`; Compose
    mounts `secrets/form.key`; INSTALL updated.
- **Why:** writes are the riskiest part of a legacy app. The browser never
  sees the legacy hidden fields, cannot replay a form from another browser
  or for another record, and every value reaches the legacy app typed and in
  its own charset.
- **Status:** tests green with the race detector, including an end-to-end
  test with a fake ISO-8859-1 legacy app: the page shows no internal hidden
  value, the rebuilt POST carries `permitId=42`, `text=Caf%E9 note`, the
  kept hidden value and the fixed value, and nothing else; ten rejection
  cases never reach the legacy app. Not done: file uploads (multipart), the
  `cookie` success check, per-form rate limits and CAPTCHA, re-encoding of
  GET query values for non-UTF-8 legacy apps, JavaScript that reads hidden
  inputs or input names (it would see the public names).
  **Requires on the VPS:** `eqtool formkey -out secrets/form.key` before
  `docker compose up`, otherwise the gateway container fails to mount it.
- **Next steps:**
  1. `tunnel-relay` and `connector`.
  2. Pass-through login and sessions in Redis (then the `cookie` check and
     per-user legacy accounts).
  3. File uploads with AV/CDR scanning; per-form rate limits.
  4. Scripts by hash with CSP, URLs inside CSS, `external_links`.
  5. Gateway Prometheus metrics; visual comparison with Playwright.

### 2026-11-18 — Translate the whole repository to English

- **Branch:** `chore/english`
- **What:**
  - Go code (gateway, eqtool): every comment, log message, error, HTTP error
    page and test translated; test data uses English entities (`guest`,
    `citizen`, `permit_id`, `/permits/{id}`). No logic changes.
  - Contracts: descriptions in the map, event and log schemas, OpenAPI
    summaries and descriptions, tunnel proto comments, metrics catalogue.
  - Example map renamed `portale-pratiche.yaml` → `permits-portal.yaml`
    and rewritten with English ids (`example-city`, `permits_list`,
    `permit_detail`, `permit_supplement`, ...).
  - Manuals renamed `amministratore.md` → `admin.md` and `cliente.md` →
    `tenant.md`; README, CLAUDE.md, INSTALL.md, architecture, map format,
    every service README, Compose files, `.env.example`, Caddy, Vector,
    Loki, CI workflows and Python scripts translated. This changelog's
    earlier entries translated as well.
  - `CLAUDE.md`: the language rule is now "everything in English",
    including commit messages; note on the vendored HTML parser and on
    opening PRs through the REST API. Stale lines fixed (INSTALL status,
    the duplicate "rendering pipeline: to do" row in the gateway README).
- **Why:** the project is international; Italian in comments and docs is a
  barrier for contributors.
- **Status:** `go vet` and `go test -race` green for gateway and tools;
  contracts validate; Compose validates. Identifiers were already English,
  so behaviour is unchanged.
- **Next steps:**
  1. Form POST: typed fields, CSRF token, legacy hidden fields held
     server-side, re-encoding into the legacy charset.
  2. `tunnel-relay` and `connector`.
  3. Pass-through login and sessions in Redis.
  4. Scripts by hash with CSP, URLs inside CSS, `external_links`.
  5. Gateway Prometheus metrics; visual comparison with Playwright.

### 2026-11-18 — Add faithful rendering and assets

- **Branch:** `feat/faithful-rendering`
- **What:**
  - Contract: `assets` section in the map (prefix, allowed types, maximum
    size, roles); prefixes containing `.` or `..` rejected; example updated.
  - `gateway/internal/thirdparty/xhtml`: copy of the HTML5 parser
    `golang.org/x/net/html` v0.40.0 (BSD licence), because the Go module
    proxy is not reachable from every development environment.
  - `internal/charset`: detection (header, `<meta>`, content) and
    conversion to UTF-8 of ISO-8859-1, ISO-8859-15, Windows-1252; other
    encodings rejected.
  - `internal/legacyurl`: translation of legacy URLs → public URLs from
    `links`, `compat` and the `legacy_request` of GET routes; assets keep
    their path; relative fallback without the internal host.
  - `internal/pipeline`: `Step` interface, `Context`, statistics; `clean`
    step (revealing comments and meta tags) and `rewrite` step (href, src,
    action, formaction, srcset, `<base>`, ...); charset declarations
    aligned to UTF-8.
  - `internal/server`: pipeline applied to HTML pages (fail closed on
    error), legacy redirects translated to public URLs (external ones
    blocked), assets checked on name, declared type and real type,
    sandboxed SVG, asset query never sent to the legacy application.
  - Router: asset prefixes (longest wins), after routes.
  - Documentation: gateway README, map format (links and assets).
- **Why:** the "100% identical interface" requirement: without assets the
  page loses its style; without rewriting, links lead to legacy URLs.
- **Status:** tests green with the race detector. Checked on sample pages:
  links, resources, forms and srcset rewritten, internal IP absent, comments
  and `generator` meta removed, ISO-8859-1 with entities converted without
  losing accents; assets with PHP content disguised as PNG or with a type not
  allowed are rejected; `../` and `//` in names rejected. Not done yet:
  scripts allowed by hash with CSP, URL rewriting inside CSS and `style`
  attributes, `external_links`, automatic visual comparison.
  **Note for forms:** pages are now output in UTF-8, so browsers will submit
  forms in UTF-8; the gateway must re-encode values into the legacy
  application's original charset before rebuilding POSTs.
- **Next steps:**
  1. Form POST: typed fields, CSRF token, legacy hidden fields held
     server-side, re-encoding into the legacy charset.
  2. `tunnel-relay` and `connector`.
  3. Pass-through login and sessions in Redis.
  4. Scripts by hash with CSP, URLs inside CSS, `external_links`.
  5. Gateway Prometheus metrics; visual comparison with Playwright.

### 2026-11-18 — Rebuild GET requests to the legacy application

- **Branch:** `feat/legacy-reconstruction`
- **What:**
  - Contract: `roles.anonymous` in the map schema (role of unauthenticated
    visitors) and in the example.
  - `gateway/internal/mapmodel`: map types moved to their own package and
    extended (types, defaults, field specs, `legacy_request`).
  - `internal/validate`: `int`, `enum`, `bool`, `date`, `string`, `email`,
    `text` types with constraints and the `no_html`, `no_sql_meta`,
    `no_template_meta` rules; undeclared query parameters dropped, repeated
    ones rejected; patterns and rules checked when the map loads.
  - `internal/placeholder`: `{{params.x}}`, `{{query.x}}`, ... placeholders.
  - `internal/legacyclient`: request built from scratch (path with encoded
    values, declared query, fixed headers), redirects not followed, response
    capped at 16 MiB, legacy headers filtered.
  - `internal/transport`: direct mode with a host:port allowlist in the
    dialer (also a defence against redirects and DNS).
  - `internal/budget`: concurrent requests to the legacy application, per
    replica.
  - `internal/site`: compiles a map version (router, validator, client,
    budget) and rejects inconsistent maps.
  - `internal/server/route.go`: anonymous role (403 on restricted routes),
    validation, rebuilding, generic pages for legacy 4xx/5xx (never stack
    traces), security headers, `request.served` events with split latency,
    `field.rejected` without the value, `legacy.error`.
  - Documentation: gateway README, map format (rules and anonymous role),
    INSTALL.
- **Why:** the first step in which the gateway really serves a site: the
  legacy application receives only rebuilt requests and validated values.
- **Status:** tests green with the race detector. End-to-end test with a fake
  legacy app: `/` and `/permits/7` served (the legacy app received only
  `GET /index.jsp` and `GET /detail.jsp?id=7`), `/permits/7abc` → 400,
  `/index.jsp` and `/../../etc/passwd` → 404. Known limits: legacy HTML
  passes through without faithful rendering (internal links still point to
  legacy URLs); legacy redirects become 502; forms, APIs, login and sessions
  are not served; `mode: connector` answers 502; the budget is not shared
  between replicas; map types are still written by hand.
- **Next steps:**
  1. Faithful rendering pipeline: HTML5 parser, cleanup, link rewriting
     (`links`, `compat`), redirect translation.
  2. Form POST with typed fields and CSRF token.
  3. `tunnel-relay` and `connector`.
  4. Pass-through login and sessions in Redis.
  5. Gateway Prometheus metrics.

### 2026-11-18 — Add base Compose, logging and gateway skeleton

- **Branch:** `feat/compose-gateway-skeleton`
- **What:**
  - `gateway/`: Go service (standard library only) with loading and
    verification of the Ed25519-signed map, hot reload (file and SIGHUP),
    allowlist router (404 unmapped, 405 method not allowed, OPTIONS and HEAD
    handled by the gateway, TRACE/CONNECT rejected), fail closed without a
    map (503), JSON logs conforming to the schema, `request_id` generated by
    the gateway and returned in `X-Eq-Request-Id`, `request.blocked` events,
    client IP only from trusted proxies, `/healthz` and `/readyz` on a
    separate port, `healthcheck` subcommand, distroless non-root Dockerfile,
    tests.
  - `tools/`: `eqtool keygen` and `eqtool sign` (map signing until
    panel-api exists), Dockerfile.
  - `scripts/map_to_json.py`: validates a YAML map and converts it to JSON.
  - `deploy/compose/`: `edge`, `control`, `lab` (internal) networks, Caddy
    with automatic TLS and dynamic balancing of replicas, gateway with 2
    replicas, Vector → Loki, Grafana on localhost only, `tools` profile,
    `.env.example`.
  - `SignedMap` contract (OpenAPI): the map travels as a base64 `payload`
    signed byte by byte instead of as a JSON object.
  - CI `go.yml`: gofmt, vet, tests with the race detector, Compose
    validation, image builds.
  - `INSTALL.md`: steps 4–6 made concrete (configuration, key and
    signing, start, logs in Grafana). `CLAUDE.md`: notes on the development
    environment.
- **Why:** to have from day one the basis for debugging (structured,
  searchable logs) and a gateway that already applies deny by default.
- **Status:** Go tests green; local end-to-end test with the signed example:
  `/` and `/permits/42` → 503 (mapped, no rebuilding yet), `/wp-admin` → 404,
  wrong method → 405, TRACE → 405, `/readyz` ready; the 12 log/event lines
  produced validate against the schemas. Compose is validated with
  `docker compose config` but **has not been started** (no Docker daemon in
  the development session): Caddy, Vector, Loki and Grafana must be tested on
  the VPS. Map types in the gateway are written by hand (a subset) because
  generation tools cannot be downloaded here: to be replaced.
- **Next steps:**
  1. Test the Compose stack on the VPS and fix it (especially the Vector
     configuration).
  2. Validation of parameter and field types in the gateway.
  3. `legacyclient` and `transport` (direct mode), then rebuilding of GET
     requests.
  4. Faithful rendering pipeline with an HTML5 parser (`golang.org/x/net/html`).
  5. `tunnel-relay` and `connector` on `tunnel.proto`.
  6. Gateway Prometheus metrics and type generation from the schemas.

### 2026-11-18 — Initialise repository, contracts and documentation

- **Branch:** `feat/bootstrap`
- **What:**
  - Monorepo layout with one folder per service and a README in each.
  - `schema/map/gateway-map.schema.json`: JSON Schema of the map (version
    `gateway-map/1.0`) with a validated example in `schema/map/examples/`.
  - `schema/api/panel-api.yaml`: OpenAPI 3.1 for panel-api (first version,
    brief bodies).
  - `schema/events/`: schemas of security events and log lines.
  - `schema/tunnel/tunnel.proto`: gRPC protocol between connector and relay.
  - `schema/metrics/catalog.md`: Prometheus metrics catalogue.
  - `scripts/validate_schemas.py` and the `contracts.yml` CI workflow.
  - `README.md`, `CLAUDE.md`, `INSTALL.md`, `docs/architecture.md`,
    `docs/map-format.md`, administrator and tenant manuals.
- **Why:** to fix the contracts between services before writing code, so
  services can be developed separately without refactoring; to make the
  project resumable by anyone.
- **Status:** contracts validate locally (schemas, example, OpenAPI, proto).
  The schema rejects the main bad cases: `text`/`file` fields without rules,
  non-`GET` routes, `template` rendering without extraction, connector
  without an id. In the map the authorisation key is called `target` (not
  `on`, which YAML reads as a boolean). No service is implemented yet:
  `INSTALL.md` and the manuals describe the planned behaviour.
- **Next steps:**
  1. `deploy/compose/`: base Compose with `edge`, `control`, `lab` networks
     and a log pipeline (Vector, Loki, Grafana).
  2. `gateway/`: HTTP server, loading and verification of the signed map,
     allowlist router, structured logs with `request_id`.
  3. `gateway/internal/pipeline/`: faithful rendering (parsing, cleanup, URL
     rewriting, forms) with tests on sample pages.
  4. `tunnel-relay/` and `connector/` on the `tunnel.proto` protocol.
  5. Type generation for Go from the schemas (`schema/` → `gen/`).
