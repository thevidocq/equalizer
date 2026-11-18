# Architecture

Equalizer is a gateway that stands in for legacy applications that cannot be
updated: it hides them from the Internet and serves an identical copy,
rebuilt on every request. A crawler and an AI agent generate the allowlist
map; people approve it from a multitenant panel.

## Non-negotiable requirements

1. **Interface 100% identical**: faithful rendering by default, verified with
   automatic visual comparison (Playwright, pixel by pixel).
2. **No exposure of the legacy application**: only the connector reaches it,
   opening the tunnel outbound, both in cloud and on-prem.
3. **Rebuild, never forward.**
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
               ▲    │                 CONTROL PLANE
          panel-ui  └──► PostgreSQL
               orchestrator ──► Docker / Kubernetes
```

## Components

| Service | Language | Responsibility | Contract |
| --- | --- | --- | --- |
| `gateway` | Go | Public traffic, allowlist, rebuilding, faithful rendering | Signed map, events, metrics |
| `tunnel-relay` | Go | Terminates connector tunnels, routes requests | `schema/tunnel/tunnel.md` |
| `connector` | Go | Next to the legacy app; outbound tunnel; allowed destinations only | `schema/tunnel/tunnel.md` |
| `orchestrator` | Go | Applies the configured capacity (replicas, autoscaling, pools) | Desired state from panel-api |
| `panel-api` | Go | Tenants, email invitations, users, sites, connectors, maps with approval, signing and rollback, templates, discovery jobs, review aid, tenant AI settings, exposure checks, status, logs (from Loki), audit, platform/tenant privacy (done); capacity, metrics, SIEM (planned) | `schema/api/panel-api.yaml` |
| `panel-ui` | TypeScript, React | Panel | Client generated from OpenAPI |
| `agent` | Python | Read-only discovery through the connector (anonymous and logged in), CMS fingerprint, map proposals, IDOR proposals for JSON APIs, optional AI with the tenant's model, review summaries | OpenAPI, `LLMProvider` interface |
| `postgres` | — | Control-plane data | Versioned migrations |
| `redis` | — | Sessions, rate limits, cache, new-map notifications | Keys documented per module |

Observability: Vector (log collection), Loki (logs, 30 days), Grafana
(analysis); the panel's Logs and Status pages read Loki. Planned: Tempo
(traces), Prometheus and Alertmanager (metrics and alerts).

## Sites, connectors and isolation

- **One gateway, many sites.** The gateway loads every published map and
  picks it by the request's host (`meta.public_domain`); an unknown host is
  404. Caddy issues certificates on demand, only for the domains the
  gateway serves (`/tls/allowed`).
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
- **Direct mode** is limited to destinations the platform allows, so no
  map can aim the gateway at the platform's own network.

## Discovery

The agent explores a site read-only through its connector and proposes a
draft map: crawl, fixed exclusions (admin, backups, actions), fingerprint
(CMS, version, components), deterministic routes and types, then an
optional AI review with the tenant's own model that can only rename,
exclude or narrow. The gateway reports the paths it refused, so discovery
also sees what real visitors miss. Continuous discovery (daily or weekly)
proposes a draft only when something changed. Every proposal goes through
the usual approval.

## Platform and tenants

Two kinds of people use the panel, and they never share a view.

- **Platform team** (`platform_role` admin or operator): runs the
  installation. Sees the list of tenants with, per site, only its name and
  whether a map is published. Creates a tenant by giving the email of its
  first administrator, who receives a single-use invitation link (48 hours)
  and chooses their own password. The platform holds no tenant role: maps,
  connectors, people and the tenant's activity are closed to it, and its own
  activity log holds platform actions only.
- **Tenant people** (`tenant_role` admin, editor, approver, auditor):
  manage their own sites. The tenant administrator invites the others.

Support access to a tenant (time-limited and audited, granted by the tenant)
is not implemented: today the platform cannot see tenant data at all.

## Faithful rendering (gateway pipeline)

For each page, independent steps implementing the same `Step` interface:

1. **Parsing** HTML5 into a DOM and serialising it again.
2. **Invisible cleanup**: comments, version meta tags, headers revealing the
   stack, internal hidden fields, debug output.
3. **URL rewriting** of links, `src`, `action`, URLs inside CSS.
4. **Forms**: `action` pointing to the gateway, legacy hidden fields kept
   server-side in a sealed token bound to the browser (CSRF), values
   re-encoded into the legacy charset on submission.
5. **Scripts** allowed only by hash (CSP).
6. **Assets** served after checking their real type.
7. **Security headers** (CSP, HSTS, ...).

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
signature and activate it. The budget towards the legacy application is
counted in Redis and applies across all replicas. Each site's capacity is
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
