# Tenant administrator manual

For whoever protects their own sites with Equalizer: adding a site,
connector, map, testing, cutover, day-to-day use.

> **Status:** the panel is in development. This manual describes the planned
> behaviour; the real status of each feature is in `CHANGELOG.md`.

## 1. The panel

- **Sidebar**: Overview, the list of **protected sites** with a status dot
  (green protected, yellow in testing or with proposals, red alert),
  **+ Protect a site**, Settings, Audit.
- **Top bar**: notifications and user menu.
- **Site page**: tabs Overview, Proposals, Map, Preview, Connectors,
  Capacity, Events, Logs, DNS.

Tenant roles: **administrator**, **editor** (edits the map), **approver**,
**auditor** (read-only). Whoever makes a change cannot approve it.

## 2. Protecting a site

The original site keeps working as before throughout the procedure: no
downtime.

### Step 1 — Site details

**+ Protect a site**: public domain (e.g. `portal.example-city.gov`) and
internal address of the legacy application (e.g. `http://10.20.0.15:8080`).

### Step 2 — Connector

1. The panel generates a token (one-time, 24 hours) and shows the command.
2. On a machine with Docker on the legacy application's network:

```bash
docker run -d --name eq-connector --restart unless-stopped \
  -e RELAY_URL=<relay address> \
  -e ENROLL_TOKEN=<token> \
  equalizer/connector:latest
```

3. The panel shows **Connector online**. Only outbound Internet access on
   port 8443 is needed: no inbound port.

For redundancy you can install a second connector with a new token.

### Step 3 — Crawl

1. Enter the test accounts for each role (anonymous, user, operator...).
   They are stored in the vault.
2. Start the crawl. Within a few hours the agent proposes the map, for
   example: "42 pages, 8 forms, 3 fields to review".

### Step 4 — Review

1. **Proposals**: for each proposal you see the diff, the risk and the
   rationale.
2. **Preview**: legacy and protected site side by side, with the result of
   the visual comparison.
3. **Map editor**: fix the flagged fields (e.g. which characters the
   "notes" field accepts).
4. Changes that open new pages or widen fields need approval from a second
   person.

### Step 5 — Testing

- **Trial**: the protected site answers on a test address
  (e.g. `new.portal.example-city.gov`). Browse it as a user would.
- **Shadow**: the gateway receives a copy of real traffic and measures how
  many pages the map covers. The panel shows the percentage and what is
  missing.

### Step 6 — Cutover

**DNS** tab, a guided checklist:

1. Lower the DNS record TTL at least 24 hours in advance.
2. Set the CNAME shown by the panel. With **Cloudflare**: CNAME in the
   dashboard; if the proxy is orange, SSL in "Full (strict)" mode.
3. The panel checks propagation.
4. A few days in **parallel**, then **final**.
5. **Close public access to the legacy application** (old IP, port 443).
   Probes verify it: as long as the legacy application answers from the
   Internet, the protection does not hold.

## 3. Day-to-day use

### Site overview

| Group | What you see |
| --- | --- |
| Traffic | Requests, estimated unique visitors, bandwidth, most viewed pages |
| Protection | Requests blocked by reason, rejected fields, rate limits, most blocked paths and IPs |
| Performance | Latency (gateway and legacy), cache, legacy errors |
| Coverage | Unmapped URLs, unrecognised historical URLs, visual comparison |
| Connectivity | Connectors online, tunnel latency |
| Agent | Last crawl, pending proposals |

### Typical notifications

- "The legacy application changed page X: proposal ready" → open Proposals
  and approve.
- "Connector offline" → check the connector machine and its network.
- "Legacy exposed" → close public access to the old IP at once.

## 4. Configuration

### Capacity

**Capacity** tab, within the plan limits:

- minimum and maximum replicas and autoscaling threshold;
- shared or dedicated pool (if the plan allows it);
- **cache** for public pages (reduces load on the legacy application);
- **budget towards the legacy application**: maximum requests per second and
  concurrent requests.

### LLM provider

**Settings → LLM**: use the platform provider or your own (Claude, ChatGPT or
a local model at an internal address). The key is write-only. With cloud
providers personal-data masking is active.

### Logs

The site's **Logs** tab: search, live tail, export. You only see the logs of
your own sites. For a problem on a page, enable **request tracing** for a few
minutes and reproduce the problem.

### Forwarding to your SIEM

**Settings → SIEM forwarding**: destination (syslog, Splunk HEC, Sentinel,
webhook), format (JSON, CEF, LEEF), streams (security, audit, application),
syslog facility per stream, test message. You only receive data about your
own sites.

### Import and export

**Settings → Import and export**: export the tenant or a single site (maps,
capacity, settings; secrets excluded). An import shows the preview first, and
maps arrive as proposals to approve.

## 5. Rollback

**Map → Versions**: every publication is a version. If a change causes
problems, republish the previous version with one click.
