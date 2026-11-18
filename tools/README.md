# tools

`eqtool`: prototype tools, in Go with no external dependencies.

| Command | What it does |
| --- | --- |
| `eqtool keygen -out secrets/map-signing.key` | Creates the Ed25519 key pair used to sign maps (private `.key`, public `.pub`) |
| `eqtool sign -key secrets/map-signing.key -map maps/map.json -key-id k1 -out maps/current.json` | Signs a JSON map and produces the document read by the gateway |

The YAML map is converted to JSON (with validation) by `scripts/map_to_json.py`.
In the final product panel-api does both signing and conversion.
