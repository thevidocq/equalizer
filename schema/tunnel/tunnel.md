# Tunnel protocol `equalizer-tunnel/1`

Contract between `connector`, `tunnel-relay` and `gateway`. It replaces the
earlier gRPC draft (`tunnel.proto`): the same guarantees with the Go standard
library plus one small multiplexer ([yamux](https://github.com/hashicorp/yamux)),
and a single public port (443) that goes through the edge proxy.

```text
gateway ──HTTP (control network)──► tunnel-relay ◄══ TLS 443, opened by the connector ══ connector ──► legacy app
```

Guarantees:

- The connector **always** opens the connection (outbound). The legacy
  network needs no inbound port.
- The connector executes requests only towards destinations in **its own
  local allowlist** (`EQ_ALLOW`): a compromised relay or gateway cannot
  widen it. A destination allowlist signed by the panel will be added on
  top, never in place of the local one.
- Requests are already rebuilt by the gateway: relay and connector do not
  change them, apart from hop-by-hop headers.

## 1. Opening the tunnel (connector → relay)

```http
GET /v1/tunnel HTTP/1.1
Host: relay.example.com
Connection: Upgrade
Upgrade: equalizer-tunnel/1
Authorization: Bearer <connector token>
X-Eq-Connector: cn-example-city-01
X-Eq-Connector-Version: 0.1.0
X-Eq-Allow: 10.20.0.15:8080,10.20.0.16:443
```

| Answer | Meaning |
| --- | --- |
| `101 Switching Protocols`, `Upgrade: equalizer-tunnel/1` | Tunnel open: yamux starts on the same connection |
| `401` | Unknown connector or wrong token |
| `426` | Missing or different `Upgrade` (other protocol version) |

- Always over TLS (HTTPS); the edge proxy terminates it and passes the
  upgraded connection to the relay.
- The token is 32 random bytes in base64url. The relay stores only its
  SHA-256 (`connectors.json`, see `tunnel-relay/README.md`).
- `X-Eq-Allow` is informational: the relay logs it and refuses early the
  requests for destinations the connector did not declare. The connector
  enforces its allowlist anyway.
- After the `101`, the connection carries a yamux session: the connector is
  the yamux client, the relay the server. Keep-alive every 15 s.
- Only the relay opens streams.
- On disconnection the connector reconnects with exponential backoff
  (1 s → 30 s, with jitter).
- Several sessions with the same connector id (several connector instances)
  are active together; the relay distributes requests among them.

## 2. Requests on a stream (relay → connector)

Each stream carries HTTP/1.1 requests and responses in the standard wire
format.

Request from the relay:

- The `Host` header is the destination (`host:port`); the request line
  carries path and query.
- `X-Eq-Scheme: http|https` says how to reach the destination.
- `X-Eq-Request-Id` carries the gateway's request id, for the logs.

The connector:

1. Checks that the destination is in its allowlist (exact `host:port`);
   otherwise it answers `403` with `X-Eq-Error: destination_not_allowed`.
2. Removes the `X-Eq-*` headers and the hop-by-hop headers.
3. Executes the request (redirects are not followed, timeout 30 s).
4. Returns status, headers and body as they are.

Connector errors:

| Status | `X-Eq-Error` |
| --- | --- |
| 403 | `destination_not_allowed` |
| 400 | `bad_request` (unknown scheme, malformed request) |
| 502 | `legacy_unreachable` |
| 504 | `legacy_timeout` |

## 3. Gateway → relay (control network)

The gateway sends the rebuilt request to the relay's internal address:

```http
POST /permits/supplement.do HTTP/1.1
Host: tunnel-relay:8082
Authorization: Bearer <gateway token>
X-Eq-Connector: cn-example-city-01
X-Eq-Destination: 10.20.0.15:8080
X-Eq-Scheme: http
X-Eq-Request-Id: r-4ffd4c18cbf386ec
```

- The gateway sends only destinations in the map's `legacy.reach.allow`
  (first check); the connector checks its own allowlist (second check).
- The relay removes `Authorization`, `X-Eq-Connector` and
  `X-Eq-Destination` and sends the request on a stream of one of the
  connector's sessions.

Relay errors (always with `X-Eq-Error`):

| Status | `X-Eq-Error` |
| --- | --- |
| 401 | `unauthorized` (wrong gateway token) |
| 400 | `bad_request` (missing headers) |
| 502 | `connector_unavailable` (no open session) |
| 403 | `destination_not_declared` (not in the connector's `X-Eq-Allow`) |
| 403, 400, 502, 504 | errors propagated from the connector |

## 4. Versions

The protocol version is the `Upgrade` token. A relay may accept several
versions; a connector with an unknown version gets `426`.
