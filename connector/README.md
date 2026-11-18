# connector

Go service installed next to the legacy application. It opens the tunnel
towards the relay (**outbound only**, HTTPS on 443) and executes the requests
that arrive on it, only towards the destinations of its **local allowlist**.
Contract: [`schema/tunnel/tunnel.md`](../schema/tunnel/tunnel.md).

## Run

```bash
docker run -d --name eq-connector --restart unless-stopped --network host \
  -e EQ_RELAY_URL=https://relay.example.com \
  -e EQ_CONNECTOR_ID=cn-example-city-01 \
  -e EQ_CONNECTOR_TOKEN=<token from eqtool connector> \
  -e EQ_ALLOW=127.0.0.1:8443,10.20.0.15:443 \
  ghcr.io/thevidocq/equalizer-connector:latest
```

## Configuration

One connector per server serves every site of the tenant on it. For sites
that share an address (virtual hosts) the gateway sends the site's name in
`X-Eq-Host`: the connector presents it as `Host` and TLS SNI, but always
connects to the allowed destination. `--network host` lets it reach sites
that listen only on the server's `127.0.0.1`.

| Variable | Default | Meaning |
| --- | --- | --- |
| `EQ_RELAY_URL` | — | Relay address, `https://...` (required) |
| `EQ_CONNECTOR_ID` | — | Connector id, as registered on the relay (required) |
| `EQ_CONNECTOR_TOKEN` or `EQ_CONNECTOR_TOKEN_FILE` | — | Connector token (required) |
| `EQ_ALLOW` | — | Allowed destinations, `host:port,host:port` (required). Nothing else can be reached. One entry serves every virtual host on that address |
| `EQ_LEGACY_CA_FILE` | — | CA (PEM) to trust for `https` legacy destinations |
| `EQ_LEGACY_INSECURE_TLS` | `false` | `true` skips certificate checks towards the legacy app (self-signed certificates on the local network; last resort) |
| `EQ_RELAY_CA_FILE` | — | CA (PEM) of the relay, only if it does not use a public certificate |
| `EQ_RELAY_INSECURE_HTTP` | `false` | `true` allows an `http://` relay, **only for local tests** |
| `HTTPS_PROXY` | — | Corporate proxy towards the relay, if needed |
| `EQ_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

The Docker healthcheck is healthy while the tunnel is open.

## Layout

```text
cmd/connector/       startup, signals, healthcheck
internal/config/     environment variables, TLS
internal/logging/    JSON logs (schema/events/log.schema.json)
internal/allowlist/  local destinations
internal/tunnel/     handshake, yamux client, reconnection with backoff
internal/executor/   requests from the tunnel → legacy application
```

## Development

```bash
go vet ./... && go test -race ./...
```

Dependency: `github.com/hashicorp/yamux` (MPL-2.0), stream multiplexing.
