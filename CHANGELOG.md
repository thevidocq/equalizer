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

Latest release: **0.1.0** (`docs/releases/0.1.0.md`), the first stable
prototype.

| Area | Status |
| --- | --- |
| Contracts (`schema/`) | First version, validated in CI |
| Documentation | Architecture, map format, installation, manuals of what exists, screenshots (`docs/screenshots`) |
| gateway | Areas (whole site served, admin included, every request rebuilt), OIDC login (off by default), map check for the panel, JSON APIs, many sites by host, virtual hosts (`reach.address`), GET routes, assets and urlencoded POST forms in direct or connector mode, validation, faithful rendering (UTF-8, cleanup, form binding, URL rewriting, redirects), pass-through login with sessions in Redis (remember me, public legacy cookies in the browser), real client IP to the legacy app, app headers, SSE, streamed uploads up to 1 GB, single-byte charsets, rate limits and behaviour rules, file uploads (real type, size, image rebuild; no virus scan) |
| tunnel-relay | Token-authenticated tunnels, forwarding with tenant check, admin endpoints; several replicas with handover |
| connector | Outbound tunnel, local allowlist, reconnection; one per server, virtual hosts |
| orchestrator | Not implemented yet |
| panel-api | Discovery jobs and continuous discovery, tenant AI settings, map templates per CMS, auth with mandatory TOTP, tenants, email invitations, tenant connectors, map reach checks, exposure check, Logs page backend (Loki), status page, platform/tenant privacy, users, sites, connectors, maps with approval and signing, audit, internal endpoints |
| panel-ui | Logo (sidebar, login), discovery tab and tenant AI settings, login, joining by invitation, platform tenant overview, sites with isolation and certificate, tenant connectors, people with invitations, maps with editor, approval, import and export, activity, logs, status |
| agent | Discovery: read-only crawl through the connector, whole-site area, IDOR proposals, CMS fingerprint, deterministic routes and types, optional AI review with the tenant's model, proposed drafts |
| deploy/compose | Caddy (protected sites use the certificate uploaded in the panel, Let's Encrypt only for relay and panel), SMTP settings, gateway (2 replicas), Redis, PostgreSQL, panel-api, panel-ui, tunnel-relay, Vector, Loki, Grafana; statically validated |
| tools (eqtool) | Map signing keys, form key, relay token, connector registration, map signing |

## Entries

### 2026-11-20 — One profile per web application, ten applications

- **Branch:** feat/agent-webapps (on top of feat/panel-verification)
- **What:**
  - agent: `equalizer_agent/webapps/`, one profile per application
    (WordPress, Joomla, Drupal, Magento / Adobe Commerce, PrestaShop,
    OpenCart, TYPO3, Moodle, MediaWiki, phpBB) and `generic.py` for custom
    sites; `base.py` holds the profile's shape and the rules every site
    shares (action links, one-time tokens now also `form_key` and `hash`,
    sensitive files). `rules.py`, `fingerprint.py` and the crawler only ask
    the recognised profile. An administration opens to a test account only
    when its profile is proven read-only in the lab (WordPress closed,
    Joomla and Drupal open; the others and custom sites closed until
    tested). A recognised site applies its own administration rules and
    the usual ones; an unrecognised one all of them; every profile's
    "never" rules apply to every site.
  - `docs/compatibility.md`, generated from the profiles
    (`python3 -m equalizer_agent.webapps.matrix`); an agent test fails
    when it is stale.
  - panel-api: map templates for magento, prestashop, opencart, typo3,
    mediawiki and phpbb (generic pages and folders, each application's
    names kept out of slugs); site kinds extended (contract `SiteKind`,
    migration 0015 widens the database check); templates listed in
    alphabetical order whatever the case.
  - panel-ui: the new kinds when protecting a site; SSO path presets for
    Magento, OpenCart, TYPO3 and phpBB.
  - Docs: agent.md (profiles), architecture, README, tenant manual (the
    kinds and the matrix), agent README.
- **Why:** each application's specifics in one place, and coverage of the
  ten most used self-hosted web applications (Shopify, Wix, Squarespace and
  Webflow are hosted services: nothing of theirs to protect).
- **Status:** agent, panel-api and panel-ui tests; lab 01 + 06 give the
  same studies and verifications as before on WordPress, Joomla and Drupal;
  on the custom intranet the study now leaves its administration closed
  (generic profile: 4 pages instead of 5). The six new profiles are written
  from each application's documentation and behaviour, not proven in the
  lab yet.
- **Next steps:**
  1. Lab: add Moodle and PrestaShop (then the others) and prove their
     profiles.
  2. Agent: correct its own draft; verify pages behind a login.

### 2026-11-20 — Show verifications in the panel

- **Branch:** feat/panel-verification (on top of feat/agent-checks)
- **What:**
  - panel-ui: `VerificationPanel` on every version (score: pages
    identical, inputs refusing every attack, share served only by the
    area; pages broken by the version; still open with reasons; **Verify**
    for editors, approvers and administrators; polls while it runs) and in
    the discovery report for the proposed draft; still-open reasons shared
    (`maps/components/stillOpen.ts`).
  - agent: a still-open input's detail names, in words, the other attack
    groups it lets through.
  - Docs: tenant manual "Verification" with a screenshot; agent.md step 5
    done.
- **Why:** step 5 of docs/agent.md: the person who approves sees the
  measured result.
- **Status:** typecheck, boundaries, vitest, build, API types; agent tests.
- **Next steps:**
  1. Agent: one profile per web application in `agent/equalizer_agent/webapps/`
     (WordPress, Joomla, Drupal, Magento, PrestaShop, OpenCart, TYPO3,
     Moodle, MediaWiki, phpBB, generic), map templates and site kinds for
     the new ones, a compatibility matrix generated from the profiles.
  2. Agent: correct its own draft; verify pages behind a login.

### 2026-11-20 — Verify every draft on the gateway

- **Branch:** feat/agent-checks
- **What:**
  - panel-api: `map_verifications` (migration 0014); `GET/POST
    /apps/{app}/maps/{version}/verify` (editors, approvers, administrators;
    one waiting verification per version); the internal queue hands the
    agent the version signed for tries (`signing.SignTry`, one hour), its
    legacy site and the pages the latest discovery studied; every draft a
    discovery proposes is queued for verification; a version read alone
    carries its latest verification.
  - agent: `verify.py` (step 4: each page directly and through the gateway
    in preview, compared by status or redirect target, visible text in each
    answer's charset, links and forms; step 5: every route parameter and
    form field gets the attack set in dry-run, after a valid submission),
    `attacks.py`, `gateway.py`; the discovery report lists the pages it
    studied; `EQ_GATEWAY_ADMIN_URL`.
  - gateway fix found by the first verification: an http site's form token
    was checked against the https binding cookie, so mapped form
    submissions were refused (`openToken` now uses the site's scheme;
    regression test).
  - compose: agent reaches `gateway:9090`; lab: `EQ_GATEWAY_ADMIN_URL`;
    test_06 waits for each draft's verification and checks the databases
    are still unchanged after it.
  - Docs: agent.md (what a verification measures; step 4 done, correction
    loop next).
- **Why:** steps 4 and 5 of docs/agent.md: a draft arrives measured.
- **Status:** unit tests (agent compare and attacks, panel-api queue and
  signature, gateway regression) and lab 01 + 06 pass. Lab scores: pages
  identical 12/12, 5/5, 4/4, 4/4; inputs refusing every attack 5/13,
  26/38, 1/19, 1/3: most of what passes falls out of a route into the site
  area.
- **Next steps:**
  1. Panel: score, broken pages, still open, Verify button.
  2. Agent: correct its own draft (narrow, widen, verify again).
  3. Verify pages behind a login (keep test accounts until the
     verification ends).

### 2026-11-20 — Study pages in a real browser behind the read-only guard

- **Branch:** feat/agent-browser (on top of chore/land-agent-study)
- **What:**
  - agent: `browser.py` opens each pass's pages (up to 40) in headless
    Chromium (Playwright). Every request is intercepted and answered by the
    Fetcher (connector, read-only guard, the pass's session) only for GET
    or HEAD on the site that the crawler's rules allow; the rest is refused
    and reported ("kept in"); the browser's network points at a closed
    port. Script requests become pages to study; JSON answers stay with the
    area and are listed as still open. Files a page loads count as assets
    and never as action links. `EQ_BROWSER` (auto, on, off),
    `EQ_BROWSER_PATH`.
  - image: Playwright and Chromium's headless shell in the agent image;
    compose sets `EQ_BROWSER` (`EQ_DISCOVERY_BROWSER`), `HOME` and
    `shm_size`.
  - lab: the agent runs with the browser; test_06 checks it opened pages
    on each site, databases still unchanged.
  - Docs: agent.md (3c done), agent README, .env.example.
- **Why:** step 3c of docs/agent.md: requests made by scripts are part of
  what a site does.
- **Status:** unit tests (script GET studied, POST and action link never
  sent, JSON listed as still open) and lab 01 + 06 pass: browser opened
  10, 14, 38 and 9 pages on the four sites; their classic pages make no
  extra same-site script requests, so the browser adds little there; it
  matters for script-heavy applications.
- **Next steps:**
  1. Step 4: the two checks (with `/maps/try`) and the score; panel-api
     verification queue.
  2. Step 5: panel score, still open, Verify button.

### 2026-11-20 — Study administrations read-only, proven in the lab

- **Branch:** feat/agent-admin-readonly (on top of feat/agent-accounts-forms)
- **What:**
  - agent: one read-only guard in `fetch.py` (GET and HEAD only, plus the
    test accounts' login POST); test accounts' passes now open the
    administration (Drupal from `/admin`, Joomla after its own
    administrator login), never what `rules.NEVER` lists (scheduled jobs,
    installers, updates, exports, "add new" screens) and never action
    links (verbs as whole words, `task=`, `layout=edit`, one-time tokens).
    WordPress's administration stays closed (`rules.WRITES_ON_VIEW`).
    A base route covers a path only when the values fit its types (deny
    lists included), and one route per shape (the gateway refuses two that
    differ only by parameter names).
  - lab: the agent runs in the lab (`up.sh`, `down.sh`); new
    `test_06_study.py` studies the four sites with an administrator account
    and checks that each site's content (WordPress, Joomla on PostgreSQL,
    Drupal, intranet) is the same before and after, and that Drupal's and
    Joomla's administrations became precise entries.
  - Docs: agent.md (read-only proof, WordPress finding, rollout 3b done),
    tenant manual.
- **Why:** step 3b of docs/agent.md. The lab found that WordPress writes
  to its database when its administration is only viewed (Quick Draft,
  global styles, privacy texts); the rule now keeps it closed.
- **Status:** agent unit tests and lab tests 01 + 06 pass (four studies,
  databases unchanged). Full lab run: see the PR.
- **Next steps:**
  1. 3c: browser (Playwright) behind the same guard, script requests,
     declared APIs.
  2. Step 4: the two checks and the score.

### 2026-11-20 — Study test accounts and form fields

- **Branch:** feat/agent-accounts-forms (on top of feat/maps-try)
- **What:**
  - panel-api: up to three test accounts per discovery (`accounts`, with a
    label; `account` counts as the first), sealed together in the new
    `discoveries.accounts_secret` (migration 0013), erased at the end like
    before; the job hands them to the agent (and the first as `account` for
    older agents).
  - agent: one pass per account, each with a new session; the report says
    how each logged in (`accounts`). `fields.py` types each form field from
    its HTML: choices (select, radio, checkbox), number ranges, email, date,
    phone, URL, colour, time, HTML patterns kept only when RE2 reads them the
    same (no lookaround, repeats ≤ 1000). Free text refuses HTML and
    template code; names and codes also refuse SQL delimiters. Query strings
    seen in links get `no_html` and `no_template_meta`. A new `open` list in
    the report says what is still open and why (not opened, action links,
    legacy errors, fields that must accept SQL characters).
  - panel-ui: up to three test accounts on the Discovery form; report shows
    the accounts and the still-open list.
  - Design correction (docs/agent.md, contract): accounts do not become map
    roles. Equalizer never decides access; the legacy site does (project
    decision). The accounts only let the agent reach pages behind a login.
  - Docs: tenant manual, agent README, agent.md rollout (3a done, 3b next).
- **Why:** step 3 of docs/agent.md, first half.
- **Status:** agent, panel-api and panel-ui tests pass; a generated map
  loads in the gateway (checked by hand with the gateway's own Build). The
  administration is still never opened: that needs step 3b's lab study
  showing nothing changes on the four lab sites.
- **Next steps:**
  1. 3b: browser and read-only proxy, administration read-only, declared
     APIs, discovery run in the lab against the four sites (database
     unchanged before and after).
  2. Step 4: the two checks and the score.

### 2026-11-19 — Let the agent try a draft map on the gateway

- **Branch:** feat/maps-try (on top of docs/agent-study)
- **What:**
  - gateway: `POST /maps/try` on the admin endpoint (`server/try.go`,
    `server/trysite.go`), behind the panel's service token (`EQ_MAP_TOKEN`;
    without it tries answer 404). The draft must be signed for tries
    (`mapstore/try.go`: purpose `try`, expiry signed, at most two hours;
    the signed message has its own prefix, so it never verifies as a
    published map). `dry_run` runs the visitor's code path with a transport
    that reads the rebuilt request and answers 204: nothing is forwarded;
    the verdict says the entry, whether only the area serves it, and the
    refusal (status, reason, field, never the value). `preview` serves
    GET/HEAD through the connector and returns the rebuilt answer. Tries
    use `limits.None` (no counters), keep their events out of the logs,
    and dry-runs read sessions through `session.ReadOnly`. A test account's
    legacy cookies open a try session with its role. Compiled drafts are
    cached by payload (16).
  - `site.Options.Transport` replaces the way to the legacy site.
  - panel-api: `signing.SignTry` (same message as the gateway, checked by
    a shared test vector).
  - Contract: `gateway-admin.yaml` gets 400, `cookies` and the session
    semantics. Docs: agent.md (step 2 done), gateway README.
- **Why:** step 2 of docs/agent.md: the agent's two checks need to ask the
  gateway what it would do with a draft, without touching the legacy site.
- **Status:** unit tests (refused callers and maps, attacks refused with
  no legacy hit, rebuilt preview, seeded session). Not used by the agent
  yet; panel-api does not hand out try maps yet.
- **Next steps:**
  1. Agent: browser and read-only proxy, accounts, forms, declared APIs,
     roles and ownership, tighter map.
  2. Agent: the two checks and the score; panel-api verification queue
     (hands the agent a try map with `SignTry`).
  3. Panel: score, still open, Verify button.
  4. Lab: studies of the four lab sites.

### 2026-11-19 — Design the agent's study of legacy sites

- **Branch:** docs/agent-study (on top of feat/site-certificates)
- **What:**
  - `docs/agent.md`: the agent as Equalizer's strong point. A study with a
    real browser, read-only (administration included, test accounts with
    few permissions per CMS), a precise map entry for every function
    understood, and two checks of its own map: pages compared with and
    without Equalizer, attack set tried on every parameter and field on the
    gateway only. Measured score, "still open" list, Verify button, rollout
    in six steps. AI unchanged (structure only); page contents for the AI
    planned but not decided.
  - Contracts: `panel-api.yaml` gets test `accounts` (up to three),
    `StudyReport`, `Verification`, `GET/POST /apps/{app}/maps/{version}/verify`,
    `MapVersion.verification`, internal verification queue;
    new `schema/api/gateway-admin.yaml` documents the gateway's admin
    endpoint and adds `POST /maps/try` (dry-run and preview of a draft
    signed for that purpose, service token).
  - UI types regenerated; architecture and agent README point to the design.
- **Why:** a map is only as good as it is tight and correct; writing one by
  hand does not scale. The agent must write it and prove it.
- **Status:** design and contracts only; nothing implemented yet. The new
  endpoints are not served yet.
- **Next steps:**
  1. Gateway: `POST /maps/try`; panel-api: signing for purpose try.
  2. Agent: browser and read-only proxy, accounts, forms, declared APIs,
     roles and ownership, tighter map.
  3. Agent: the two checks and the score; panel-api verification queue.
  4. Panel: score, still open, Verify button.
  5. Lab: studies of the four lab sites with expected scores.
  6. Separately: remove the map's `risk` field (a label with no effect).

### 2026-11-19 — Serve protected sites with their own certificates

- **Branch:** feat/site-certificates
- **What:**
  - Contracts: map `meta.public_scheme` (`https` default, `http`); panel API
    PUT/DELETE `/apps/{app}/certificate`, `public_scheme` on PATCH `/apps/{app}`,
    `App.tls` (`SiteTLS`), `/internal/certificates`, `certificate_expires` in
    the site status.
  - panel-api: `certs` package (PEM or PKCS#12, one matching key, RSA ≥2048
    or ECDSA P-256/P-384, chain to system roots or an uploaded private CA,
    validity, coverage of the domain and aliases); vendored SSLMate
    go-pkcs12 using stdlib pbkdf2; migration 0012; the key is sealed per
    site; every tenant admin gets an email on upload and removal; health
    check "Certificate" and reminder emails at 30/14/7/1 days; publish and
    alias re-sign carry `public_scheme`.
  - gateway: `certfeed` pulls the certificates every 10 s (keeps the last
    good set on errors); admin `/tls/certificate` gives Caddy the bundle;
    `/tls/allowed` always 404, so Caddy never asks Let's Encrypt for a
    protected site; plain listener `EQ_PUBLIC_HTTP_ADDR` (`:8081`) serves
    http sites and redirects https ones (301/308); http sites use
    `eq_session`/`eq_form` cookies without Secure and refuse OIDC.
  - deploy: Caddyfile `get_certificate http` for sites, http catch-all to
    gateway 8081; compose sets `EQ_PUBLIC_HTTP_ADDR`.
  - panel-ui: Site → Certificate page (status, upload of .pfx or PEM,
    removal, http/https switch); publish warns when an https site has no
    certificate.
  - Lab: front mimics Caddy's certificate fetch and port 80 routing; tests
    upload real lab-CA certificates, compare fingerprints, check redirects,
    refuse another site's certificate; the intranet runs in http.
  - Docs: tenant manual section and screenshot, admin manual rows,
    architecture, map format, lab README.
- **Why:** visitors must keep seeing the legacy site's real certificate;
  Equalizer never issues new or temporary ones for protected sites. Let's
  Encrypt stays only for the panel and relay domains.
- **Status:** unit tests, lab tests and full lab pass. Deploy note: Caddy must
  reach the gateway on port 8081 (set in compose).
- **Next steps:**
  1. Manual test on the server with a real certificate (.pfx from IIS and PEM).
  2. Optional: panel behind Cloudflare.
  3. SIEM/S3 log forwarding, external pentest, Moodle in the lab.

### 2026-11-19 — Harden the relay, the panel and the agent's model calls

- **Branch:** fix/relay-panel-hardening
- **What:**
  - relay `tunnel`: 20 wrong tokens a minute per address, then refused
    without a check and reported once (`failures.go`); the connector's
    address is the last `X-Forwarded-For` value; streams opened by the
    connector are closed (`AcceptBacklog` 4); no deadline left on a
    hijacked tunnel; idle and admin timeouts.
  - panel `clientIP`: `X-Forwarded-For` behind `EQ_TRUSTED_PROXIES` (new
    setting; compose and lab set it); panel-ui's Caddy trusts the edge.
  - panel login: 5 failures per address and account, 20 per address, 50
    per account (`Throttle.Exceeds`); a successful login clears only its
    own pair; server read, write and idle timeouts.
  - agent `llm._post`: no redirects followed; the connected address must be
    public (checked after connecting, against DNS rebinding).
  - lab `test_03c_relay_panel.py`; docs: architecture "Exposed services
    under attack", admin manual, lab README.
- **Why:** behind Caddy every panel visitor had the proxy's address: five
  wrong passwords from anyone locked every user out, and the audit showed
  the proxy. The relay let anyone guess tokens without limit and logged
  the address the client wrote. A tenant's model server could redirect the
  agent into the platform's networks. Two of the new lab tests fail on the
  previous code and pass now.
- **Status:** unit tests pass (Go with -race, agent); the clean lab passes
  61 of 61.
- **Next steps:**
  1. Manual test on a server by the project owner, then tag v0.1.0.
  2. External penetration test before real portals.
  3. Optional customer certificate per site (upload, encrypted, expiry
     alert), if wanted.

### 2026-11-19 — Harden the public gateway against hostile clients

- **Branch:** fix/gateway-hardening
- **What:**
  - `server/guard.go`: panics answer 500 and are logged; request bodies
    keep a minimum pace; answers the client stops reading are given up;
    `IdleTimeout` on the public server, full timeouts on the admin one.
  - `server/oidc.go`: `safeReturn` refuses control characters and `\`,
    which browsers drop or read as `/` (defence in depth: its callers
    already normalise the address).
  - `server/area.go`, `asset.go`: `..;`, `..\`, `.. ` and `...` refused,
    as Tomcat and Windows servers resolve them.
  - `rebuild`: at most 10,000 fields in a query or form and parts in a
    multipart form; in-memory bodies down to 16 MB with a shared 256 MB
    budget (`server/membudget.go`); precise forms capped too.
  - `server/sessionlimit.go`: a new anonymous session lives 5 minutes
    until its visitor comes back; at most 600 a minute per address after a
    burst of 300 (60 was too few for a NAT: the lab, all on one address,
    hit it).
  - `clientip`: every `X-Forwarded-For` line read from the right, mapped
    IPv4 normalised; rate limits count IPv6 per /64.
  - area: the legacy origin in `Link` headers becomes the public one.
  - lab `test_03b_hostile.py`: paths that climb, request smuggling, huge
    headers, other hosts, too many fields, forged addresses, session
    flood, a body sent at a drip; `down.sh` stops lab processes started by
    any path, `up.sh` refuses to start on busy ports; login helpers say
    what the site answered when a login does not go through.
  - docs: architecture, "The public gateway under attack".
- **Why:** the gateway faces the Internet. A review found a slow-body hold,
  memory exhaustion by a form of millions of fields, a session-store
  flood, path variants that climb on Tomcat and IIS, a client address
  spoofable with two forwarding lines and IPv6 rate limits per address. Four of the new lab
  tests fail on the previous gateway and pass now.
- **Status:** unit tests pass (also with -race); the clean lab passes 57
  of 57.
- **Next steps:**
  1. Manual test on a server by the project owner, then tag v0.1.0.
  2. External penetration test before real portals.
  3. Panel and relay: the same review.

### 2026-11-19 — Lab with Joomla, Drupal and a custom app; fixes found

- **Branch:** feat/lab-cms-sites
- **What:**
  - lab: Joomla 5.4 (PostgreSQL), Drupal 11.4 (SQLite) and Acme Intranet,
    an old-style PHP app (`apps/intranet`: sessions, ISO-8859-1 pages,
    attachments), next to WordPress; built from GitHub only (`cms.sh`,
    `joomla-setup.php`, `drupal-setup.php`); published from their
    templates in the onboarding test; `test_02c_sites.py` reads pages,
    searches, logs in, writes articles in Joomla's and Drupal's editors,
    attaches files; the pixel comparison covers about 40 pages of the four
    sites.
  - gateway `legacyclient`: a relative `Location` is kept as written and
    resolved against the legacy page; it was resolved against the relay,
    so redirects pointed browsers to the relay's internal address.
  - gateway `rebuild/files.go`: files in area forms and raw uploads are
    refused (415) when the server could run them: server extensions (also
    inside the name), `.htaccess` and the like, programs, PHP code anywhere,
    pages sent as files, pictures or PDFs that are not.
  - gateway `pipeline/fragment.go`: HTML fragments (AJAX) are rendered as
    fragments, table rows in their context; JSON or text sent as text/html
    is passed on as it is.
  - gateway: a query the route does not declare sends the request to the
    area instead of being dropped; entries with `authz` are never bypassed,
    by query or by path parameters.
  - templates: favicons as `image/vnd.microsoft.icon`; Drupal's
    `/sites/default/files/` served by the area (generated CSS, JS and image
    styles need their query).
  - docs: map format, lab README.
- **Why:** test more kinds of legacy sites before the manual test. The lab
  found an internal address leaked by relative redirects, a PHP script
  uploaded through an ordinary form and run by the legacy server, broken
  AJAX answers in Joomla's admin, Joomla's search served as its home page,
  and Drupal's styles missing.
- **Status:** unit tests pass; the clean lab passes 48 of 48 (tests 3–5
  in the full run before the last fidelity tweak, 1–2c after it).
- **Next steps:**
  1. Manual test on a server by the project owner, then tag v0.1.0.
  2. Moodle in the lab (its template is not tested yet).
  3. Log forwarding to S3/block storage, then SIEM.

### 2026-11-19 — Keep wp-admin faithful: OPTIONS, legacy address in forms

- **Branch:** fix/lab-fidelity
- **What:**
  - gateway: `OPTIONS` goes to the legacy application when the area lists
    it (`server/options.go`); CORS preflights stay answered by the gateway.
  - gateway `internal/origin`: single-pass scanner that replaces whole host
    names only and is exact across stream writes; `NewReverse` turns the
    public origin in area queries and bodies back into the legacy one.
  - lab: `fidelity.py` and `test_02b_fidelity.py` compare 12 public and
    admin pages directly and through Equalizer (pixels, text, failed
    resources); new tests save Settings → General and publish from the
    block editor; comments are unique per run.
  - docs: map format and lab README.
- **Why:** the comparison found that the block editor's `OPTIONS` calls
  were answered by the gateway with an empty 204, and that forms showed
  the public https address, so saving WordPress's settings would have
  changed the site's own address.
- **Status:** unit tests pass; the clean lab passes 35 of 35 (worst pixel
  difference 0.06%, the clock on Settings → General).
- **Next steps:**
  1. Manual test on a server by the project owner, then tag v0.1.0.
  2. Log forwarding to S3/block storage, then SIEM.

### 2026-11-19 — Add local end-to-end lab and fix what it found

- **Branch:** feat/local-lab
- **What:**
  - `lab/e2e/`: a whole platform on one machine without Docker (real
    services, real WordPress on SQLite, HTTPS front with a lab CA, OIDC
    provider, SMTP sink), `up.sh`/`down.sh`, and 32 pytest + Playwright
    tests from onboarding to SSO, attacks, password reset and alerts;
    README; `go vet` in CI.
  - gateway `internal/origin`: the legacy origin (both schemes, also
    JSON-escaped) becomes `https://` + public domain in inline scripts,
    JSON, JavaScript, CSS and feeds (areas, routes, APIs, assets), streamed
    where the body is streamed.
  - gateway: path parameters are match constraints (`pathfit.go`); URLs are
    rewritten to a route only when captured values fit its types; a
    trailing `/` is kept.
  - gateway: raw uploads reach the legacy site with
    `Content-Disposition: attachment` and a cleaned file name.
  - templates: slugs refuse dots (no `/wp-config.php` matched as a page);
    the WordPress comment form has its optional fields and hidden inputs.
  - docs: map format (origin rewriting, path parameters, raw uploads).
- **Why:** test real customer flows before the manual test on a server.
  The lab found mixed content in wp-admin (WordPress on http behind
  https), files matched as page slugs, broken comment posting and raw
  uploads without a file name.
- **Status:** all 32 lab tests pass; unit tests pass. Not covered by the
  lab: real TLS/ACME, DNS, Compose images, real SMTP, Cloudflare, Loki,
  backups, replicas.
- **Next steps:**
  1. Manual test on a server by the project owner, then tag v0.1.0.
  2. Log forwarding to S3/block storage, then SIEM.
  3. Capacity settings from the panel.

### 2026-11-19 — Account card at the bottom of the sidebar

- **Branch:** fix/sidebar-account
- **What:** panel-ui `app/SidebarFooter.tsx`: initials, name and role
  (opens the account) with Help and Log out as small icon buttons on the
  same line, instead of the bare email next to the icons. Guides and
  screenshots updated.
- **Why:** owner's request: the icons did not sit well with the email.
- **Status:** checked in the browser for customers and platform users.
- **Next steps:** none.

### 2026-11-19 — Help and logout as icons in the sidebar

- **Branch:** feat/sidebar-icons
- **What:** panel-ui `app/SidebarFooter.tsx`: the account on the left,
  Help (question mark) and Log out (muted red) as SVG icon buttons on the
  right, with labels for screen readers and tooltips. Guides and
  screenshots updated.
- **Why:** owner's request.
- **Status:** checked in the browser.
- **Next steps:** none.

### 2026-11-19 — Other domains per site, connector releases in the panel

- **Branch:** feat/aliases-connector-version
- **What:**
  - contracts: map `meta.aliases`; API `PUT /apps/{app}/aliases`, `App.aliases`;
    `Connector.online`, `versions`, `outdated` and `platform_version`.
  - gateway: aliases registered with the sites (a domain belongs to one
    site), certificates allowed for them (`/tls/allowed`), requests
    redirected to the public domain (301, 308 for writes; the path kept,
    never another host).
  - panel-api: aliases stored (migration 0011, unique across public domains
    and aliases), written in meta at publish, and the published version
    signed again when they change (same version); the gateways' ETag now
    covers the signature. Connectors list and Status read the release of
    each tunnel from the relays; older than the platform: outdated (Status:
    degraded).
  - panel-ui: Isolation → Other domains; Connectors: state, release,
    update notice; the install command uses the platform's release instead
    of `latest`.
  - docs: map format, guides, release notes.
- **Why:** almost every site has two names (www and bare); a connector left
  on an old release silently loses features.
- **Status:** tests for alias registration and conflicts, redirects,
  republishing, release comparison.
- **Next steps:** owner's lab tests, then the `v0.1.0` tag.

### 2026-11-19 — Forgotten password, alerts, backups, raw uploads

- **Branch:** feat/mvp-ops
- **What:**
  - panel-api: forgotten password (`/auth/password-reset`, contract first):
    same answer whether the account exists, email sent in the background,
    one-hour single-use link stored as a hash, every session ends, the
    authenticator stays; limited per IP and per address. Migration 0009.
  - panel-api: email alerts (`internal/alerts`): every 2 minutes the status
    of each site is evaluated (the Status page's rules); a connector
    offline, a failing legacy site or an exposed site that lasts 3 minutes
    sends one email to the tenant's administrators and extra addresses,
    and one when it works again; states are stored so a restart sends
    nothing twice. Settings `GET/PUT /tenants/{t}/alerts`. Migration 0010.
  - panel-ui: "Forgot your password?", `/forgot`, `/reset/:token`;
    Settings → Alerts.
  - gateway: files sent as the whole body (`image/*`, video, audio, PDF,
    office, `application/octet-stream`) in areas: first bytes must match the
    declared type, pages, scripts and programs refused (`upload/raw.go`),
    streamed through a temporary file. Logs word the new reasons.
  - deploy: `backup.sh` and `restore.sh` (database, secrets, `.env`;
    optional encryption, retention); INSTALL section 10.
  - docs: guides (Help), INSTALL, map format, release notes.
- **Why:** an MVP given to customers needs these (owner's list: 2, 3, 4, 5).
- **Status:** tests for reset, alerts (grace, once only, recovery, off),
  settings, raw uploads; backup/restore exercised with a fake docker.
- **Next steps:** owner to decide on domain aliases (www and bare domain)
  and the connector version in the panel.

### 2026-11-19 — Ask for the identity provider's login on chosen paths

- **Branch:** feat/oidc-paths (on top of feat/help-guides, #36)
- **What:**
  - schema: `roles.auth.paths` (both copies): only these prefixes need the
    OIDC login, the rest keeps the anonymous role.
  - gateway: `loginNeeded`/`loginPath` (`server/oidcpaths.go`): paths are
    matched on a cleaned, lower-case path (`\`, `//`, dot segments,
    `;params`, trailing dots and spaces), so no spelling slips past;
    `Authorization` (legacy HTTP login, application passwords) reaches the
    legacy site only after the provider.
  - panel-ui: Login tab: "The whole site / Only some paths / Optional", path
    list, ready paths for WordPress (with XML-RPC), Joomla, Drupal, Moodle.
  - docs: map format, customer guide (synced Help), release notes.
- **Why:** owner's request: protect admin and login pages with the
  organisation's SSO without a login on the public site.
- **Status:** tests cover the bypass spellings, 401 on writes, the
  Authorization rule and the panel form.
- **Next steps:** single login into the legacy site after the provider
  (later, if customers ask).

### 2026-11-19 — Add the in-panel Help, scroll long map documents

- **Branch:** feat/help-guides
- **What:**
  - panel-ui: **Help** above the account in the sidebar
    (`features/help`): topics on the left (macro topics and topics), search
    across the guide, previous/next. Customers get the customer guide,
    platform users the administrator guide.
  - docs: `docs/manual/tenant.md` and `admin.md` rewritten as guides
    (`##` macro topics, `###` topics); they are the only source: `npm run
    sync:help` copies them and their screenshots into the panel, CI fails
    when the copy is stale (`check:help`).
  - panel-ui: the map document of a version and the editor scroll inside a
    box with an always-visible scrollbar; the sidebar stays in place.
  - INSTALL: admin pages are served (not 404), maps start from templates.
  - Screenshots taken again (scrollbars visible).
- **Why:** owner's requests: a guide inside the product for customers and
  administrators; long maps made the page and the screenshots endless.
- **Status:** tests pass (parser and search covered); help checked in the
  browser for both kinds of user.
- **Next steps:** login with the identity provider on chosen paths only
  (owner's request, to be agreed).

### 2026-11-19 — Keep the tagline within the width of the name

- **Branch:** fix/logo-tagline
- **What:** panel-ui `Logo`: the tagline is drawn in an SVG as wide as
  "Equalizer" (box with no width of its own, letters spaced with
  `textLength`), in the sidebar and on the login steps. README banner
  (`docs/brand/logo-banner.*`) aligned the same way. Screenshots taken
  again.
- **Why:** owner's request: "PROTECT LEGACY WEB APPS" must never be longer
  than the name.
- **Status:** measured in the browser: same left and right edges.
- **Next steps:** none for the logo.

### 2026-11-19 — Add the logo, fix legacy compatibility bugs, align docs

- **Branch:** feat/logo-and-fixes
- **What:**
  - panel-ui: Equalizer logo (`shared/ui/Logo.tsx`, `BrandBanner.tsx`) in
    the sidebar and on the login steps, tagline "PROTECT LEGACY WEB APPS";
    new favicon; the old `Wordmark` is gone. Logo files in `docs/brand/`
    (SVG, PNG), banner in the README. Screenshots taken again.
  - gateway: the legacy site's HTTP login (`Authorization`) was lost in
    connector mode: the relay token took its header. It now travels as
    `X-Eq-Authorization` and the connector puts it back (contract
    `schema/tunnel/tunnel.md` updated first).
  - gateway: the browser's User-Agent becomes a fixed string for its kind
    of device (`internal/useragent`): mobile versions work, the client's
    own string never reaches the legacy app.
  - gateway: `Accept-Encoding: identity` towards the legacy app; gzip and
    deflate answers are decompressed (`legacyclient/encoding.go`), so pages
    of apps that always compress render instead of breaking.
  - docs: architecture (pipeline as built, CSP by hash and visual
    comparison marked planned, legacy budget per replica), map format,
    tenant and admin manuals, INSTALL (`EQ_BEHIND_CLOUDFLARE`, update by
    release tag, connector version), release notes.
- **Why:** owner's request (logo, a status point) and a bugfix round
  before the lab tests.
- **Status:** tests pass in every service. Older connectors keep working
  but drop the legacy HTTP login: update them with the VPS.
- **Next steps:** lab tests by the owner, then tag `v0.1.0`; WebSocket;
  SIEM forwarding; capacity from the panel.

### 2026-11-19 — Fix review findings in the compatibility work

- **Branch:** fix/compat-review
- **What:**
  - gateway: live updates were never flushed in production: the status
    recorder hid the server's flusher (`statusRecorder.Unwrap`,
    `commitWriter.Flush` through `http.ResponseController`); end-to-end test.
  - gateway: every cookie of a `session.start` answer is private, so
    `success.cookie` works whatever the cookie's name.
  - gateway: "remember me" ends when the legacy login cookie is deleted,
    not when a stale cookie is; deletions alone create no session; public
    cookies still reach the browser when the session store is down; answers
    setting cookies get `Cache-Control: private, no-store`.
  - gateway: more cookie names kept private (`uid`, `pass`, `pwd`,
    `secret`, `hash`, `member`, `account`).
  - gateway: request headers trusted as set by a proxy are refused (method
    override, SSO identity like `X-Remote-User`, `X-Middleware-Subrequest`,
    `X-True-Client-IP`…); response headers for front proxies or debugging
    are dropped (`X-Accel-Redirect`, `X-Sendfile`, `X-Debug*`, `X-Varnish`…).
- **Why:** bugfix session on the compatibility work before the first
  customer tests.
- **Status:** tests pass; not yet tried on real sites.
- **Next steps:** tag `v0.1.0` (by the owner: this session cannot push
  tags); lab tests on real CMS.

### 2026-11-19 — Widen compatibility with real legacy sites

- **Branch:** feat/compat
- **What:**
  - gateway: real client IP to the legacy app (`X-Forwarded-For`,
    `X-Real-IP`; map `legacy.client_ip`, default `forwarded`);
    `CF-Connecting-IP` honoured only with `EQ_BEHIND_CLOUDFLARE=true`
    (it was spoofable through Caddy).
  - gateway: legacy cookies split into private (HttpOnly or session-like
    names, kept in the session) and public (sent to the browser, read back
    filtered: `server/cookies.go`); cookies written by page scripts reach
    the legacy app.
  - gateway: "remember me" (`defaults.session.remember_days`, default 30):
    a persistent private legacy cookie makes the session persistent.
  - gateway: application `X-*` headers both ways, `Authorization` (Basic,
    Bearer, Digest, Token), `WWW-Authenticate`, `Link`, `Retry-After`;
    revealing headers dropped.
  - gateway: `text/event-stream` flushed event by event.
  - gateway: multipart bodies of areas rebuilt in streaming into a
    temporary file (`server/areabody.go`, `rebuild.Multipart`); other
    bodies capped at 64 MB in memory.
  - gateway: single-byte charsets (windows-125x, iso-8859-*, koi8, ibm866)
    generated by `scripts/gen_charset_tables.py`.
  - tunnel-relay: handover keeps large bodies in a temporary file (up to
    1 GB) instead of failing above 50 MB.
  - panel-api/panel-ui: `max_upload_mb` on `POST /apps/{app}/map-template`
    and a **Max upload (MB)** field; templates loosened (unicode slugs,
    longer searches, strings and comments).
  - deploy: `EQ_BEHIND_CLOUDFLARE`. Docs: map format, tenant manual,
    0.1.0 release notes.
- **Why:** first customer tests: the prototype must work with as many real
  sites as possible without manual tuning.
- **Status:** unit and integration tests pass (gateway, relay, panel-api,
  panel-ui). Not yet tried on real sites in the lab. WebSocket still 501;
  East Asian charsets (Shift_JIS, GBK, Big5, EUC-KR) still unsupported.
- **Next steps:** merge after #31; tag `v0.1.0` with this included; lab
  tests on WordPress/Moodle with uploads, remember me and SSE; WebSocket.

### 2026-11-19 — Stabilization and release 0.1.0

- **Branch:** `fix/stabilize-0.1` (on top of `feat/areas`)
- **What:**
  - Areas: the legacy charset is learnt from the site's own pages, so forms
    posting into an area reach an ISO-8859-1 site with the right accents
    (an area's `charset` still wins); WebSocket upgrades get a clear 501
    (`websocket_not_supported` in the Logs) instead of a broken request;
    files are revalidated (`If-None-Match`, `If-Modified-Since`, 304) while
    rendered pages carry no validators (they hold per-visitor form tokens).
  - Connector: a request was cut after 30 seconds in total, body included,
    so long downloads through an area broke; now 30 seconds for the answer
    to start and 10 minutes in all. Relay handover between replicas had the
    same 50-second cut: now only the wait for headers is bounded.
  - Release 0.1.0: `VERSION`, release notes (`docs/releases/0.1.0.md`:
    contents, security properties, known limits), images published with the
    version on every `vX.Y.Z` tag, INSTALL and `.env.example` install a
    release (`EQ_VERSION`).
- **Why:** close a first stable prototype with what is developed.
- **Status:** all tests pass. The tag `v0.1.0` goes on `main` after #28,
  #29, #30 and this pull request are merged.
- **Next steps:**
  1. Merge, tag `v0.1.0`, GitHub release with the notes.
  2. Lab campaign (WordPress, Moodle, Juice Shop fully through the gateway).
  3. SIEM forwarding; one legacy account per user behind OIDC.

### 2026-11-19 — The whole site is served: areas

- **Branch:** `feat/areas` (on top of `fix/bugfix-session-2`)
- **Decision (project owner):** Equalizer is in between and must not
  interfere: the whole site is served, admin included, and access control
  stays with the legacy application. Leftover files on the legacy server are
  not Equalizer's concern. Recorded in CLAUDE.md.
- **What:**
  - Map contract: `areas` (prefix, methods, optional roles, body limit,
    legacy charset). Precise entries take precedence.
  - gateway: requests no entry matches go to the area with the longest
    prefix; path checked, query and body parsed and written again
    (`internal/rebuild`: urlencoded and multipart forms in order, files with
    cleaned names, JSON token by token, XML from its tokens with DOCTYPE
    refused, UTF-8 text; anything else 415); a few request headers rebuilt,
    Referer/Origin translated; legacy status and content passed on, pages
    rendered, downloads streamed (`legacyclient.Open`), redirects to other
    sites passed on; legacy cookies stay on the gateway; rate limits and
    form tokens only on precise entries. Forms posting into an area keep
    their hidden values (nonces); the link rewriter keeps area paths.
  - Templates: every CMS template has the `/` area with all methods.
  - Agent: always proposes the `/` area (`site_area` in the report); admin
    pages and destructive links are still not opened, but served.
  - Review aid: `area_added` (low), `area_removed` (high).
  - Docs: map format (areas, principles), tenant manual, gateway README.
- **Why:** sites must work entirely through Equalizer with little setup,
  with or without discovery.
- **Status:** tests pass (end-to-end area test with a WordPress-like admin:
  ajax, REST JSON, downloads, legacy 403, redirects, nonces kept; XXE,
  binary, NUL, malformed query, dot segments refused before the legacy
  app; rebuild unit tests).
- **Next steps:** lab campaign with WordPress and Moodle fully through the
  gateway.

### 2026-11-19 — Bugfix session 2

- **Branch:** `fix/bugfix-session-2` (on top of `fix/bugfix-session-1`)
- **What (each with a test):**
  - **Uploads, decompression bomb:** with `scan: cdr` a PNG of a few KB
    declaring 60000 × 60000 pixels made the gateway allocate gigabytes.
    Images are measured before decoding and refused past 40 megapixels.
  - **Uploads, crash:** a file under 30 bytes starting like a zip made the
    Office sniffer panic (slice bounds).
  - **Discovery, test account:** the login was "ok" as soon as any cookie
    existed, and most PHP sites set one on the first visit; now a login
    needs a new or changed cookie and no login form in the answer (a
    redirect back to the login page is a failure; Moodle's test session is
    not).
  - **Discovery, logged-in pass:** it shared the page budget and the
    per-shape limit with the anonymous pass, so it usually explored nothing;
    each pass has its own, and pages only members see come first.
  - **Discovery drafts** are checked by a gateway too, like drafts written in
    the panel.
- **Why:** second review of the code of the last days.
- **Status:** all tests pass.
- **Next steps:** lab campaign.

### 2026-11-19 — Bugfix session 1

- **Branch:** `fix/bugfix-session-1`
- **What (each with a test):**
  - **Maps the gateway would refuse could be published.** The panel checked
    only the JSON schema; the gateway then kept the previous version (the
    panel said v2, v1 was served) or, for a first version, did not serve the
    site. Now the gateway answers `POST /maps/check` on its admin endpoint
    with its own checks, and panel-api asks it on every draft
    (`map_inconsistent`, 422) and before publishing (no gateway answering:
    `map_unchecked`, 503, never published unchecked) and on rollback.
  - **Entries open to an undeclared role** were accepted and silently closed
    to everyone: the gateway now refuses them (routes, forms, APIs, assets).
    The panel's CMS templates are checked to load (test).
  - **OIDC, several tabs:** a second login overwrote the first; the first
    tab came back with 400. Up to 4 logins in progress per session, the one
    finished is removed, the others stay valid.
  - **OIDC, scripts:** a `fetch()` without `Accept: application/json` got a
    redirect to the provider; now any non-navigation (Sec-Fetch-Mode) gets
    401.
  - **Relay handover** copied hop-by-hop headers (Connection,
    Transfer-Encoding…) between replicas: now dropped both ways.
  - **Logs page:** with Live on, the detail jumped to another event every
    refresh (it followed the row, not the event); "Load older" while Live
    left gaps in the list (Live now pauses, and resuming starts from the
    newest page); the cursor's own event could appear twice across pages.
- **Why:** bugs found by reading the code written in the last days.
- **Status:** all tests pass.
- **Next steps:** bugfix session 2 (agent, discovery jobs, uploads,
  sessions), then the lab campaign.

### 2026-11-19 — Login with the identity provider (OIDC), off by default

- **Branch:** `feat/oidc` (on top of `docs/align-screenshots`)
- **What:**
  - Map contract: `roles.auth` with `provider: oidc`, issuer (https),
    client, scopes, groups claim, group → role rules (first match, `*` for
    everyone), `required`. Without it nothing changes.
  - gateway `internal/oidc`: discovery, keys (RSA ≥ 2048, EC P-256/P-384;
    never `none` or HMAC), ID token checks (signature, issuer, audience and
    azp, expiry, iat, nonce), code flow with PKCE, end-session. Standard
    library only. `server/oidc.go`: `/.eq/oidc/login|callback|logout`, the
    gate before routing (pages redirect, APIs and writes 401), login state in
    the session for 10 minutes, new session id after login, role from the
    groups only, local return paths only. Refusals are `request.blocked`
    events (`oidc_*`, `login_required`).
  - Client secrets: sealed per site in panel-api (`app_oidc`, migration
    0008), `PUT/DELETE /apps/{app}/oidc/secret` (administrators, recorded
    without the secret), `GET /apps/{app}/oidc` (addresses to register),
    `/v1/internal/oidc` for the gateways (`EQ_OIDC_SECRETS_URL`).
  - Review aid: `oidc_on`, `oidc_off` (high), `oidc_issuer_changed` (high),
    `oidc_optional`, `oidc_everyone`, `oidc_group`.
  - panel-ui `features/sitelogin`: Site → Login tab; settings saved as a new
    draft (usual review and approval), secret saved apart.
  - Docs: map format, tenant manual (with screenshot), gateway README.
- **Why:** many legacy sites have a weak login; the organisation's identity
  provider (with its MFA) goes in front of them without touching them.
- **Status:** tests pass (end-to-end with a fake provider: login, roles,
  logout, forged state, tampered signature, unknown group, 401 for APIs;
  token checks; secrets endpoints; review rules; schema). Panel checked in
  a browser. Not yet tried with a real provider.
- **Next steps:**
  1. Try with Entra ID or Keycloak in the lab.
  2. One legacy account per user behind the OIDC login.
  3. Single sign-on to the panel; SIEM forwarding.

### 2026-11-18 — Documentation aligned, screenshots

- **Branch:** `docs/align-screenshots` (on top of `feat/status-page`)
- **What:**
  - Manuals rewritten on what the panel does today (tenant and platform),
    with a roles table and a "Not available yet" section; planned features
    no longer described as present.
  - Screenshots in `docs/screenshots/` (fictitious company Acme, domains
    under `example`), used by the manuals and the README.
  - Architecture: panel-api and agent responsibilities, relay replicas,
    observability, status and logs; INSTALL: status line and checks;
    gateway README: uploads no longer listed as to do.
- **Why:** several documents still described the prototype of a few weeks
  ago; earlier screenshots used a real organisation's name.
- **Status:** documentation only.
- **Next steps:** SIEM forwarding; lab campaign.

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
