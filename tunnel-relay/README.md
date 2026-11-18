# tunnel-relay

Servizio Go che termina i tunnel aperti dai connettori e instrada le richieste del gateway e dell'agent verso il connettore giusto.

Struttura prevista: `cmd/relay/`, `internal/{sessions,routing,auth}/`.

Contratto: `schema/tunnel/tunnel.proto`. Stato: da implementare.
