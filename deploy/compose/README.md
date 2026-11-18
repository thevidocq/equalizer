# deploy/compose

Docker Compose for the prototype on a VPS. Full guide in [`INSTALL.md`](../../INSTALL.md).

| Service | Network | Exposed | Notes |
| --- | --- | --- | --- |
| `caddy` | edge | 80, 443 | Automatic TLS, balances the gateway replicas |
| `gateway` | edge, control | no | 2 replicas by default |
| `loki` | control | no | Log store, 30 days |
| `vector` | control | no | Collects logs from containers labelled `eq.collect=true` |
| `grafana` | control | `127.0.0.1:3000` only | Log analysis, over an SSH tunnel |
| `eqtool` | none | no | `tools` profile: keys and map signing |

The `lab` network is `internal`: test apps have no way out to the Internet.
Local folders: `maps/` (signed maps, not in the repository) and `secrets/`
(keys, not in the repository: `map-signing.key`/`.pub` and `form.key`).
