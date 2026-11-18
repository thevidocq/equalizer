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
| gateway | GET routes and assets in direct mode, validation, faithful rendering (UTF-8, cleanup, URL rewriting, redirects); no forms, sessions or connector yet |
| tunnel-relay | Not implemented yet |
| connector | Not implemented yet |
| orchestrator | Not implemented yet |
| panel-api | Not implemented yet |
| panel-ui | Not implemented yet |
| agent | Not implemented yet |
| deploy/compose | Caddy, gateway (2 replicas), Vector, Loki, Grafana; statically validated |
| tools (eqtool) | Keys and map signing |

## Entries

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
