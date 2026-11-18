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
| gateway | Rotte GET e asset in modalità diretta, validazione, resa fedele (UTF-8, pulizia, riscrittura URL, redirect); niente form, sessioni, connettore |
| tunnel-relay | Da implementare |
| connector | Da implementare |
| orchestrator | Da implementare |
| panel-api | Da implementare |
| panel-ui | Da implementare |
| agent | Da implementare |
| deploy/compose | Caddy, gateway (2 repliche), Vector, Loki, Grafana; validato staticamente |
| tools (eqtool) | Chiavi e firma delle mappe |

## Voci

### 2026-11-18 — Aggiungi la resa fedele e gli asset

- **Branch:** `feat/faithful-rendering`
- **Cosa:**
  - Contratto: sezione `assets` nella mappa (prefisso, tipi ammessi,
    dimensione massima, ruoli), prefissi con `.` o `..` rifiutati; esempio
    aggiornato.
  - `gateway/internal/thirdparty/xhtml`: copia del parser HTML5
    `golang.org/x/net/html` v0.40.0 (licenza BSD), perché il proxy dei
    moduli Go non è raggiungibile da tutti gli ambienti di sviluppo.
  - `internal/charset`: rilevamento (header, `<meta>`, contenuto) e
    conversione in UTF-8 di ISO-8859-1, ISO-8859-15, Windows-1252; codifiche
    diverse rifiutate.
  - `internal/legacyurl`: traduzione URL del legacy → URL pubblici da
    `links`, `compat` e dal `legacy_request` delle rotte GET; asset con lo
    stesso percorso; fallback relativo senza host interno.
  - `internal/pipeline`: interfaccia `Step`, `Context`, statistiche;
    passi `clean` (commenti e meta rivelatori) e `rewrite` (href, src,
    action, formaction, srcset, `<base>`, ...); dichiarazioni di codifica
    allineate a UTF-8.
  - `internal/server`: pipeline applicata alle pagine HTML (fail closed se
    fallisce), redirect del legacy tradotti verso URL pubblici (quelli
    esterni bloccati), asset con verifica di nome, tipo dichiarato e tipo
    reale, SVG in sandbox, query degli asset mai inviata al legacy.
  - Router: prefissi asset (il più lungo vince), dopo le rotte.
  - Documentazione: README del gateway, formato mappa (link e asset).
- **Perché:** requisito "interfaccia identica al 100%": senza asset la pagina
  perde lo stile, senza riscrittura i link portano agli URL del legacy.
- **Stato:** test verdi con race detector. Verificato su pagine di prova:
  link, risorse, form e srcset riscritti, IP interno assente, commenti e
  meta `generator` tolti, ISO-8859-1 con entità convertito senza perdere gli
  accenti; asset con contenuto PHP travestito da PNG o tipo non ammesso
  rifiutati; `../` e `//` nei nomi rifiutati. Non ancora fatto: script
  ammessi per hash con CSP, riscrittura degli URL dentro i CSS e negli
  attributi `style`, `external_links`, confronto visivo automatico.
  **Attenzione per i form:** le pagine escono in UTF-8, quindi i browser
  invieranno i form in UTF-8; il gateway dovrà ricodificare i valori nella
  codifica originale del legacy prima di ricostruire le POST.
- **Prossimi passi:**
  1. Form POST: campi tipizzati, token CSRF, campi nascosti del legacy
     lato server, ricodifica nella codifica del legacy.
  2. `tunnel-relay` e `connector`.
  3. Login passante e sessioni in Redis.
  4. Script per hash con CSP, URL nei CSS, `external_links`.
  5. Metriche Prometheus del gateway; confronto visivo con Playwright.

### 2026-11-18 — Ricostruisci le richieste GET verso il legacy

- **Branch:** `feat/legacy-reconstruction`
- **Cosa:**
  - Contratto: `roles.anonymous` nello schema della mappa (ruolo dei
    visitatori non autenticati) e nell'esempio.
  - `gateway/internal/mapmodel`: tipi della mappa spostati in un pacchetto
    proprio ed estesi (tipi, valori predefiniti, specifiche dei campi,
    `legacy_request`).
  - `internal/validate`: tipi `int`, `enum`, `bool`, `date`, `string`,
    `email`, `text` con vincoli e regole `no_html`, `no_sql_meta`,
    `no_template_meta`; query non dichiarate scartate, ripetute rifiutate;
    pattern e regole verificati al caricamento della mappa.
  - `internal/placeholder`: segnaposto `{{params.x}}`, `{{query.x}}`, ecc.
  - `internal/legacyclient`: richiesta costruita da zero (percorso con
    valori codificati, query dichiarate, header fissi), redirect non
    seguiti, risposta limitata a 16 MiB, header del legacy filtrati.
  - `internal/transport`: modalità diretta con allowlist host:porta nel
    dialer (difesa anche contro redirect e DNS).
  - `internal/budget`: richieste contemporanee verso il legacy, per replica.
  - `internal/site`: compila una versione della mappa (router, validatore,
    client, budget) e rifiuta mappe incoerenti.
  - `internal/server/route.go`: ruolo anonimo (403 sulle rotte riservate),
    validazione, ricostruzione, pagine generiche per 4xx/5xx del legacy
    (mai stack trace), header di sicurezza, eventi `request.served` con
    latenza divisa, `field.rejected` senza il valore, `legacy.error`.
  - Documentazione: README del gateway, formato mappa (regole e ruolo
    anonimo), INSTALL.
- **Perché:** primo passo in cui il gateway serve davvero un sito: il
  legacy riceve solo richieste ricostruite e valori validati.
- **Stato:** test verdi con race detector. Prova end-to-end con un legacy
  finto: `/` e `/pratiche/7` serviti (il legacy ha ricevuto solo
  `GET /index.jsp` e `GET /detail.jsp?id=7`), `/pratiche/7abc` → 400,
  `/index.jsp` e `/../../etc/passwd` → 404. Limiti noti: l'HTML del legacy
  passa senza resa fedele (i link interni puntano ancora agli URL del
  legacy); i redirect del legacy diventano 502; form, API, login e
  sessioni non sono serviti; con `mode: connector` risposta 502; il budget
  non è condiviso tra repliche; i tipi della mappa sono ancora scritti a mano.
- **Prossimi passi:**
  1. Pipeline di resa fedele: parser HTML5, pulizia, riscrittura dei link
     (`links`, `compat`), traduzione dei redirect.
  2. Form POST con campi tipizzati e token CSRF.
  3. `tunnel-relay` e `connector`.
  4. Login passante e sessioni in Redis.
  5. Metriche Prometheus del gateway.

### 2026-11-18 — Aggiungi Compose di base, log e scheletro del gateway

- **Branch:** `feat/compose-gateway-skeleton`
- **Cosa:**
  - `gateway/`: servizio Go (solo libreria standard) con caricamento e
    verifica della mappa firmata Ed25519, ricarica a caldo (file e SIGHUP),
    router in allowlist (404 non mappato, 405 metodo non ammesso, OPTIONS e
    HEAD gestiti dal gateway, TRACE/CONNECT rifiutati), fail closed senza
    mappa (503), log JSON conformi allo schema, `request_id` generato dal
    gateway e restituito in `X-Eq-Request-Id`, eventi `request.blocked`, IP
    del client solo da proxy fidati, `/healthz` e `/readyz` su porta separata,
    sottocomando `healthcheck`, Dockerfile distroless non root, test.
  - `tools/`: `eqtool keygen` e `eqtool sign` (firma delle mappe finché non
    c'è panel-api), Dockerfile.
  - `scripts/map_to_json.py`: valida una mappa YAML e la converte in JSON.
  - `deploy/compose/`: reti `edge`, `control`, `lab` (internal), Caddy con
    TLS automatico e bilanciamento dinamico delle repliche, gateway con 2
    repliche, Vector → Loki, Grafana solo su localhost, profilo `tools`,
    `.env.example`.
  - Contratto `SignedMap` (OpenAPI): la mappa viaggia come `payload` base64
    firmato byte per byte invece che come oggetto JSON.
  - CI `go.yml`: gofmt, vet, test con race detector, validazione Compose,
    build delle immagini.
  - `INSTALL.md`: passi 4–6 resi concreti (configurazione, chiave e firma,
    avvio, log in Grafana). `CLAUDE.md`: note sull'ambiente di sviluppo.
- **Perché:** avere dal primo giorno la base per fare debug (log strutturati
  e cercabili) e un gateway che già applica il deny by default.
- **Stato:** test Go verdi; prova end-to-end in locale con l'esempio firmato:
  `/` e `/pratiche/42` → 503 (mappate, ricostruzione assente), `/wp-admin` →
  404, metodo sbagliato → 405, TRACE → 405, `/readyz` pronto; le 12 righe di
  log/eventi prodotte validano contro gli schemi. Il Compose è validato con
  `docker compose config` ma **non è stato avviato** (nessun demone Docker
  nella sessione di sviluppo): Caddy, Vector, Loki e Grafana vanno collaudati
  sulla VPS. I tipi della mappa nel gateway sono scritti a mano (sottoinsieme)
  perché qui non si scaricano strumenti di generazione: da sostituire.
- **Prossimi passi:**
  1. Collaudo del Compose sulla VPS e correzioni (in particolare la
     configurazione di Vector).
  2. Validazione dei tipi di parametri e campi nel gateway.
  3. `legacyclient` e `transport` (modalità diretta), poi ricostruzione delle
     richieste GET.
  4. Pipeline di resa fedele con parser HTML5 (`golang.org/x/net/html`).
  5. `tunnel-relay` e `connector` su `tunnel.proto`.
  6. Metriche Prometheus del gateway e generazione dei tipi dagli schemi.

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
