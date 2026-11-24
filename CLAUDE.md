# Instructions for Equalizer developers

This file is for whoever picks up development, in particular another Claude
session or account. Read it in full before touching the code.

## Where to start

1. Read [`CHANGELOG.md`](CHANGELOG.md): the latest entry says **what was done,
   what was left half-finished and what the next steps are**.
2. Read [`docs/architecture.md`](docs/architecture.md) and
   [`docs/map-format.md`](docs/map-format.md).
3. Run `python3 scripts/validate_schemas.py`: it must pass before and after
   your work.

## Non-negotiable requirements

1. **Interface 100% identical** to the legacy site (faithful rendering by default).
2. **The legacy application is never exposed**: only Equalizer reaches it,
   through the connector or, in direct mode, at an address that accepts
   only Equalizer's.
3. **Rebuild, never forward**: no client byte reaches the legacy application as is.
4. **Deny by default**: whatever is not in the map does not exist.
5. **Everything configurable from the panel**, including LLM, capacity, logs and SIEM forwarding.
6. **Modularity**: no monolithic code, neither backend nor frontend.

## Development rules

- **Contracts before code.** Service boundaries live in `schema/`. If a
  service needs something new, change the contract first, then the code.
  Go and Python types are generated from the schemas.
- **One service per folder**, no database shared between services.
- **Go**: one package, one responsibility; depend on small interfaces; no
  `utils`/`common` packages. Warning threshold: 400 lines per file.
- **Step-based rendering pipeline**: every step implements the same `Step`
  interface; the order is configurable.
- **React**: organised by feature (`features/<name>/`), pages that compose
  components, data only through hooks and the client generated from OpenAPI,
  `shared/ui` never imports from `features/`. Threshold: 200 lines per file.
- **Configuration from environment variables**, services with no local state.
- **JSON logs** conforming to `schema/events/log.schema.json`, with
  `request_id` propagated. Never secrets or personal data in logs.
- **Language**: everything in English — documentation, code comments,
  messages, logs, tests and commit messages. The project is international.

## Git workflow

- One branch per task (`feat/...`, `fix/...`, `docs/...`, `chore/...`), then a
  pull request to `main`.
- **Every commit adds an entry to `CHANGELOG.md`** in the format given at the
  top of that file: what changed, why, status and next steps. CI rejects PRs
  that do not touch the changelog.
- Commit messages in English, imperative mood, first line under 72
  characters.

## Definition of done

A change is done only when, in the same PR, without being asked:

1. Tests cover it and every touched service's checks pass (`gofmt`/`vet`/
   `test`, agent `unittest`, panel-ui `tsc`/`test`/`check:help`/boundaries,
   `scripts/validate_schemas.py`).
2. **Every document that describes the behaviour is updated**: the user
   manuals (`docs/manual/tenant.md`, `docs/manual/admin.md`) and the in-app
   help (`cd panel-ui && node scripts/sync-help.mjs`), `docs/map-format.md`,
   `docs/agent.md`, `docs/architecture.md`, `INSTALL.md`, as they apply.
   Grep the docs for the feature's words before opening the PR.
3. `CHANGELOG.md` has the entry.
4. Anything the customer would have to do by hand is a bug of the product,
   not an instruction: the agent and the panel do it. The only manual step
   is publishing a map.
5. Before a release: the efficacy study passes on every kind of site the
   lab can build, the generic one included
   (`python3 lab/e2e/efficacy/efficacy.py`), and `docs/efficacy.md` has its
   numbers.

## Development environment in Claude sessions

- The Go module proxy (`proxy.golang.org`) may be blocked. Modules hosted on
  GitHub download with `GOPROXY=direct GOSUMDB=off`.
- `golang.org/x/...` modules cannot be downloaded directly. The HTML parser
  is therefore vendored in `gateway/internal/thirdparty/xhtml` (see its
  README). For other `golang.org/x` modules, clone the GitHub mirror (e.g.
  `github.com/golang/net`) at a tag compatible with the installed Go and use
  a local `go.work` with `replace`, **without committing it**.
- Docker may have no daemon: validate Compose with
  `docker compose config --quiet`; CI builds the images.
- The `gh` GraphQL API may be blocked: open PRs with the REST API
  (`gh api repos/thevidocq/equalizer/pulls`).
- The npm registry may be blocked, so `npm run gen:api` cannot run:
  `python3 scripts/openapi_ts.py schema/api/panel-api.yaml >
  panel-ui/src/shared/api/schema.d.ts` emulates openapi-typescript 7 and
  reproduces the committed file byte for byte. CI's `check:api` still
  runs the real generator.

## Lab safety

- Vulnerable lab apps publish no ports and live on an `internal` Docker
  network.
- Attack campaigns run **only** against systems owned by the project or with
  the owner's written authorisation.

## Decisions already made

| Topic | Decision |
| --- | --- |
| Name | Equalizer (domain `equalizer.ai` under evaluation) |
| Repository | GitHub `thevidocq/equalizer`, CI with GitHub Actions |
| Languages | Go (gateway, relay, connector, orchestrator, panel-api), Python (agent), TypeScript + React (panel) |
| Initial LLM | Claude Haiku via API; local and ChatGPT configurable |
| UI components | Custom library in `panel-ui/src/shared/ui`, built on Radix UI and Tailwind CSS |
| Prototype | One VPS with Docker Compose; Kubernetes driver later |
| Tunnel | HTTPS upgrade to `equalizer-tunnel/1` + yamux + HTTP/1.1 per stream (no gRPC); connector local allowlist is the last barrier |
| Direct mode | A site may be reached without a connector at its public IP address, accepting only Equalizer's addresses; the panel accepts only public IP literals (or `EQ_DIRECT_DESTINATIONS`) |
| Map approval | One administrator publishes alone; tenant roles are administrator and viewer |
| Site certificates | Always the legacy site's own, uploaded in the panel; no Let's Encrypt for protected sites |
| Lab and remote servers | Prepared by the project owner |
| Language | English for everything in the repository |
| Coverage | The whole site is served, admin included (map `areas`, `/` by default in templates and discovery). Equalizer is in between and rebuilds every request; it never decides access, the legacy application does. Leftover files on the legacy server are not Equalizer's concern |
