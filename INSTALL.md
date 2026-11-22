# Installation

> **Status:** prototype under construction. Steps marked *(available)* already
> have code and configuration in the repository; the others describe the
> planned installation and will be completed over time (see `CHANGELOG.md`).
> Today the gateway serves pages, forms (with file uploads) and JSON APIs of
> legacy applications reached **directly** (`legacy.reach.mode: direct`) or
> **through the connector** (`mode: connector`), with faithful rendering,
> pass-through login and authorisation per object; the web panel has
> tenants, sites, connectors, people, maps with templates, discovery, review
> aid, approval, signing, import and export, isolation, status, logs and
> activity. Not available yet: SIEM forwarding, capacity from the panel,
> OIDC.

The prototype runs on **one VPS** with Docker Compose. The legacy application
to protect can be anywhere: the **connector**, installed next to it, reaches it.

## 1. Requirements

### VPS

- 4 vCPU, 16 GB RAM, 160 GB SSD.
- Ubuntu 24.04 LTS.
- Two DNS names pointing to the VPS IP: the panel (e.g.
  `panel.example.com`) and the relay (e.g. `relay.example.com`). If you use
  Cloudflare, leave the relay record **DNS only** (grey cloud): the tunnel is
  a long-lived upgraded connection.
- The gateway has no name of its own: each protected site keeps its domain
  (e.g. `permits.example-city.gov`), and its DNS record moves to the VPS IP
  when the site goes live (section 6).

### Connector machine

- Linux with Docker, on the same network as the legacy application.
- Outbound Internet access on port **443** (HTTPS to the relay; a corporate
  proxy works through `HTTPS_PROXY`).
- No inbound port needed.

## 2. Prepare the VPS

```bash
# Docker and the Compose plugin
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER   # then log out and back in

# Firewall: gateway, relay and panel (80/443), SSH and the admin VPN
sudo ufw default deny incoming
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 51820/udp        # WireGuard to reach the panel
sudo ufw enable
```

Customers reach the panel on its own domain (`EQ_PANEL_DOMAIN`), behind
Caddy; the API for gateway and relay (`/v1/internal`) is never published.
The platform team can also use the VPS localhost port 8088 over a WireGuard
VPN or an SSH tunnel.

## 3. Get the project *(available)*

```bash
git clone https://github.com/thevidocq/equalizer.git
cd equalizer
git checkout v0.2.9        # a release; main is the development version
```

Releases and what they contain: `docs/releases/`.

Check the contracts *(available)*:

```bash
pip install jsonschema pyyaml openapi-spec-validator
python3 scripts/validate_schemas.py
```

## 4. Configure *(available)*

```bash
cd deploy/compose
cp .env.example .env
nano .env        # domains, email, Grafana password, EQ_UID/EQ_GID (id -u, id -g)
```

Main values in `.env`:

| Variable | Example | Meaning |
| --- | --- | --- |
| `EQ_VERSION` | `0.2.9` | Release being installed (shown by the services; `dev` when unset) |
| `EQ_RELAY_DOMAIN` | `relay.example.com` | Public domain of the relay, where connectors open their tunnel |
| `EQ_PANEL_DOMAIN` | `panel.example.com` | Public domain of the panel (customers, invitation links) |
| `EQ_ACME_EMAIL` | `admin@example.com` | Email for Let's Encrypt |
| `EQ_DIRECT_DESTINATIONS` | `vuln-app:80` | Lab only: private destinations a map may reach without a connector. Each entry is `host:port` (the port is required); a malformed entry stops `panel-api` at start. Not needed for a public site reached directly: its IP or name goes in the site's reach |
| `EQ_OUTBOUND_IPS` | `203.0.113.5` | The address(es) this server connects from, shown to customers who reach a site directly. Empty: worked out from the panel and relay names — wrong when the panel is behind Cloudflare (it would show Cloudflare's addresses), so set it then. Find it with `curl -4 ifconfig.me` on the VPS |
| `EQ_EXPOSURE_INTERVAL` | `1h` | How often protected sites are checked on their old IP |
| `EQ_GATEWAY_REPLICAS` | `2` | Gateway replicas |
| `EQ_LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `EQ_MAP_KEY_ID` | `k1` | Identifier of the map signing key |
| `EQ_GRAFANA_PASSWORD` | *(choose one)* | Password of the Grafana `admin` user |
| `EQ_UID` / `EQ_GID` | `1000` | VPS user owning the files created by `eqtool` |
| `EQ_BEHIND_CLOUDFLARE` | `false` | `true` only when protected sites are reachable through Cloudflare alone (proxied records, VPS firewall open to Cloudflare only): the visitor's IP is then read from `CF-Connecting-IP` |

Connectors and the VPS must run the same release: an older connector
does not pass the legacy site's own HTTP login (Basic, Bearer).

## 5. Keys and secrets *(available)*

```bash
# from deploy/compose
docker compose --profile tools run --rm eqtool keygen  -out secrets/map-signing.key   # panel signs, gateway verifies
docker compose --profile tools run --rm eqtool formkey -out secrets/form.key          # gateway forms and sessions
docker compose --profile tools run --rm eqtool secret  -out secrets/relay.token       # gateway → relay
docker compose --profile tools run --rm eqtool secret  -out secrets/panel-internal.token  # gateway and relay → panel
docker compose --profile tools run --rm eqtool secret  -out secrets/postgres.password
docker compose --profile tools run --rm eqtool formkey -out secrets/mfa.key           # panel authenticator secrets
# SMTP password for invitation emails (leave the file empty if the server needs none)
install -m 600 /dev/null secrets/smtp.password && nano secrets/smtp.password
```

Invitations, forgotten-password links and the customers' alerts are sent
by email: set `EQ_SMTP_ADDR`, `EQ_SMTP_USER` and
`EQ_MAIL_FROM` in `.env` (port 587 STARTTLS or 465 TLS). Without a mail
server set `EQ_MAIL_MODE=off`: tenants are created but nobody can join, so
use it only for a first test.

All files are created `0600` and owned by `EQ_UID`/`EQ_GID`; the services
that read them run as that user.

## 6. Start *(available)*

```bash
docker compose up -d --build
docker compose ps            # gateway "healthy", two replicas
```

Quick checks:

```bash
curl -I https://panel.example.com/                  # the panel
curl -I https://permits.example-city.gov/           # once its map is published: the site
curl -I https://permits.example-city.gov/wp-admin   # served too: the whole site, admin included
```

### First administrator and first site *(available)*

```bash
docker compose run --rm -i panel-api create-admin -email you@example.com -name "Your Name"
# type the password (at least 12 characters) and press Enter
```

Open `https://panel.example.com` (your `EQ_PANEL_DOMAIN`), or
`http://localhost:8088` through `ssh -L 8088:localhost:8088 user@vps`, and
log in. The first time, the panel asks you
to link an authenticator app (scan the QR code, type a code) and shows ten
recovery codes once: keep them. From then on every login asks for the code
of the app. Then:

1. **Tenants → Add a customer** (for the lab, e.g. "Lab"), with the name
   and email of its administrator. They receive a link (single use, 48
   hours), choose a password and set up an authenticator. As platform
   administrator you then see only the tenant's name and its sites' names
   and status: everything below is done by the tenant's administrator.
   (For a lab you can invite yourself with a second email address.)
2. In the tenant: **+ Protect a site**, with its name and public domain.
3. **Connectors → Add a connector**, one per legacy server: copy the
   `docker run` command it shows (the token appears only once) and run it
   on that server (step 7). The same connector serves all its sites. A
   site that stays at its public address and accepts only this server's
   address needs no connector: choose **Directly** in the next step.
4. **People → Invite a person**, if more people look after the sites:
   administrators do everything, viewers read.
5. **Map → New version**: start from the template of the site's CMS (or
   import or write the map in YAML), **Save draft**, **Publish**.

The same steps are available on the API (`/v1`, see
`schema/api/panel-api.yaml`); every request that changes something needs
the header `X-Eq-Panel: 1`.

6. Point the site's DNS record to the VPS. The gateway downloads every
   published map from the panel (checked every 15 seconds), verifies its
   signature and serves it on the site's domain. In https the site uses its
   own certificate, uploaded in **Site → Certificate**: Equalizer never
   issues one for a protected site. A site without one can stay in http.
7. **Site → Isolation**: enter the site's old public IP and apply the
   configuration it shows on the web server, so the site no longer answers
   there. The status turns green when the check cannot reach it.

An administrator publishes a version alone; every action is in
`GET /v1/audit`.

For a site reached through the connector, the map says:

```yaml
legacy:
  base_url: https://permits.example-city.gov   # name the web server knows
  reach:
    mode: connector
    connector_id: cn-lab-01                    # a connector of this tenant
    address: "10.20.0.15:443"                  # where to connect (virtual hosts)
    allow: ["10.20.0.15:443"]
```

The relay reads the connectors from the panel every 15 seconds: new
connectors need no restart; deleting one or rotating its token closes its
open tunnels.

With `EQ_MAP_URL=` (empty) in `.env` the gateway reads
`maps/current.json` instead, signed with `eqtool sign` (lab without panel).

### Viewing logs *(available)*

Grafana listens only on the VPS localhost. From your computer:

```bash
ssh -L 3000:localhost:3000 user@vps
```

Open `http://localhost:3000` (user `admin`, password from `.env`), go to
**Explore**, source **Loki**, and use queries such as:

```text
{service="gateway"}                                  all gateway logs
{service="gateway", stream="security"}               security events only
{service="gateway"} |= "r-4ffd4c18cbf386ec"           one specific request
{service="gateway", level="error"}                    errors only
```

The panel's **Logs** page (tenant administrators and viewers) reads the security events from Loki (`EQ_LOKI_URL`, already set in Compose); Grafana stays for in-depth analysis.

### File uploads *(available)*

Forms with files work with no extra service: the gateway checks the real
type and the size, renames the file and can rebuild images (`scan: cdr`).
It does not scan for viruses: keep that in the security stack around the
legacy application (server antivirus, EDR, mail and file gateways).

## 7. Install the connector on the legacy server *(available)*

One per server, whatever the number of sites on it. On the server itself
(or a machine with Docker on its network):

```bash
docker run -d --name eq-connector --restart unless-stopped --network host \
  -e EQ_RELAY_URL=https://relay.example.com \
  -e EQ_CONNECTOR_ID=cn-lab-01 \
  -e EQ_CONNECTOR_TOKEN=<token printed by the panel> \
  -e EQ_ALLOW=127.0.0.1:8443,10.20.0.15:443 \
  ghcr.io/thevidocq/equalizer-connector:latest
```

- `EQ_ALLOW` lists the only `host:port` destinations the connector will
  reach, whatever it is asked: the addresses of the protected sites.
  Virtual hosts on one address need one entry; the site is chosen by name.
- `--network host` lets it reach sites that listen only on `127.0.0.1`,
  the recommended way to take a protected site off the public IP.
- For an `https` legacy app with a self-signed certificate: mount its CA and
  set `EQ_LEGACY_CA_FILE`, or as a last resort `EQ_LEGACY_INSECURE_TLS=true`.
- Use the connector of the same release as the platform:
  `ghcr.io/thevidocq/equalizer-connector:0.2.9` (`latest` follows `main`).
- The image is published by CI on every merge to `main` and on every
  release tag. GitHub packages
  start private: make `equalizer-connector` public in the repository's
  *Packages* settings, or run `docker login ghcr.io` first. Without the
  registry, build it on the machine:
  `git clone https://github.com/thevidocq/equalizer.git && docker build -t equalizer/connector equalizer/connector`.

Check that the tunnel is open:

```bash
docker logs eq-connector | grep "tunnel open"       # on the connector machine
docker compose logs tunnel-relay | grep "connector connected"
```

## 8. Checks

| Check | How |
| --- | --- |
| Services running | `docker compose ps`; as platform administrator, **Platform status**: everything "Working" |
| Connector | Tenant → **Status**: the site's connector "online"; tunnel in `docker compose logs tunnel-relay` |
| Isolation | Tenant → site → **Isolation**: "Not reachable directly" |
| Full path | Open the site's domain: the page answers; tenant → **Logs** shows it as served |
| Logs | Tenant → **Logs**; platform logs in Grafana (above) |
| Exposure | After cutover, the legacy application does not answer from the Internet |

## 9. Update

Move to the new release, set it in `.env`, rebuild:

```bash
git fetch --tags
git checkout v0.2.9            # the release to install
cd deploy/compose
sed -i 's/^EQ_VERSION=.*/EQ_VERSION=0.2.9/' .env
docker compose pull            # third-party images (Caddy, Postgres, …)
docker compose up -d --build
```

Database migrations run on their own when `panel-api` starts. Update the
connectors to the same release: on each connector's server, recreate it
with the same settings (token included) and the new image:

```bash
v=0.2.9   # the release you are installing
docker pull ghcr.io/thevidocq/equalizer-connector:$v
docker inspect eq-connector --format '{{range .Config.Env}}{{println .}}{{end}}' | grep '^EQ_' > ~/eq-connector.env
chmod 600 ~/eq-connector.env      # it holds the connector token
docker rm -f eq-connector
docker run -d --name eq-connector --restart unless-stopped --network host \
  --env-file ~/eq-connector.env ghcr.io/thevidocq/equalizer-connector:$v
```

**Always check after an update**, from `deploy/compose`:

```bash
./check.sh            # or ./check.sh <connector container name>
```

It shows the services and their state, the release they run (it must be
`EQ_VERSION`), the migrations applied, errors in the last 10 minutes and the
connector (release and tunnel), and ends with `OK` or the problems found. A
"panel unreachable" error in the first seconds after the restart is normal
(gateway and relay retry while `panel-api` starts) and is not counted. Then,
in the panel: the release at the bottom of the sidebar and **Connectors**
online with the same release.

## 10. Backup and restore

`deploy/compose/backup.sh` saves the panel database (tenants, people, maps,
connectors, settings), the secrets and `.env` in one archive; logs are not
included (they expire after 30 days). With `EQ_BACKUP_PASSPHRASE` set the
archive is encrypted; without it, it holds the secrets in clear.

```bash
cd deploy/compose
EQ_BACKUP_PASSPHRASE='a long passphrase' ./backup.sh /var/backups/equalizer
```

Every night, keeping the last 14 (`EQ_BACKUP_KEEP`), with `crontab -e`:

```text
17 3 * * * cd /opt/equalizer/deploy/compose && EQ_BACKUP_PASSPHRASE=$(cat /root/.eq-backup-pass) ./backup.sh /var/backups/equalizer >> /var/log/equalizer-backup.log 2>&1
```

Copy the archives off the VPS (another server, object storage). To
restore, on the same VPS or a new one with the same release checked out:

```bash
EQ_BACKUP_PASSPHRASE='a long passphrase' ./restore.sh /var/backups/equalizer/equalizer-backup-20261119T031700Z.tar.gz.enc
```

It asks to type `RESTORE`, replaces the panel database and puts back the
secrets and `.env` only where they are missing (`EQ_RESTORE_SECRETS=yes`
to overwrite them). Gateways reload the published maps within a minute.

## 11. Uninstall

```bash
cd deploy/compose
docker compose down          # add -v to delete data as well
docker rm -f eq-connector    # on the connector machine
```
