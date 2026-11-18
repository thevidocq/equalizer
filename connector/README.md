# connector

Go service installed next to the legacy application. It opens the outbound tunnel to the relay and only executes requests to destinations in the allowlist signed by the panel.

Planned layout: `cmd/connector/`, `internal/{tunnel,allowlist,executor}/`.

Contract: `schema/tunnel/tunnel.proto`. Status: not implemented yet.
