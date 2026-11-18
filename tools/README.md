# tools

`eqtool`: strumenti del prototipo, in Go senza dipendenze esterne.

| Comando | Cosa fa |
| --- | --- |
| `eqtool keygen -out secrets/map-signing.key` | Crea la coppia di chiavi Ed25519 per firmare le mappe (`.key` privata, `.pub` pubblica) |
| `eqtool sign -key secrets/map-signing.key -map maps/mappa.json -key-id k1 -out maps/current.json` | Firma una mappa JSON e produce il documento letto dal gateway |

La mappa YAML si converte in JSON (con validazione) con `scripts/map_to_json.py`.
Nel prodotto finale firma e conversione le fa panel-api.
