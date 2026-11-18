# Equalizer

Equalizer protects legacy web applications that cannot be updated. It hides
them from the Internet and serves an **identical copy**, rebuilt on every
request by a gateway that only lets through what is declared in an allowlist
map.

> **Status:** in development — prototype. See [`CHANGELOG.md`](CHANGELOG.md)
> for what is already implemented.

## How it works, in short

```text
users → gateway (only exposed part) → tunnel-relay ⇠ outbound tunnel ⇠ connector → legacy app
                    ↑ signed map
panel → panel-api ← proposals ← AI agent (crawls the legacy app)
```

- The legacy application is never reachable from the Internet: it only
  answers the connector installed next to it, which opens the tunnel **from
  the inside**.
- Every request is **rebuilt**, never forwarded as is.
- An AI agent proposes the map; people approve it from a multitenant panel.

## Documentation

| Document | Contents |
| --- | --- |
| [`docs/architecture.md`](docs/architecture.md) | Architecture, components, design rules |
| [`docs/map-format.md`](docs/map-format.md) | Map format |
| [`INSTALL.md`](INSTALL.md) | Installation (VPS, connector) |
| [`docs/manual/admin.md`](docs/manual/admin.md) | Platform administrator manual |
| [`docs/manual/tenant.md`](docs/manual/tenant.md) | Tenant administrator manual |
| [`CHANGELOG.md`](CHANGELOG.md) | What changes in each commit, and next steps |
| [`CLAUDE.md`](CLAUDE.md) | Instructions for whoever picks up development (people or Claude) |

## Repository layout

| Folder | Contents | Language |
| --- | --- | --- |
| `schema/` | Contracts: map, API, events, logs, tunnel, metrics | JSON Schema, OpenAPI, protobuf |
| `gateway/` | Gateway: allowlist, rebuilding, faithful rendering | Go |
| `tunnel-relay/` | Terminates connector tunnels | Go |
| `connector/` | Connector installed next to the legacy application | Go |
| `orchestrator/` | Applies the capacity configured in the panel | Go |
| `panel-api/` | Control-plane API | Go |
| `panel-ui/` | Web panel | TypeScript, React |
| `agent/` | Crawler and AI agent | Python |
| `lab/` | Test lab (isolated vulnerable apps) | Docker |
| `deploy/compose/` | Docker Compose for the prototype | YAML |
| `tools/` | `eqtool`: keys and map signing (prototype) | Go |
| `scripts/` | Contract validation, YAML → JSON map conversion | Python |

## Checking the contracts

```bash
pip install jsonschema pyyaml openapi-spec-validator
python3 scripts/validate_schemas.py
```
