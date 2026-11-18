# Equalizer

Equalizer protegge le applicazioni web legacy che non si possono aggiornare.
Le nasconde da Internet e ne serve una **copia identica**, ricostruita a ogni
richiesta da un gateway che lascia passare solo ciò che è dichiarato in una
mappa in allowlist.

> **Stato:** in sviluppo — prototipo. Vedi [`CHANGELOG.md`](CHANGELOG.md) per
> cosa è già implementato.

## Come funziona in breve

```text
utenti → gateway (unico esposto) → tunnel-relay ⇠ tunnel in uscita ⇠ connettore → app legacy
                    ↑ mappa firmata
pannello → panel-api ← proposte ← agent AI (crawl del legacy)
```

- Il legacy non è mai raggiungibile da Internet: risponde solo al connettore
  installato accanto a lui, che apre il tunnel **dall'interno**.
- Ogni richiesta viene **ricostruita**, mai inoltrata così com'è.
- Un agent AI propone la mappa; le persone la approvano da un pannello
  multitenant.

## Documentazione

| Documento | Contenuto |
| --- | --- |
| [`docs/architecture.md`](docs/architecture.md) | Architettura, componenti, regole di progetto |
| [`docs/map-format.md`](docs/map-format.md) | Formato della mappa |
| [`INSTALL.md`](INSTALL.md) | Installazione (VPS, connettore) |
| [`docs/manual/amministratore.md`](docs/manual/amministratore.md) | Manuale dell'amministratore di piattaforma |
| [`docs/manual/cliente.md`](docs/manual/cliente.md) | Manuale dell'amministratore del cliente |
| [`CHANGELOG.md`](CHANGELOG.md) | Cosa cambia a ogni commit e prossimi passi |
| [`CLAUDE.md`](CLAUDE.md) | Istruzioni per chi riprende lo sviluppo (persone o Claude) |

## Struttura del repository

| Cartella | Contenuto | Linguaggio |
| --- | --- | --- |
| `schema/` | Contratti: mappa, API, eventi, log, tunnel, metriche | JSON Schema, OpenAPI, protobuf |
| `gateway/` | Gateway: allowlist, ricostruzione, resa fedele | Go |
| `tunnel-relay/` | Termina i tunnel dei connettori | Go |
| `connector/` | Connettore da installare accanto al legacy | Go |
| `orchestrator/` | Applica la capacità configurata dal pannello | Go |
| `panel-api/` | API del control plane | Go |
| `panel-ui/` | Pannello web | TypeScript, React |
| `agent/` | Crawler e agent AI | Python |
| `lab/` | Laboratorio di test (app vulnerabili isolate) | Docker |
| `deploy/compose/` | Docker Compose del prototipo | YAML |
| `scripts/` | Script di supporto (validazione contratti) | Python |

## Verificare i contratti

```bash
pip install jsonschema pyyaml openapi-spec-validator
python3 scripts/validate_schemas.py
```
