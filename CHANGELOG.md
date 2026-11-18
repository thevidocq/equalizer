# Changelog

Registro di **ogni commit**, pensato perché chiunque (persona o altra
sessione Claude) possa riprendere il lavoro senza contesto.

## Come scrivere una voce

Aggiungi in cima a "Voci" una sezione per ogni commit:

```markdown
### AAAA-MM-GG — <titolo del commit>

- **Branch:** <nome>
- **Cosa:** cosa è cambiato, per file o area
- **Perché:** motivo della modifica
- **Stato:** cosa funziona, cosa è a metà, cosa non è testato
- **Prossimi passi:** elenco ordinato di ciò che viene dopo
```

## Stato attuale del progetto

| Area | Stato |
| --- | --- |
| Contratti (`schema/`) | Prima versione, validata in CI |
| Documentazione | Architettura, formato mappa, installazione e manuali (funzionalità previste) |
| gateway | Da implementare |
| tunnel-relay | Da implementare |
| connector | Da implementare |
| orchestrator | Da implementare |
| panel-api | Da implementare |
| panel-ui | Da implementare |
| agent | Da implementare |
| deploy/compose | Da implementare |

## Voci

### 2026-11-18 — Inizializza repository, contratti e documentazione

- **Branch:** `feat/bootstrap`
- **Cosa:**
  - Struttura del monorepo con una cartella per servizio e un README in ognuna.
  - `schema/map/gateway-map.schema.json`: JSON Schema della mappa (versione
    `gateway-map/1.0`) con esempio validato in `schema/map/examples/`.
  - `schema/api/panel-api.yaml`: OpenAPI 3.1 di panel-api (prima versione,
    corpi sintetici).
  - `schema/events/`: schema degli eventi di sicurezza e delle righe di log.
  - `schema/tunnel/tunnel.proto`: protocollo gRPC tra connettore e relay.
  - `schema/metrics/catalog.md`: catalogo delle metriche Prometheus.
  - `scripts/validate_schemas.py` e workflow CI `contracts.yml`.
  - `README.md`, `CLAUDE.md`, `INSTALL.md`, `docs/architecture.md`,
    `docs/map-format.md`, manuali amministratore e cliente.
- **Perché:** fissare i contratti tra i servizi prima di scrivere codice, così
  i servizi si sviluppano separatamente senza refactoring; rendere il
  progetto riprendibile da chiunque.
- **Stato:** i contratti validano in locale (schemi, esempio, OpenAPI, proto).
  Lo schema rifiuta i casi principali: campi `text`/`file` senza regole,
  rotte non `GET`, resa `template` senza estrazione, connettore senza id.
  Nella mappa la chiave di autorizzazione si chiama `target` (non `on`, che
  YAML interpreta come booleano). Nessun servizio è ancora implementato:
  `INSTALL.md` e i manuali descrivono il funzionamento previsto.
- **Prossimi passi:**
  1. `deploy/compose/`: Compose di base con reti `edge`, `control`, `lab` e
     pipeline di log (Vector, Loki, Grafana).
  2. `gateway/`: server HTTP, caricamento e verifica della mappa firmata,
     router in allowlist, log strutturati con `request_id`.
  3. `gateway/internal/pipeline/`: resa fedele (parsing, pulizia, riscrittura
     URL, form) con test su pagine di esempio.
  4. `tunnel-relay/` e `connector/` sul protocollo `tunnel.proto`.
  5. Generazione dei tipi Go dagli schemi (`schema/` → `gen/`).
