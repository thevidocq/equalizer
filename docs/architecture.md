# Architecture

Equalizer is a gateway that stands in for legacy applications that cannot be
updated: it hides them from the Internet and serves an identical copy,
rebuilt on every request. A signed map says what exists: by default the
whole site, admin included (areas), plus precise entries with typed values
where they are declared. Templates per CMS and a crawler with an optional AI
agent propose the map; people approve it from a multitenant panel.

## Non-negotiable requirements

1. **Interface 100% identical**: faithful rendering by default. Planned:
   automatic visual comparison (Playwright, pixel by pixel).
2. **No exposure of the legacy application**: only Equalizer reaches it.
   By default through the connector, which opens the tunnel outbound, both
   in cloud and on-prem; or directly, when the site stays at its public
   address and accepts only Equalizer's address (decision of the project
   owner, 2026-11-20: saves the connector where the customer already
   controls who reaches the site).
3. **Rebuild, never forward.** Values are decoded once and written again as
   they were: never decoded twice, normalised or mapped to a look-alike
   character. What parsers read differently (a JSON key written twice, an
   unpaired surrogate, a character the legacy charset cannot hold) is refused,
   not interpreted. Fuzz tests in `gateway/internal/rebuild`, `pathkey` and
   `jsontext` check that a rebuilt request reads back as the same values.
4. **Continuity**: historical URLs keep working (`compat`).
5. **Two modes**: cloud with repointed DNS and on-prem, same software.
6. **Configurable LLM**: Claude, ChatGPT or a local model, from the panel.
7. **Everything configurable from the panel**, including capacity (SaaS/PaaS).
8. **Modularity**: separate services that talk only through contracts.

## Flow

```text
                 DATA PLANE                                   CUSTOMER NETWORK
Internet ──► gateway ──► tunnel-relay ◄══ outbound tunnel ══ connector ──► legacy app
               ▲  (only exposed)   ▲
               │ signed map        │ crawl
            panel-api ◄── proposals ── agent ──► LLM provider
               │  └──── assistant (tenant's model) ──────► LLM provider
               ▲    │                 CONTROL PLANE
          panel-ui  └──► PostgreSQL
               orchestrator ──► Docker / Kubernetes
```

## Components

| Service | Language | Responsibility | Contract |
| --- | --- | --- | --- |
| `gateway` | Go | Public traffic, areas and precise entries, rebuilding, faithful rendering, sessions with legacy cookies, optional OIDC login, rate limits | Signed map, events, metrics |
| `tunnel-relay` | Go | Terminates connector tunnels, routes requests | `schema/tunnel/tunnel.md` |
| `connector` | Go | Next to the legacy app; outbound tunnel; allowed destinations only | `schema/tunnel/tunnel.md` |
| `orchestrator` | Go | Applies the configured capacity (replicas, autoscaling, pools) | Desired state from panel-api |
| `panel-api` | Go | Tenants, email invitations, users, sites, connectors, maps with approval, signing and rollback, templates, discovery jobs, review aid, tenant AI settings, site assistant, exposure checks, status, logs (from Loki), audit, platform/tenant privacy (done); capacity, metrics, SIEM (planned) | `schema/api/panel-api.yaml` |
| `panel-ui` | TypeScript, React | Panel | Client generated from OpenAPI |
| `agent` | Python | Read-only discovery through the connector (anonymous and logged in), CMS fingerprint, map proposals, IDOR proposals for JSON APIs, optional AI with the tenant's model, review summaries | OpenAPI, `LLMProvider` interface |
| `postgres` | — | Control-plane data | Versioned migrations |
| `redis` | — | Sessions, rate limits and behaviour counters, shared by the gateway replicas | Keys documented per module |

Observability: Vector (log collection), Loki (logs, 30 days), Grafana
(analysis); the panel's Logs and Status pages read Loki. Planned: Tempo
(traces), Prometheus and Alertmanager (metrics and alerts).

## Sites, connectors and isolation

- **One gateway, many sites.** The gateway loads every published map and
  picks it by the request's host (`meta.public_domain`); an unknown host is
  404. Each https site has its own certificate (the legacy site's,
  uploaded in the panel, key sealed with the panel's secret): the gateways
  read them from the panel (`/v1/internal/certificates`) and give them to
  Caddy at each TLS handshake (`/tls/certificate`); Caddy never gets one
  itself for a protected site (`/tls/allowed` always says no). Only the
  platform's domains (panel, relay) have Let's Encrypt certificates. A site
  can stay in http (`meta.public_scheme`): Caddy's port 80 goes to the
  gateway's plain port, which serves it, and sends https sites to https. A
  site whose https ends at a proxy or CDN in front (`https_proxy`) is served
  on the plain port too, with addresses in https and `Secure` cookies.
- **One connector per server.** A connector belongs to a tenant and serves
  all its sites on the addresses in its `EQ_ALLOW`. For virtual hosts the
  map sets `reach.address`; the site's name travels as `X-Eq-Host` and
  reaches the web server as `Host` and SNI. The panel refuses maps using
  another tenant's connector, and the relay checks the tenant again at
  every request.
- **Isolation.** A protected site must stop answering on its old public
  IP, while the other sites of the server keep working. The panel gives the
  configuration (internal listener, or connector-only) and checks it from
  outside every hour; a site that answers again turns "exposed", in the
  tenant's activity and in the logs.
- **Direct mode.** A site can be reached directly, without a connector: it
  stays at its public IP address and accepts only Equalizer's addresses
  (`EQ_OUTBOUND_IPS`, or those of the panel's and relay's domains, shown in
  the panel). The map then has `reach.mode: direct` with the site's public
  IP and port in `reach.address`; the site's name goes in `Host` and SNI and
  its certificate is checked against it. The panel accepts only a public
  IP address written as such (a name could later resolve to the platform's
  network), or a destination the platform allows (`EQ_DIRECT_DESTINATIONS`,
  lab applications). Isolation cannot be checked from the platform, whose
  requests are the accepted ones: the panel gives a command to run from
  another network, and the hourly check skips these sites.

## Discovery

The agent explores a site read-only through its connector and proposes a
draft map: crawl (it never follows admin, backup or action links: it only
reads), fingerprint (CMS, version, components), deterministic routes and
types, IDOR protection for JSON APIs, then an optional AI review with the
tenant's own model that can only rename, exclude or narrow. The draft always
keeps the `/` area, so the whole site, admin included, is served. The gateway reports the paths it refused, so discovery
also sees what real visitors miss; it counts distinct visitors by a hash of
their address keyed with a secret only the gateway holds (never the
address), and the agent puts a
visitor's request in its draft only once it has checked it on the site,
page options with the values the site offers
([agent.md](agent.md#visitor-requests)). The tenant is emailed when a
discovery's draft is checked and ready to publish. Continuous discovery (daily or weekly)
proposes a draft only when something changed. Every proposal goes through
the usual approval.

The agent is being extended into a **study** of the site
([agent.md](agent.md), [compatibility.md](compatibility.md)): one profile
per kind of web application, a real browser read-only (administration included,
with test accounts), a precise entry for every function it understands, and
two checks before the draft is handed over: every page compared with and
without Equalizer, and every parameter and field tried with an attack set on
the gateway only (`POST /maps/try`, `schema/api/gateway-admin.yaml`). The
draft comes with a measured score; the **Check this version on the site** button runs the checks
again on a version changed by hand.

## Platform and tenants

Two kinds of people use the panel, and they never share a view.

- **Platform team** (`platform_role` admin or operator): runs the
  installation. Sees the list of tenants with, per site, only its name and
  whether a map is published. Creates a tenant by giving the email of its
  first administrator, who receives a single-use invitation link (48 hours)
  and chooses their own password. The platform holds no tenant role: maps,
  connectors, people and the tenant's activity are closed to it, and its own
  activity log holds platform actions only.
- **Tenant people** (`tenant_role` admin or viewer): administrators manage
  their own sites and publish maps alone; viewers read. The tenant
  administrator invites the others. The platform administrator can delete a
  tenant with everything it owns.

Support access to a tenant (time-limited and audited, granted by the tenant)
is not implemented: today the platform cannot see tenant data at all.

## Faithful rendering (gateway pipeline)

For each page, independent steps implementing the same `Step` interface
(`gateway/internal/pipeline`):

1. **Parsing** HTML5 into a DOM (after conversion to UTF-8 from the
   legacy charset) and serialising it again.
2. **Invisible cleanup**: comments, version meta tags, internal hidden
   fields, debug output.
3. **URL rewriting** of links, `src`, `action`, URLs inside CSS, so that
   nothing points to the legacy address.
4. **Forms**: `action` pointing to the gateway; for precise entries, legacy
   hidden fields kept server-side in a sealed token bound to the browser
   (CSRF), values re-encoded into the legacy charset on submission.

Around the pipeline: assets served after checking their real type, legacy
headers filtered (revealing and internal ones dropped), security headers
(HSTS, `nosniff`, frame and referrer policies). Planned: scripts allowed by
hash (CSP).

Towards the legacy application every request is written again: path,
query and body parsed and re-encoded, headers from a list (the browser's
User-Agent becomes a fixed string for its kind of device), the client's
address in `X-Forwarded-For`, private legacy cookies from the session.

## Modularity rules

- Backend: one package, one responsibility; depend on interfaces; types
  generated from `schema/`; configuration from env; 400 lines/file threshold.
- Frontend: `features/<name>/{pages,components,hooks,index.ts}`; pages
  compose; data only from hooks; `shared/ui` has no dependency on features;
  boundaries checked by lint; 200 lines/file threshold.

## LLM providers

A single interface in the agent (`agent/llm/`), one module per provider:
`anthropic` (default: Claude Haiku), `openai`, `local` (OpenAI-compatible
endpoint, e.g. Ollama, vLLM). Per-tenant configuration from the panel; keys
in the vault, write-only. Page content is passed to the model as data, the
output must validate against the map schema, the model has no tools that
write, and every proposal that widens the surface needs human approval.

### The assistant

panel-api also talks to the tenant's model, for the assistant of a site
(`panel-api/internal/assistant`, `POST /v1/apps/{app}/assistant`). The
model gets its tools, scoped to that one site and to what the person
asking may read: `get_map`, `get_report` (latest discovery,
plain facts first and every list capped, so a long list never hides the
CMS version), `search_logs` (Loki, up to 500 events summarised by type,
reason, entry and path), `get_activity` (audit), `get_site` (the Status
page's evaluation), `list_versions`, `get_check` (a version's
verification), `compare_versions` (the review aid's comparison),
`read_guide` (the tenant guide, embedded from `docs/manual/tenant.md` by
`panel-ui/scripts/sync-help.mjs`, checked in CI), `show_card` and
`propose_change`. At most 10 rounds of tools, then words only; 100
seconds in all.

- **No writes.** `propose_change` stores a change in
  `assistant_proposals` after checking it on the map it would go on; only
  three kinds exist (a file type for a folder, a query key with an int,
  date, enum or markup-free string check, more values for an enum), and
  types that can run code are refused. An administrator's click
  (`POST …/assistant/proposals/{id}/draft`) checks it again, makes a draft
  and queues the agent's verification; publishing stays a separate,
  human step. Drafts from proposals are made one at a time per site (a
  PostgreSQL advisory lock), on the newest draft or the published map,
  never beside a version under review.
- **Cards.** `show_card` makes the panel build an event card (looked up
  by request ID in Loki), a refusals table or an entry's checks (from the
  map) and return it with the answer; the model gets a one-line summary
  (kind, masked path, count), never the card's raw data. Answers are Markdown parsed into data in the browser and
  rendered by React as text: no HTML, no images, links only to panel
  pages.
- **Masking.** Client IPs, in the logs or typed in a question, become
  HMAC codes (`V-…`) under a key that lives only in the panel-api
  process; emails in paths and questions are hidden, panel users become `U-…` codes that only the
  answer shown to the person turns back into emails.
- **Data, not instructions.** Tool results are wrapped in a random-tagged
  fence the content cannot close; the system prompt says what is inside
  is data.
- **Outbound.** The same rule as the agent: public addresses only, no
  redirects, https for a tenant's own server.
- **Trace.** Activity records `assistant.asked` (tools used, number of
  proposals and cards), `assistant.draft` and `assistant.settings`; never
  the question or the answer. 20 questions per person every 10 minutes.
- **Conversations.** `assistant_conversations` and `assistant_messages`
  belong to one user and one site; every read and write is filtered by
  the user's id, never by role. The server builds the model's past turns
  from them (the panel sends only the new question and the conversation
  id, so a client cannot forge an earlier answer). Kept
  `tenants.assistant_keep_days` days after the last message (1–365, 30 by
  default), proposals as many days after they were made; panel-api
  deletes what is past retention every hour. Visitor codes use an HMAC key derived from
  the signing key and the tenant, so a kept code still means the same
  visitor after a restart and never matches another tenant's.

## Connector and tunnel

- The connector always opens the connection to the relay: HTTPS on 443
  through the edge proxy, upgraded to `equalizer-tunnel/1`, then yamux
  streams carrying plain HTTP/1.1 (contract in `schema/tunnel/tunnel.md`).
- Prototype authentication: per-connector token (the relay stores only its
  SHA-256). Planned: enrolment with a one-time token from the panel → client
  certificate (mTLS).
- The connector's **local allowlist** (`EQ_ALLOW`) is the last barrier: a
  compromised relay or gateway cannot widen it. A panel-signed allowlist
  will be added on top.
- Several connector instances per app, active-active (round robin).
- The relay keeps tunnels in memory and runs as several replicas: a replica
  without the connector's tunnel hands the request to its peers (found
  through DNS), at most once; no shared state.

## Horizontal scaling

Services with no local state; shared state in Redis and PostgreSQL; no sticky
sessions. When a map is published, replicas download it, verify the
signature and activate it. Rate limits and behaviour counters are kept
in Redis and apply across all replicas; the budget of concurrent requests
towards the legacy application is per replica for now. Each site's capacity is
configured from the panel; the `orchestrator` applies it with the `compose`
driver (prototype) or the `kubernetes` driver (cloud: operator with a
`ProtectedSite` resource, HPA, PDB).

## Metrics, status, logs

- **Status** (done): per site the published map, the connector (tunnels on
  every relay replica), the legacy application (errors in the last 15
  minutes, from Loki), isolation and 7 days of history, for the tenant;
  database, gateway and relay replicas and Loki for the platform, without
  tenant data. Planned: heartbeats and an end-to-end synthetic check.
- **Logs** (done): JSON (`schema/events/log.schema.json`), propagated
  `request_id`, never field values; the tenant's Logs page reads its own
  security events from Loki with filters, live refresh and CSV export.
  Planned: hot log levels with expiry, request tracing, SIEM forwarding
  (syslog RFC 5424, Splunk HEC, Sentinel, webhook; JSON/CEF/LEEF).
- **Metrics** (planned): traffic, protection, performance, coverage per
  site; catalogue in `schema/metrics/catalog.md`.

## Product security

Ed25519-signed maps; the gateway runs on its own if the panel is down; fail
closed; secrets only in the vault; dependency scanning and SBOM in CI;
non-root containers; mandatory review on `pipeline/`, `legacyclient/`,
`mapstore/`; penetration test before use on real portals.

### Exposed services under attack

#### The gateway

The gateway is the only part reachable from the Internet besides the panel
and the relay, and it is built to hold against clients that do not play by
the rules:

- **Slow and idle clients**: request headers within 10 s, at most 32 KB;
  a request body must keep a minimum pace (no 30 s without data, at least
  2 KB/s on average after the first 30 s); an answer the client stops
  reading is given up after 60 s; idle connections close after 120 s.
- **Memory**: forms, JSON, XML and text bodies are rebuilt in memory up to
  16 MB each and 256 MB for all requests together (past that, 503 and the
  client retries); uploads go to temporary files; queries and forms have
  at most 10,000 fields.
- **Sessions**: a new anonymous session lives 5 minutes until its visitor
  comes back with the cookie (a bot never does), and an address creates at
  most 600 a minute after a burst of 300, enough for an office or a mobile
  carrier behind one address; past that the page is served without a
  session and the gateway logs one warning for that address.
- **Client address**: taken from `X-Forwarded-For` only behind trusted
  proxies, reading every line from the right; IPv6 rate limits count per
  /64, the network one subscriber gets.
- **Paths**: `..` and its variants that some servers resolve (`..;`,
  `..\`, `.. `, `...`) are refused before anything is sent on.
- **Panics** answer 500 and are logged; the connection is not dropped
  silently.

The lab's `test_03b_hostile.py` sends these requests byte by byte on the
wire.

#### The relay

- Connector tokens are compared in constant time; an address that sends
  20 wrong tokens in a minute is refused without a check until the minute
  ends, and reported once.
- The connector's address in logs is the one the edge proxy added (the
  last in `X-Forwarded-For`), never one the client wrote.
- Requests go only from the relay to the connector: streams the connector
  opens are closed at once, so a stolen token cannot fill the relay's
  memory with unread streams.
- Idle connections close; a tunnel has no deadline left from its upgrade
  (yamux keep-alives find a dead peer).

#### The panel

- The visitor's address comes from `X-Forwarded-For` only behind
  `EQ_TRUSTED_PROXIES` (Caddy, then panel-ui): login limits and the audit
  log see the visitor, not the proxy.
- Failed logins in 15 minutes: 5 per address and account, 20 per
  address, 50 per account. Locking an account takes many addresses; a
  successful login clears only its own address and account.
- Writes need the `X-Eq-Panel` header (no cross-site forms), bodies are at
  most 4 MB, and the server gives a client a minute to send a request and
  two to read the answer.
- The agent, and panel-api for the assistant, call a tenant's model
  server without following redirects and only if the address they
  connected to is public (no way into the
  platform's networks by a redirect or a changing DNS answer).

The lab's `test_03c_relay_panel.py` tries these on the relay and the
panel.
