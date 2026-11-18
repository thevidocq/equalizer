# Platform administrator manual

For whoever runs an Equalizer installation (SaaS or on-prem): tenants,
plans, capacity, global health, logs, SIEM forwarding and configurations.

> **Status:** the panel is in development. This manual describes the planned
> behaviour; the real status of each feature is in `CHANGELOG.md`.

## 1. Access

- The panel answers on `EQ_PANEL_DOMAIN` (customers log in there) and on
  the VPS localhost (port 8088, over SSH tunnel or VPN). Limiting platform
  logins to the VPN is planned.
- The first platform administrator is created on the server with
  `panel-api create-admin`; MFA is mandatory.
- Platform roles: **administrator** (everything) and **operator** (read-only
  on status, logs and metrics).

## 2. Service status

**Menu → Service status.** This is the first page to check.

| Area | What to check |
| --- | --- |
| Services | Every service and replica operational; versions aligned |
| Full path | End-to-end synthetic check green for every pool |
| Data | PostgreSQL replication lag, Redis memory |
| Connectors | Online out of total, per tenant |
| External dependencies | LLM provider, expiring certificates, exposure probes |
| Resources | CPU, RAM, disk per node; actual replicas equal to desired |

States: **operational**, **degraded**, **unavailable**, always with the
reason. Alerts arrive even if the panel is down (Alertmanager).

## 3. Tenants and plans

1. **Menu → Plans and limits**: define the plans (maximum replicas, maximum
   sites, log retention, dedicated pool yes/no).
2. **Tenants → Add a customer** *(available)*: name, id, name and email of
   the tenant's first administrator.
3. The administrator receives an email with a link (single use, 48 hours),
   chooses a password and sets up an authenticator app; then configures
   everything else and invites their colleagues.
4. Until someone joins, the tenant shows **Waiting for the administrator to
   join** with **Invite again** (email lost or not sent, wrong address): the
   previous link stops working. Once anyone has joined, only the tenant's
   administrators invite people.

**Privacy** *(available)*: the platform sees each tenant's name and, per
site, only its name and status (protected or being set up). Maps,
connectors, people and the tenant's activity are visible only to the
tenant's people; the platform's activity page lists platform actions only.

## 4. Capacity and pools

**Menu → Platform capacity.**

- **Pools**: sets of gateways, shared or dedicated to one site.
- Available **regions** (in cloud).
- **Maximum limits** per plan: the tenant chooses only within these limits.
- On a VPS replicas grow up to the machine's resources; to go further add a
  node (in cloud the cluster autoscaler does it).

## 5. Default LLM providers

**Menu → Settings → LLM.**

- Platform default provider (prototype: Claude Haiku).
- Write-only API keys: once saved they are never shown again.
- **Test connection** to check latency and outcome.
- Tenants can use the default or configure their own provider.

## 6. Logs

**Menu → Logs.**

- Search by time, service, level, tenant, site, `request_id`, text.
- **Live tail** to follow logs in real time.
- **Export** as JSON or CSV.
- **Hot log levels**: per service, module and site; the `debug` level expires
  by itself (default 30 minutes).
- **Request tracing** on a site: shows step by step what the gateway did for
  each request. For debugging, for short periods.

## 7. SIEM forwarding

**Menu → SIEM forwarding → New destination** (platform level).

1. **Type**: syslog TCP+TLS (recommended), TCP, UDP; Splunk HEC; Sentinel;
   webhook.
2. **Format**: JSON, CEF or LEEF.
3. **Streams**: security, audit, application (with a minimum level).
4. **Facility** per stream (defaults: security `local4`, audit `local5`,
   application `local6`). Severity is derived from the log level.
5. **Send test message** and check that it reaches the SIEM.

Messages go through an on-disk queue: if the SIEM does not answer, they are
sent when it is available again.

## 8. Import and export

**Menu → Import and export.**

- **Platform export**: plans, pools, regions, policies, default LLM
  providers, tenant list. For backups, migration between installations,
  staging to production.
- **Import**: upload the archive, review the preview (diff and conflicts),
  choose *overwrite*, *skip* or *rename* for each conflict, then apply.
- Secrets are never exported: on import the panel asks for them again.
- Imported maps arrive as proposals and need approval.

## 9. Audit

**Menu → Audit**: who did what and when (approvals, publications, capacity
changes, import/export, log level changes).

## 10. Routine procedures

| Situation | What to do |
| --- | --- |
| End-to-end check red | Service status → find the component; Logs filtered by service |
| Connector offline for a tenant | Check from the tenant's panel; notify the tenant (network or connector machine) |
| Certificate expiring | Check ACME renewal and the domain's DNS |
| Exposure alert | A tenant's legacy app answers from the Internet: tell the tenant at once to close the old IP |
| Load peak | Capacity → check replicas and the budget towards the legacy app |
| Update | `git pull`, `docker compose pull`, `docker compose up -d` (see `INSTALL.md`) |
