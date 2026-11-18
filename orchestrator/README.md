# orchestrator

Servizio Go che legge da panel-api la capacità desiderata di ogni sito e la applica: driver `compose` (prototipo) e `kubernetes` (cloud) con la stessa interfaccia `Driver`.

Struttura prevista: `cmd/orchestrator/`, `internal/{desired,reconcile,drivers/compose,drivers/kubernetes}/`. Stato: da implementare.
