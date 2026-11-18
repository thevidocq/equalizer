# Installation

> **Status:** prototype under construction. Steps marked *(available)* already
> have code and configuration in the repository; the others describe the
> planned installation and will be completed over time (see `CHANGELOG.md`).
> Today the gateway serves pages and urlencoded POST forms of a legacy
> application reached **directly** (`legacy.reach.mode: direct`) or **through
> the connector** (`mode: connector`), with faithful rendering and
> pass-through login (sessions in Redis). File uploads and the panel are not
> available yet.

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

## 5. Keys, connector and first map *(available)*

Until the panel exists, maps are signed by hand with `eqtool`.

```bash
# from deploy/compose
docker compose --profile tools run --rm eqtool keygen -out secrets/map-signing.key
# key that seals form tokens, shared by the gateway replicas
docker compose --profile tools run --rm eqtool formkey -out secrets/form.key
# token the gateway presents to the relay
docker compose --profile tools run --rm eqtool secret -out secrets/relay.token
# register the connector: copy the EQ_CONNECTOR_TOKEN it prints (shown once)
docker compose --profile tools run --rm eqtool connector -file secrets/connectors.json \
  -id cn-lab-01 -tenant lab -app vulnerable-app

# convert and validate a YAML map (from the repository root)
cd ../..
pip install jsonschema pyyaml
python3 scripts/map_to_json.py schema/map/examples/permits-portal.yaml > deploy/compose/maps/map.json

# sign the map: the gateway reads maps/current.json
cd deploy/compose
docker compose --profile tools run --rm eqtool sign \
  -key secrets/map-signing.key -map maps/map.json -key-id k1 -out maps/current.json
```

The gateway reloads the map by itself when `maps/current.json` changes (within 15 seconds).

For a legacy app reached through the connector, the map says:

```yaml
legacy:
  base_url: http://10.20.0.15:8080        # address as seen from the connector
  reach:
    mode: connector
    connector_id: cn-lab-01                # as registered above
    allow: ["10.20.0.15:8080"]
```

The relay rereads `secrets/connectors.json` every 15 seconds: new connectors
need no restart; removing one or rotating its token (`-rotate`) closes its
open tunnels.

## 6. Start *(available)*

```bash
docker compose up -d --build
docker compose ps            # gateway "healthy", two replicas
```

Quick checks:

```bash
curl -I https://gw.example.com/            # mapped route: legacy page (mode: direct) or 502 (mode: connector)
curl -I https://gw.example.com/wp-admin    # not mapped: 404
```

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
  -e EQ_CONNECTOR_TOKEN=<token printed by eqtool connector> \
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
