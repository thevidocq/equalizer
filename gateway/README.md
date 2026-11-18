# gateway

Servizio Go, unico punto esposto. Applica la mappa firmata, ricostruisce le richieste verso il legacy e rende le pagine con la resa fedele.

Struttura prevista: `cmd/gateway/`, `internal/{router,legacyclient,transport,pipeline,forms,upload,ratelimit,compat,mapstore,telemetry}/`.

Contratti: `schema/map/`, `schema/events/`, `schema/metrics/`. Stato: da implementare.
