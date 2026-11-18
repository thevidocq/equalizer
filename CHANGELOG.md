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
| gateway | JSON APIs, many sites by host, virtual hosts (`reach.address`), GET routes, assets and urlencoded POST forms in direct or connector mode, validation, faithful rendering (UTF-8, cleanup, form binding, URL rewriting, redirects), pass-through login with sessions in Redis, rate limits and behaviour rules, file uploads (real type, size, image rebuild; no virus scan) |
| tunnel-relay | Token-authenticated tunnels, forwarding with tenant check, admin endpoints; several replicas with handover |
| connector | Outbound tunnel, local allowlist, reconnection; one per server, virtual hosts |
| orchestrator | Not implemented yet |
| panel-api | Discovery jobs and continuous discovery, tenant AI settings, map templates per CMS, auth with mandatory TOTP, tenants, email invitations, tenant connectors, map reach checks, exposure check, Logs page backend (Loki), status page, platform/tenant privacy, users, sites, connectors, maps with approval and signing, audit, internal endpoints |
| panel-ui | Discovery tab and tenant AI settings, login, joining by invitation, platform tenant overview, sites with isolation, tenant connectors, people with invitations, maps with editor, approval, import and export, activity, logs, status |
| agent | Discovery: read-only crawl through the connector, fixed exclusions, CMS fingerprint, deterministic routes and types, optional AI review with the tenant's model, proposed drafts |
| deploy/compose | Caddy (on-demand certificates for protected sites, relay and panel domains), SMTP settings, gateway (2 replicas), Redis, PostgreSQL, panel-api, panel-ui, tunnel-relay, Vector, Loki, Grafana; statically validated |
| tools (eqtool) | Map signing keys, form key, relay token, connector registration, map signing |

## Entries

### 2026-11-18 — Status page

- **Branch:** `feat/status-page` (on top of `feat/map-import-export`)
- **What:**
  - Contract: `TenantStatus`, `SiteStatus`, `PlatformStatus`, `HealthState`
    for `/tenants/{tenant}/status` and `/status/platform` (were open objects).
  - panel-api `internal/health`: each site's map (published version),
    connector (tunnels on every relay replica through DNS), legacy
    application (served and errors in the last 15 minutes from Loki),
    isolation (exposure check), overall state = the worst; problems per day
    over 7 days (legacy errors, connector drops). Platform: database,
    gateway and relay replicas ready, Loki. Tenant people see their sites;
    platform users only the platform page. `EQ_RELAY_ADMIN`,
    `EQ_GATEWAY_ADMIN` (set in Compose).
  - panel-ui `features/status`: Status (tenant) and Platform status pages,
    refreshed every 30 seconds; states as a word and a dot.
- **Why:** to tell at once whether a site is down because of Equalizer, the
  connector or the legacy server.
- **Status:** tests pass (evaluation, probes over replicas, counts from
  Loki, permissions). Not yet run against the Compose stack.
- **Next steps:** docs and screenshots; SIEM forwarding.

### 2026-11-18 — Map import and export

- **Branch:** `feat/map-import-export` (on top of `feat/discovery-issues`)
- **What:** panel-ui: **Download as YAML** on every version (without the
  meta the panel owns); **Import a map file** on New version loads a YAML or
  JSON file (at most 2 MB, must have `legacy` and `roles`) into the editor.
  No new endpoint: the import is saved as a draft through the usual
  creation, so schema, reach and connector checks and approval apply.
- **Why:** backups, moving a map from a test site to the real one, reusing
  a map on a similar site, keeping maps in git.
- **Status:** tests pass (YAML and JSON read, meta dropped, non-maps and
  large files refused).
- **Next steps:** status page; docs and screenshots.

### 2026-11-18 — Discovery proposes IDOR protection for JSON APIs

- **Branch:** `feat/discovery-issues` (on top of `feat/logs-page`)
- **What:** agent `idor.py`: when a GET list API and a GET detail API one
  segment below it share an id of the same type, the draft gets
  `response.issues` on the list and `owned_by_user` on the detail for the
  roles without a rule; a rule a person chose is never replaced. The report
  lists them ("Protection against IDOR proposed"); the review aid flags
  `issues_added` as usual.
- **Why:** `issues` had to be written by hand.
- **Status:** tests pass (pair found, types must match, human rule kept,
  idempotent). The crawler does not run JavaScript, so APIs are taken from
  the map (template, manual edits, published map), not found by crawling.
- **Next steps:** map import/export; status page; docs and screenshots.

### 2026-11-18 — Logs page

- **Branch:** `feat/logs-page` (on top of `feat/authz-json`)
- **What:**
  - Contract: `GET /tenants/{tenant}/logs`, `/logs/stats`, `/logs/export`
    (`LogEvent`, `LogStats`).
  - panel-api: `internal/logsearch` reads security events from Loki; the
    tenant always comes from the user, filters are validated (site, types,
    period) and quoted (search) before they enter LogQL; only the shown
    fields leave, never field values. Stats: events per type and refused
    per interval. CSV export up to 10000 events, formulas neutralised,
    recorded in the activity. Admins, approvers and auditors; platform users
    see no tenant's events. `EQ_LOKI_URL` (set in Compose).
  - panel-ui: `features/logs` as in the approved mockup: filters, chips with
    counts, refused chart, event table, detail in plain words with "Add to
    the map" (unmapped paths), "Show this IP only", "Copy request ID", live
    refresh, CSV. Opens on refused events (served is off by default).
- **Why:** the project owner asked for a Logs page in the panel.
- **Status:** tests pass (LogQL quoting against injection, tenant filter,
  paging, stats; handler permissions and CSV); checked with screenshots on a
  fake Loki. Not yet on the real Loki of the Compose stack.
- **Next steps:**
  1. Run it against the Compose stack's Loki.
  2. SIEM forwarding configured from the panel.
  3. Lab campaign (DVWA, WordPress, Juice Shop).

### 2026-11-18 — owned_by_user for JSON APIs

- **Branch:** `feat/authz-json`
- **What:**
  - Map: `response.issues` on API routes (response field → `entry.ns.name`).
    Every value of that field in the answer is issued to the session for the
    entry's `owned_by_user` target, as links in pages already are.
  - Gateway: loading checks the field is declared and the entry has that
    rule; `jsonshape.Values` reads the values from the rebuilt answer,
    issued before the session is written.
  - Review aid: `issues_added` (medium) when an answer grants more objects.
  - Docs: map format, gateway README.
- **Why:** single-page apps load their lists as JSON; without this,
  `owned_by_user` on their APIs refused the user's own objects too, so
  those sites stayed without IDOR protection.
- **Status:** tests pass (list then detail: own ids 200, others 403; bad
  `issues` refused at load; review finding). Not yet on a real SPA.
- **Next steps:**
  1. Logs page in the panel (mockup shown to the project owner).
  2. Lab campaign (DVWA, WordPress, Juice Shop).
  3. Discovery: propose `issues` when a list API and a detail API share an id.

### 2026-11-18 — Uploads without ClamAV; uploads and relay replicas reach main

- **Branch:** `feat/uploads-no-av`
- **What:**
  - Brings the work of #19 (uploads, relay replicas) to `main`: that pull
    request was merged into `feat/authz-rollback` instead of `main`.
  - Removes ClamAV: no `clamav` service, no `EQ_CLAMD_ADDR`, no scanner in
    the gateway. `scan` in the map is now `cdr` (rebuild images) or `none`;
    file fields need `mime` and `max_size_kb`. Forms with files no longer
    answer 503 when nothing else is installed.
- **Why:** the project owner prefers to leave virus scanning to the security
  stack around the legacy application, and ClamAV needed about 3 GB of
  memory.
- **Status:** gateway tests pass (type, size, image rebuild, undeclared
  file, multipart towards the legacy app). Replicas not yet run in Docker.
- **Next steps:**
  1. Lab campaign (DVWA, WordPress, Juice Shop) with discovery, review aid,
     authorisation and uploads.
  2. Advisories feed; issue values from API responses for `owned_by_user`.
  3. SIEM forwarding, status page, import/export.

### 2026-11-18 — File uploads, relay replicas

- **Branch:** `feat/uploads-relay` (on top of `feat/authz-rollback`)
- **What:**
  - gateway: `internal/upload` (real type sniffed from the bytes, OOXML and
    ODF recognised, size bound, ClamAV INSTREAM client failing closed,
    images rebuilt from their pixels with `av+cdr`, random names); forms
    with files read as multipart (declared file fields only, one file each)
    and rebuilt as multipart towards the legacy application; map loading
    checks `mime`, `max_size_kb` and what `av+cdr` can rebuild; without
    `EQ_CLAMD_ADDR` such forms answer 503.
  - tunnel-relay: replicas; a request for a connector whose tunnel is on
    another replica is handed over to the peers found through DNS
    (`EQ_PEERS`), never more than once; the peer checks everything again.
  - Compose: relay replicas (`EQ_RELAY_REPLICAS`, default 2), Caddy spreads
    tunnels over them; optional `clamav` service (profile `uploads`).
  - Docs: map format (uploads), relay README (replicas), INSTALL.
- **Why:** the last two limits of the prototype: legacy forms with
  attachments, and a relay that was a single point of failure.
- **Status:** tests pass (type, size, infected, scanner down, polyglot
  image cleaned, undeclared file; handover between replicas, tenant check
  after handover). Not run with a real ClamAV or with replicas in Docker.
- **Next steps:**
  1. Lab campaign (DVWA, WordPress, Juice Shop) with discovery, review aid,
     authorisation and uploads.
  2. Advisories feed; issue values from API responses for `owned_by_user`.
  3. SIEM forwarding, status page, import/export.

### 2026-11-18 — Authorisation rules against IDOR, map rollback

- **Branch:** `feat/authz-rollback` (on top of `feat/review-assistant`)
- **What:**
  - Contracts: `authz` on API routes, the meaning of `owned_by_user`;
    `rollback` map action.
  - gateway: rules per role on routes and APIs. `owned_by_user` lets a
    session open only the values the legacy site showed it (rewritten links
    and redirects issue them; stored in the session, bounded, cleared when
    the legacy session ends); `never` refuses; checked after validation,
    before the legacy request; refusals are `authz_not_owned` /
    `authz_never` events. Map loading checks roles and targets.
  - panel-api: `rollback` publishes again a retired signed version (approvers
    and administrators), audited.
  - panel-ui: "Roll back to this version" on retired versions.
  - Docs: map format (authorisation, rollback), tenant manual, gateway README.
- **Why:** an allowlist does not stop a user changing an id in the address
  (IDOR); and a broken version must be undone in seconds.
- **Status:** tests pass (issue and check, redirects, another session,
  invalid targets; rollback flow). Values shown only inside JSON API
  responses are not issued yet (only links and redirects).
- **Next steps:** issue values from API responses; uploads; relay replicas.

### 2026-11-18 — Review aid for approvers (human in the loop)

- **Branch:** `feat/review-assistant`
- **What:**
  - Contract: `GET/POST /apps/{app}/maps/{version}/review`, agent
    `/internal/reviews/claim` and `/{id}/result`, `MapReview`.
  - panel-api: `internal/review` compares a version with the published map
    and flags risks with fixed rules (type widened, pattern, rules or deny
    removed, authz removed, opened to anonymous, sensitive or personal
    paths, direct mode, reach or allow changed, rate limits raised, assets
    served as is, new APIs, removals); migration `0007` for the AI part;
    an AI review is queued when a version is sent for review.
  - agent: review jobs, a plain summary and questions from the tenant's
    model, given the changes and findings only.
  - panel-ui: "Review aid" on each version (risks, AI summary and
    questions, changes, "Ask the AI again").
- **Why:** the approver is the last barrier: they must see at once what a
  change opens and what to check, whoever (person or agent) wrote it.
- **Status:** tests pass (rules, API flow, agent); checked in a browser.
  Not run with a real AI provider.
- **Next steps:** authorisation rules (`owned_by_user`) and rollback;
  uploads; relay replicas.

### 2026-11-18 — Discovery agent, continuous discovery, tenant AI settings

- **Branch:** `feat/discovery-agent`
- **What:**
  - Contracts: `/tenants/{tenant}/llm` (tenant only: the platform has no AI
    settings), `/apps/{app}/discoveries`, agent endpoints
    `/internal/discoveries/claim|{id}/progress|{id}/result`, gateway
    `/internal/unmapped`, site `discovery_schedule`; old planned
    proposals/crawls endpoints removed.
  - agent (new, Python standard library): crawler (read-only, rate limited,
    per-shape cap, second pass with a test account), fixed exclusions
    (admin, tooling, backups, actions), fingerprint (CMS, version,
    components, advisories file), clustering into typed routes, forms and
    assets, map built on the published map or the CMS template, report,
    AI review that can only rename, exclude or narrow (Anthropic, OpenAI,
    OpenAI-compatible servers; endpoint must be public). Tests with a fake
    WordPress site; Dockerfile; CI workflow; image published.
  - panel-api: migration `0006`; discoveries with secrets sealed and the
    test password erased at the end; proposals saved as drafts by the
    requester (no draft when nothing changed); scheduler for continuous
    discovery; unmapped paths from the gateway.
  - gateway: `internal/unmapped`, refused paths counted per site and sent
    every minute (path only, clients counted not sent).
  - panel-ui: Site → Discovery (start, AI on by default with a notice when
    none is configured, test account, continuous discovery, history,
    report, link to the draft); tenant Settings with the AI model.
  - Compose: `agent` service. Docs: agent README, READMEs, architecture,
    tenant manual.
- **Why:** knowing what to cover is the core of the product; discovery
  gives a working first map from day one and keeps it current.
- **Status:** tests pass in every module; checked end to end locally
  (panel-api + agent + fake site + UI): discovery done, draft proposed,
  report shown. Not run against a real CMS or a real AI provider; the
  advisories file is empty until a feed is connected.
- **Next steps:**
  1. Lab: discovery on DVWA and a real WordPress through a connector.
  2. Advisories feed (WPScan, Joomla, Drupal, NVD) into `advisories.json`.
  3. AI review assistant for approvers (explains a map diff).
  4. Map authorisation rules (`owned_by_user`) and map rollback.

### 2026-11-18 — Serve JSON APIs, start maps from CMS templates

- **Branch:** `feat/json-api` (on top of `feat/shared-connectors`)
- **What:**
  - Contracts: map `deny` on types and fields; panel `GET /map-templates`,
    `POST /apps/{app}/map-template`, site `kind`.
  - gateway: `internal/jsonshape` (client JSON bodies checked key by key,
    unknown keys and nested values refused; legacy JSON rebuilt with only
    the declared fields and types); `serveAPI` with roles, limits,
    same-origin check on writes, JSON errors that never show the legacy
    body; legacy client with PUT, PATCH, DELETE and typed JSON bodies;
    validator `deny` lists.
  - panel-api: migration `0005` (site kind); `internal/templates`
    (WordPress, Joomla, Drupal, Moodle, generic), each tested against the
    map schema; template endpoints.
  - panel-ui: "Built with" when protecting a site; "Start from a template"
    in New version (template, connector, address, protocol).
  - Docs: map format (JSON APIs, templates, deny), READMEs, tenant manual.
- **Why:** modern sites and the dynamic parts of old ones load data with
  JavaScript; templates make the first map of a common CMS quick, with the
  admin areas closed from the start.
- **Status:** tests pass (gateway API end to end with a fake legacy JSON
  service, shaper, templates valid and compiled); template flow checked in a
  browser. Not tested on a real CMS.
- **Next steps:**
  1. Lab: DVWA and a real WordPress behind the gateway, then Juice Shop.
  2. Map authorisation rules (`owned_by_user`) and map rollback.
  3. Limit platform logins to the VPN; per-site fail-closed rate limits.

### 2026-11-18 — One connector per server, many sites per gateway, isolation check

- **Branch:** `feat/shared-connectors`
- **What:**
  - Contracts: map `meta.public_domain` and `legacy.reach.address`; panel
    connectors at `/tenants/{tenant}/connectors` (with `used_by`), site
    `PATCH` (`legacy_ip`) and `/exposure/check`, `Exposure` on sites,
    `exposed` in the platform overview, `/internal/maps`; tunnel headers
    `X-Eq-Tenant` (gateway → relay) and `X-Eq-Host` (to the connector).
  - Security fixes: a map can no longer use another tenant's connector
    (panel at draft and publish, relay at every request:
    `connector_not_owned`); `direct` mode only towards
    `EQ_DIRECT_DESTINATIONS`, so no map can reach Redis, PostgreSQL or the
    panel from the gateway.
  - panel-api: migration `0004` (connectors per tenant, exposure columns);
    `internal/exposure` (probe of the old IP on 443/80 with SNI and Host,
    compared with a random name; public unicast IPs only; hourly runner;
    status changes in logs and tenant activity); connector removal refused
    while a published map uses it.
  - gateway: all published maps, chosen by host (unknown host 404), a
    refused map keeps only its own site's previous version; `/tls/allowed`
    for Caddy; transports connect to `reach.address` with the site's name
    in Host/SNI; the tenant goes to the relay.
  - tunnel-relay: sessions carry the tenant, checked per request;
    `X-Eq-Host` passed on. connector: dials only the checked destination,
    presents `X-Eq-Host` as Host and SNI.
  - eqtool: connectors without app. Compose/Caddy: on-demand TLS for
    published domains, `EQ_GATEWAY_DOMAIN` and `EQ_APP` removed,
    `EQ_MAP_URL` defaults to every published map, new
    `EQ_DIRECT_DESTINATIONS` and `EQ_EXPOSURE_INTERVAL`.
  - panel-ui: tenant Connectors page; Site → Isolation (status, check now,
    old IP, configuration for Apache, nginx, IIS and the map lines); red
    status for exposed sites.
  - Docs: map format, tunnel spec, INSTALL (no gateway domain any more:
    each site keeps its own), architecture, READMEs, manual.
- **Why:** many sites on one server must not need many containers; a
  protected site must really stop being public while its neighbours stay
  online; and two isolation holes found while reading the code.
- **Status:** unit and integration tests pass in every module; UI checked
  in a browser; Caddyfile validated with Caddy 2.8. Not tested end to end
  with a real web server, real DNS and Let's Encrypt.
- **Next steps:**
  1. Lab campaign: Juice Shop behind the gateway (owner's tests below).
  2. Map authorisation rules (`owned_by_user`) and map rollback.
  3. Limit platform logins to the VPN; per-site fail-closed rate limits.

### 2026-11-18 — Separate the platform from tenants, invite people by email

- **Branch:** `feat/tenant-privacy`
- **What:**
  - Contract: `/tenants/{tenant}/invitations` (list, invite, revoke),
    public `/invitations/{token}` and `/invitations/{token}/accept`,
    `/platform/tenants` (overview) and `/platform/tenants/{tenant}/invitation`
    (invite the first administrator again); `POST /tenants` now takes the
    first administrator's name and email; `POST /tenants/{tenant}/users`
    removed; `/audit` lost its `tenant` parameter.
  - panel-api: migration `0003_invitations.sql`; `internal/mail` (SMTP with
    STARTTLS or implicit TLS, `off`, `log` for development); invitation
    tokens of 32 random bytes stored as SHA-256, single use, 48 hours,
    revocable, sent only by email; accepting creates the user and logs in.
    Platform users hold no tenant role any more: every tenant endpoint
    answers them 403/404; they see tenant names, whether anyone joined, and
    per site only name and status. Audit split: platform actions vs tenant
    actions. Re-inviting the first administrator works only while nobody
    has joined. Tests rewritten around invitations, plus privacy checks.
  - panel-ui: `/invite/:token` page; platform tenants overview with status
    dots, "Add a customer" (with the administrator's email) and "Invite
    again"; tenant picker removed (platform and tenant panels never mix);
    people page invites and lists pending invitations with revoke; role
    helpers mirror the backend; activity sentences for the new actions.
  - Compose and Caddy: `EQ_PANEL_DOMAIN` publishes the panel to customers
    (`/v1/internal` answers 404 there and in panel-ui's Caddy); SMTP
    settings and `secrets/smtp.password`. Docs: INSTALL, READMEs,
    architecture ("Platform and tenants"), both manuals.
- **Why:** customers own their tenant; the platform operator must see only
  high-level status, never configuration, maps or people.
- **Status:** `go test -race` for panel-api, UI build, tests and boundary
  check pass; flow checked in a browser (create customer, join, invite,
  revoke, reused link refused, platform reading a site gets 404). Not
  tested against a real SMTP server.
- **Next steps:**
  1. Lab campaign: Juice Shop behind the gateway, measure compatibility,
     false blocks and bypasses.
  2. Limit platform logins to the VPN / localhost address.
  3. Optional support access granted by the tenant (time-limited, audited).
  4. Per-site fail-closed option for rate limits when Redis is down.
  5. Map authorisation rules (`owned_by_user`) and map rollback.

### 2026-11-18 — Add two-step login, password change and user deactivation

- **Branch:** `feat/mfa`
- **What:**
  - Contract: `/auth/mfa/setup`, `/auth/mfa/enable`, `/auth/mfa/verify`,
    `/auth/password`, `DELETE /tenants/{tenant}/users/{id}`,
    `/tenants/{tenant}/users/{id}/reset-mfa`; `mfa_enabled` and
    `mfa_verified` on users; the two-step rule in the description.
  - panel-api: migration `0002_mfa.sql`; `internal/mfa` (TOTP RFC 6238
    checked against the RFC vectors, ±1 step, replay refused; ten
    single-use recovery codes stored as SHA-256; secrets encrypted with
    `EQ_MFA_KEY`, user id as additional data); sessions carry `mfa_ok`, the
    API answers 403 `mfa_required` outside `/auth` until the second step,
    and the session id changes when it succeeds; throttle of wrong codes;
    password change ends the other sessions; tenant administrators
    deactivate people and reset authenticators (sessions end); audit of
    every step. MFA mandatory unless `EQ_MFA_REQUIRED=false`.
  - panel-ui: login steps (code from the app or a recovery code), first
    login with QR code (drawn in the browser) and manual key, recovery codes
    with copy and download, account page with password change, people page
    with authenticator status, reset and deactivate; activity sentences for
    the new actions.
  - Compose: `secrets/mfa.key` for panel-api. INSTALL updated.
- **Why:** the panel signs what the gateway lets through: a stolen password
  alone must not be enough to publish a map.
- **Status:** Go tests with the race detector green (TOTP vectors,
  replay, recovery code single use, throttle, password change logging out
  other browsers, deactivation, reset) against a real PostgreSQL. Driven in
  Chrome: first login forced to the authenticator setup, code accepted,
  recovery codes shown, panel opened; second login refuses a wrong code and
  accepts the next one; people and account pages. Not done: recovery code
  regeneration, WebAuthn keys, platform users management from the panel,
  forced password change on first login.
- **Next steps:**
  1. Test the whole stack on the VPS with the remote vulnerable server.
  2. Map diff between versions; form helpers for common map edits.
  3. Service status page; log search.
  4. Gateway serving several sites by host name.
  5. File uploads with AV/CDR scanning.

### 2026-11-18 — Add the web panel (panel-ui)

- **Branch:** `feat/panel-ui`
- **What:**
  - Contract: typed responses for tenants, users, apps, connectors, map
    versions and audit entries (the UI client is generated from them);
    `document` and `detail` declared as free objects.
  - `panel-ui/` (TypeScript, React 19, Vite, React Router, TanStack Query,
    Radix, Tailwind 4): login; tenants; protected sites with sidebar and
    status; site tabs Map (version list with the approval rail, version
    page with document and next action, new version from scratch or from a
    version with a YAML editor that checks syntax and shows the panel's
    format errors) and Connectors (add with the `docker run` command and the
    token shown once, rotate, remove); people with roles; activity in plain
    sentences. Generated client (`openapi-typescript` + `openapi-fetch`)
    with the CSRF header; error messages that say what to do.
  - Rules enforced in CI (`ui.yml`): generated client equal to the
    contract, module boundaries, files ≤ 200 lines, type check, build,
    tests.
  - Docker image on Caddy (static files, `/v1` to panel-api on the same
    origin, CSP); Compose `panel-ui` on `127.0.0.1:8088` (panel-api no
    longer published). panel-api can serve the build with `EQ_UI_DIR`
    (SPA fallback, cache for hashed assets); its CSP now allows inline
    style attributes.
  - INSTALL: first site through the web panel.
- **Why:** the panel API alone still meant `curl` on the VPS; now the whole
  first-site flow is done from the browser.
- **Status:** type check, unit tests and boundary check green. Driven in a
  real browser (Chrome via Playwright) against panel-api and PostgreSQL:
  login, protect a site, add a connector (command shown), editor writes
  version 2 from version 1 and sends it for review, administrator approves
  and publishes it (version 1 retired), activity page, no console errors.
  Not done: MFA, password change and user deactivation, map diff between
  versions, service status, logs, SIEM, LLM settings, import/export,
  dark mode.
- **Next steps:**
  1. Test the whole stack on the VPS with the remote vulnerable server.
  2. MFA (TOTP) and password change for panel users.
  3. Map diff between versions and form-based helpers for common map
     edits.
  4. Service status page; log search.
  5. Gateway serving several sites by host name.

### 2026-11-18 — Add panel-api: tenants, sites, connectors, maps

- **Branch:** `feat/panel-api`
- **What:**
  - Contract: OpenAPI updated with auth, tenants, users, apps, connectors
    (create, rotate, delete), map lifecycle (draft, submit, approve, reject,
    publish), internal endpoints for gateway and relay, error body; the
    `X-Eq-Panel` CSRF header. Planned sections kept.
  - `panel-api/` (new Go module): PostgreSQL (`lib/pq`), embedded
    migrations with an advisory lock; PBKDF2 passwords, sessions stored as
    SHA-256, login throttle, roles (platform admin/operator; tenant admin,
    editor, approver, auditor) with tenant isolation; maps validated against
    the embedded schema copy, panel-owned `meta`, Ed25519 signing at
    publication, one published version per site and approver ≠ author
    enforced by the database; audit log of every change;
    `create-admin` subcommand; health endpoints; Dockerfile.
  - Gateway: map source abstraction — file (as before) or panel
    (`EQ_MAP_URL`, `EQ_MAP_TOKEN[_FILE]`, ETag polling every 15 s, retry
    after a rejected map).
  - tunnel-relay: connectors from the panel (`EQ_CONNECTORS_URL`,
    `EQ_PANEL_TOKEN[_FILE]`), changes detected by digest; file kept as
    fallback.
  - Compose: `postgres` (16, volume, health check) and `panel-api`
    (`127.0.0.1:8088`); gateway reads the map of `EQ_APP` from the panel;
    relay reads connectors from the panel. **Fix:** services that read
    `secrets/` now run as `EQ_UID:EQ_GID` (the files are `0600`; as
    distroless `nonroot` the gateway could not read `form.key`).
  - CI: panel-api in the Go matrix (tests use the runner's PostgreSQL) and
    in image build/publish. `scripts/validate_schemas.py` checks embedded
    schema copies.
  - Docs: INSTALL (secrets, first administrator, panel workflow with
    `curl`), panel-api README, architecture.
- **Why:** without a panel every change meant files and commands on the
  VPS. Now sites, connectors and maps live in one place, with four-eyes
  approval and an audit trail.
- **Status:** tests green with the race detector, including the full API
  flow against a real PostgreSQL (isolation, roles, CSRF, author cannot
  approve, signature verified with the public key, relay view of
  connectors, audit) and login throttling. End to end with real binaries
  and PostgreSQL: admin created, tenant, site, user, connector, map drafted
  by an editor, approved and published by the admin; the gateway downloaded
  the signed map from the panel and served the legacy app through relay and
  connector, whose token came from the panel. **No web interface yet.** Not
  done: MFA, user deactivation and password change, change classes
  (automatic approval of narrowing changes), map diff, per-request
  semantic checks of maps (only the schema), gateway serving several sites.
- **Next steps:**
  1. panel-ui: login, sidebar with sites, site page (connectors, map
     versions with editor, approve and publish), users, audit.
  2. MFA (TOTP) for panel users.
  3. Service status page and log search.
  4. Test the whole stack on the VPS.
  5. Gateway serving several sites by host name.

### 2026-11-18 — Add rate limits and behaviour rules

- **Branch:** `feat/rate-limits`
- **What:**
  - `internal/limits`: fixed-window counters (Redis Lua `INCR`+`PEXPIRE`,
    sets for distinct values, marks with TTL; in-memory store as fallback).
    Rate limits from `defaults.rate_limit`, `routes[].rate_limit` and
    `forms[].rate_limit` (`per_ip`, `per_session`, `per_user`). Behaviour
    rules `distinct_param_values`, `failed_logins`, `request_rate` with
    actions `throttle` (`challenge` treated as throttle), `block`
    (site-wide), `alert`; `missing_predecessor` accepted and reported as not
    applied. `defaults.legacy_budget.max_rps` shared across replicas.
  - Gateway: checks before serving (assets only see blocks), 429 with
    `Retry-After` or 403, events `ratelimit.hit`, `request.blocked`
    (`behavior_block`), `behavior.alert`; enumeration observed on validated
    route values, failed logins observed on `session.start` forms; fail open
    when the store is unreachable (error logged at most every 10 s). Notes
    about unapplied map features logged at map load. The Redis client is
    shared with the session store.
  - Example map: `login_bruteforce` rule. Docs: map format, gateway README.
- **Why:** after login, brute force and record enumeration (IDOR) are the
  most common attacks on legacy portals; the map already described these
  limits but the gateway ignored them.
- **Status:** tests green with the race detector, including the limits
  engine against a real `redis-server` and HTTP tests: page limit with
  `Retry-After`, assets not counted, enumeration throttled at the third
  distinct id without reaching the legacy app, brute force blocked
  site-wide after three failed logins (even with the right password), no
  password in events. Not done: CAPTCHA challenge, `missing_predecessor`,
  sliding windows, per-tenant limits from the panel.
- **Next steps:**
  1. Test on the VPS with the remote vulnerable server (tunnel, login,
     limits).
  2. File uploads with AV/CDR scanning.
  3. Scripts by hash with CSP, URLs inside CSS, `external_links`.
  4. Prometheus metrics (gateway, relay).
  5. panel-api: tenants, maps, approval and signing.

### 2026-11-18 — Add pass-through login and sessions

- **Branch:** `feat/sessions`
- **What:**
  - Contract: `password` base type (requires rules, like `text`);
    `defaults.session` (`idle_minutes`, `absolute_hours`); `session` on
    forms (`start: <role>` or `end: true`) and routes (`end: true`); example
    map with login and logout, `roles.auth.provider: passthrough`.
  - `internal/session`: sessions keyed by the SHA-256 of a random id,
    content encrypted with AES-256-GCM (key derived from `EQ_FORM_KEY`, id
    as additional data), absolute and idle limits; Redis store (go-redis)
    and in-memory store.
  - `internal/legacyclient`: legacy `Set-Cookie` captured (still never sent
    to the client).
  - `internal/server`: session loaded per request, role from the session,
    legacy cookies added to rebuilt requests (routes, forms, assets),
    session created when the legacy app sets a cookie, login with a new id,
    `success.cookie` check, legacy expiry (`legacy.session_expired`) drops
    the role, logout, `Cache-Control: private, no-store` on session pages,
    `__Host-eq_session` cookie written once before the headers; session
    hash in events (`session`). Store down: visitors are anonymous, logins
    503.
  - Configuration `EQ_REDIS_URL`; Compose `redis` service (control network,
    no persistence, 256 MB, volatile-lru).
  - Docs: map format (login and sessions), gateway README, INSTALL.
- **Why:** most legacy apps worth protecting have a login. Their session
  cookies are a prime target (fixation, theft, weak flags): now the browser
  never sees them, and the gateway decides who may reach restricted pages.
- **Status:** tests green with the race detector, including a real
  `redis-server` test and an in-process login flow (pre-login legacy
  cookie, failed and successful login, new id, old id rejected, legacy
  expiry, logout). End to end with real binaries: two gateway replicas on
  one Redis, login on replica A, private page served by replica B; the
  stored blob does not contain the legacy cookie; Redis down → private page
  403, public page 200. Not done: cookie `Path`/`Domain`, OIDC, per-user
  legacy accounts, session listing/revocation from the panel, login rate
  limits (`rate_limit` is still ignored).
- **Next steps:**
  1. Test on the VPS with the remote vulnerable server (tunnel + login).
  2. Rate limits in Redis (routes, forms, login), then behavioural rules.
  3. File uploads with AV/CDR scanning.
  4. Scripts by hash with CSP, URLs inside CSS, `external_links`.
  5. Prometheus metrics; panel-api.

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
