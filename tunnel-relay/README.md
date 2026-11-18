# tunnel-relay

Go service that terminates the tunnels opened by connectors and forwards the
gateway's rebuilt requests into them. Contract: [`schema/tunnel/tunnel.md`](../schema/tunnel/tunnel.md).

## Ports

| Variable | Default | Who calls it |
| --- | --- | --- |
| `EQ_TUNNEL_ADDR` | `:8081` | Connectors, through the edge proxy (Caddy terminates TLS on 443) |
| `EQ_INTERNAL_ADDR` | `:8082` | Gateway, control network only |
| `EQ_ADMIN_ADDR` | `:9091` | `/healthz`, `/readyz`, `/v1/tunnels` (open tunnels), control network only |

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `EQ_CONNECTORS_FILE` | `/secrets/connectors.json` | Registered connectors; reloaded every 15 s and on `SIGHUP` |
| `EQ_GATEWAY_TOKEN` or `EQ_GATEWAY_TOKEN_FILE` | — | Token the gateway presents (required, at least 32 characters) |
| `EQ_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `EQ_INSTANCE` | hostname | Instance name in logs |

`connectors.json` (create entries with `eqtool connector`):

```json
{
  "connectors": [
    { "id": "cn-example-city-01", "tenant": "example-city",
      "token_sha256": "<hex SHA-256 of the connector token>" }
  ]
}
```

Removing a connector or changing its token closes its open tunnels at the
next reload.

## Layout

```text
cmd/relay/           startup, signals, reload loop, healthcheck
internal/config/     environment variables
internal/logging/    JSON logs and events (schema/events)
internal/registry/   connectors.json, token check
internal/sessions/   open tunnels by connector, round robin
internal/tunnel/     upgrade to equalizer-tunnel/1, yamux server
internal/forward/    gateway requests → tunnel streams
internal/admin/      health and tunnel list
```

Logs: `connector connected`, `connector disconnected`, `tunnel refused`.
Events: `request.blocked` (`tunnel_unauthorized`), `connector.down` when the
last tunnel of a connector closes.

## Development

```bash
go vet ./... && go test -race ./...
```

Dependency: `github.com/hashicorp/yamux` (MPL-2.0), stream multiplexing.
