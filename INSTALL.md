# Installation

> **Status:** prototype under construction. Steps marked *(available)* already
> have code and configuration in the repository; the others describe the
> planned installation and will be completed over time (see `CHANGELOG.md`).
> Today the gateway serves read-only pages of a legacy application reached
> **directly** (`legacy.reach.mode: direct`), with faithful rendering and URL
> rewriting, and accepts urlencoded POST forms declared in the map; with
> `mode: connector` it answers 502 until the connector is ready. File uploads
> and login are not served yet.

The prototype runs on **one VPS** with Docker Compose. The legacy application
to protect can be anywhere: the **connector**, installed next to it, reaches it.

## 1. Requirements

### VPS

- 4 vCPU, 16 GB RAM, 160 GB SSD.
- Ubuntu 24.04 LTS.
- A DNS name for the gateway (e.g. `gw.example.com`) and one for the relay
  (e.g. `relay.example.com`), both pointing to the VPS IP.

### Connector machine

- Linux with Docker, on the same network as the legacy application.
- Outbound Internet access on port **8443** (to the relay).
- No inbound port needed.

## 2. Prepare the VPS

```bash
# Docker and the Compose plugin
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER   # then log out and back in

# Firewall: only gateway (80/443), relay (8443), SSH and the admin VPN
sudo ufw default deny incoming
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 8443/tcp
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
nano .env        # domain, email, Grafana password, EQ_UID/EQ_GID (id -u, id -g)
```

Main values in `.env`:

| Variable | Example | Meaning |
| --- | --- | --- |
| `EQ_GATEWAY_DOMAIN` | `gw.example.com` | Public domain of the gateway (automatic certificate) |
| `EQ_ACME_EMAIL` | `admin@example.com` | Email for Let's Encrypt |
| `EQ_GATEWAY_REPLICAS` | `2` | Gateway replicas |
| `EQ_LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `EQ_MAP_KEY_ID` | `k1` | Identifier of the map signing key |
| `EQ_GRAFANA_PASSWORD` | *(choose one)* | Password of the Grafana `admin` user |
| `EQ_UID` / `EQ_GID` | `1000` | VPS user owning the files created by `eqtool` |

## 5. Signing key and first map *(available)*

Until the panel exists, maps are signed by hand with `eqtool`.

```bash
# from deploy/compose
docker compose --profile tools run --rm eqtool keygen -out secrets/map-signing.key
# key that seals form tokens, shared by the gateway replicas
docker compose --profile tools run --rm eqtool formkey -out secrets/form.key

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

## 7. Install the connector next to the legacy application

1. In the panel: **Protected sites → Protect a site**, enter the public
   domain and the internal address of the legacy application.
2. In the **Connectors** tab generate an enrolment token (one-time, valid
   24 hours).
3. On the connector machine:

```bash
docker run -d --name eq-connector --restart unless-stopped \
  -e RELAY_URL=relay.example.com:8443 \
  -e ENROLL_TOKEN=<token from the panel> \
  equalizer/connector:latest
```

4. In the panel the connector shows as **online** within a few seconds.

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
