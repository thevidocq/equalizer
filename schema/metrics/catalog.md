# Metrics catalogue

Prometheus metrics exposed by the services on `/metrics`. Allowed labels:
`tenant`, `app`, `route_id`, `reason`, `instance`, `connector_id`.
**Never** paths, IPs or users as labels (cardinality): that data belongs
in the event store.

| Name | Type | Service | Labels | Meaning |
| --- | --- | --- | --- | --- |
| `eq_requests_total` | counter | gateway | tenant, app, route_id, status | Requests served |
| `eq_requests_blocked_total` | counter | gateway | tenant, app, reason | Requests blocked, by reason |
| `eq_fields_rejected_total` | counter | gateway | tenant, app, route_id | Fields rejected by validation |
| `eq_ratelimit_hits_total` | counter | gateway | tenant, app | Rate limits triggered |
| `eq_request_duration_seconds` | histogram | gateway | tenant, app, phase (`gateway`, `legacy`) | Latency split between gateway and legacy |
| `eq_cache_hits_total` | counter | gateway | tenant, app | Responses served from cache |
| `eq_parse_failures_total` | counter | gateway | tenant, app, route_id | Parsing/extraction errors |
| `eq_legacy_errors_total` | counter | gateway | tenant, app | Legacy errors (5xx, timeouts) |
| `eq_unmapped_requests_total` | counter | gateway | tenant, app | Requests to unmapped URLs |
| `eq_compat_misses_total` | counter | gateway | tenant, app | Historical URLs not recognised |
| `eq_bytes_sent_total` | counter | gateway | tenant, app | Outbound bandwidth |
| `eq_legacy_inflight` | gauge | gateway | tenant, app | Requests in flight to the legacy app (budget) |
| `eq_tunnels_active` | gauge | tunnel-relay | tenant, app, connector_id | Active tunnels |
| `eq_tunnel_rtt_seconds` | histogram | tunnel-relay | tenant, app, connector_id | Tunnel latency |
| `eq_tunnel_reconnects_total` | counter | tunnel-relay | tenant, app, connector_id | Reconnections |
| `eq_replicas_desired` / `eq_replicas_actual` | gauge | orchestrator | tenant, app | Desired and actual replicas |
| `eq_crawl_jobs_queued` | gauge | panel-api | tenant | Queued crawls |
| `eq_llm_tokens_total` | counter | agent | tenant, provider, model | Tokens consumed |
| `eq_synthetic_check_success` | gauge | probe | pool | End-to-end check result (1/0) |

Unique visitors are estimated with HyperLogLog in Redis (`eq:hll:<tenant>:<app>:<day>`) and read by panel-api, not by Prometheus.
