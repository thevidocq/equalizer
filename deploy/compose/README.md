# deploy/compose

Docker Compose del prototipo su una VPS. Guida completa in [`INSTALL.md`](../../INSTALL.md).

| Servizio | Rete | Esposto | Note |
| --- | --- | --- | --- |
| `caddy` | edge | 80, 443 | TLS automatico, bilancia le repliche del gateway |
| `gateway` | edge, control | no | 2 repliche predefinite |
| `loki` | control | no | archivio dei log, 30 giorni |
| `vector` | control | no | raccoglie i log dei container con etichetta `eq.collect=true` |
| `grafana` | control | solo `127.0.0.1:3000` | analisi dei log, via tunnel SSH |
| `eqtool` | nessuna | no | profilo `tools`: chiavi e firma delle mappe |

La rete `lab` è `internal`: le app di test non hanno uscita verso Internet.
Cartelle locali: `maps/` (mappe firmate, non nel repository) e `secrets/`
(chiavi, non nel repository).
