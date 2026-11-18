# Architettura

Equalizer è un gateway che sostituisce sul web le applicazioni legacy non
aggiornabili: le nasconde da Internet e ne serve una copia identica,
ricostruita a ogni richiesta. Un crawler e un agent AI generano la mappa in
allowlist, le persone la approvano da un pannello multitenant.

## Requisiti non negoziabili

1. **Interfaccia identica al 100%**: resa fedele predefinita, verificata con
   confronto visivo automatico (Playwright, pixel per pixel).
2. **Nessuna esposizione del legacy**: lo raggiunge solo il connettore, che
   apre il tunnel in uscita, sia in cloud sia on-prem.
3. **Ricostruzione, mai inoltro.**
4. **Continuità**: gli URL storici continuano a funzionare (`compat`).
5. **Due modalità**: cloud con DNS ripuntato e on-prem, stesso software.
6. **LLM configurabile**: Claude, ChatGPT o modello locale, dal pannello.
7. **Tutto configurabile dal pannello**, compresa la capacità (SaaS/PaaS).
8. **Modularità**: servizi separati che parlano solo tramite contratti.

## Flusso

```text
                 DATA PLANE                                   RETE DEL CLIENTE
Internet ──► gateway ──► tunnel-relay ◄══ tunnel in uscita ══ connettore ──► app legacy
               ▲  (unico esposto)  ▲
               │ mappa firmata     │ crawl
            panel-api ◄── proposte ── agent ──► provider LLM
               ▲    │                 CONTROL PLANE
          panel-ui  └──► PostgreSQL
               orchestrator ──► Docker / Kubernetes
```

## Componenti

| Servizio | Linguaggio | Responsabilità | Contratto |
| --- | --- | --- | --- |
| `gateway` | Go | Traffico pubblico, allowlist, ricostruzione, resa fedele | Mappa firmata, eventi, metriche |
| `tunnel-relay` | Go | Termina i tunnel dei connettori, instrada le richieste | `schema/tunnel/tunnel.proto` |
| `connector` | Go | Accanto al legacy; tunnel in uscita; solo destinazioni ammesse | `schema/tunnel/tunnel.proto` |
| `orchestrator` | Go | Applica la capacità configurata (repliche, autoscaling, pool) | Stato desiderato da panel-api |
| `panel-api` | Go | Tenant, mappe, proposte, approvazioni, LLM, connettori, capacità, metriche, stato, log, SIEM, import/export, audit, firma | `schema/api/panel-api.yaml` |
| `panel-ui` | TypeScript, React | Pannello | Client generato da OpenAPI |
| `agent` | Python | Crawl per ruolo, analisi, proposte di mappa | OpenAPI, interfaccia `LLMProvider` |
| `postgres` | — | Dati del control plane | Migrazioni versionate |
| `redis` | — | Sessioni, rate limit, cache, notifiche di nuova mappa | Chiavi documentate per modulo |

Osservabilità: Vector (raccolta log), Loki (log), Tempo (tracce),
Prometheus + Alertmanager (metriche e allarmi), Grafana (analisi).

## Resa fedele (pipeline del gateway)

Per ogni pagina, passi indipendenti con la stessa interfaccia `Step`:

1. **Parsing** HTML5 in DOM e riserializzazione.
2. **Pulizia invisibile**: commenti, meta di versione, header che rivelano lo
   stack, campi nascosti interni, debug.
3. **Riscrittura URL** di link, `src`, `action`, URL nei CSS.
4. **Form**: `action` verso il gateway, token CSRF, campi nascosti reinseriti
   lato server.
5. **Script** ammessi solo per hash (CSP).
6. **Asset** serviti dopo verifica del tipo reale.
7. **Header di sicurezza** (CSP, HSTS, ...).

## Regole di modularità

- Backend: un pacchetto, una responsabilità; dipendenze verso interfacce;
  tipi generati da `schema/`; configurazione da env; soglia 400 righe/file.
- Frontend: `features/<nome>/{pages,components,hooks,index.ts}`; pagine che
  compongono; dati solo da hook; `shared/ui` senza dipendenze dalle
  funzionalità; confini verificati da lint; soglia 200 righe/file.

## Provider LLM

Interfaccia unica nell'agent (`agent/llm/`), un modulo per provider:
`anthropic` (predefinito: Claude Haiku), `openai`, `local` (endpoint
compatibile OpenAI, es. Ollama, vLLM). Configurazione per tenant dal pannello;
chiavi nel vault, sola scrittura. Il contenuto delle pagine passa al modello
come dati, l'output deve validare contro lo schema della mappa, il modello
non ha strumenti che scrivono, ogni proposta che allarga la superficie
richiede approvazione umana.

## Connettore e tunnel

- Il connettore apre sempre la connessione verso il relay (TLS, porta 8443),
  gRPC bidirezionale con mTLS.
- Registrazione con token monouso del pannello → certificato client.
- Allowlist delle destinazioni **firmata dal pannello**: un relay
  compromesso non può allargarla.
- Più connettori per app in attivo-attivo.

## Scalabilità orizzontale

Servizi senza stato locale; stato condiviso in Redis e PostgreSQL; nessuna
sessione sticky. Alla pubblicazione di una mappa le repliche la scaricano,
verificano la firma e la attivano. Il budget verso il legacy è contato in
Redis e vale per tutte le repliche. La capacità di ogni sito si configura dal
pannello; l'`orchestrator` la applica con il driver `compose` (prototipo) o
`kubernetes` (cloud: operator con risorsa `ProtectedSite`, HPA, PDB).

## Metriche, stato, log

- **Scheda sito**: traffico, protezione, prestazioni, copertura,
  collegamento, agent. Catalogo in `schema/metrics/catalog.md`.
- **Stato dei servizi**: vista globale per l'amministratore, vista ristretta
  per il cliente; `/healthz`, `/readyz`, heartbeat, controllo sintetico
  end-to-end ogni minuto.
- **Log**: JSON (`schema/events/log.schema.json`), `request_id` propagato,
  mascheramento alla fonte, livelli a caldo con scadenza, traccia richiesta,
  pagina di ricerca con live tail ed export, inoltro SIEM (syslog RFC 5424,
  Splunk HEC, Sentinel, webhook; JSON/CEF/LEEF; facility per flusso).

## Sicurezza del prodotto

Mappe firmate Ed25519; gateway autonomo se il pannello è giù; fail closed;
segreti solo nel vault; scansione dipendenze e SBOM in CI; container non root;
revisione obbligatoria su `pipeline/`, `legacyclient/`, `mapstore/`; pentest
prima dell'uso su portali reali.
