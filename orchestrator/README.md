# orchestrator

Go service that reads the desired capacity of each site from panel-api and applies it: a `compose` driver (prototype) and a `kubernetes` driver (cloud), both behind the same `Driver` interface.

Planned layout: `cmd/orchestrator/`, `internal/{desired,reconcile,drivers/compose,drivers/kubernetes}/`. Status: not implemented yet.
