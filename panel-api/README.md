# panel-api

Go control-plane API. Contract: [`schema/api/panel-api.yaml`](../schema/api/panel-api.yaml).

## Implemented

| Area | Endpoints | Who |
| --- | --- | --- |
| Auth | `POST /v1/auth/login`, `POST /v1/auth/logout`, `GET /v1/auth/me`, `POST /v1/auth/mfa/{setup,enable,verify}`, `POST /v1/auth/password` | everyone |
| Tenants | `GET/POST /v1/tenants` | create: platform admin |
| Users | `GET/POST /v1/tenants/{tenant}/users`, `DELETE …/{id}` (deactivate), `POST …/{id}/reset-mfa` | tenant admin, platform admin |
| Sites | `GET/POST /v1/tenants/{tenant}/apps`, `GET /v1/apps/{app}` | create: tenant admin |
| Connectors | `GET/POST /v1/apps/{app}/connectors`, `POST …/{id}/rotate`, `DELETE …/{id}` | tenant admin; token shown once |
| Maps | `GET/POST /v1/apps/{app}/maps`, `GET …/{version}`, `POST …/{version}/{submit,approve,reject,publish}` | draft/submit: editor; approve/reject/publish: approver, never the author |
| Audit | `GET /v1/audit` | tenant admin, approver, auditor; platform |
| Internal | `GET /v1/internal/apps/{app}/map` (gateway, ETag), `GET /v1/internal/connectors` (relay) | service token |

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
- Maps are validated against `schema/map` (embedded copy, kept equal by
  `scripts/validate_schemas.py`); `meta.tenant`, `meta.app`, `meta.version`
  and `meta.status` are set by the panel. Publication signs the exact
  payload with Ed25519; at most one published version per site (database
  constraint); the approver is never the author (database constraint).
- Every change is in the audit log (never passwords or tokens).

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `EQ_ADDR` | `:8080` | API (localhost or VPN only) |
| `EQ_ADMIN_ADDR` | `:9092` | `/healthz`, `/readyz` |
| `EQ_DATABASE_URL` | — | `postgres://user@host:5432/db?sslmode=…` |
| `EQ_DATABASE_PASSWORD` or `_FILE` | — | Database password, kept out of the URL |
| `EQ_MAP_SIGNING_KEY` or `_FILE` | — | Ed25519 seed in base64 (`eqtool keygen`) |
| `EQ_MAP_KEY_ID` | `k1` | Key id written in signed maps |
| `EQ_INTERNAL_TOKEN` or `_FILE` | — | Token of gateway and relay on `/v1/internal` |
| `EQ_MFA_KEY` or `_FILE` | — | 32 bytes in base64 (`eqtool formkey`): encrypts authenticator secrets |
| `EQ_MFA_REQUIRED` | `true` | `false` lets users log in with a password only (tests) |
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
