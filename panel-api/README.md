# panel-api

Go control-plane API. Contract: [`schema/api/panel-api.yaml`](../schema/api/panel-api.yaml).

## Implemented

| Area | Endpoints | Who |
| --- | --- | --- |
| Auth | `POST /v1/auth/login`, `POST /v1/auth/logout`, `GET /v1/auth/me`, `POST /v1/auth/mfa/{setup,enable,verify}`, `POST /v1/auth/password` | everyone |
| Tenants | `GET /v1/tenants`, `POST /v1/tenants` (creates the tenant and invites its first administrator) | create: platform admin |
| Platform overview | `GET /v1/platform/tenants`: tenant names, site count, site names with `protected`/`setting_up` | platform users only |
| Invitations | `GET/POST /v1/tenants/{tenant}/invitations`, `DELETE …/{id}` (revoke) | tenant admin |
| Joining | `GET /v1/invitations/{token}`, `POST /v1/invitations/{token}/accept` (choose a password, logs in) | anyone with the link |
| Users | `GET /v1/tenants/{tenant}/users`, `DELETE …/{id}` (deactivate), `POST …/{id}/reset-mfa` | tenant admin |
| Sites | `GET/POST /v1/tenants/{tenant}/apps`, `GET /v1/apps/{app}` | create: tenant admin |
| Connectors | `GET/POST /v1/tenants/{tenant}/connectors`, `POST …/{id}/rotate`, `DELETE …/{id}` (409 while a published map uses it) | tenant admin; token shown once; one per server, shared by the sites |
| Templates | `GET /v1/map-templates`, `POST /v1/apps/{app}/map-template` (fills a template for the site, saves nothing); sites take `kind` (wordpress, joomla, drupal, moodle, generic) | everyone; editors and administrators |
| Isolation | `PATCH /v1/apps/{app}` (`legacy_ip`), `POST /v1/apps/{app}/exposure/check` (once a minute) | tenant admin |
| Maps | `GET/POST /v1/apps/{app}/maps`, `GET …/{version}`, `POST …/{version}/{submit,approve,reject,publish}` | draft/submit: editor; approve/reject/publish: approver, never the author |
| Audit | `GET /v1/audit`: the tenant's own actions, or the platform's own for platform users | tenant admin, approver, auditor; platform |
| Internal | `GET /v1/internal/maps` (every published map, ETag), `GET /v1/internal/apps/{app}/map`, `GET /v1/internal/connectors` (relay) | service token |

Security:

- Passwords: PBKDF2-HMAC-SHA256, 600 000 iterations, at least 12
  characters. Five failed logins per IP or email within 15 minutes → 429.
- Two-step login: a password opens a session limited to `/v1/auth/*`; the
  rest answers 403 `mfa_required` until a TOTP code (RFC 6238; a code is
  never accepted twice) or a recovery code (10, single use, stored as
  SHA-256) is given; the session id changes then. Authenticator secrets
  are encrypted with `EQ_MFA_KEY` (AES-256-GCM, user id as additional
  data). Five wrong codes in 15 minutes → 429. Mandatory by default
  (`EQ_MFA_REQUIRED=false` only for tests).
- Password change ends the user's other sessions; deactivating a user or
  resetting their authenticator ends all of them.
- Session cookie `__Host-eq_panel` (Secure, HttpOnly, SameSite=Strict, 8
  hours), stored as SHA-256 in PostgreSQL. Every state-changing request
  needs `X-Eq-Panel: 1` (CSRF).
- Tenant isolation: another tenant's site answers 404.
- Platform privacy: platform users hold no tenant role. They see tenant
  names and, per site, only its name and whether a map is published
  (`/v1/platform/tenants`); every tenant endpoint answers them 403 or 404,
  and the audit log they read holds platform actions only.
- Maps: a `connector_id` must be a connector of the same tenant (422
  `connector_not_found`, rechecked at publish: 409); `direct` mode only
  towards `EQ_DIRECT_DESTINATIONS` (422 `direct_not_allowed`), so no map can
  point the gateway at the platform's own services; `meta.public_domain` is
  set from the site.
- Exposure check (`internal/exposure`): every `EQ_EXPOSURE_INTERVAL` each
  site with a `legacy_ip` is probed on ports 443 and 80 with its name in SNI
  and `Host`, comparing with the answer for a random name. Only public
  unicast IPs are accepted (no private, loopback, link-local or reserved
  ranges); nothing of the answers is stored. A change of status is logged
  and written to the tenant's activity; the platform sees only `exposed`.
- Invitations: 32 random bytes, stored as SHA-256, single use, valid 48
  hours, revocable. The link travels only by email (`internal/mail`, SMTP
  with STARTTLS or implicit TLS); the API never returns it. If the email
  cannot be sent the invitation is kept with `sent: false`, to revoke and
  redo. `EQ_MAIL_MODE=log` writes the link to the log: development only.
- Maps are validated against `schema/map` (embedded copy, kept equal by
  `scripts/validate_schemas.py`); `meta.tenant`, `meta.app`, `meta.version`
  and `meta.status` are set by the panel. Publication signs the exact
  payload with Ed25519; at most one published version per site (database
  constraint); the approver is never the author (database constraint).
- Every change is in the audit log (never passwords or tokens).

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `EQ_ADDR` | `:8080` | API; published only through panel-ui, which hides `/v1/internal` |
| `EQ_ADMIN_ADDR` | `:9092` | `/healthz`, `/readyz` |
| `EQ_DATABASE_URL` | — | `postgres://user@host:5432/db?sslmode=…` |
| `EQ_DATABASE_PASSWORD` or `_FILE` | — | Database password, kept out of the URL |
| `EQ_MAP_SIGNING_KEY` or `_FILE` | — | Ed25519 seed in base64 (`eqtool keygen`) |
| `EQ_MAP_KEY_ID` | `k1` | Key id written in signed maps |
| `EQ_INTERNAL_TOKEN` or `_FILE` | — | Token of gateway and relay on `/v1/internal` |
| `EQ_MFA_KEY` or `_FILE` | — | 32 bytes in base64 (`eqtool formkey`): encrypts authenticator secrets |
| `EQ_MFA_REQUIRED` | `true` | `false` lets users log in with a password only (tests) |
| `EQ_PANEL_URL` | `http://localhost:8088` | Public address of the panel, used in invitation links |
| `EQ_MAIL_MODE` | `smtp` with `EQ_SMTP_ADDR`, else `off` | `smtp`, `off` (invitations are created but not sent), `log` (link in the log: development only) |
| `EQ_SMTP_ADDR` | — | `host:port`; 465 implicit TLS, otherwise STARTTLS (required) |
| `EQ_SMTP_USER`, `EQ_SMTP_PASSWORD` or `_FILE` | — | SMTP credentials; empty user: no authentication |
| `EQ_MAIL_FROM` | `equalizer@localhost` | Sender of the invitations |
| `EQ_DIRECT_DESTINATIONS` | — | `host:port` list allowed in `direct` mode (lab apps); empty: connectors only |
| `EQ_EXPOSURE_INTERVAL` | `1h` | Interval between exposure checks of a site (at least `5m`; `0` turns them off) |
| `EQ_UI_DIR` | — | Static files of panel-ui, served on `/` |
| `EQ_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

`panel-api create-admin -email E -name N` creates a platform administrator
(password from standard input). Migrations run at start-up.

## Layout

```text
cmd/panel-api/       start, create-admin, healthcheck
internal/config/     environment variables
internal/logging/    JSON logs
internal/db/         connection, embedded migrations (migrations/*.sql)
internal/dbtest/     throwaway PostgreSQL for tests
internal/auth/       passwords, sessions, roles, login throttle
internal/mfa/        TOTP, recovery codes, secret encryption
internal/store/      tenants, users, apps, connectors, maps, audit
internal/mapschema/  map validation (embedded schema copy)
internal/signing/    SignedMap
internal/api/        HTTP handlers
```

## Development

```bash
go vet ./... && go test -race ./...
```

Tests that need PostgreSQL start a throwaway cluster with the local
`initdb`/`postgres` (or `EQ_PG_BIN`) and are skipped without them.
Dependencies: `github.com/lib/pq`, `github.com/santhosh-tekuri/jsonschema/v5`.
