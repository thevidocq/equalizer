# gateway

Servizio Go, unico punto esposto. Applica la mappa firmata, ricostruisce le
richieste verso il legacy e rende le pagine con la resa fedele.

## Stato

| Funzione | Stato |
| --- | --- |
| Caricamento e verifica della mappa firmata (Ed25519), ricarica a caldo | Fatto |
| Instradamento in allowlist (404 non mappato, 405 metodo non ammesso, OPTIONS dal gateway) | Fatto |
| Fail closed: senza mappa valida risponde 503 | Fatto |
| Log JSON conformi a `log.schema.json`, `request_id` generato dal gateway | Fatto |
| Eventi di sicurezza conformi a `event.schema.json` | Fatto (`request.blocked`, `request.served`, `field.rejected`, `legacy.error`) |
| IP reale del client solo da proxy fidati (`X-Forwarded-For`, `CF-Connecting-IP`) | Fatto |
| `/healthz`, `/readyz` su porta di amministrazione separata | Fatto |
| Validazione dei tipi di parametri e query (tipi, vincoli, regole `no_html`, `no_sql_meta`, `no_template_meta`) | Fatto |
| Ruolo anonimo (`roles.anonymous`): le rotte riservate rispondono 403 | Fatto |
| Ricostruzione delle richieste GET verso il legacy in modalità diretta, con allowlist delle destinazioni | Fatto |
| Filtro degli header del legacy, header di sicurezza, pagine d'errore generiche | Fatto |
| Budget di richieste contemporanee verso il legacy (per replica) | Fatto |
| Resa fedele: conversione in UTF-8 (UTF-8, ISO-8859-1, ISO-8859-15, Windows-1252), parser HTML5, passi `clean` e `rewrite` | Fatto |
| Riscrittura degli URL interni (link, risorse, form, srcset, `<base>`), mai l'host del legacy nella pagina | Fatto |
| Asset per prefisso con verifica del tipo dichiarato e reale, SVG in sandbox | Fatto |
| Traduzione dei redirect del legacy verso URL pubblici | Fatto |
| Script ammessi per hash (CSP), riscrittura degli URL nei CSS | Da fare |
| Form (POST), API, login e sessioni | Da fare |
| Modalità via connettore | Da fare (oggi: 502 `connector_unavailable`) |
| Pipeline di resa fedele | Da fare |
| Metriche Prometheus | Da fare |

## Struttura

```text
cmd/gateway/          avvio, segnali, healthcheck
internal/config/      variabili d'ambiente
internal/telemetry/   log, request_id, eventi
internal/clientip/    IP reale del client
internal/mapmodel/    tipi della mappa
internal/mapstore/    mappa firmata: verifica e ricarica
internal/site/        mappa compilata: router, validatore, client, budget
internal/router/      allowlist delle rotte
internal/validate/    tipi e vincoli dei valori in ingresso
internal/placeholder/ segnaposto {{spazio.nome}} di legacy_request
internal/legacyclient/ richieste ricostruite e risposte filtrate
internal/legacyurl/   URL del legacy → URL pubblici (link, compat, rotte)
internal/charset/     codifica delle pagine e conversione in UTF-8
internal/pipeline/    resa fedele: Step, Context, passi clean/ e rewrite/
internal/thirdparty/xhtml/ parser HTML5 (copia di golang.org/x/net/html)
internal/transport/   collegamento al legacy (diretto; connettore in arrivo)
internal/budget/      richieste contemporanee verso il legacy
internal/server/      handler pubblico e di amministrazione
```

## Variabili d'ambiente

| Variabile | Predefinito | Significato |
| --- | --- | --- |
| `EQ_PUBLIC_ADDR` | `:8080` | Traffico pubblico (dietro Caddy) |
| `EQ_ADMIN_ADDR` | `:9090` | `/healthz`, `/readyz` (solo reti interne) |
| `EQ_INSTANCE` | hostname | Nome dell'istanza nei log |
| `EQ_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `EQ_MAP_FILE` | `/maps/current.json` | Documento firmato (SignedMap) |
| `EQ_MAP_PUBKEY` o `EQ_MAP_PUBKEY_FILE` | — | Chiave pubblica Ed25519 in base64 (obbligatoria) |
| `EQ_MAP_KEY_ID` | — | Id chiave atteso (facoltativo) |
| `EQ_TRUSTED_PROXIES` | — | Reti CIDR da cui accettare l'IP del client negli header |

## Sviluppo

```bash
go vet ./... && go test -race ./...
```

Solo libreria standard di Go, per ora. `kill -HUP` ricarica la mappa subito.
