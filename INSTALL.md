# Installation

> **Status:** prototype under construction. Steps marked *(available)* already
> have code and configuration in the repository; the others describe the
> planned installation and will be completed over time (see `CHANGELOG.md`).
> Today the gateway serves pages and urlencoded POST forms of a legacy
> application reached **directly** (`legacy.reach.mode: direct`) or **through
> the connector** (`mode: connector`), with faithful rendering and
> pass-through login (sessions in Redis), and the panel API (tenants, sites,
> connectors, maps with approval and signing). The panel web interface and
> file uploads are not available yet.

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

# Firewall: only gateway and relay (80/443), SSH and the admin VPN
sudo ufw default deny incoming
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 51820/udp        # WireGuard to reach the panel
sudo ufw enable
```

The panel is **not** exposed to the Internet: reach it over a WireGuard VPN
or an SSH tunnel (`ssh -L 8080:localhost:8080 user@vps`).

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
| `EQ_GATEWAY_DOMAIN` | `gw.example.com` | Public domain of the gateway (automatic certificate) |
| `EQ_RELAY_DOMAIN` | `relay.example.com` | Public domain of the relay, where connectors open their tunnel |
| `EQ_ACME_EMAIL` | `admin@example.com` | Email for Let's Encrypt |
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
```

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

The panel listens only on the VPS localhost. From your computer:

```bash
ssh -L 8088:localhost:8088 user@vps
```

The web interface arrives with panel-ui (next step); until then, the panel
API at `http://localhost:8088` does everything. With `curl`:

```bash
P=http://localhost:8088
api() { curl -s -b .panel -c .panel -H "X-Eq-Panel: 1" -H "Content-Type: application/json" -X "$1" "$P$2" ${3:+-d "$3"}; }

api POST /v1/auth/login '{"email":"you@example.com","password":"..."}'
api POST /v1/tenants '{"id":"lab","name":"Lab"}'
api POST /v1/tenants/lab/apps '{"id":"vuln-app","name":"Vulnerable app","public_domain":"vuln.example.com"}'
api POST /v1/tenants/lab/users '{"email":"approver@example.com","name":"Approver","password":"...","role":"approver"}'
api POST /v1/apps/vuln-app/connectors '{"id":"cn-lab-01"}'     # prints the connector token, once

# a map: YAML → JSON (validated), then a draft in the panel
python3 scripts/map_to_json.py my-map.yaml > /tmp/map.json
api POST /v1/apps/vuln-app/maps "$(python3 -c 'import json;print(json.dumps({"document":json.load(open("/tmp/map.json"))}))')"
api POST /v1/apps/vuln-app/maps/1/submit
# the approver (another person, logged in with their own account):
api POST /v1/apps/vuln-app/maps/1/approve
api POST /v1/apps/vuln-app/maps/1/publish
```

Then set `EQ_APP=vuln-app` in `.env` and run `docker compose up -d`: the
gateway downloads the published map from the panel (checked every 15
seconds) and verifies its signature. A version is approved and published by
someone other than its author; every action is in `GET /v1/audit`.

For a legacy app reached through the connector, the map says:

```yaml
legacy:
  base_url: http://10.20.0.15:8080        # address as seen from the connector
  reach:
    mode: connector
    connector_id: cn-lab-01                # as registered in the panel
    allow: ["10.20.0.15:8080"]
```

The relay reads the connectors from the panel every 15 seconds: new
connectors need no restart; deleting one or rotating its token
(`POST /v1/apps/vuln-app/connectors/cn-lab-01/rotate`) closes its open
tunnels.

Without the panel (`EQ_APP` empty) the gateway still reads
`maps/current.json`, signed with `eqtool sign`.

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

## 7. Install the connector next to the legacy application *(available)*

On a machine with Docker on the legacy application's network (the remote
server itself is fine):

```bash
docker run -d --name eq-connector --restart unless-stopped \
  -e EQ_RELAY_URL=https://relay.example.com \
  -e EQ_CONNECTOR_ID=cn-lab-01 \
  -e EQ_CONNECTOR_TOKEN=<token printed by the panel> \
  -e EQ_ALLOW=10.20.0.15:8080 \
  ghcr.io/thevidocq/equalizer-connector:latest
```

- `EQ_ALLOW` lists the only `host:port` destinations the connector will
  reach, whatever it is asked. If the legacy app runs on the same machine,
  use its LAN address or, with `--network host`, `127.0.0.1:<port>`.
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
| Connector | Panel → site → **Connectors**: online, latency shown |
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
