# connector

Servizio Go da installare accanto al legacy. Apre il tunnel in uscita verso il relay ed esegue solo richieste verso destinazioni presenti nell'allowlist firmata dal pannello.

Struttura prevista: `cmd/connector/`, `internal/{tunnel,allowlist,executor}/`.

Contratto: `schema/tunnel/tunnel.proto`. Stato: da implementare.
