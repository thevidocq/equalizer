# Installation

> **Status:** prototype under construction. Steps marked *(available)* already
> have code and configuration in the repository; the others describe the
> planned installation and will be completed over time (see `CHANGELOG.md`).
> Today the gateway serves pages and urlencoded POST forms of a legacy
> application reached **directly** (`legacy.reach.mode: direct`) or **through
> the connector** (`mode: connector`), with faithful rendering and
> pass-through login (sessions in Redis), and the web panel (tenants, sites,
> connectors, people, maps with approval and signing, activity). File
> uploads, service status and log pages are not available yet.

The prototype runs on **one VPS** with Docker Compose. The legacy application
to protect can be anywhere: the **connector**, installed next to it, reaches it.

## 1. Requirements

### VPS

- 4 vCPU, 16 GB RAM, 160 GB SSD.
- Ubuntu 24.04 LTS.
- A DNS name for the gateway (e.g. `gw.example.com`) and one for the relay
  (e.g. `relay.example.com`), both pointing to the VPS IP. If you use
  Cloudflare, leave the relay record **DNS only** (grey cloud): the tunnel is
  a long-lived upgraded connection.

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
```

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
| `EQ_RELAY_DOMAIN` | `relay.example.com` | Public domain of the relay, where connectors open their tunnel |
| `EQ_PANEL_DOMAIN` | `panel.example.com` | Public domain of the panel (customers, invitation links) |
| `EQ_ACME_EMAIL` | `admin@example.com` | Email for Let's Encrypt |
| `EQ_DIRECT_DESTINATIONS` | `vuln-app:80` | Lab only: destinations a map may reach without a connector |
| `EQ_EXPOSURE_INTERVAL` | `1h` | How often protected sites are checked on their old IP |
| `EQ_GATEWAY_REPLICAS` | `2` | Gateway replicas |
| `EQ_LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `EQ_MAP_KEY_ID` | `k1` | Identifier of the map signing key |
| `EQ_GRAFANA_PASSWORD` | *(choose one)* | Password of the Grafana `admin` user |
| `EQ_UID` / `EQ_GID` | `1000` | VPS user owning the files created by `eqtool` |

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

Invitations are sent by email: set `EQ_SMTP_ADDR`, `EQ_SMTP_USER` and
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
curl -I https://gw.example.com/            # 503 until a map is published, then the legacy page
curl -I https://gw.example.com/wp-admin    # not mapped: 404
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
   on that server (step 7). The same connector serves all its sites.
4. **People → Invite a person** with the *Approver* role: maps are approved by
   someone other than their author.
5. **Map → New version**: write the map in YAML (the editor checks the
   syntax, the panel checks the format), **Save draft**, **Send for
   review**. The approver opens the version, **Approve**, **Publish**.

The same steps are available on the API (`/v1`, see
`schema/api/panel-api.yaml`); every request that changes something needs
the header `X-Eq-Panel: 1`.

6. Point the site's DNS record to the VPS. The gateway downloads every
   published map from the panel (checked every 15 seconds), verifies its
   signature and serves it on the site's domain; Caddy gets the certificate
   at the first visit, only for domains with a published map.
7. **Site → Isolation**: enter the site's old public IP and apply the
   configuration it shows on the web server, so the site no longer answers
   there. The status turns green when the check cannot reach it.

A version is approved and published by someone other than its author;
every action is in `GET /v1/audit`.

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

The panel's Logs page will arrive with panel-ui; Grafana stays for in-depth analysis.

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
- The image is published by CI on every merge to `main`. GitHub packages
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
| Services running | Panel → **Service status**, everything "operational" |
| Connector | Panel → **Connectors**: sites using it; tunnel in `docker compose logs tunnel-relay` |
| Isolation | Panel → site → **Isolation**: "Not reachable directly" |
| Full path | End-to-end synthetic check green |
| Logs | Panel → **Logs**, filter by service |
| Exposure | After cutover, the legacy application does not answer from the Internet |

## 9. Update

```bash
git pull
cd deploy/compose
docker compose pull
docker compose up -d --build
```

## 10. Uninstall

```bash
cd deploy/compose
docker compose down          # add -v to delete data as well
docker rm -f eq-connector    # on the connector machine
```
