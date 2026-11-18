# Catalogo delle metriche

Metriche Prometheus esposte dai servizi su `/metrics`. Etichette ammesse:
`tenant`, `app`, `route_id`, `reason`, `instance`, `connector_id`.
**Mai** percorsi, IP o utenti come etichette (cardinalità): quei dati
vanno nell'archivio eventi.

| Nome | Tipo | Servizio | Etichette | Significato |
| --- | --- | --- | --- | --- |
| `eq_requests_total` | counter | gateway | tenant, app, route_id, status | Richieste servite |
| `eq_requests_blocked_total` | counter | gateway | tenant, app, reason | Richieste bloccate per motivo |
| `eq_fields_rejected_total` | counter | gateway | tenant, app, route_id | Campi rifiutati dalla validazione |
| `eq_ratelimit_hits_total` | counter | gateway | tenant, app | Rate limit scattati |
| `eq_request_duration_seconds` | histogram | gateway | tenant, app, phase (`gateway`, `legacy`) | Latenza divisa tra gateway e legacy |
| `eq_cache_hits_total` | counter | gateway | tenant, app | Risposte servite da cache |
| `eq_parse_failures_total` | counter | gateway | tenant, app, route_id | Errori di parsing/estrazione |
| `eq_legacy_errors_total` | counter | gateway | tenant, app | Errori del legacy (5xx, timeout) |
| `eq_unmapped_requests_total` | counter | gateway | tenant, app | Richieste a URL non mappati |
| `eq_compat_misses_total` | counter | gateway | tenant, app | URL storici non riconosciuti |
| `eq_bytes_sent_total` | counter | gateway | tenant, app | Banda in uscita |
| `eq_legacy_inflight` | gauge | gateway | tenant, app | Richieste in corso verso il legacy (budget) |
| `eq_tunnels_active` | gauge | tunnel-relay | tenant, app, connector_id | Tunnel attivi |
| `eq_tunnel_rtt_seconds` | histogram | tunnel-relay | tenant, app, connector_id | Latenza del tunnel |
| `eq_tunnel_reconnects_total` | counter | tunnel-relay | tenant, app, connector_id | Riconnessioni |
| `eq_replicas_desired` / `eq_replicas_actual` | gauge | orchestrator | tenant, app | Repliche desiderate e reali |
| `eq_crawl_jobs_queued` | gauge | panel-api | tenant | Crawl in coda |
| `eq_llm_tokens_total` | counter | agent | tenant, provider, model | Token consumati |
| `eq_synthetic_check_success` | gauge | probe | pool | Esito del controllo end-to-end (1/0) |

I visitatori unici sono stimati con HyperLogLog in Redis (`eq:hll:<tenant>:<app>:<giorno>`) e letti da panel-api, non da Prometheus.
