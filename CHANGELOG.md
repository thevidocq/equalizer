# Changelog

A record of **every commit**, written so that anyone (a person or another
Claude session) can pick up the work with no prior context.

## How to write an entry

Add a section at the top of "Entries" for every commit:

```markdown
### YYYY-MM-DD — <commit title>

- **Branch:** <name>
- **What:** what changed, by file or area
- **Why:** reason for the change
- **Status:** what works, what is half-done, what is untested
- **Next steps:** ordered list of what comes next
```

## Current project status

Latest release: **0.2.9** (`docs/releases/0.2.9.md`); before it 0.2.8
(`docs/releases/0.2.8.md`), 0.2.7
(`docs/releases/0.2.7.md`), 0.2.6
(`docs/releases/0.2.6.md`), 0.2.5 (`docs/releases/0.2.5.md`), 0.2.4 (`docs/releases/0.2.4.md`), 0.2.3 (`docs/releases/0.2.3.md`) and 0.2.2
(`docs/releases/0.2.2.md`); 0.2.0 was for the first installations; before
it, 0.1.0 (`docs/releases/0.1.0.md`, tag on the merge of #39).

| Area | Status |
| --- | --- |
| Contracts (`schema/`) | First version, validated in CI |
| Documentation | Architecture, map format, installation, manuals of what exists, screenshots (`docs/screenshots`) |
| gateway | Areas (whole site served, admin included, every request rebuilt), OIDC login (off by default), map check for the panel, JSON APIs, many sites by host, virtual hosts (`reach.address`), GET routes, assets and urlencoded POST forms in direct or connector mode, validation, faithful rendering (UTF-8, cleanup, form binding, URL rewriting, redirects), pass-through login with sessions in Redis (remember me, public legacy cookies in the browser), real client IP to the legacy app, app headers, SSE, streamed uploads up to 1 GB, single-byte charsets, rate limits and behaviour rules, file uploads (real type, size, image rebuild; no virus scan) |
| tunnel-relay | Token-authenticated tunnels, forwarding with tenant check, admin endpoints; several replicas with handover |
| connector | Outbound tunnel, local allowlist, reconnection; one per server, virtual hosts |
| orchestrator | Not implemented yet |
| panel-api | Discovery jobs and continuous discovery, tenant AI settings, map templates per CMS, auth with mandatory TOTP, tenants, email invitations, tenant connectors, map reach checks, exposure check, Logs page backend (Loki), status page, platform/tenant privacy, users, sites, connectors, maps with approval and signing, audit, internal endpoints |
| panel-ui | Logo (sidebar, login), discovery tab and tenant AI settings, login, joining by invitation, platform tenant overview, sites with isolation and certificate, tenant connectors, people with invitations, maps with editor, approval, import and export, activity, logs, status |
| agent | Study: read-only crawl and real browser through the connector, test accounts, typed form fields, administrations read-only where proven, profiles for ten web applications, verification of every draft on the gateway with self-correction, IDOR proposals, optional AI review with the tenant's model |
| deploy/compose | Caddy (protected sites use the certificate uploaded in the panel, Let's Encrypt only for relay and panel), SMTP settings, gateway (2 replicas), Redis, PostgreSQL, panel-api, panel-ui, tunnel-relay, Vector, Loki, Grafana; statically validated |
| tools (eqtool) | Map signing keys, form key, relay token, connector registration, map signing |

## Entries

### 2026-11-22 — Visitors' requests: the agent judges, a person adds in one click

- **Branch:** feat/report-actions (on top of fix/independent-review, #88)
- **What:**
  - agent `mapbuild._files_refused` / `_dropped_keys`: each item has
    `verdict` (`site` if a studied page, stylesheet or link names it, else
    `suspect`), `why` (what adding it would open: "every text/csv file under
    /uploads/") and `add` (the change, or none when the map cannot take
    it); the site's own first. Nothing is opened for them, nothing added.
    Keys the draft already declares are no longer listed.
  - contract `schema/api/panel-api.yaml`: `POST
    /apps/{app}/discoveries/{id}/adopt` (`kind` file|key and the item's
    path or route and key); panel-ui `schema.d.ts` updated by hand to the
    generator's shape (no `node_modules` here: CI's `check:api` compares).
  - panel-api `internal/api/adopt.go`: an administrator adds one listed
    item; the change comes from the report, never the request; it goes on
    the discovery's draft (while it is one) or the published map, through
    the schema and gateway checks, into a new draft; audit `map.adopted`.
    Test `TestAdoptFromTheReport` (file, key, unlisted or change-less items
    refused, no type from the request).
  - panel-ui `features/discovery/components/VisitorRequests.tsx` and
    `hooks/useAdopt.ts`: the site's items with **Add to a new draft**, the
    suspect ones folded with why; a link to the new version.
  - Docs: tenant manual and in-app help (Discovery), `docs/agent.md`.
- **Why:** the customer never edits the map by hand: what only visitors
  asked for needs a person's decision, made in one click with the agent's
  judgement next to it.
- **Status:** agent, panel-api green. panel-ui not type-checked here.
- **Next steps:** independent review; then merge after #88.

### 2026-11-22 — Sixth re-review: other words for ending a session

- **Branch:** fix/independent-review
- **What:** a tenth independent reviewer confirmed the normalised rule
  holds for every spelling of log out / sign off, and found logouts with
  other words (`/connect/endsession`, `/abmelden`, `/deconnexion`,
  `/session/end`) and logout routes with a file extension (`/logout.png`
  on Rails, `/logout;.png` on Tomcat) opened by the browser pass and by
  verification. `ends_session` now knows end session, log me out,
  disconnect and the usual words in nine languages, `/session(s)/end|kill…`,
  folds text with NFKC, case folding and accents, reads a file name
  without its extension, and narrows the exceptions (offer but not error,
  offset but not setup). `Crawler._allowed_quiet` and verification's `_acts`
  apply it to files. Every case is in `test_actions.py`.
- **Status:** agent green. A list of words cannot be complete: see the
  next step.
- **Next steps:** detect a lost session during the logged-in passes (the
  agent's login cookie no longer valid) and log in again, so an unknown
  logout word costs a re-login, not the rest of the pass.

### 2026-11-22 — Fifth re-review: session end judged on normalised URLs

- **Branch:** fix/independent-review
- **What:** a ninth independent reviewer found that the exceptions decided
  by what precedes the word hid real logout links (`/WebLogOff.aspx`,
  `/codeSignOff`) and that separated camelCase spellings were missed
  (`/Account/DoLog_Off`). The regular expressions on the raw URL are gone:
  `webapps.base.ends_session` decodes each piece of the URL, lowers it and
  drops separators, then looks for log out / log off / sign out / sign off
  anywhere, with exceptions decided only by what follows (offer(s), offset,
  office, offline). `is_action` and verification's folder check use it.
  Every case from the five reviews is in `test_actions.py`; pages about
  signing off are no longer studied (safety first).
- **Status:** agent green.
- **Next steps:** as above.

### 2026-11-22 — Fourth re-review: log off and sign off, safety first

- **Branch:** fix/independent-review
- **What:** an eighth independent reviewer found that the word-boundary
  rule for log off / sign off let real logout links through (`/dologoff`,
  `/doLOGOFF`, `?cmd=dologoff`, `/sessions/log-out-all-devices`) and that
  separated spellings had left `STRONG`. `webapps/base.py` now treats
  `logout`, `logoff`, `signout`, `signoff` as actions anywhere, with an
  explicit list of the words that hold them by chance (catalog-offers,
  analog-offset, blog-offers, dialog-off…, design-office), and the
  separated spellings (`log-out`, `sign_out`, `log-off`) as strong words, so
  no title rescues them (`sign-off` stays content). Every case found in the
  four reviews is a test (`test_actions.py`, `test_verify_assets.py`).
  panel-api `RetireDraft` answers `ErrNotDraft` when nothing was retired,
  so no withdrawal is audited that did not happen.
- **Status:** agent, panel-api green.
- **Next steps:** as above.

### 2026-11-22 — Third re-review of the review fixes

- **Branch:** fix/independent-review
- **What:** a seventh independent reviewer found four problems; fixed:
  - gateway: a visitor closing or seeking a video was reported as
    `legacy.error stream_cut` (the legacy read stops with the request's
    context), which turns a site's health Degraded; `copyMedia` now treats
    a read error after the client left as a normal end
    (`TestMediaClientGoneIsNoLegacyError`, which fails without the fix).
  - agent: log off / sign off match as a word of their own or a camelCase
    step, never inside a word (`/catalogoffers/`, `/designoffice/` are
    pages; `/doLogoff`, `/SignOff.aspx`, `/Home/LogOffUser` actions);
    verification judges a file's folders against the strong words only
    (`/lib/dropzone/…js`, `/assets/update/x.png` checked again).
  - panel-api: the withdrawal of a correction is now tested on its real
    path (a trigger takes the draft on mid-correction:
    `TestCorrectionWithdrawnWhenTakenOnMidway`), and audited only when the
    retirement succeeded.
  - Docs: `docs/agent.md`, `docs/map-format.md`.
- **Status:** gateway (race), panel-api, agent green.
- **Next steps:** as above.

### 2026-11-22 — Second re-review of the review fixes

- **Branch:** fix/independent-review
- **What:** a sixth independent reviewer found regressions in the previous
  fixes; all fixed, each with a test:
  - gateway fonts: a font of another format is served only if `types`
    allows that format (`fontsig.Allowed`), otherwise refused; EOT needs its
    version field, so 34 bytes + `LP` + HTML is no font
    (`TestEOTNeedsItsVersion`, `TestCheckAsset`).
  - gateway media: a stream the legacy server cuts is aborted, never served
    as whole (`copyMedia` reports `mediaCut`; `TestMediaCutStreamFails`);
    with `max_concurrency: 1` a stream holds no budget slot
    (`TestMediaBudgetOfOne`); per-visitor cap tested apart
    (`TestMediaStreamsPerVisitor`).
  - agent: `logoff`/`signoff` actions anywhere again (`/doLogoff`,
    `/SignOff.aspx`), separated spellings as whole words; verification
    reads a file's folders for the action rule (`/account/logout/bye.png`
    not opened, `reset.css` checked); `_secret_like` finds random tokens by
    letter/digit alternation in each part (`aB3dE5gH7…_q` skipped,
    `iphone_15_pro_max_256gb` and `electronics2024` kept).
  - panel-api: a withdrawn correction logs a failed retirement and is
    audited (`map.correction_withdrawn`, described in the panel);
    `TestNoCorrectionOverAPersonsDraft`.
  - Docs: `docs/map-format.md`, `docs/agent.md`.
- **Why:** re-review verdict "do not ship".
- **Status:** gateway (race), panel-api, agent green.
- **Next steps:** as above.

### 2026-11-22 — Re-review of the review fixes

- **Branch:** fix/independent-review
- **What:** a fifth reviewer, independent of the first four, re-checked the
  fixes above and confirmed regressions in them; all fixed:
  - gateway media: releasing the budget early only moved the outage to the
    legacy server's workers. Streams now hold their slot while read, within
    a media share of half the budget (`site.Media`, `mediaSlots`) and two
    streams per visitor (`mediaClients`); 503 beyond. A first range shorter
    than the signature (`bytes=0-1`, Safari) is probed instead of refused.
    An oversized unannounced stream in a preview gives the verdict
    `asset_too_large` instead of dropping the admin connection
    (`copyMedia` reports, the caller aborts only real clients).
  - gateway fonts: a declared font of another format is served under its
    real type instead of refused; TrueType/OpenType/collections are one
    format (`fontsig.Same`, `ttcf` → `font/collection`). CSP sandbox only
    for HTML and XML documents (`text/xml`, `application/xml`, `+xml`), not
    office types containing "xml".
  - agent: `log-off`/`log_out`/`sign_out` match whole words only
    (`/blog-offers/`, `/sign-off-letter-examples/` are pages again);
    verification no longer skips files named like actions (`reset.css`,
    `delete-icon.svg`), as the crawl; `query_dropped` covers literal routes
    and keeps them out of `visitors_missing`; `_secret_like` no longer takes
    `iphone_15_pro_max_256gb` for a token; `gateway_route` chooses by
    structure like the router.
  - panel-api: if a person takes the draft on while the agent corrects it,
    the correction is withdrawn (`RetireDraft`) and the draft keeps no link
    to it, so the discovery page never points at an orphan.
  - Tests: gateway `TestMediaPausedPlayersKeepTheSiteUp` (media share),
    `TestMediaShortFirstRange`, `TestTryAssetHead` (preview of an oversized
    stream), `TestCheckAsset`, `fontsig.TestSfntFamily`; agent
    `test_actions.py`, `test_query_merge.py` (`SeedsTest`, files refused,
    literal routes), `test_verify_assets.py`. Docs: `docs/map-format.md`,
    `docs/agent.md`.
- **Why:** the re-review verdict was "do not ship".
- **Status:** gateway (race), panel-api, agent (62) green. Not covered by
  tests: the withdrawal of a correction in panel-api (store path only) and
  the panel-ui states (no `node_modules` here; CI type-checks).
- **Next steps:** discovery on metaliquid and the WordPress site, then
  release 0.2.10; distinct-client counting on the panel side.

### 2026-11-22 — Independent review: visitors are never evidence; media, fonts, verification

- **Branch:** fix/independent-review
- **What:** fixes for the findings four independent reviewers confirmed
  with reproducing tests on the changes since 0.2.8 (gateway, discovery,
  verification and panel, adversarial).
  - **Principle (agent):** what visitors ask for is never evidence. Reported
    files and query keys are no longer opened nor added: `discovery.seeds`
    skips reported files, `path?key=` and capability-like paths (long
    random segments, UUIDs); `mapbuild` lists them for the person
    (`files_refused`, `query_dropped`); `_reported_keys` and `WIDE_QUERY`
    are gone. `_merge_query` adds only keys the site's own links use with a
    value, typed as for a new route, to the route the gateway would pick
    (`gateway_route`: literal before `/{slug}`); `covered()` checks public
    names.
  - **Passive types only (agent, new `assettypes.py`):** discovery and
    verification add to an existing prefix only images, fonts, audio,
    video, stylesheets, PDF and office documents, and scripts a page loads;
    never text, CSV, archives, SQL, JSON, HTML or XML, and never a type the
    schema rejects.
  - **Verification (agent `verifyassets.py`):** applies the crawler's rules
    (no sensitive file, no action link), skips pages answering under a
    prefix, checks each file once, never removes a prefix, and a prefix
    with videos never drops below their 512 MiB (`mapbuild.size_limit`).
  - **Discovery (agent):** `is_action` treats as content only 4+-word
    titles with the action word inside (`/cart/item-12-remove`,
    `/my-plan-cancel`, `/Account/LogOff`, `sign_out`, `log_out` are actions
    again; regression from #85); no new asset prefix over skipped actions,
    excluded paths, script requests or any non-file URL seen; a file's
    type sent by the site is never overwritten by a guess; a page answering
    for a file name is not that file; sitemap `<loc>` unescaped (`&amp;`)
    and CDATA read; `http.client.HTTPException` becomes `ReachError` (one
    broken answer no longer aborts a discovery).
  - **Gateway media (`assetmedia.go`):** the legacy budget slot is released
    once the response is checked, not after the client read it (paused
    players took the site down); a stalled client is dropped after 60 s;
    the signature is checked whatever range is asked (a separate
    `bytes=0-11` probe for mid-file ranges); a 206 nobody asked for or
    without `Content-Range` is refused; a 416 only for a file the map would
    serve, with security headers and an event; a body over `max_size_kb`
    the legacy server did not announce is aborted, never served short.
  - **Gateway assets (`asset.go`):** a declared font type must match the
    signature; the response carries the one checked `Content-Type`; HTML
    and XML types get the CSP sandbox. `dropped.go`: the route's pattern is
    reported, never the request path (it held visitors' values), after
    authorisation.
  - **Panel:** `store.Supersede` locks the draft and does nothing if a
    person has taken it on (`ErrNotDraft`); an automatic verification of a
    version no longer a draft creates no corrected draft. `DraftNotice`
    says when the agent could not verify its draft. The report shows the
    files and page options visitors asked for that the map refuses.
  - Tests: gateway `TestMediaPausedPlayersKeepTheSiteUp`,
    `TestMediaRangeKeepsTheSignatureCheck`, `TestMedia416OnlyForServedFiles`,
    `TestMediaUnannouncedSizeAborted`, `TestCheckAsset` (fonts),
    `TestDroppedQueryKeysReported` (pattern); agent `test_verify_assets.py`,
    `test_query_merge.py`, `test_assets_study.py`, `test_actions.py`,
    `test_crawl_sources.py` (sitemaps, malformed answers, prefix guards,
    route choice); panel-api `TestDiscovery` (no supersede once in review).
  - Docs: `docs/agent.md`, `docs/map-format.md`, tenant manual and in-app
    help.
- **Why:** #80–#86 let anonymous visitors widen the map (expose CSV/log
  files, push query keys to the legacy app, open capability URLs), let a
  few paused videos take a site down, let a chosen range skip the media
  check, and reintroduced crawling of action links.
- **Status:** gateway (race), panel-api, agent tests and `check:help`
  green. Not yet run against the real sites.
- **Next steps:**
  1. Independent re-review of this PR, then discovery on metaliquid and the
     WordPress site before releasing 0.2.10.
  2. The unmapped reporter counts one IP once per minute, so one client
     can look like many (thresholds for page seeds): count distinct
     clients on the panel side.
  3. Resume strict mode (`feat/strict-mode`).

### 2026-11-22 — After a discovery, one draft to review

- **Branch:** fix/one-draft
- **What:**
  - panel-api `store.Supersede`: when the agent corrects the draft a
    discovery proposed (automatic verification, `requested_by` empty), the
    first draft is retired (unsigned: never a rollback target) and the
    discovery's `draft_version` becomes the corrected version; audit
    `map.superseded`. `RecordVerification` stores the corrections (and the
    measure before them) on the corrected version. A verification asked by
    a person keeps both versions.
  - panel-ui `features/discovery/components/DraftNotice.tsx`: while the
    agent verifies and corrects, the discovery page says so instead of
    "review it and publish it"; then it links the draft to review (the
    corrected one) with its verification. `useVerification` takes
    `enabled`; audit text for `map.superseded`.
  - Test: `TestDiscovery` (first draft retired, discovery proposes the
    corrected version, corrections on it). Docs: `docs/agent.md`, tenant
    manual and in-app help.
- **Why:** the discovery page told the person to review and publish the
  first draft (version 9) while the agent had already corrected it into
  version 10: publishing as told shipped the uncorrected map.
- **Status:** panel-api tests green (PostgreSQL), `check:help` green.
  panel-ui not type-checked in this session (no `node_modules`, network
  off): CI runs `tsc`, tests and boundaries.
- **Next steps:**
  1. Release 0.2.10.
  2. Resume strict mode (`feat/strict-mode`).

### 2026-11-22 — Multipart: refuse field names with a backslash

- **Branch:** fix/route-building
- **What:** `gateway/internal/rebuild/formats.go` (`Multipart`): a part
  whose name contains `\` is refused as malformed. Regression seed
  `internal/rebuild/testdata/fuzz/FuzzMultipart/backslash-name`. Doc
  `docs/map-format.md` (rebuilding).
- **Why:** CI's `FuzzMultipart` found that a part named `\` was rebuilt as
  `name="\"`, which Go (and some legacy servers) read as an escaped quote:
  the field reached the legacy application with another name (empty).
  Browsers never send a backslash in a field name.
- **Status:** `go test ./internal/rebuild` green; 40 s of fuzzing clean.
- **Next steps:** release 0.2.10.

### 2026-11-22 — Routes learn the query keys they lacked

- **Branch:** fix/route-building
- **What:**
  - agent `mapbuild._merge_query`: a route already in the map (same
    shape) gets the query keys the study saw and it lacked, instead of
    being skipped; integer when seen with 3+ integer values, otherwise
    `WIDE_QUERY` (text ≤1000, `no_html`, `no_template_meta`), narrowed later
    by verification. Only GET routes with a GET legacy request, only keys a
    map can hold. Report key `query_added`. `covered()` reads a reported
    `path?key=` as covered only when the route declares the key.
  - gateway `internal/server/dropped.go` (`reportDropped`, called by
    `serveRoute`): the names of undeclared query keys a route dropped are
    reported as unmapped `path?key=` (at most 5 per request; never values;
    never `utm_*`, `fbclid`, `gclid` and other tracking keys).
  - Tests: gateway `TestDroppedQueryKeysReported`; agent
    `test_query_merge.py`. Docs: `docs/agent.md`, `docs/map-format.md`,
    tenant manual and in-app help.
- **Why:** the gateway drops undeclared query keys silently: a sort or a
  filter the study had not seen made the page differ from the legacy one
  with no refusal, no log and no way for discovery to learn it; and routes
  already in the map never got new keys.
- **Status:** gateway tests, agent unittests (57), `check:help` green.
- **Next steps:**
  1. Release 0.2.10 (#83, #84, #85 and this).
  2. Resume strict mode (`feat/strict-mode`).

### 2026-11-22 — Crawler: sitemaps by any name, the site's other names, slugs

- **Branch:** fix/crawler-sources
- **What:**
  - `crawler._crawl`: `/robots.txt` is read again (since `.txt` became a
    file type it was treated as a file and its `Sitemap:` lines ignored);
    any XML answer that is a sitemap or sitemap index is read
    (`extract.is_sitemap`), not only paths starting with `/sitemap`
    (WordPress's `/wp-sitemap.xml` was skipped).
  - `crawler._known_file`: the crawler's rules are applied before the file
    is opened (a visitor's `/backup/…pdf` was fetched, then dropped).
  - `fetch.Fetcher.add_hosts` / `same_site`: the site's public domain,
    aliases and the `www.`/bare twin of each are the site's own; discovery,
    verification and the browser guard use them (absolute links and sitemap
    entries with the public name were dropped as other sites).
  - `webapps.base.is_action`: a path segment that is a content slug (3+
    words, not starting with an action verb, no `logout`/`delete`/…) is not
    an action (`/blog/how-to-remove-stains/` was skipped and reported as a
    link that changes something).
  - Tests: `agent/tests/test_crawl_sources.py`, `test_actions.py`.
    Doc `docs/agent.md` (Seeds, read-only rule 2).
- **Why:** study coverage: whole sitemaps, absolute links and blog posts
  were lost; a safety rule was applied too late.
- **Status:** agent unittests (56) green.
- **Next steps:**
  1. Release 0.2.10 (#83, #84 and this).
  2. Resume strict mode (`feat/strict-mode`).

### 2026-11-22 — Verification checks the files; no asset prefix over pages

- **Branch:** feat/study-depth
- **What:**
  - agent `verifyassets.py` (new): verification opens every file the public
    pages name (HTML, links, stylesheets; up to 200 files, 30 sheets) under
    an asset prefix of the draft, directly and through the gateway (preview
    `HEAD`), and reports refused ones as broken; the correction adds the type
    the site sends (never a generic one) or `max_size_kb`, and removes an
    asset prefix that refused a page (the area serves that folder).
    `Gateway.preview` takes a method; `Measure` has `files`, `asset_fixes`,
    `broken_files`; `correct.widen` calls `verifyassets.widen`.
  - agent `mapbuild`: no new asset prefix over a folder holding pages, form
    actions or script requests the study saw (the router tries asset
    prefixes before the area, POSTs included: such a prefix refused pages).
  - agent `crawler.ASSET_EXT`: documents (`.doc(x)`, `.xls(x)`, `.ppt(x)`,
    `.od*`, `.rtf`, `.csv`, `.txt`, `.epub`), `.mjs`, `.webmanifest`,
    `.wasm`; archives stay excluded (`rules.SENSITIVE`).
  - agent `extract`: `<meta http-equiv="refresh">`, `<frame>` as links;
    `<link rel="manifest">`, `<input type="image">` as files.
  - Tests: agent `test_verify_assets.py` (files checked and mended, prefix
    over a page removed, file the site lacks), `test_assets_study.py`
    (documents, no prefix over pages); gateway `TestTryAssetHead`.
  - Docs: `docs/agent.md` (step 1 Files/Links, step 4 Files), tenant manual
    and in-app help (Verification).
- **Why:** a draft blocking fonts or videos was measured identical (only
  pages were compared); an asset prefix over a folder with pages refused
  them and the correction could not mend it; linked documents under an
  asset prefix were never added.
- **Status:** agent unittests (50), gateway tests, `check:help` green.
- **Next steps:**
  1. Release 0.2.10.
  2. Resume strict mode (`feat/strict-mode`).

### 2026-11-22 — Stability fixes on fonts, media and file discovery

- **Branch:** fix/stability-review
- **What:**
  - gateway `checkAsset`: a font sent with no `Content-Type` or an
    unreadable one is treated as generic (the signature decides) instead of
    refused; AVIF/HEIF images pass the raster content check (`isRaster`,
    net/http does not recognise them). Test `TestCheckAsset`.
  - agent `extract.srcset`: candidates split as the HTML standard says, so a
    CDN URL with commas (`w_400,h_300`) stays whole.
  - agent `crawler`: new extensions (`.avif`, `.bmp`, `.mov`, `.flac`,
    `.aac`, `.opus`); a file opened because the gateway refused it keeps the
    type the site sends when it is an image/audio/video type of the
    extension's family (`audio/x-wav`), so the gateway's check passes; its
    `Content-Length` is recorded; `http.client.HTTPException` no longer
    escapes `_known_file`.
  - agent `mapbuild._fit_sizes`: an asset prefix whose files exceed the
    gateway's default limit (10 MiB, 512 MiB for media) gets `max_size_kb`
    = largest size + 10% (up to 4 GiB), reported in `assets_added`.
  - Tests in `agent/tests/test_assets_study.py` (srcset, `audio/x-wav`,
    30 MiB PDF). Docs: `docs/agent.md`, `docs/map-format.md`, tenant manual
    and in-app help.
- **Why:** review of #77–#81: fonts with no type, AVIF images, CDN srcsets,
  sites sending non-canonical media types and files above the default size
  would still have needed a hand edit or stayed blocked forever.
- **Status:** gateway tests, agent unittests (47), `check:help` green.
- **Next steps:**
  1. Release 0.2.10.
  2. Resume strict mode (`feat/strict-mode`).

### 2026-11-22 — CLAUDE.md: definition of done

- **Branch:** chore/release-0.2.9
- **What:** `CLAUDE.md` gets a *Definition of done*: tests and checks, every
  describing document (manuals, in-app help, map format, agent, architecture,
  install) updated in the same PR, changelog, and no manual step for the
  customer except publishing a map.
- **Why:** the tenant manual was updated only after the owner asked.
- **Status:** documentation only.
- **Next steps:** resume strict mode (`feat/strict-mode`).

### 2026-11-22 — Tenant manual: discovery finds every file

- **Branch:** chore/release-0.2.9
- **What:** `docs/manual/tenant.md` (Discovery) and the in-app help
  (`panel-ui/src/features/help/content/tenant.md`, via `sync-help.mjs`)
  say that discovery finds every file the pages load and learns from files
  the gateway refused; release notes list the documentation touched.
- **Why:** #80 and #81 changed what the customer sees in discovery; the
  manual still implied types might be added by hand.
- **Status:** `check:help` green.
- **Next steps:** resume strict mode (`feat/strict-mode`).

### 2026-11-22 — Release 0.2.9

- **Branch:** chore/release-0.2.9
- **What:** `VERSION`, agent version, `INSTALL.md`, `README.md`,
  `deploy/compose/.env.example` point to 0.2.9; release notes
  `docs/releases/0.2.9.md`.
- **Why:** release the agent's full file study (#80, #81).
- **Status:** documents only. Tag `v0.2.9` on the merge commit publishes the
  images.
- **Next steps:** resume strict mode (`feat/strict-mode`).

### 2026-11-22 — The agent studies every file the site loads

- **Branch:** feat/agent-full-study (on top of fix/agent-asset-types, #80)
- **What:**
  - `agent/equalizer_agent/css.py` (new): `url()` and `@import` references
    of a piece of CSS, comments and `data:` skipped.
  - `extract.py`: images, video, audio, `track`, `embed`, `object` give
    `src`, `data-src`, `data-lazy-src`, `data-original`, `poster`,
    `data-poster`, `data` and every `srcset`/`data-srcset` candidate;
    `<style>` blocks and `style=""` attributes give their CSS references.
  - `crawler.py`: stylesheets found are read (up to 60, imports followed)
    for fonts and images, at the end of each pass; a seed that is a file is
    fetched and becomes an asset only if the site answers 200.
  - `discovery.py`: a file the gateway reported is a seed with one visitor
    (pages still need two).
  - gateway `internal/server/asset.go` (`refuseAsset`): an asset refused
    for its type or size is reported as unmapped, so discovery studies it;
    a content mismatch is not.
  - Tests: `agent/tests/test_assets_study.py` (CSS refs, HTML attributes,
    whole study with imported stylesheets, a visitor's file, a file the
    site does not serve), gateway `TestRefusedAssetsReported`. Doc
    `docs/agent.md` (Explore → Files).
- **Why:** the customer never edits the map by hand except to publish it.
  Fonts named only in CSS, lazy images and files blocked in production
  (metaliquid's fonts and video) escaped discovery.
- **Status:** agent unittests (46) and gateway tests green.
- **Next steps:**
  1. Release 0.2.9 (#80 and this).
  2. Resume strict mode (`feat/strict-mode`).

### 2026-11-22 — Discovery widens the types of existing asset prefixes

- **Branch:** fix/agent-asset-types
- **What:** `agent/equalizer_agent/mapbuild.py`: new `_widen_assets` adds to
  the asset prefixes already in the map the types of the files crawled under
  them (longest prefix wins); the additions are in the report's
  `assets_added` with `"widened": true`. New prefixes are proposed as before.
  `crawler.py` `ASSET_EXT` knows more media and fonts (`.otf`, `.m4v`,
  `.webm`, `.ogv`, `.mp3`, `.m4a`, `.ogg`, `.wav`). Test
  `AssetTypesTest`.
- **Why:** after 0.2.8 a video under `/site/templates/` on
  metaliquid.thevidocq.com still needed `video/mp4` added by hand: discovery
  skipped prefixes already in the map. The customer must never edit types by
  hand.
- **Status:** agent unittests green (43). Types come from the file
  extension, as before (never a generic `application/octet-stream`).
- **Next steps:**
  1. Release 0.2.9 with this fix.
  2. Resume strict mode (`feat/strict-mode`).

### 2026-11-22 — Release 0.2.8

- **Branch:** chore/release-0.2.8 (on top of #77 and #78)
- **What:** `VERSION`, agent version (`agent/pyproject.toml`,
  `equalizer_agent/__init__.py`), `INSTALL.md`, `README.md`,
  `deploy/compose/.env.example` (it still said 0.2.3) point to 0.2.8;
  release notes `docs/releases/0.2.8.md`.
- **Why:** release fonts with a generic type (#77) and streamed media (#78).
- **Status:** documents only. Tag `v0.2.8` on the merge commit publishes the
  images.
- **Next steps:**
  1. Discovery/panel: propose media types for asset prefixes already in the
     map.
  2. Resume strict mode (`feat/strict-mode`).

### 2026-11-22 — Stream audio and video assets with byte ranges

- **Branch:** feat/asset-media-stream (on top of fix/asset-font-types, #77)
- **What:** new package `gateway/internal/mediasig` (media file names,
  `audio/`/`video/` types, container signatures: MP4/MOV, WebM, Ogg, MP3,
  WAV, FLAC). New `gateway/internal/server/assetmedia.go`: when an asset file
  looks like media and its prefix allows an audio/video type, the legacy
  response is opened with `Legacy.Open` and streamed (no 16 MiB buffer), a
  single checked `Range` is passed on (206/416 answered as the legacy app
  does), the declared type must be allowed, the signature is checked when the
  response starts at byte 0, and the full size must fit `max_size_kb`
  (default 512 MiB for media). Tests `TestMediaAssets` (17 MiB video, range,
  range list dropped, fake and HTML "videos" refused) and `mediasig` tests.
  Map format doc updated.
- **Why:** `homepage-video.mp4` on metaliquid.thevidocq.com failed with
  `response_too_large`: every asset was buffered up to 16 MiB, and players
  need ranges to seek.
- **Status:** gateway `gofmt`/`vet`/`test` (and `-race` on server) green.
  The site's map must list `video/mp4` in the `/site/templates/` types.
  A mid-file range has no signature to check: its declared type is still
  checked and `nosniff` is set.
- **Next steps:**
  1. Panel/agent: propose `video/mp4` (and other media seen) for asset
     prefixes already in the map, not only for new prefixes.
  2. Resume strict mode (`feat/strict-mode`).

### 2026-11-22 — Fonts served with a generic or old media type

- **Branch:** fix/asset-font-types
- **What:** new package `gateway/internal/fontsig` recognises web fonts by
  signature (WOFF, WOFF2, TrueType, OpenType, EOT) and maps old names
  (`application/x-font-woff`, `font/x-woff`, ...) to canonical types.
  `checkAsset` (`internal/server/asset.go`): when the declared type is not in
  the prefix's `types`, a font whose signature is an allowed type is served
  anyway, with the canonical `Content-Type`; a declared font type that
  disagrees with the signature is still refused. Tests in `fontsig` and
  `TestAssets`. Map format doc updated.
- **Why:** on metaliquid.thevidocq.com (ProcessWire, origin meta-liquid.com)
  every `.woff` under `/site/templates/` was refused with
  `asset_type_not_allowed` although the map allows `font/woff`: the origin
  sends fonts with a type the map does not list.
- **Status:** gateway `gofmt`/`vet`/`test` green. Not yet checked against the
  live site (the exact type the origin sends is in the gateway WARN log
  `asset rejected`, field `content_type`).
- **Next steps:**
  1. Large media assets: `homepage-video.mp4` fails with
     `response_too_large` (legacyclient buffers up to 16 MiB). Stream
     audio/video assets with `Range` support and `video/mp4` in asset types.
  2. Resume strict mode (`feat/strict-mode`): the branch was never pushed,
     nothing of it is on `main`.

### 2026-11-21 — The legacy site is asked for its own name (release 0.2.7)

- **Branch:** feat/legacy-host
- **What:** `legacy.base_url`'s host (Host, SNI, certificate check) is no
  longer always the public domain. `panel-api/internal/api/legacy_host.go`
  (`legacyHost`): an advanced override wins; in direct mode with a name as
  address, that name; otherwise the public domain. Used by discovery start
  (`discoveryReach`), the hand-opened template (`templates.go`) and
  `PUT /apps/{app}/reach` (new `store.SetBaseHost`). Optional `legacy_host`
  in the three bodies (contract + client). UI: `ReachFields` gets
  *Advanced → Name asked of the site* and a clearer direct-mode notice.
  Docs: manual (the name asked, *A parallel domain* step by step), map
  format, INSTALL (`EQ_OUTBOUND_IPS` behind Cloudflare,
  `EQ_DIRECT_DESTINATIONS` needs a port, connector update example), env
  example. Release 0.2.7.
- **Why:** a parallel public domain (metreta.thevidocq.com) in front of a
  site behind Cloudflare (www.metreta.com) sent the test name to the origin,
  which Cloudflare refuses. The direct address already names the site.
- **Status:** panel-api `gofmt`/`vet`/`test` green (new `TestLegacyHost`,
  `TestSetBaseHost`, `TestDiscoveryLegacyHost`, `TestReachLegacyHost`);
  panel-ui `tsc`/boundaries green; schemas valid. Screenshots of the reach
  form not regenerated.
- **Next steps:** `check.sh`/panel to report a malformed
  `EQ_DIRECT_DESTINATIONS` and an `EQ_OUTBOUND_IPS` that looks like
  Cloudflare's; regenerate screenshots; resume strict mode
  (`feat/strict-mode`).

### 2026-11-21 — Discovery: the chosen reach replaces the template's (release 0.2.6)

- **Branch:** fix/discovery-direct-template
- **What:** `panel-api/internal/api/agent.go`: when the first discovery hands
  the agent a CMS template, the site's chosen reach (`withReach`) replaces
  the template's connector reach. Test `TestDiscoveryDirectTemplate` (fails
  without the fix). Release 0.2.6 (`VERSION`, agent version,
  `docs/releases/0.2.6.md`, `INSTALL.md`, `README.md`).
- **Why:** templates are written for a connector; in direct mode the
  connector stayed empty and the proposed map was refused with
  `no connector "" in this tenant`, so direct mode never got past the first
  discovery. The hand-opened template path already applied the reach.
- **Status:** panel-api `gofmt`/`vet`/`test` green.
- **Next steps:** make a malformed `EQ_DIRECT_DESTINATIONS` visible instead
  of stopping panel-api silently; resume strict mode (`feat/strict-mode`).

### 2026-11-21 — Agent: direct mode by name for discovery (release 0.2.5)

- **Branch:** fix/agent-direct-name
- **What:** the discovery agent (`agent/equalizer_agent/fetch.py`) had its
  own direct-mode check that accepted only a public IP literal or an
  `EQ_DIRECT_DESTINATIONS` entry, so discovery against a site reached
  directly by name (added in 0.2.4) failed with the old message. The agent
  now mirrors the gateway: `_reachable_public` accepts a public IP or a name
  that resolves to public addresses only; `_safe_connect` resolves a name at
  connect time and connects only to a public address (rebinding to a private
  or reserved address is refused). `_Pinned` uses `_safe_connect`. Tests
  added (`test_fetch_direct.py`): name→public allowed, name→private refused,
  name not resolving refused. Release 0.2.5 (`VERSION`,
  `agent/__init__.py`, `agent/pyproject.toml`, `docs/releases/0.2.5.md`,
  `INSTALL.md`, `README.md`).
- **Why:** 0.2.4 enabled direct-by-name in the panel and gateway but missed
  the agent, so the feature failed exactly where the user first hit it — the
  first discovery of a site behind a CDN.
- **Status:** `compileall` and the 42 agent unittests green. No change to
  gateway/panel/ui/schemas.
- **Next steps:** resume strict mode (branch `feat/strict-mode`).

### 2026-11-21 — Onboarding flow, direct mode by name, delete a site (release 0.2.4)

- **Branch:** fix/onboarding-reach-ux
- **What:** five fixes reported after 0.2.3, plus the release.
  - *Onboarding (36):* adding a site now opens **Discovery**, not an empty
    map. `router.tsx` index redirects to `discovery`; `SiteLayout` tab order
    is Discovery → Map → Isolation → Certificate → Login;
    `CreateSiteDialog` navigates to `discovery` after creating; `MapsPage`
    empty state points to Discovery (with a hand-built fallback).
  - *Delete a site (34):* `DELETE /v1/apps/{app}` (admin), `store.DeleteApp`,
    migration `0024_app_delete_cascade.sql` (a DO block reads each real FK
    name from the catalogue and re-adds it `ON DELETE CASCADE`), contract
    `delete` op, `useDeleteApp` hook and `DeleteSite` component (type the
    name to confirm) in the site header.
  - *Direct mode by name (37):* `transport.Direct` resolves a name at dial
    time and connects only to a public address (`gateway/internal/publicip`);
    panel `checkReach`/`checkDirectDest` accepts a public IP or a name that
    resolves to public addresses only; `ReachFields` hint/placeholder and
    the manuals updated.
  - *Double spinner (38):* the Discovery table Status column no longer shows
    a spinner; the live one (with page count) stays in the report.
  - *Mobile "Protect a site" (35):* the add-site dialog moved from `Sidebar`
    to `Shell`, so closing the phone menu no longer unmounts it (regression
    from 0.2.3's fix 32).
  - *Release:* `VERSION` 0.2.4, `docs/releases/0.2.4.md`, `INSTALL.md`,
    `README.md`.
- **Why:** the first-run experience started on an empty map and confused
  people; direct mode could not point at a site behind a CDN; a site could
  not be deleted; two spinners and a broken mobile dialog were reported.
- **Status:** gateway + panel-api `gofmt`/`vet`/`test` green; panel-ui
  `tsc`/`test`/`build`/boundaries green; `scripts/validate_schemas.py` green.
  New tests: `publicip`, `transport` name-guard, `checkDirectDest`,
  `TestDeleteApp`. Screenshots not regenerated (no visual-regression need;
  text docs updated).
- **Next steps:** resume strict mode (branch `feat/strict-mode`): frontend
  enforcement selector, coverage metric, observe `would_block` filter,
  attacklab rerun, docs, PR.

### 2026-11-21 — Release 0.2.3 documents

- **Branch:** chore/release-0.2.3
- **What:** `VERSION`, `.env.example`, `INSTALL.md` and `README.md` point to
  0.2.3, and `docs/releases/0.2.3.md` holds the release notes (reach from the
  site page, mobile menu fix, request-rebuilding hardening and fuzzing).
- **Why:** the v0.2.3 tag was created from #71 without the release documents,
  so the panel showed no release notes and `VERSION` still read 0.2.2.
- **Status:** the images for 0.2.3 are already built and published from the
  tag. These documents are cosmetic and do not block updating the server.
- **Next steps:** optionally move the v0.2.3 tag onto this commit so the tag
  carries VERSION and the notes; otherwise the next tag picks them up.

### 2026-11-21 — Reach from the site page, mobile menu over dialogs

- **Branch:** feat/reach-and-mobile-menu
- **What:**
  - **Reach slegata dalla discovery.** How Equalizer reaches a site
    (connector or direct, which connector, which address) can now be changed
    from Site → Isolation, without rerunning discovery:
    - `PUT /v1/apps/{app}/reach` republishes the current map with the new
      `legacy.reach`, keeping the tuned entries (`store.SetReach`);
    - the App exposes `reach_connector_id` and `reach_address` so the form
      is pre-filled;
    - a `ReachForm` at the top of the Isolation tab; `app.reach` in the
      activity log.

    The first reach is still chosen when discovery runs the first time (the
    map does not exist yet); afterwards it is changed in Isolation. The
    discovery StartForm already only asked for it the first time.
  - **Menù mobile sopra i dialog.** Opening a dialog (e.g. "Protect a site")
    from the mobile sidebar closes the menu, and dialogs now sit above the
    menu (z-50). The dialog was appearing behind the open menu.
  - Tenant manual and in-app help updated; the Isolation screenshot
    regenerated with the new section.
- **Why:** feedback 32 and 33 from the phone. The reach was fixed at the
  first discovery and could only be changed by rerunning it.
- **Status:** Go (with PostgreSQL), vitest, `check:api`, `check:help` and
  boundaries pass.
- **Next steps:** strict mode (PR B): open/observe/strict per site, routes
  recognised in the query, coverage indicator.

### 2026-11-21 — Request rebuilding hardened, fuzz tests

- **Branch:** fix/rebuild-hardening
- **What:**
  - **Fix (security):** areas with roles could be stepped around with paths
    that legacy servers read as the protected one:
    - `//admin/` (Apache, nginx);
    - `/admin;x/` (Tomcat, Jetty);
    - `/admin./` and `/Admin/` (Windows).

    Entries and areas are now matched on `pathkey.Of(path)`, the path as
    servers read it. A path that falls in another area without case is
    refused (`ambiguous_path`, 400). A test reproduces the bypass on the old
    code.
  - JSON:
    - invalid UTF-8 and unpaired surrogate escapes are refused instead of
      being turned into U+FFFD (new package `jsontext`, used by area
      rebuilding and API bodies);
    - a key written twice in an object is refused.
  - Multipart: field and file names are written as browsers write them
    (`%22`, `%0D`, `%0A`), never with backslash escapes that some servers read
    and others do not.
  - XML: processing instructions other than the declaration (`<?php …?>`)
    are dropped.
  - Tests fix the rule "decoded once, never transformed":
    - `%2527` stays `%2527`;
    - repetitions and `;` are kept;
    - full-width or modifier apostrophes are never mapped to `'` (refused for
      a latin1 legacy app);
    - `%u0027`, `%00` and bad UTF-8 are refused.
  - Fuzz tests run for a few minutes each locally with no failure:
    `FuzzPairs`, `FuzzJSON`, `FuzzXML`, `FuzzMultipart`, `pathkey.FuzzOf` and
    `jsontext.FuzzValid`. CI runs each for 20 s.
  - Docs: map format (how paths are matched) and the architecture principle.
- **Why:** the rebuilding code is the most critical part of the product. A
  rebuild that interprets ambiguous input, or matches paths differently from
  the legacy server, can open a hole instead of closing one.
- **Status:**
  - Gateway tests pass with `-race`.
  - Checked and found sound: a non-UTF-8 legacy charset refuses unrepresentable
    characters (no best-fit); precise fields refuse repeated parameters;
    precise forms rebuild only mapped fields.
- **Next steps:**
  1. Strict mode (POST and APIs only through precise entries), with routes
     recognised in the query (`rest_route`, `option`/`task`, the `action` of
     `admin-ajax.php`) and PATH_INFO (`/x.php/anything` runs `x.php`).
  2. A longer fuzz campaign.
  3. An external pentest of the gateway before launch.
### 2026-11-21 — Post-upgrade check, release 0.2.2 documents

- **Branch:** chore/post-upgrade-check
- **What:** `deploy/compose/check.sh` checks an installation after an
  update. It covers:
  - the services and their state;
  - the release the services run against `EQ_VERSION`;
  - the migrations applied;
  - errors in the last 10 minutes, apart from "panel unreachable" while
    panel-api restarts;
  - the connector's release and tunnel.

  It ends with OK or the problems found (exit code 1). The "Update" section
  of `INSTALL.md` now gives the connector update commands and the check.
  `VERSION`, `.env.example`, `INSTALL.md` and `README.md` point to 0.2.2,
  and `docs/releases/0.2.2.md` holds the release notes.
- **Why:** the owner wants the same check after every upgrade. The 0.2.2 tag
  was created from #68 without the release documents.
- **Status:** the script was tried with a stubbed `docker`, and the same
  checks were run by hand on the VPS after the 0.2.2 update. Not yet run
  as a script on the VPS.
- **Next steps:** run `./check.sh` on the VPS. Mention it in every
  release's notes.

### 2026-11-21 — Second feedback round: HTTPS at a proxy, time zones, activity

- **Branch:** feat/second-feedback
- **What:**
  - HTTPS handled by a proxy in front (Cloudflare or any other):
    - new `public_scheme` value `https_proxy` (map schema, API, migration 0021);
    - the gateway serves such a site on the plain port with https addresses and
      Secure cookies, and sends no HSTS;
    - the panel asks for no certificate (TLS status `at_proxy`);
    - Site → Certificate offers the three choices.
  - Dates:
    - one format everywhere ("21 Nov 2026, 02:33:07 UTC+1", `shared/ui/dates.ts`);
    - a time zone per person (`users.time_zone`, migration 0022,
      `PATCH /v1/auth/me`, Account page, default UTC);
    - alert emails are written in each administrator's zone (UTC for the extra
      addresses).
  - Activity:
    - server-side search, time range and paging (`GET /v1/audit` with `q`, `from`,
      `to`, `cursor`);
    - a period picker (presets or from day to day, in the person's zone);
    - every action written as a sentence, "The agent" and "Equalizer" for what
      nobody did by hand;
    - `alert.sent` records what was reported and to whom.
  - Logs:
    - alert events have a title per kind and name the connector (`connector` in
      LogEvent, from the event detail);
    - request-only rows (map entry, request ID, the field-values footer) are
      hidden when not relevant;
    - no repeated "Served / Served".
  - Continuous discovery:
    - runs at a chosen hour and weekday (migration 0023, `discovery_hour` and
      `discovery_weekday` on PATCH /apps, `discovery_time` with next and last run
      on App);
    - a schedule never fires for a slot that passed before it was set.
  - Discovery:
    - a spinner while it runs;
    - the report and the list say when the AI was asked but not used, with the
      provider's refusal in plain words (no credit, key refused, provider down)
      from the agent.
  - Release shown at the bottom of the sidebar, with an About dialog (release
    notes link; service releases for platform users) from `GET /v1/about`.
  - The review compares a published version with the one published before it;
    leftover approval wording removed.
  - Tenant manual and in-panel help updated.
- **Why:** feedback 23–31 from using 0.2.1 on the VPS. The WordPress editor
  behind Cloudflare could not publish: mixed content, since the site was
  served in http behind an https proxy.
- **Status:**
  - Go, vitest, agent tests, schema validation, `check:api`, `check:help` and
    boundaries pass.
  - Not yet run in the lab end-to-end.
  - Rate limits deliberately unchanged (a later product feature).
- **Next steps:**
  1. Review and merge.
  2. Release 0.2.2 and update the VPS.
  3. On the VPS, switch the Cloudflare site to "HTTPS handled by a proxy in front".

### 2026-11-21 — Release 0.2.1

- **Branch:** chore/release-0.2.1
- **What:** `VERSION` 0.2.1, `EQ_VERSION` in `.env.example`, `INSTALL.md`
  (checkout, version, connector image, update example) and `README.md`
  point to 0.2.1; release notes `docs/releases/0.2.1.md`.
- **Why:** the fixes and decisions of the first installation (#66) and
  go-redis 9.7.3 (#65) go to the VPS as a release.
- **Status:** ready; the owner tags `v0.2.1` on main after the merge, and
  the publish workflow builds the images.
- **Next steps:** update the VPS to v0.2.1.

### 2026-11-20 — Lab run: redirect after a form, tagline, Dependabot

- **Branch:** fix/first-install-feedback
- **What:** the redirect-loop guard ignored the method: a form sent with
  POST and answered with a redirect to the same page in the other scheme
  (Joomla's administrator login) was stopped, though the browser follows
  with GET and nothing loops; now only GET and HEAD are checked (test
  added). Tagline: a minimum gap between the words. Lab: an https site
  is sent to https with 302 (`test_01`). CI: the changelog check skips
  Dependabot's pull requests (`contracts.yml`), which never carry an entry.
- **Why:** the full lab run (79 tests) failed only on Joomla's admin
  login; the tagline still touched at phone size; Dependabot's #65 failed
  the changelog check.
- **Status:** lab end to end: 78 passed, then onboarding and the other
  sites again after the fix, all passed. Panel checked at phone width
  (390 px) with Playwright: menu, sites, status, maps, isolation, new
  version, settings, people.
- **Next steps:** merge; tag v0.2.1 (owner); update the VPS; then
  `@dependabot rebase` on #65.

### 2026-11-20 — Direct mode for customer sites, manuals

- **Branch:** fix/first-install-feedback
- **What:**
  - Direct mode as a deployment option: a site that stays at its public
    address and accepts only Equalizer's addresses needs no connector.
    `checkReach` accepts in direct mode a public IP address and port written
    as such (`publicDest`; never a name, which could later resolve to the
    platform's network) or `EQ_DIRECT_DESTINATIONS`. Template and first
    discovery take `mode` (`newReach`). The agent reaches such a site pinned
    to the address with the site's name in Host and SNI (`_Pinned`), as the
    gateway does. Equalizer's addresses (`EQ_OUTBOUND_IPS`, or those of the
    panel's and relay's domains) are returned as `direct_from` and shown in
    the panel. Isolation of such a site cannot be checked from the
    platform, whose requests are the accepted ones: Status says so, the
    hourly check and "Check now" skip it, the Isolation page gives a
    command to run from another network. `App.reach_mode` tells the panel.
    UI: "How Equalizer reaches the site" (`ReachFields`) in the template and
    first discovery forms; neutral wording.
  - Requirement 2 and the decision table in `CLAUDE.md` and
    `docs/architecture.md` updated as decisions of the owner: direct mode,
    one administrator publishes, site certificates always uploaded.
  - Manuals (tenant, admin), `INSTALL.md`, `docs/map-format.md`: two roles,
    publish alone, direct mode, never-public sites, alert kinds, AI model
    list, tenant deletion, email reasons. `INSTALL.md` no longer says Caddy
    gets a certificate for a protected site (it never does).
  - Lab end-to-end tests: one administrator publishes, the second person is
    a viewer who cannot.
- **Why:** owner's decision after the first installation: a site already
  behind a perimeter that accepts only Equalizer does not need a connector.
  Automatic certificates for protected sites were proposed and declined:
  the certificate stays the one issued for the original site.
- **Status:** unit, API and agent tests pass; lab end-to-end not run yet in
  this commit.
- **Next steps:** run the lab end to end and the panel at phone width; open
  the PR; release 0.2.1 (tag by the owner) and update the VPS.

### 2026-11-20 — One person publishes, two roles, tenant deletion, AI models, alert kinds

- **Branch:** fix/first-install-feedback
- **What:**
  - Approval: no second person needed. An administrator publishes a
    version straight from draft (`publish` from draft, in review or
    approved), the author included; submit, approve and reject stay as
    optional steps. Publishing retires older versions still pending. The
    database rule `approver_id <> author_id` is dropped (migration 0019).
    Panel: a single "Publish" button (and "Discard"), a two-step rail.
  - Roles: administrator and viewer only. Editors and approvers become
    administrators, auditors viewers (migration 0019); viewers read sites,
    maps, status, activity and logs.
  - Tenant deletion: `DELETE /v1/platform/tenants/{tenant}` with the id
    typed again (`Store.DeleteTenant`, one transaction): sites and maps,
    connectors, people, invitations, settings and activity; recorded in the
    platform's activity. Traffic logs expire on their own. Panel: Delete on
    the Tenants page, confirmed by typing the id.
  - Isolation: "the site never had a public address" (`never_public`,
    migration 0018) shows Isolation as fine; an old IP that is Equalizer's
    own address (the panel's or relay's domain resolves to it) is refused,
    since the check would reach Equalizer and report the site exposed; a
    port after the IP is dropped; the error is in plain words.
  - AI settings: `POST /v1/tenants/{tenant}/llm/models` asks the provider
    which models the key may use (Anthropic, OpenAI, OpenAI-compatible
    server; package `llmmodels`, never to a private address); the panel
    shows them in a list, the typed name only when the provider cannot be
    asked.
  - Alerts: each kind is turned on or off (`kinds`: connector, legacy_site,
    isolation, certificate, certificate_expiry, recovered; migration 0020).
    The state is followed even when a kind is off.
- **Why:** first installation feedback: one person runs a small customer,
  four roles were too many, tenants could not be removed, the model had to
  be typed, alerts were all or nothing.
- **Status:** Go and vitest tests pass. Manuals not yet updated (next
  commit); lab end-to-end tests still use the old roles.
- **Next steps:** automatic certificates for protected sites, direct mode,
  manuals, lab tests, a run at phone width.

### 2026-11-20 — Fix what the first installation showed

- **Branch:** fix/first-install-feedback
- **What:** the problems found on the first VPS installation:
  - Platform status: a gateway with no site published was shown "Down,
    0 of 2 replicas ready". `health.Probe.Replicas` now tells running
    (`/healthz`) from ready (`/readyz`); `health.ReplicaCheck` shows
    "running, no site loaded yet" as OK.
  - Email: `EQ_MAIL_FROM` with a display name (`Equalizer <noreply@…>`)
    went whole into `MAIL FROM` and servers answered 501. The envelope now
    carries the bare address, the header the name; the address is checked
    at startup. Each failure has a reason (`mail.Problem`: unreachable,
    tls, auth_rejected, sender_rejected, recipient_rejected, rejected,
    not_configured), returned as `Invitation.send_problem` and shown in the
    toasts instead of "check the mail settings".
  - Invitations: refused with 409 when the email already has an account or
    a pending invitation (`Store.EmailUse`), for a new tenant, a tenant
    invitation and the first administrator's new invitation; before, the
    link failed only when opened. "Invite again" opens with the name and
    email of the last invitation (`TenantOverview.pending_admin`).
  - Panel on phones: the sidebar becomes a menu under a top bar below the
    `lg` breakpoint (`app/Shell.tsx`), tables scroll sideways, the form
    grids get fewer columns; before, pages were squeezed into a narrow
    column and the sidebar was cut in landscape.
  - Logo: the tagline was SVG `textLength`, which Safari draws with the
    spaces collapsed; now plain words spread over the name's width.
  - Duplicate buttons on empty Tenants, People and Sites pages (header and
    empty state).
  - Isolation page: the old-IP form appeared twice after saving: it and the
    aliases form had the same React key when both were empty.
  - Address on the server (template and first discovery): empty with an
    example, not a prefilled `127.0.0.1:8443` that looked like a value.
  - Connector command: the relay address is filled in
    (`EQ_RELAY_PUBLIC_URL`, set by Compose from `EQ_RELAY_DOMAIN`, returned
    as `relay_url` with the connector list).
  - Gateway: http to https of an https site is now 302/307, not 301/308,
    since browsers keep a 301 after the site is switched to http; no HSTS
    on the plain port. A legacy redirect to the same page in the other
    scheme (WordPress with an https address served in http) is stopped with
    a page and a `legacy.error` event `scheme_mismatch` that the panel
    explains, instead of a redirect loop.
  - Agent: route handlers removed before the browser closes, so no
    `CancelledError` tracebacks outside the JSON logs.
- **Why:** first installation on a real VPS (WordPress through a connector,
  phone only).
- **Status:** unit and API tests pass (Go, vitest, agent). Not yet seen on
  a phone: checked at phone width in the lab with Playwright.
- **Next steps:** simpler approval and roles, tenant deletion, AI model
  list, alert choices, automatic certificates, "never public" isolation,
  direct mode (same branch).

### 2026-11-20 — Prepare release 0.2.0

- **Branch:** chore/release-0.2.0
- **What:** release notes `docs/releases/0.2.0.md`; version 0.2.0 in
  `VERSION`, `.env.example`, `INSTALL.md` (checkout and connector image),
  the OpenAPI contracts, the tunnel example, the agent and the panel UI
  packages; README status; the lab reads its version from `VERSION`
  (`up.sh`, test 01) instead of a fixed 0.1.0; project status above.
- **Why:** there was no tag at all, so `INSTALL.md` (`git checkout v0.1.0`,
  connector `:0.1.0`) could not be followed; 0.1.0 predates the agent's
  study.
- **Status:** contracts valid. Tags are created by the owner (this session
  cannot push them): `v0.1.0` on 604100e (merge of #39, the last change the
  0.1.0 notes describe), `v0.2.0` on the merge of this PR; the publish
  workflow then builds the images `0.1.0` and `0.2.0`.
- **Next steps:** first installation on the VPS with a remote connector;
  measure again there (`lab/e2e/perf.py` adapted); external penetration
  test.

### 2026-11-20 — Measure what Equalizer adds to a visit

- **Branch:** docs/performance
- **What:** `lab/e2e/perf.py` opens the same pages of the ten lab sites
  directly and through Equalizer (HTML timing, whole page in Chrome, 10
  clients at once; each simulated visitor has its own 127.0.0.x address so
  the per-address limit does not slow the measure); results in
  `docs/performance.md`; lab README.
- **Why:** to know whether the gateway slows browsing, and by how much.
- **Status:** one page: 3.6 ms added (median, 47 pages), about 11%; whole
  page in a browser: +10 to +60 ms, part of it TLS; under load on 2 shared
  CPUs: 15–18% fewer pages per second, no errors. Measured on one machine:
  the network to the customer's server is not included.
- **Next steps:** measure again on the VPS with a remote connector.

### 2026-11-20 — Fix what a review of the latest work found

- **Branch:** fix/review-latest
- **What:**
  - gateway: an HTML or SVG asset whose type comes in capitals
    (`Text/HTML`) was served without the sandbox policy; now the parsed
    media type decides (`asset.go`, test). A form whose action carries a
    per-session value in its query (phpBB's `ucp.php?mode=login&sid=…`) is
    now recognised: `legacy_request.query` accepts `{{hidden.x}}`, kept
    server-side and sent back on submission (`formmap`, tests); before,
    the form was not bound and its submissions were refused
    (`form_token_missing`).
  - agent: the login moved to `login.py` (the crawler stays small); it
    uses this session's copy of the form (names and values); a login made
    in script that answers in JSON is judged by that answer, not by the
    cookies (a refusal that sets a cookie was taken for a success); a form
    action's per-session query values become `{{hidden.x}}`; the
    verification tries the page's form that posts to the entry (final
    slash ignored), not another one with the same fields; the correction
    keeps a widening that mends a refused form even when no page view
    changes; Magento's fragments also under `/pub/static/`.
  - lab: `down.sh` stops Magento's server too; `webapps.sh` no longer
    redefines the shell's `hash`; README lists zip and unzip.
  - docs: map format (`{{hidden.x}}` in a form's query), agent (JSON
    logins), agent README.
- **Why:** bugfix round on the latest implementations.
- **Status:** unit tests pass; lab tests 01 and 06: 26 passed, every
  draft verified with no broken page or form (phpBB's login form, refused
  before, now passes; PrestaShop's wishlist form is sent back to the area by
  the correction). Magento's verification re-run after the final-slash
  fix: 20 of 20 pages, 14 inputs tried.
- **Next steps:** prove the six newer administrations read-only where
  possible; SIEM/S3 log forwarding.
### 2026-11-20 — Remove the maps' risk field

- **Branch:** chore/remove-risk
- **What:** the `risk` label (low, medium, high) is gone from the map
  contract (`schema/map`, the panel's copy), the example, the CMS templates,
  the agent (drafts and the AI's answer), the map editor's sample and field
  order, and `docs/map-format.md`. Migration `0017_drop_map_risk.sql`
  removes it from stored documents (signed payloads untouched: the gateway
  ignores the field); the panel drops it from any document it receives (an
  old export still imports). A field or parameter named `risk` holds an
  object and stays. Tests: `store/retired_test.go`, `db_test.go`
  (migration), the agent's AI test.
- **Why:** nothing enforced it: decorative.
- **Status:** unit tests of every module pass; contracts valid.
- **Next steps:** bugfix round on the latest work (agent login, fragments,
  lab scripts).

### 2026-11-20 — Study Magento's pages and templates faithfully

- **Branch:** fix/magento-study
- **What:** gateway `server/asset.go`: an asset of type `text/html` (an
  HTML fragment a page's scripts load as a template) is served as the
  legacy site sends it, with `Content-Security-Policy: sandbox` as for SVG
  (test in `public_test.go`); before, the knockout templates went through
  page rendering, which escaped their `<%- … %>` and broke them. Agent:
  profiles name such fragments (`fragments`, Magento's `/static/…/*.html`)
  and the crawler records them as `text/html` assets; asset prefixes stop
  before a deployment folder (`/static/version1795173710/` → `/static/`);
  the verification treats a link or a 301 that only differs by a final
  slash as the same page (Magento answers `/page/` with a 301 to `/page`).
  `docs/map-format.md` says when `text/html` is allowed for assets.
- **Why:** with its first draft Magento had 7 of 39 pages identical, and
  the agent's correction sent most of the site back to the area.
- **Status:** lab test 06 on Magento: 32 of 32 pages identical with the
  first draft, no correction needed, database unchanged. Unit tests for the
  slash rule, the fragments and the prefixes.
- **Next steps:** 1. Prove the six administrations read-only where
  possible. 2. Remove the map `risk` field.

### 2026-11-20 — Prove the six other web application profiles in the lab

- **Branch:** feat/lab-six-webapps
- **What:** `lab/e2e/webapps.sh` installs OpenCart 4.0, PrestaShop 8.2,
  phpBB 3.3, MediaWiki 1.43, TYPO3 13.4 and Magento 2.4.7 from GitHub only,
  with their test accounts (`prestashop-setup.php`, `magento-setup.php`,
  `typo3-setup.php`, `typo3-content.sql`, `phpbb-install.yml`); `up.sh`
  starts MariaDB (127.0.0.1:33061) and the six sites (ports 18101–18106),
  `down.sh` stops and cleans them; `tests/lab.py` publishes them and lets
  the connector reach them; `test_06_study.py` checks each database before
  and after the study, with the right test account per site. Agent fixes the
  lab found: the login form is chosen over a sign-up form, reopened in the
  account's own session (OpenCart's token in the action), sent with its
  button and after a short pause (phpBB), with the hidden values of that
  form only (MediaWiki); a login in script that keeps the session cookie is
  recognised by its JSON redirect (OpenCart); paths a map cannot hold
  (Magento's base64 referers) stay in the site area with a reason. Profiles
  say what the lab proved; `docs/compatibility.md` regenerated; lab README
  and `docs/agent.md` updated.
- **Why:** the six profiles were written but not tested.
- **Status:** lab tests 01 and 06 pass on the ten sites (last run: 25 passed,
  the one failure was the test's own expectation for TYPO3, fixed). Databases
  unchanged everywhere. Pages identical through the draft: PrestaShop
  220/220, phpBB 197/197, MediaWiki 38/38, TYPO3 3/3, OpenCart 163/165 after
  the agent's correction. Magento: only 7/39 with its first draft, 35/39
  after the correction sent most entries back to the site area; its
  `/static` `.html` templates are refused (`asset_type_not_allowed`).
  Administrations of the six stay closed to the agent.
- **Next steps:** 1. Magento: find why its precise entries differ and allow
  its knockout templates as assets. 2. Prove the six administrations
  read-only where possible. 3. Remove the map `risk` field.

### 2026-11-20 — Verify behind the logins and correct the draft

- **Branch:** feat/agent-correct-and-login
- **What:**
  - panel-api: a discovery hands its test accounts, still sealed, to the
    verification of its draft (migration 0016), which erases them when it
    ends; `POST .../verify` takes `accounts` and `correct`; the job carries
    the accounts, the site kind, the pages studied per account; new
    internal `POST /verifications/{id}/try-map` signs a corrected version
    after the same checks as any draft; a result with a corrected
    `document` becomes a new draft ("Corrected by the agent from version
    N", by the discovery's requester or the person who asked), recorded
    with its last measure, while the original keeps its own with the
    corrections and the new version's number.
  - agent: `verify.py` logs each test account in, opens a try session on
    the gateway with its legacy cookies and checks the pages it studied;
    comparison fixes found in the lab (relative links resolved against the
    page, the site's own address equal as legacy or public host, text
    differences name a few words); `correct.py` (widen what broke, narrow
    from values seen, up to three rounds, keep only what does not break a
    page; every change with its reason); `crawler.login_only`; the report
    lists pages per account.
  - panel-ui: Verify takes test accounts and "let the agent correct",
    shows corrections, the corrected version and the accounts verified;
    `TestAccounts` moved to the maps feature.
  - Docs: tenant manual, agent.md (step 4 complete); screenshot.
- **Why:** pages behind a login were never verified, and the agent only
  measured; now a draft arrives verified behind the logins and corrected.
- **Status:** unit tests (agent correct/compare, panel-api accounts kept
  and erased, corrected draft and measures); lab 01 + 06: pages identical
  12/12, 155/155, 154/154, 4/4 with the administrator's pages; nothing to
  correct on the lab sites (corrections proven by unit tests).
- **Next steps:**
  1. Lab: Moodle, PrestaShop and the other new profiles.
  2. SIEM / S3 log forwarding from the panel.
  3. Remove the map's `risk` field.

### 2026-11-20 — One profile per web application, ten applications

- **Branch:** feat/agent-webapps (on top of feat/panel-verification)
- **What:**
  - agent: `equalizer_agent/webapps/`, one profile per application
    (WordPress, Joomla, Drupal, Magento / Adobe Commerce, PrestaShop,
    OpenCart, TYPO3, Moodle, MediaWiki, phpBB) and `generic.py` for custom
    sites; `base.py` holds the profile's shape and the rules every site
    shares (action links, one-time tokens now also `form_key` and `hash`,
    sensitive files). `rules.py`, `fingerprint.py` and the crawler only ask
    the recognised profile. An administration opens to a test account only
    when its profile is proven read-only in the lab (WordPress closed,
    Joomla and Drupal open; the others and custom sites closed until
    tested). A recognised site applies its own administration rules and
    the usual ones; an unrecognised one all of them; every profile's
    "never" rules apply to every site.
  - `docs/compatibility.md`, generated from the profiles
    (`python3 -m equalizer_agent.webapps.matrix`); an agent test fails
    when it is stale.
  - panel-api: map templates for magento, prestashop, opencart, typo3,
    mediawiki and phpbb (generic pages and folders, each application's
    names kept out of slugs); site kinds extended (contract `SiteKind`,
    migration 0015 widens the database check); templates listed in
    alphabetical order whatever the case.
  - panel-ui: the new kinds when protecting a site; SSO path presets for
    Magento, OpenCart, TYPO3 and phpBB.
  - Docs: agent.md (profiles), architecture, README, tenant manual (the
    kinds and the matrix), agent README.
- **Why:** each application's specifics in one place, and coverage of the
  ten most used self-hosted web applications (Shopify, Wix, Squarespace and
  Webflow are hosted services: nothing of theirs to protect).
- **Status:** agent, panel-api and panel-ui tests; lab 01 + 06 give the
  same studies and verifications as before on WordPress, Joomla and Drupal;
  on the custom intranet the study now leaves its administration closed
  (generic profile: 4 pages instead of 5). The six new profiles are written
  from each application's documentation and behaviour, not proven in the
  lab yet.
- **Next steps:**
  1. Lab: add Moodle and PrestaShop (then the others) and prove their
     profiles.
  2. Agent: correct its own draft; verify pages behind a login.

### 2026-11-20 — Show verifications in the panel

- **Branch:** feat/panel-verification (on top of feat/agent-checks)
- **What:**
  - panel-ui: `VerificationPanel` on every version (score: pages
    identical, inputs refusing every attack, share served only by the
    area; pages broken by the version; still open with reasons; **Verify**
    for editors, approvers and administrators; polls while it runs) and in
    the discovery report for the proposed draft; still-open reasons shared
    (`maps/components/stillOpen.ts`).
  - agent: a still-open input's detail names, in words, the other attack
    groups it lets through.
  - Docs: tenant manual "Verification" with a screenshot; agent.md step 5
    done.
- **Why:** step 5 of docs/agent.md: the person who approves sees the
  measured result.
- **Status:** typecheck, boundaries, vitest, build, API types; agent tests.
- **Next steps:**
  1. Agent: one profile per web application in `agent/equalizer_agent/webapps/`
     (WordPress, Joomla, Drupal, Magento, PrestaShop, OpenCart, TYPO3,
     Moodle, MediaWiki, phpBB, generic), map templates and site kinds for
     the new ones, a compatibility matrix generated from the profiles.
  2. Agent: correct its own draft; verify pages behind a login.

### 2026-11-20 — Verify every draft on the gateway

- **Branch:** feat/agent-checks
- **What:**
  - panel-api: `map_verifications` (migration 0014); `GET/POST
    /apps/{app}/maps/{version}/verify` (editors, approvers, administrators;
    one waiting verification per version); the internal queue hands the
    agent the version signed for tries (`signing.SignTry`, one hour), its
    legacy site and the pages the latest discovery studied; every draft a
    discovery proposes is queued for verification; a version read alone
    carries its latest verification.
  - agent: `verify.py` (step 4: each page directly and through the gateway
    in preview, compared by status or redirect target, visible text in each
    answer's charset, links and forms; step 5: every route parameter and
    form field gets the attack set in dry-run, after a valid submission),
    `attacks.py`, `gateway.py`; the discovery report lists the pages it
    studied; `EQ_GATEWAY_ADMIN_URL`.
  - gateway fix found by the first verification: an http site's form token
    was checked against the https binding cookie, so mapped form
    submissions were refused (`openToken` now uses the site's scheme;
    regression test).
  - compose: agent reaches `gateway:9090`; lab: `EQ_GATEWAY_ADMIN_URL`;
    test_06 waits for each draft's verification and checks the databases
    are still unchanged after it.
  - Docs: agent.md (what a verification measures; step 4 done, correction
    loop next).
- **Why:** steps 4 and 5 of docs/agent.md: a draft arrives measured.
- **Status:** unit tests (agent compare and attacks, panel-api queue and
  signature, gateway regression) and lab 01 + 06 pass. Lab scores: pages
  identical 12/12, 5/5, 4/4, 4/4; inputs refusing every attack 5/13,
  26/38, 1/19, 1/3: most of what passes falls out of a route into the site
  area.
- **Next steps:**
  1. Panel: score, broken pages, still open, Verify button.
  2. Agent: correct its own draft (narrow, widen, verify again).
  3. Verify pages behind a login (keep test accounts until the
     verification ends).

### 2026-11-20 — Study pages in a real browser behind the read-only guard

- **Branch:** feat/agent-browser (on top of chore/land-agent-study)
- **What:**
  - agent: `browser.py` opens each pass's pages (up to 40) in headless
    Chromium (Playwright). Every request is intercepted and answered by the
    Fetcher (connector, read-only guard, the pass's session) only for GET
    or HEAD on the site that the crawler's rules allow; the rest is refused
    and reported ("kept in"); the browser's network points at a closed
    port. Script requests become pages to study; JSON answers stay with the
    area and are listed as still open. Files a page loads count as assets
    and never as action links. `EQ_BROWSER` (auto, on, off),
    `EQ_BROWSER_PATH`.
  - image: Playwright and Chromium's headless shell in the agent image;
    compose sets `EQ_BROWSER` (`EQ_DISCOVERY_BROWSER`), `HOME` and
    `shm_size`.
  - lab: the agent runs with the browser; test_06 checks it opened pages
    on each site, databases still unchanged.
  - Docs: agent.md (3c done), agent README, .env.example.
- **Why:** step 3c of docs/agent.md: requests made by scripts are part of
  what a site does.
- **Status:** unit tests (script GET studied, POST and action link never
  sent, JSON listed as still open) and lab 01 + 06 pass: browser opened
  10, 14, 38 and 9 pages on the four sites; their classic pages make no
  extra same-site script requests, so the browser adds little there; it
  matters for script-heavy applications.
- **Next steps:**
  1. Step 4: the two checks (with `/maps/try`) and the score; panel-api
     verification queue.
  2. Step 5: panel score, still open, Verify button.

### 2026-11-20 — Study administrations read-only, proven in the lab

- **Branch:** feat/agent-admin-readonly (on top of feat/agent-accounts-forms)
- **What:**
  - agent: one read-only guard in `fetch.py` (GET and HEAD only, plus the
    test accounts' login POST); test accounts' passes now open the
    administration (Drupal from `/admin`, Joomla after its own
    administrator login), never what `rules.NEVER` lists (scheduled jobs,
    installers, updates, exports, "add new" screens) and never action
    links (verbs as whole words, `task=`, `layout=edit`, one-time tokens).
    WordPress's administration stays closed (`rules.WRITES_ON_VIEW`).
    A base route covers a path only when the values fit its types (deny
    lists included), and one route per shape (the gateway refuses two that
    differ only by parameter names).
  - lab: the agent runs in the lab (`up.sh`, `down.sh`); new
    `test_06_study.py` studies the four sites with an administrator account
    and checks that each site's content (WordPress, Joomla on PostgreSQL,
    Drupal, intranet) is the same before and after, and that Drupal's and
    Joomla's administrations became precise entries.
  - Docs: agent.md (read-only proof, WordPress finding, rollout 3b done),
    tenant manual.
- **Why:** step 3b of docs/agent.md. The lab found that WordPress writes
  to its database when its administration is only viewed (Quick Draft,
  global styles, privacy texts); the rule now keeps it closed.
- **Status:** agent unit tests and lab tests 01 + 06 pass (four studies,
  databases unchanged). Full lab run: see the PR.
- **Next steps:**
  1. 3c: browser (Playwright) behind the same guard, script requests,
     declared APIs.
  2. Step 4: the two checks and the score.

### 2026-11-20 — Study test accounts and form fields

- **Branch:** feat/agent-accounts-forms (on top of feat/maps-try)
- **What:**
  - panel-api: up to three test accounts per discovery (`accounts`, with a
    label; `account` counts as the first), sealed together in the new
    `discoveries.accounts_secret` (migration 0013), erased at the end like
    before; the job hands them to the agent (and the first as `account` for
    older agents).
  - agent: one pass per account, each with a new session; the report says
    how each logged in (`accounts`). `fields.py` types each form field from
    its HTML: choices (select, radio, checkbox), number ranges, email, date,
    phone, URL, colour, time, HTML patterns kept only when RE2 reads them the
    same (no lookaround, repeats ≤ 1000). Free text refuses HTML and
    template code; names and codes also refuse SQL delimiters. Query strings
    seen in links get `no_html` and `no_template_meta`. A new `open` list in
    the report says what is still open and why (not opened, action links,
    legacy errors, fields that must accept SQL characters).
  - panel-ui: up to three test accounts on the Discovery form; report shows
    the accounts and the still-open list.
  - Design correction (docs/agent.md, contract): accounts do not become map
    roles. Equalizer never decides access; the legacy site does (project
    decision). The accounts only let the agent reach pages behind a login.
  - Docs: tenant manual, agent README, agent.md rollout (3a done, 3b next).
- **Why:** step 3 of docs/agent.md, first half.
- **Status:** agent, panel-api and panel-ui tests pass; a generated map
  loads in the gateway (checked by hand with the gateway's own Build). The
  administration is still never opened: that needs step 3b's lab study
  showing nothing changes on the four lab sites.
- **Next steps:**
  1. 3b: browser and read-only proxy, administration read-only, declared
     APIs, discovery run in the lab against the four sites (database
     unchanged before and after).
  2. Step 4: the two checks and the score.

### 2026-11-19 — Let the agent try a draft map on the gateway

- **Branch:** feat/maps-try (on top of docs/agent-study)
- **What:**
  - gateway: `POST /maps/try` on the admin endpoint (`server/try.go`,
    `server/trysite.go`), behind the panel's service token (`EQ_MAP_TOKEN`;
    without it tries answer 404). The draft must be signed for tries
    (`mapstore/try.go`: purpose `try`, expiry signed, at most two hours;
    the signed message has its own prefix, so it never verifies as a
    published map). `dry_run` runs the visitor's code path with a transport
    that reads the rebuilt request and answers 204: nothing is forwarded;
    the verdict says the entry, whether only the area serves it, and the
    refusal (status, reason, field, never the value). `preview` serves
    GET/HEAD through the connector and returns the rebuilt answer. Tries
    use `limits.None` (no counters), keep their events out of the logs,
    and dry-runs read sessions through `session.ReadOnly`. A test account's
    legacy cookies open a try session with its role. Compiled drafts are
    cached by payload (16).
  - `site.Options.Transport` replaces the way to the legacy site.
  - panel-api: `signing.SignTry` (same message as the gateway, checked by
    a shared test vector).
  - Contract: `gateway-admin.yaml` gets 400, `cookies` and the session
    semantics. Docs: agent.md (step 2 done), gateway README.
- **Why:** step 2 of docs/agent.md: the agent's two checks need to ask the
  gateway what it would do with a draft, without touching the legacy site.
- **Status:** unit tests (refused callers and maps, attacks refused with
  no legacy hit, rebuilt preview, seeded session). Not used by the agent
  yet; panel-api does not hand out try maps yet.
- **Next steps:**
  1. Agent: browser and read-only proxy, accounts, forms, declared APIs,
     roles and ownership, tighter map.
  2. Agent: the two checks and the score; panel-api verification queue
     (hands the agent a try map with `SignTry`).
  3. Panel: score, still open, Verify button.
  4. Lab: studies of the four lab sites.

### 2026-11-19 — Design the agent's study of legacy sites

- **Branch:** docs/agent-study (on top of feat/site-certificates)
- **What:**
  - `docs/agent.md`: the agent as Equalizer's strong point. A study with a
    real browser, read-only (administration included, test accounts with
    few permissions per CMS), a precise map entry for every function
    understood, and two checks of its own map: pages compared with and
    without Equalizer, attack set tried on every parameter and field on the
    gateway only. Measured score, "still open" list, Verify button, rollout
    in six steps. AI unchanged (structure only); page contents for the AI
    planned but not decided.
  - Contracts: `panel-api.yaml` gets test `accounts` (up to three),
    `StudyReport`, `Verification`, `GET/POST /apps/{app}/maps/{version}/verify`,
    `MapVersion.verification`, internal verification queue;
    new `schema/api/gateway-admin.yaml` documents the gateway's admin
    endpoint and adds `POST /maps/try` (dry-run and preview of a draft
    signed for that purpose, service token).
  - UI types regenerated; architecture and agent README point to the design.
- **Why:** a map is only as good as it is tight and correct; writing one by
  hand does not scale. The agent must write it and prove it.
- **Status:** design and contracts only; nothing implemented yet. The new
  endpoints are not served yet.
- **Next steps:**
  1. Gateway: `POST /maps/try`; panel-api: signing for purpose try.
  2. Agent: browser and read-only proxy, accounts, forms, declared APIs,
     roles and ownership, tighter map.
  3. Agent: the two checks and the score; panel-api verification queue.
  4. Panel: score, still open, Verify button.
  5. Lab: studies of the four lab sites with expected scores.
  6. Separately: remove the map's `risk` field (a label with no effect).

### 2026-11-19 — Serve protected sites with their own certificates

- **Branch:** feat/site-certificates
- **What:**
  - Contracts: map `meta.public_scheme` (`https` default, `http`); panel API
    PUT/DELETE `/apps/{app}/certificate`, `public_scheme` on PATCH `/apps/{app}`,
    `App.tls` (`SiteTLS`), `/internal/certificates`, `certificate_expires` in
    the site status.
  - panel-api: `certs` package (PEM or PKCS#12, one matching key, RSA ≥2048
    or ECDSA P-256/P-384, chain to system roots or an uploaded private CA,
    validity, coverage of the domain and aliases); vendored SSLMate
    go-pkcs12 using stdlib pbkdf2; migration 0012; the key is sealed per
    site; every tenant admin gets an email on upload and removal; health
    check "Certificate" and reminder emails at 30/14/7/1 days; publish and
    alias re-sign carry `public_scheme`.
  - gateway: `certfeed` pulls the certificates every 10 s (keeps the last
    good set on errors); admin `/tls/certificate` gives Caddy the bundle;
    `/tls/allowed` always 404, so Caddy never asks Let's Encrypt for a
    protected site; plain listener `EQ_PUBLIC_HTTP_ADDR` (`:8081`) serves
    http sites and redirects https ones (301/308); http sites use
    `eq_session`/`eq_form` cookies without Secure and refuse OIDC.
  - deploy: Caddyfile `get_certificate http` for sites, http catch-all to
    gateway 8081; compose sets `EQ_PUBLIC_HTTP_ADDR`.
  - panel-ui: Site → Certificate page (status, upload of .pfx or PEM,
    removal, http/https switch); publish warns when an https site has no
    certificate.
  - Lab: front mimics Caddy's certificate fetch and port 80 routing; tests
    upload real lab-CA certificates, compare fingerprints, check redirects,
    refuse another site's certificate; the intranet runs in http.
  - Docs: tenant manual section and screenshot, admin manual rows,
    architecture, map format, lab README.
- **Why:** visitors must keep seeing the legacy site's real certificate;
  Equalizer never issues new or temporary ones for protected sites. Let's
  Encrypt stays only for the panel and relay domains.
- **Status:** unit tests, lab tests and full lab pass. Deploy note: Caddy must
  reach the gateway on port 8081 (set in compose).
- **Next steps:**
  1. Manual test on the server with a real certificate (.pfx from IIS and PEM).
  2. Optional: panel behind Cloudflare.
  3. SIEM/S3 log forwarding, external pentest, Moodle in the lab.

### 2026-11-19 — Harden the relay, the panel and the agent's model calls

- **Branch:** fix/relay-panel-hardening
- **What:**
  - relay `tunnel`: 20 wrong tokens a minute per address, then refused
    without a check and reported once (`failures.go`); the connector's
    address is the last `X-Forwarded-For` value; streams opened by the
    connector are closed (`AcceptBacklog` 4); no deadline left on a
    hijacked tunnel; idle and admin timeouts.
  - panel `clientIP`: `X-Forwarded-For` behind `EQ_TRUSTED_PROXIES` (new
    setting; compose and lab set it); panel-ui's Caddy trusts the edge.
  - panel login: 5 failures per address and account, 20 per address, 50
    per account (`Throttle.Exceeds`); a successful login clears only its
    own pair; server read, write and idle timeouts.
  - agent `llm._post`: no redirects followed; the connected address must be
    public (checked after connecting, against DNS rebinding).
  - lab `test_03c_relay_panel.py`; docs: architecture "Exposed services
    under attack", admin manual, lab README.
- **Why:** behind Caddy every panel visitor had the proxy's address: five
  wrong passwords from anyone locked every user out, and the audit showed
  the proxy. The relay let anyone guess tokens without limit and logged
  the address the client wrote. A tenant's model server could redirect the
  agent into the platform's networks. Two of the new lab tests fail on the
  previous code and pass now.
- **Status:** unit tests pass (Go with -race, agent); the clean lab passes
  61 of 61.
- **Next steps:**
  1. Manual test on a server by the project owner, then tag v0.1.0.
  2. External penetration test before real portals.
  3. Optional customer certificate per site (upload, encrypted, expiry
     alert), if wanted.

### 2026-11-19 — Harden the public gateway against hostile clients

- **Branch:** fix/gateway-hardening
- **What:**
  - `server/guard.go`: panics answer 500 and are logged; request bodies
    keep a minimum pace; answers the client stops reading are given up;
    `IdleTimeout` on the public server, full timeouts on the admin one.
  - `server/oidc.go`: `safeReturn` refuses control characters and `\`,
    which browsers drop or read as `/` (defence in depth: its callers
    already normalise the address).
  - `server/area.go`, `asset.go`: `..;`, `..\`, `.. ` and `...` refused,
    as Tomcat and Windows servers resolve them.
  - `rebuild`: at most 10,000 fields in a query or form and parts in a
    multipart form; in-memory bodies down to 16 MB with a shared 256 MB
    budget (`server/membudget.go`); precise forms capped too.
  - `server/sessionlimit.go`: a new anonymous session lives 5 minutes
    until its visitor comes back; at most 600 a minute per address after a
    burst of 300 (60 was too few for a NAT: the lab, all on one address,
    hit it).
  - `clientip`: every `X-Forwarded-For` line read from the right, mapped
    IPv4 normalised; rate limits count IPv6 per /64.
  - area: the legacy origin in `Link` headers becomes the public one.
  - lab `test_03b_hostile.py`: paths that climb, request smuggling, huge
    headers, other hosts, too many fields, forged addresses, session
    flood, a body sent at a drip; `down.sh` stops lab processes started by
    any path, `up.sh` refuses to start on busy ports; login helpers say
    what the site answered when a login does not go through.
  - docs: architecture, "The public gateway under attack".
- **Why:** the gateway faces the Internet. A review found a slow-body hold,
  memory exhaustion by a form of millions of fields, a session-store
  flood, path variants that climb on Tomcat and IIS, a client address
  spoofable with two forwarding lines and IPv6 rate limits per address. Four of the new lab
  tests fail on the previous gateway and pass now.
- **Status:** unit tests pass (also with -race); the clean lab passes 57
  of 57.
- **Next steps:**
  1. Manual test on a server by the project owner, then tag v0.1.0.
  2. External penetration test before real portals.
  3. Panel and relay: the same review.

### 2026-11-19 — Lab with Joomla, Drupal and a custom app; fixes found

- **Branch:** feat/lab-cms-sites
- **What:**
  - lab: Joomla 5.4 (PostgreSQL), Drupal 11.4 (SQLite) and Acme Intranet,
    an old-style PHP app (`apps/intranet`: sessions, ISO-8859-1 pages,
    attachments), next to WordPress; built from GitHub only (`cms.sh`,
    `joomla-setup.php`, `drupal-setup.php`); published from their
    templates in the onboarding test; `test_02c_sites.py` reads pages,
    searches, logs in, writes articles in Joomla's and Drupal's editors,
    attaches files; the pixel comparison covers about 40 pages of the four
    sites.
  - gateway `legacyclient`: a relative `Location` is kept as written and
    resolved against the legacy page; it was resolved against the relay,
    so redirects pointed browsers to the relay's internal address.
  - gateway `rebuild/files.go`: files in area forms and raw uploads are
    refused (415) when the server could run them: server extensions (also
    inside the name), `.htaccess` and the like, programs, PHP code anywhere,
    pages sent as files, pictures or PDFs that are not.
  - gateway `pipeline/fragment.go`: HTML fragments (AJAX) are rendered as
    fragments, table rows in their context; JSON or text sent as text/html
    is passed on as it is.
  - gateway: a query the route does not declare sends the request to the
    area instead of being dropped; entries with `authz` are never bypassed,
    by query or by path parameters.
  - templates: favicons as `image/vnd.microsoft.icon`; Drupal's
    `/sites/default/files/` served by the area (generated CSS, JS and image
    styles need their query).
  - docs: map format, lab README.
- **Why:** test more kinds of legacy sites before the manual test. The lab
  found an internal address leaked by relative redirects, a PHP script
  uploaded through an ordinary form and run by the legacy server, broken
  AJAX answers in Joomla's admin, Joomla's search served as its home page,
  and Drupal's styles missing.
- **Status:** unit tests pass; the clean lab passes 48 of 48 (tests 3–5
  in the full run before the last fidelity tweak, 1–2c after it).
- **Next steps:**
  1. Manual test on a server by the project owner, then tag v0.1.0.
  2. Moodle in the lab (its template is not tested yet).
  3. Log forwarding to S3/block storage, then SIEM.

### 2026-11-19 — Keep wp-admin faithful: OPTIONS, legacy address in forms

- **Branch:** fix/lab-fidelity
- **What:**
  - gateway: `OPTIONS` goes to the legacy application when the area lists
    it (`server/options.go`); CORS preflights stay answered by the gateway.
  - gateway `internal/origin`: single-pass scanner that replaces whole host
    names only and is exact across stream writes; `NewReverse` turns the
    public origin in area queries and bodies back into the legacy one.
  - lab: `fidelity.py` and `test_02b_fidelity.py` compare 12 public and
    admin pages directly and through Equalizer (pixels, text, failed
    resources); new tests save Settings → General and publish from the
    block editor; comments are unique per run.
  - docs: map format and lab README.
- **Why:** the comparison found that the block editor's `OPTIONS` calls
  were answered by the gateway with an empty 204, and that forms showed
  the public https address, so saving WordPress's settings would have
  changed the site's own address.
- **Status:** unit tests pass; the clean lab passes 35 of 35 (worst pixel
  difference 0.06%, the clock on Settings → General).
- **Next steps:**
  1. Manual test on a server by the project owner, then tag v0.1.0.
  2. Log forwarding to S3/block storage, then SIEM.

### 2026-11-19 — Add local end-to-end lab and fix what it found

- **Branch:** feat/local-lab
- **What:**
  - `lab/e2e/`: a whole platform on one machine without Docker (real
    services, real WordPress on SQLite, HTTPS front with a lab CA, OIDC
    provider, SMTP sink), `up.sh`/`down.sh`, and 32 pytest + Playwright
    tests from onboarding to SSO, attacks, password reset and alerts;
    README; `go vet` in CI.
  - gateway `internal/origin`: the legacy origin (both schemes, also
    JSON-escaped) becomes `https://` + public domain in inline scripts,
    JSON, JavaScript, CSS and feeds (areas, routes, APIs, assets), streamed
    where the body is streamed.
  - gateway: path parameters are match constraints (`pathfit.go`); URLs are
    rewritten to a route only when captured values fit its types; a
    trailing `/` is kept.
  - gateway: raw uploads reach the legacy site with
    `Content-Disposition: attachment` and a cleaned file name.
  - templates: slugs refuse dots (no `/wp-config.php` matched as a page);
    the WordPress comment form has its optional fields and hidden inputs.
  - docs: map format (origin rewriting, path parameters, raw uploads).
- **Why:** test real customer flows before the manual test on a server.
  The lab found mixed content in wp-admin (WordPress on http behind
  https), files matched as page slugs, broken comment posting and raw
  uploads without a file name.
- **Status:** all 32 lab tests pass; unit tests pass. Not covered by the
  lab: real TLS/ACME, DNS, Compose images, real SMTP, Cloudflare, Loki,
  backups, replicas.
- **Next steps:**
  1. Manual test on a server by the project owner, then tag v0.1.0.
  2. Log forwarding to S3/block storage, then SIEM.
  3. Capacity settings from the panel.

### 2026-11-19 — Account card at the bottom of the sidebar

- **Branch:** fix/sidebar-account
- **What:** panel-ui `app/SidebarFooter.tsx`: initials, name and role
  (opens the account) with Help and Log out as small icon buttons on the
  same line, instead of the bare email next to the icons. Guides and
  screenshots updated.
- **Why:** owner's request: the icons did not sit well with the email.
- **Status:** checked in the browser for customers and platform users.
- **Next steps:** none.

### 2026-11-19 — Help and logout as icons in the sidebar

- **Branch:** feat/sidebar-icons
- **What:** panel-ui `app/SidebarFooter.tsx`: the account on the left,
  Help (question mark) and Log out (muted red) as SVG icon buttons on the
  right, with labels for screen readers and tooltips. Guides and
  screenshots updated.
- **Why:** owner's request.
- **Status:** checked in the browser.
- **Next steps:** none.

### 2026-11-19 — Other domains per site, connector releases in the panel

- **Branch:** feat/aliases-connector-version
- **What:**
  - contracts: map `meta.aliases`; API `PUT /apps/{app}/aliases`, `App.aliases`;
    `Connector.online`, `versions`, `outdated` and `platform_version`.
  - gateway: aliases registered with the sites (a domain belongs to one
    site), certificates allowed for them (`/tls/allowed`), requests
    redirected to the public domain (301, 308 for writes; the path kept,
    never another host).
  - panel-api: aliases stored (migration 0011, unique across public domains
    and aliases), written in meta at publish, and the published version
    signed again when they change (same version); the gateways' ETag now
    covers the signature. Connectors list and Status read the release of
    each tunnel from the relays; older than the platform: outdated (Status:
    degraded).
  - panel-ui: Isolation → Other domains; Connectors: state, release,
    update notice; the install command uses the platform's release instead
    of `latest`.
  - docs: map format, guides, release notes.
- **Why:** almost every site has two names (www and bare); a connector left
  on an old release silently loses features.
- **Status:** tests for alias registration and conflicts, redirects,
  republishing, release comparison.
- **Next steps:** owner's lab tests, then the `v0.1.0` tag.

### 2026-11-19 — Forgotten password, alerts, backups, raw uploads

- **Branch:** feat/mvp-ops
- **What:**
  - panel-api: forgotten password (`/auth/password-reset`, contract first):
    same answer whether the account exists, email sent in the background,
    one-hour single-use link stored as a hash, every session ends, the
    authenticator stays; limited per IP and per address. Migration 0009.
  - panel-api: email alerts (`internal/alerts`): every 2 minutes the status
    of each site is evaluated (the Status page's rules); a connector
    offline, a failing legacy site or an exposed site that lasts 3 minutes
    sends one email to the tenant's administrators and extra addresses,
    and one when it works again; states are stored so a restart sends
    nothing twice. Settings `GET/PUT /tenants/{t}/alerts`. Migration 0010.
  - panel-ui: "Forgot your password?", `/forgot`, `/reset/:token`;
    Settings → Alerts.
  - gateway: files sent as the whole body (`image/*`, video, audio, PDF,
    office, `application/octet-stream`) in areas: first bytes must match the
    declared type, pages, scripts and programs refused (`upload/raw.go`),
    streamed through a temporary file. Logs word the new reasons.
  - deploy: `backup.sh` and `restore.sh` (database, secrets, `.env`;
    optional encryption, retention); INSTALL section 10.
  - docs: guides (Help), INSTALL, map format, release notes.
- **Why:** an MVP given to customers needs these (owner's list: 2, 3, 4, 5).
- **Status:** tests for reset, alerts (grace, once only, recovery, off),
  settings, raw uploads; backup/restore exercised with a fake docker.
- **Next steps:** owner to decide on domain aliases (www and bare domain)
  and the connector version in the panel.

### 2026-11-19 — Ask for the identity provider's login on chosen paths

- **Branch:** feat/oidc-paths (on top of feat/help-guides, #36)
- **What:**
  - schema: `roles.auth.paths` (both copies): only these prefixes need the
    OIDC login, the rest keeps the anonymous role.
  - gateway: `loginNeeded`/`loginPath` (`server/oidcpaths.go`): paths are
    matched on a cleaned, lower-case path (`\`, `//`, dot segments,
    `;params`, trailing dots and spaces), so no spelling slips past;
    `Authorization` (legacy HTTP login, application passwords) reaches the
    legacy site only after the provider.
  - panel-ui: Login tab: "The whole site / Only some paths / Optional", path
    list, ready paths for WordPress (with XML-RPC), Joomla, Drupal, Moodle.
  - docs: map format, customer guide (synced Help), release notes.
- **Why:** owner's request: protect admin and login pages with the
  organisation's SSO without a login on the public site.
- **Status:** tests cover the bypass spellings, 401 on writes, the
  Authorization rule and the panel form.
- **Next steps:** single login into the legacy site after the provider
  (later, if customers ask).

### 2026-11-19 — Add the in-panel Help, scroll long map documents

- **Branch:** feat/help-guides
- **What:**
  - panel-ui: **Help** above the account in the sidebar
    (`features/help`): topics on the left (macro topics and topics), search
    across the guide, previous/next. Customers get the customer guide,
    platform users the administrator guide.
  - docs: `docs/manual/tenant.md` and `admin.md` rewritten as guides
    (`##` macro topics, `###` topics); they are the only source: `npm run
    sync:help` copies them and their screenshots into the panel, CI fails
    when the copy is stale (`check:help`).
  - panel-ui: the map document of a version and the editor scroll inside a
    box with an always-visible scrollbar; the sidebar stays in place.
  - INSTALL: admin pages are served (not 404), maps start from templates.
  - Screenshots taken again (scrollbars visible).
- **Why:** owner's requests: a guide inside the product for customers and
  administrators; long maps made the page and the screenshots endless.
- **Status:** tests pass (parser and search covered); help checked in the
  browser for both kinds of user.
- **Next steps:** login with the identity provider on chosen paths only
  (owner's request, to be agreed).

### 2026-11-19 — Keep the tagline within the width of the name

- **Branch:** fix/logo-tagline
- **What:** panel-ui `Logo`: the tagline is drawn in an SVG as wide as
  "Equalizer" (box with no width of its own, letters spaced with
  `textLength`), in the sidebar and on the login steps. README banner
  (`docs/brand/logo-banner.*`) aligned the same way. Screenshots taken
  again.
- **Why:** owner's request: "PROTECT LEGACY WEB APPS" must never be longer
  than the name.
- **Status:** measured in the browser: same left and right edges.
- **Next steps:** none for the logo.

### 2026-11-19 — Add the logo, fix legacy compatibility bugs, align docs

- **Branch:** feat/logo-and-fixes
- **What:**
  - panel-ui: Equalizer logo (`shared/ui/Logo.tsx`, `BrandBanner.tsx`) in
    the sidebar and on the login steps, tagline "PROTECT LEGACY WEB APPS";
    new favicon; the old `Wordmark` is gone. Logo files in `docs/brand/`
    (SVG, PNG), banner in the README. Screenshots taken again.
  - gateway: the legacy site's HTTP login (`Authorization`) was lost in
    connector mode: the relay token took its header. It now travels as
    `X-Eq-Authorization` and the connector puts it back (contract
    `schema/tunnel/tunnel.md` updated first).
  - gateway: the browser's User-Agent becomes a fixed string for its kind
    of device (`internal/useragent`): mobile versions work, the client's
    own string never reaches the legacy app.
  - gateway: `Accept-Encoding: identity` towards the legacy app; gzip and
    deflate answers are decompressed (`legacyclient/encoding.go`), so pages
    of apps that always compress render instead of breaking.
  - docs: architecture (pipeline as built, CSP by hash and visual
    comparison marked planned, legacy budget per replica), map format,
    tenant and admin manuals, INSTALL (`EQ_BEHIND_CLOUDFLARE`, update by
    release tag, connector version), release notes.
- **Why:** owner's request (logo, a status point) and a bugfix round
  before the lab tests.
- **Status:** tests pass in every service. Older connectors keep working
  but drop the legacy HTTP login: update them with the VPS.
- **Next steps:** lab tests by the owner, then tag `v0.1.0`; WebSocket;
  SIEM forwarding; capacity from the panel.

### 2026-11-19 — Fix review findings in the compatibility work

- **Branch:** fix/compat-review
- **What:**
  - gateway: live updates were never flushed in production: the status
    recorder hid the server's flusher (`statusRecorder.Unwrap`,
    `commitWriter.Flush` through `http.ResponseController`); end-to-end test.
  - gateway: every cookie of a `session.start` answer is private, so
    `success.cookie` works whatever the cookie's name.
  - gateway: "remember me" ends when the legacy login cookie is deleted,
    not when a stale cookie is; deletions alone create no session; public
    cookies still reach the browser when the session store is down; answers
    setting cookies get `Cache-Control: private, no-store`.
  - gateway: more cookie names kept private (`uid`, `pass`, `pwd`,
    `secret`, `hash`, `member`, `account`).
  - gateway: request headers trusted as set by a proxy are refused (method
    override, SSO identity like `X-Remote-User`, `X-Middleware-Subrequest`,
    `X-True-Client-IP`…); response headers for front proxies or debugging
    are dropped (`X-Accel-Redirect`, `X-Sendfile`, `X-Debug*`, `X-Varnish`…).
- **Why:** bugfix session on the compatibility work before the first
  customer tests.
- **Status:** tests pass; not yet tried on real sites.
- **Next steps:** tag `v0.1.0` (by the owner: this session cannot push
  tags); lab tests on real CMS.

### 2026-11-19 — Widen compatibility with real legacy sites

- **Branch:** feat/compat
- **What:**
  - gateway: real client IP to the legacy app (`X-Forwarded-For`,
    `X-Real-IP`; map `legacy.client_ip`, default `forwarded`);
    `CF-Connecting-IP` honoured only with `EQ_BEHIND_CLOUDFLARE=true`
    (it was spoofable through Caddy).
  - gateway: legacy cookies split into private (HttpOnly or session-like
    names, kept in the session) and public (sent to the browser, read back
    filtered: `server/cookies.go`); cookies written by page scripts reach
    the legacy app.
  - gateway: "remember me" (`defaults.session.remember_days`, default 30):
    a persistent private legacy cookie makes the session persistent.
  - gateway: application `X-*` headers both ways, `Authorization` (Basic,
    Bearer, Digest, Token), `WWW-Authenticate`, `Link`, `Retry-After`;
    revealing headers dropped.
  - gateway: `text/event-stream` flushed event by event.
  - gateway: multipart bodies of areas rebuilt in streaming into a
    temporary file (`server/areabody.go`, `rebuild.Multipart`); other
    bodies capped at 64 MB in memory.
  - gateway: single-byte charsets (windows-125x, iso-8859-*, koi8, ibm866)
    generated by `scripts/gen_charset_tables.py`.
  - tunnel-relay: handover keeps large bodies in a temporary file (up to
    1 GB) instead of failing above 50 MB.
  - panel-api/panel-ui: `max_upload_mb` on `POST /apps/{app}/map-template`
    and a **Max upload (MB)** field; templates loosened (unicode slugs,
    longer searches, strings and comments).
  - deploy: `EQ_BEHIND_CLOUDFLARE`. Docs: map format, tenant manual,
    0.1.0 release notes.
- **Why:** first customer tests: the prototype must work with as many real
  sites as possible without manual tuning.
- **Status:** unit and integration tests pass (gateway, relay, panel-api,
  panel-ui). Not yet tried on real sites in the lab. WebSocket still 501;
  East Asian charsets (Shift_JIS, GBK, Big5, EUC-KR) still unsupported.
- **Next steps:** merge after #31; tag `v0.1.0` with this included; lab
  tests on WordPress/Moodle with uploads, remember me and SSE; WebSocket.

### 2026-11-19 — Stabilization and release 0.1.0

- **Branch:** `fix/stabilize-0.1` (on top of `feat/areas`)
- **What:**
  - Areas: the legacy charset is learnt from the site's own pages, so forms
    posting into an area reach an ISO-8859-1 site with the right accents
    (an area's `charset` still wins); WebSocket upgrades get a clear 501
    (`websocket_not_supported` in the Logs) instead of a broken request;
    files are revalidated (`If-None-Match`, `If-Modified-Since`, 304) while
    rendered pages carry no validators (they hold per-visitor form tokens).
  - Connector: a request was cut after 30 seconds in total, body included,
    so long downloads through an area broke; now 30 seconds for the answer
    to start and 10 minutes in all. Relay handover between replicas had the
    same 50-second cut: now only the wait for headers is bounded.
  - Release 0.1.0: `VERSION`, release notes (`docs/releases/0.1.0.md`:
    contents, security properties, known limits), images published with the
    version on every `vX.Y.Z` tag, INSTALL and `.env.example` install a
    release (`EQ_VERSION`).
- **Why:** close a first stable prototype with what is developed.
- **Status:** all tests pass. The tag `v0.1.0` goes on `main` after #28,
  #29, #30 and this pull request are merged.
- **Next steps:**
  1. Merge, tag `v0.1.0`, GitHub release with the notes.
  2. Lab campaign (WordPress, Moodle, Juice Shop fully through the gateway).
  3. SIEM forwarding; one legacy account per user behind OIDC.

### 2026-11-19 — The whole site is served: areas

- **Branch:** `feat/areas` (on top of `fix/bugfix-session-2`)
- **Decision (project owner):** Equalizer is in between and must not
  interfere: the whole site is served, admin included, and access control
  stays with the legacy application. Leftover files on the legacy server are
  not Equalizer's concern. Recorded in CLAUDE.md.
- **What:**
  - Map contract: `areas` (prefix, methods, optional roles, body limit,
    legacy charset). Precise entries take precedence.
  - gateway: requests no entry matches go to the area with the longest
    prefix; path checked, query and body parsed and written again
    (`internal/rebuild`: urlencoded and multipart forms in order, files with
    cleaned names, JSON token by token, XML from its tokens with DOCTYPE
    refused, UTF-8 text; anything else 415); a few request headers rebuilt,
    Referer/Origin translated; legacy status and content passed on, pages
    rendered, downloads streamed (`legacyclient.Open`), redirects to other
    sites passed on; legacy cookies stay on the gateway; rate limits and
    form tokens only on precise entries. Forms posting into an area keep
    their hidden values (nonces); the link rewriter keeps area paths.
  - Templates: every CMS template has the `/` area with all methods.
  - Agent: always proposes the `/` area (`site_area` in the report); admin
    pages and destructive links are still not opened, but served.
  - Review aid: `area_added` (low), `area_removed` (high).
  - Docs: map format (areas, principles), tenant manual, gateway README.
- **Why:** sites must work entirely through Equalizer with little setup,
  with or without discovery.
- **Status:** tests pass (end-to-end area test with a WordPress-like admin:
  ajax, REST JSON, downloads, legacy 403, redirects, nonces kept; XXE,
  binary, NUL, malformed query, dot segments refused before the legacy
  app; rebuild unit tests).
- **Next steps:** lab campaign with WordPress and Moodle fully through the
  gateway.

### 2026-11-19 — Bugfix session 2

- **Branch:** `fix/bugfix-session-2` (on top of `fix/bugfix-session-1`)
- **What (each with a test):**
  - **Uploads, decompression bomb:** with `scan: cdr` a PNG of a few KB
    declaring 60000 × 60000 pixels made the gateway allocate gigabytes.
    Images are measured before decoding and refused past 40 megapixels.
  - **Uploads, crash:** a file under 30 bytes starting like a zip made the
    Office sniffer panic (slice bounds).
  - **Discovery, test account:** the login was "ok" as soon as any cookie
    existed, and most PHP sites set one on the first visit; now a login
    needs a new or changed cookie and no login form in the answer (a
    redirect back to the login page is a failure; Moodle's test session is
    not).
  - **Discovery, logged-in pass:** it shared the page budget and the
    per-shape limit with the anonymous pass, so it usually explored nothing;
    each pass has its own, and pages only members see come first.
  - **Discovery drafts** are checked by a gateway too, like drafts written in
    the panel.
- **Why:** second review of the code of the last days.
- **Status:** all tests pass.
- **Next steps:** lab campaign.

### 2026-11-19 — Bugfix session 1

- **Branch:** `fix/bugfix-session-1`
- **What (each with a test):**
  - **Maps the gateway would refuse could be published.** The panel checked
    only the JSON schema; the gateway then kept the previous version (the
    panel said v2, v1 was served) or, for a first version, did not serve the
    site. Now the gateway answers `POST /maps/check` on its admin endpoint
    with its own checks, and panel-api asks it on every draft
    (`map_inconsistent`, 422) and before publishing (no gateway answering:
    `map_unchecked`, 503, never published unchecked) and on rollback.
  - **Entries open to an undeclared role** were accepted and silently closed
    to everyone: the gateway now refuses them (routes, forms, APIs, assets).
    The panel's CMS templates are checked to load (test).
  - **OIDC, several tabs:** a second login overwrote the first; the first
    tab came back with 400. Up to 4 logins in progress per session, the one
    finished is removed, the others stay valid.
  - **OIDC, scripts:** a `fetch()` without `Accept: application/json` got a
    redirect to the provider; now any non-navigation (Sec-Fetch-Mode) gets
    401.
  - **Relay handover** copied hop-by-hop headers (Connection,
    Transfer-Encoding…) between replicas: now dropped both ways.
  - **Logs page:** with Live on, the detail jumped to another event every
    refresh (it followed the row, not the event); "Load older" while Live
    left gaps in the list (Live now pauses, and resuming starts from the
    newest page); the cursor's own event could appear twice across pages.
- **Why:** bugs found by reading the code written in the last days.
- **Status:** all tests pass.
- **Next steps:** bugfix session 2 (agent, discovery jobs, uploads,
  sessions), then the lab campaign.

### 2026-11-19 — Login with the identity provider (OIDC), off by default

- **Branch:** `feat/oidc` (on top of `docs/align-screenshots`)
- **What:**
  - Map contract: `roles.auth` with `provider: oidc`, issuer (https),
    client, scopes, groups claim, group → role rules (first match, `*` for
    everyone), `required`. Without it nothing changes.
  - gateway `internal/oidc`: discovery, keys (RSA ≥ 2048, EC P-256/P-384;
    never `none` or HMAC), ID token checks (signature, issuer, audience and
    azp, expiry, iat, nonce), code flow with PKCE, end-session. Standard
    library only. `server/oidc.go`: `/.eq/oidc/login|callback|logout`, the
    gate before routing (pages redirect, APIs and writes 401), login state in
    the session for 10 minutes, new session id after login, role from the
    groups only, local return paths only. Refusals are `request.blocked`
    events (`oidc_*`, `login_required`).
  - Client secrets: sealed per site in panel-api (`app_oidc`, migration
    0008), `PUT/DELETE /apps/{app}/oidc/secret` (administrators, recorded
    without the secret), `GET /apps/{app}/oidc` (addresses to register),
    `/v1/internal/oidc` for the gateways (`EQ_OIDC_SECRETS_URL`).
  - Review aid: `oidc_on`, `oidc_off` (high), `oidc_issuer_changed` (high),
    `oidc_optional`, `oidc_everyone`, `oidc_group`.
  - panel-ui `features/sitelogin`: Site → Login tab; settings saved as a new
    draft (usual review and approval), secret saved apart.
  - Docs: map format, tenant manual (with screenshot), gateway README.
- **Why:** many legacy sites have a weak login; the organisation's identity
  provider (with its MFA) goes in front of them without touching them.
- **Status:** tests pass (end-to-end with a fake provider: login, roles,
  logout, forged state, tampered signature, unknown group, 401 for APIs;
  token checks; secrets endpoints; review rules; schema). Panel checked in
  a browser. Not yet tried with a real provider.
- **Next steps:**
  1. Try with Entra ID or Keycloak in the lab.
  2. One legacy account per user behind the OIDC login.
  3. Single sign-on to the panel; SIEM forwarding.

### 2026-11-18 — Documentation aligned, screenshots

- **Branch:** `docs/align-screenshots` (on top of `feat/status-page`)
- **What:**
  - Manuals rewritten on what the panel does today (tenant and platform),
    with a roles table and a "Not available yet" section; planned features
    no longer described as present.
  - Screenshots in `docs/screenshots/` (fictitious company Acme, domains
    under `example`), used by the manuals and the README.
  - Architecture: panel-api and agent responsibilities, relay replicas,
    observability, status and logs; INSTALL: status line and checks;
    gateway README: uploads no longer listed as to do.
- **Why:** several documents still described the prototype of a few weeks
  ago; earlier screenshots used a real organisation's name.
- **Status:** documentation only.
- **Next steps:** SIEM forwarding; lab campaign.

### 2026-11-18 — Status page

- **Branch:** `feat/status-page` (on top of `feat/map-import-export`)
- **What:**
  - Contract: `TenantStatus`, `SiteStatus`, `PlatformStatus`, `HealthState`
    for `/tenants/{tenant}/status` and `/status/platform` (were open objects).
  - panel-api `internal/health`: each site's map (published version),
    connector (tunnels on every relay replica through DNS), legacy
    application (served and errors in the last 15 minutes from Loki),
    isolation (exposure check), overall state = the worst; problems per day
    over 7 days (legacy errors, connector drops). Platform: database,
    gateway and relay replicas ready, Loki. Tenant people see their sites;
    platform users only the platform page. `EQ_RELAY_ADMIN`,
    `EQ_GATEWAY_ADMIN` (set in Compose).
  - panel-ui `features/status`: Status (tenant) and Platform status pages,
    refreshed every 30 seconds; states as a word and a dot.
- **Why:** to tell at once whether a site is down because of Equalizer, the
  connector or the legacy server.
- **Status:** tests pass (evaluation, probes over replicas, counts from
  Loki, permissions). Not yet run against the Compose stack.
- **Next steps:** docs and screenshots; SIEM forwarding.

### 2026-11-18 — Map import and export

- **Branch:** `feat/map-import-export` (on top of `feat/discovery-issues`)
- **What:** panel-ui: **Download as YAML** on every version (without the
  meta the panel owns); **Import a map file** on New version loads a YAML or
  JSON file (at most 2 MB, must have `legacy` and `roles`) into the editor.
  No new endpoint: the import is saved as a draft through the usual
  creation, so schema, reach and connector checks and approval apply.
- **Why:** backups, moving a map from a test site to the real one, reusing
  a map on a similar site, keeping maps in git.
- **Status:** tests pass (YAML and JSON read, meta dropped, non-maps and
  large files refused).
- **Next steps:** status page; docs and screenshots.

### 2026-11-18 — Discovery proposes IDOR protection for JSON APIs

- **Branch:** `feat/discovery-issues` (on top of `feat/logs-page`)
- **What:** agent `idor.py`: when a GET list API and a GET detail API one
  segment below it share an id of the same type, the draft gets
  `response.issues` on the list and `owned_by_user` on the detail for the
  roles without a rule; a rule a person chose is never replaced. The report
  lists them ("Protection against IDOR proposed"); the review aid flags
  `issues_added` as usual.
- **Why:** `issues` had to be written by hand.
- **Status:** tests pass (pair found, types must match, human rule kept,
  idempotent). The crawler does not run JavaScript, so APIs are taken from
  the map (template, manual edits, published map), not found by crawling.
- **Next steps:** map import/export; status page; docs and screenshots.

### 2026-11-18 — Logs page

- **Branch:** `feat/logs-page` (on top of `feat/authz-json`)
- **What:**
  - Contract: `GET /tenants/{tenant}/logs`, `/logs/stats`, `/logs/export`
    (`LogEvent`, `LogStats`).
  - panel-api: `internal/logsearch` reads security events from Loki; the
    tenant always comes from the user, filters are validated (site, types,
    period) and quoted (search) before they enter LogQL; only the shown
    fields leave, never field values. Stats: events per type and refused
    per interval. CSV export up to 10000 events, formulas neutralised,
    recorded in the activity. Admins, approvers and auditors; platform users
    see no tenant's events. `EQ_LOKI_URL` (set in Compose).
  - panel-ui: `features/logs` as in the approved mockup: filters, chips with
    counts, refused chart, event table, detail in plain words with "Add to
    the map" (unmapped paths), "Show this IP only", "Copy request ID", live
    refresh, CSV. Opens on refused events (served is off by default).
- **Why:** the project owner asked for a Logs page in the panel.
- **Status:** tests pass (LogQL quoting against injection, tenant filter,
  paging, stats; handler permissions and CSV); checked with screenshots on a
  fake Loki. Not yet on the real Loki of the Compose stack.
- **Next steps:**
  1. Run it against the Compose stack's Loki.
  2. SIEM forwarding configured from the panel.
  3. Lab campaign (DVWA, WordPress, Juice Shop).

### 2026-11-18 — owned_by_user for JSON APIs

- **Branch:** `feat/authz-json`
- **What:**
  - Map: `response.issues` on API routes (response field → `entry.ns.name`).
    Every value of that field in the answer is issued to the session for the
    entry's `owned_by_user` target, as links in pages already are.
  - Gateway: loading checks the field is declared and the entry has that
    rule; `jsonshape.Values` reads the values from the rebuilt answer,
    issued before the session is written.
  - Review aid: `issues_added` (medium) when an answer grants more objects.
  - Docs: map format, gateway README.
- **Why:** single-page apps load their lists as JSON; without this,
  `owned_by_user` on their APIs refused the user's own objects too, so
  those sites stayed without IDOR protection.
- **Status:** tests pass (list then detail: own ids 200, others 403; bad
  `issues` refused at load; review finding). Not yet on a real SPA.
- **Next steps:**
  1. Logs page in the panel (mockup shown to the project owner).
  2. Lab campaign (DVWA, WordPress, Juice Shop).
  3. Discovery: propose `issues` when a list API and a detail API share an id.

### 2026-11-18 — Uploads without ClamAV; uploads and relay replicas reach main

- **Branch:** `feat/uploads-no-av`
- **What:**
  - Brings the work of #19 (uploads, relay replicas) to `main`: that pull
    request was merged into `feat/authz-rollback` instead of `main`.
  - Removes ClamAV: no `clamav` service, no `EQ_CLAMD_ADDR`, no scanner in
    the gateway. `scan` in the map is now `cdr` (rebuild images) or `none`;
    file fields need `mime` and `max_size_kb`. Forms with files no longer
    answer 503 when nothing else is installed.
- **Why:** the project owner prefers to leave virus scanning to the security
  stack around the legacy application, and ClamAV needed about 3 GB of
  memory.
- **Status:** gateway tests pass (type, size, image rebuild, undeclared
  file, multipart towards the legacy app). Replicas not yet run in Docker.
- **Next steps:**
  1. Lab campaign (DVWA, WordPress, Juice Shop) with discovery, review aid,
     authorisation and uploads.
  2. Advisories feed; issue values from API responses for `owned_by_user`.
  3. SIEM forwarding, status page, import/export.

### 2026-11-18 — File uploads, relay replicas

- **Branch:** `feat/uploads-relay` (on top of `feat/authz-rollback`)
- **What:**
  - gateway: `internal/upload` (real type sniffed from the bytes, OOXML and
    ODF recognised, size bound, ClamAV INSTREAM client failing closed,
    images rebuilt from their pixels with `av+cdr`, random names); forms
    with files read as multipart (declared file fields only, one file each)
    and rebuilt as multipart towards the legacy application; map loading
    checks `mime`, `max_size_kb` and what `av+cdr` can rebuild; without
    `EQ_CLAMD_ADDR` such forms answer 503.
  - tunnel-relay: replicas; a request for a connector whose tunnel is on
    another replica is handed over to the peers found through DNS
    (`EQ_PEERS`), never more than once; the peer checks everything again.
  - Compose: relay replicas (`EQ_RELAY_REPLICAS`, default 2), Caddy spreads
    tunnels over them; optional `clamav` service (profile `uploads`).
  - Docs: map format (uploads), relay README (replicas), INSTALL.
- **Why:** the last two limits of the prototype: legacy forms with
  attachments, and a relay that was a single point of failure.
- **Status:** tests pass (type, size, infected, scanner down, polyglot
  image cleaned, undeclared file; handover between replicas, tenant check
  after handover). Not run with a real ClamAV or with replicas in Docker.
- **Next steps:**
  1. Lab campaign (DVWA, WordPress, Juice Shop) with discovery, review aid,
     authorisation and uploads.
  2. Advisories feed; issue values from API responses for `owned_by_user`.
  3. SIEM forwarding, status page, import/export.

### 2026-11-18 — Authorisation rules against IDOR, map rollback

- **Branch:** `feat/authz-rollback` (on top of `feat/review-assistant`)
- **What:**
  - Contracts: `authz` on API routes, the meaning of `owned_by_user`;
    `rollback` map action.
  - gateway: rules per role on routes and APIs. `owned_by_user` lets a
    session open only the values the legacy site showed it (rewritten links
    and redirects issue them; stored in the session, bounded, cleared when
    the legacy session ends); `never` refuses; checked after validation,
    before the legacy request; refusals are `authz_not_owned` /
    `authz_never` events. Map loading checks roles and targets.
  - panel-api: `rollback` publishes again a retired signed version (approvers
    and administrators), audited.
  - panel-ui: "Roll back to this version" on retired versions.
  - Docs: map format (authorisation, rollback), tenant manual, gateway README.
- **Why:** an allowlist does not stop a user changing an id in the address
  (IDOR); and a broken version must be undone in seconds.
- **Status:** tests pass (issue and check, redirects, another session,
  invalid targets; rollback flow). Values shown only inside JSON API
  responses are not issued yet (only links and redirects).
- **Next steps:** issue values from API responses; uploads; relay replicas.

### 2026-11-18 — Review aid for approvers (human in the loop)

- **Branch:** `feat/review-assistant`
- **What:**
  - Contract: `GET/POST /apps/{app}/maps/{version}/review`, agent
    `/internal/reviews/claim` and `/{id}/result`, `MapReview`.
  - panel-api: `internal/review` compares a version with the published map
    and flags risks with fixed rules (type widened, pattern, rules or deny
    removed, authz removed, opened to anonymous, sensitive or personal
    paths, direct mode, reach or allow changed, rate limits raised, assets
    served as is, new APIs, removals); migration `0007` for the AI part;
    an AI review is queued when a version is sent for review.
  - agent: review jobs, a plain summary and questions from the tenant's
    model, given the changes and findings only.
  - panel-ui: "Review aid" on each version (risks, AI summary and
    questions, changes, "Ask the AI again").
- **Why:** the approver is the last barrier: they must see at once what a
  change opens and what to check, whoever (person or agent) wrote it.
- **Status:** tests pass (rules, API flow, agent); checked in a browser.
  Not run with a real AI provider.
- **Next steps:** authorisation rules (`owned_by_user`) and rollback;
  uploads; relay replicas.

### 2026-11-18 — Discovery agent, continuous discovery, tenant AI settings

- **Branch:** `feat/discovery-agent`
- **What:**
  - Contracts: `/tenants/{tenant}/llm` (tenant only: the platform has no AI
    settings), `/apps/{app}/discoveries`, agent endpoints
    `/internal/discoveries/claim|{id}/progress|{id}/result`, gateway
    `/internal/unmapped`, site `discovery_schedule`; old planned
    proposals/crawls endpoints removed.
  - agent (new, Python standard library): crawler (read-only, rate limited,
    per-shape cap, second pass with a test account), fixed exclusions
    (admin, tooling, backups, actions), fingerprint (CMS, version,
    components, advisories file), clustering into typed routes, forms and
    assets, map built on the published map or the CMS template, report,
    AI review that can only rename, exclude or narrow (Anthropic, OpenAI,
    OpenAI-compatible servers; endpoint must be public). Tests with a fake
    WordPress site; Dockerfile; CI workflow; image published.
  - panel-api: migration `0006`; discoveries with secrets sealed and the
    test password erased at the end; proposals saved as drafts by the
    requester (no draft when nothing changed); scheduler for continuous
    discovery; unmapped paths from the gateway.
  - gateway: `internal/unmapped`, refused paths counted per site and sent
    every minute (path only, clients counted not sent).
  - panel-ui: Site → Discovery (start, AI on by default with a notice when
    none is configured, test account, continuous discovery, history,
    report, link to the draft); tenant Settings with the AI model.
  - Compose: `agent` service. Docs: agent README, READMEs, architecture,
    tenant manual.
- **Why:** knowing what to cover is the core of the product; discovery
  gives a working first map from day one and keeps it current.
- **Status:** tests pass in every module; checked end to end locally
  (panel-api + agent + fake site + UI): discovery done, draft proposed,
  report shown. Not run against a real CMS or a real AI provider; the
  advisories file is empty until a feed is connected.
- **Next steps:**
  1. Lab: discovery on DVWA and a real WordPress through a connector.
  2. Advisories feed (WPScan, Joomla, Drupal, NVD) into `advisories.json`.
  3. AI review assistant for approvers (explains a map diff).
  4. Map authorisation rules (`owned_by_user`) and map rollback.

### 2026-11-18 — Serve JSON APIs, start maps from CMS templates

- **Branch:** `feat/json-api` (on top of `feat/shared-connectors`)
- **What:**
  - Contracts: map `deny` on types and fields; panel `GET /map-templates`,
    `POST /apps/{app}/map-template`, site `kind`.
  - gateway: `internal/jsonshape` (client JSON bodies checked key by key,
    unknown keys and nested values refused; legacy JSON rebuilt with only
    the declared fields and types); `serveAPI` with roles, limits,
    same-origin check on writes, JSON errors that never show the legacy
    body; legacy client with PUT, PATCH, DELETE and typed JSON bodies;
    validator `deny` lists.
  - panel-api: migration `0005` (site kind); `internal/templates`
    (WordPress, Joomla, Drupal, Moodle, generic), each tested against the
    map schema; template endpoints.
  - panel-ui: "Built with" when protecting a site; "Start from a template"
    in New version (template, connector, address, protocol).
  - Docs: map format (JSON APIs, templates, deny), READMEs, tenant manual.
- **Why:** modern sites and the dynamic parts of old ones load data with
  JavaScript; templates make the first map of a common CMS quick, with the
  admin areas closed from the start.
- **Status:** tests pass (gateway API end to end with a fake legacy JSON
  service, shaper, templates valid and compiled); template flow checked in a
  browser. Not tested on a real CMS.
- **Next steps:**
  1. Lab: DVWA and a real WordPress behind the gateway, then Juice Shop.
  2. Map authorisation rules (`owned_by_user`) and map rollback.
  3. Limit platform logins to the VPN; per-site fail-closed rate limits.

### 2026-11-18 — One connector per server, many sites per gateway, isolation check

- **Branch:** `feat/shared-connectors`
- **What:**
  - Contracts: map `meta.public_domain` and `legacy.reach.address`; panel
    connectors at `/tenants/{tenant}/connectors` (with `used_by`), site
    `PATCH` (`legacy_ip`) and `/exposure/check`, `Exposure` on sites,
    `exposed` in the platform overview, `/internal/maps`; tunnel headers
    `X-Eq-Tenant` (gateway → relay) and `X-Eq-Host` (to the connector).
  - Security fixes: a map can no longer use another tenant's connector
    (panel at draft and publish, relay at every request:
    `connector_not_owned`); `direct` mode only towards
    `EQ_DIRECT_DESTINATIONS`, so no map can reach Redis, PostgreSQL or the
    panel from the gateway.
  - panel-api: migration `0004` (connectors per tenant, exposure columns);
    `internal/exposure` (probe of the old IP on 443/80 with SNI and Host,
    compared with a random name; public unicast IPs only; hourly runner;
    status changes in logs and tenant activity); connector removal refused
    while a published map uses it.
  - gateway: all published maps, chosen by host (unknown host 404), a
    refused map keeps only its own site's previous version; `/tls/allowed`
    for Caddy; transports connect to `reach.address` with the site's name
    in Host/SNI; the tenant goes to the relay.
  - tunnel-relay: sessions carry the tenant, checked per request;
    `X-Eq-Host` passed on. connector: dials only the checked destination,
    presents `X-Eq-Host` as Host and SNI.
  - eqtool: connectors without app. Compose/Caddy: on-demand TLS for
    published domains, `EQ_GATEWAY_DOMAIN` and `EQ_APP` removed,
    `EQ_MAP_URL` defaults to every published map, new
    `EQ_DIRECT_DESTINATIONS` and `EQ_EXPOSURE_INTERVAL`.
  - panel-ui: tenant Connectors page; Site → Isolation (status, check now,
    old IP, configuration for Apache, nginx, IIS and the map lines); red
    status for exposed sites.
  - Docs: map format, tunnel spec, INSTALL (no gateway domain any more:
    each site keeps its own), architecture, READMEs, manual.
- **Why:** many sites on one server must not need many containers; a
  protected site must really stop being public while its neighbours stay
  online; and two isolation holes found while reading the code.
- **Status:** unit and integration tests pass in every module; UI checked
  in a browser; Caddyfile validated with Caddy 2.8. Not tested end to end
  with a real web server, real DNS and Let's Encrypt.
- **Next steps:**
  1. Lab campaign: Juice Shop behind the gateway (owner's tests below).
  2. Map authorisation rules (`owned_by_user`) and map rollback.
  3. Limit platform logins to the VPN; per-site fail-closed rate limits.

### 2026-11-18 — Separate the platform from tenants, invite people by email

- **Branch:** `feat/tenant-privacy`
- **What:**
  - Contract: `/tenants/{tenant}/invitations` (list, invite, revoke),
    public `/invitations/{token}` and `/invitations/{token}/accept`,
    `/platform/tenants` (overview) and `/platform/tenants/{tenant}/invitation`
    (invite the first administrator again); `POST /tenants` now takes the
    first administrator's name and email; `POST /tenants/{tenant}/users`
    removed; `/audit` lost its `tenant` parameter.
  - panel-api: migration `0003_invitations.sql`; `internal/mail` (SMTP with
    STARTTLS or implicit TLS, `off`, `log` for development); invitation
    tokens of 32 random bytes stored as SHA-256, single use, 48 hours,
    revocable, sent only by email; accepting creates the user and logs in.
    Platform users hold no tenant role any more: every tenant endpoint
    answers them 403/404; they see tenant names, whether anyone joined, and
    per site only name and status. Audit split: platform actions vs tenant
    actions. Re-inviting the first administrator works only while nobody
    has joined. Tests rewritten around invitations, plus privacy checks.
  - panel-ui: `/invite/:token` page; platform tenants overview with status
    dots, "Add a customer" (with the administrator's email) and "Invite
    again"; tenant picker removed (platform and tenant panels never mix);
    people page invites and lists pending invitations with revoke; role
    helpers mirror the backend; activity sentences for the new actions.
  - Compose and Caddy: `EQ_PANEL_DOMAIN` publishes the panel to customers
    (`/v1/internal` answers 404 there and in panel-ui's Caddy); SMTP
    settings and `secrets/smtp.password`. Docs: INSTALL, READMEs,
    architecture ("Platform and tenants"), both manuals.
- **Why:** customers own their tenant; the platform operator must see only
  high-level status, never configuration, maps or people.
- **Status:** `go test -race` for panel-api, UI build, tests and boundary
  check pass; flow checked in a browser (create customer, join, invite,
  revoke, reused link refused, platform reading a site gets 404). Not
  tested against a real SMTP server.
- **Next steps:**
  1. Lab campaign: Juice Shop behind the gateway, measure compatibility,
     false blocks and bypasses.
  2. Limit platform logins to the VPN / localhost address.
  3. Optional support access granted by the tenant (time-limited, audited).
  4. Per-site fail-closed option for rate limits when Redis is down.
  5. Map authorisation rules (`owned_by_user`) and map rollback.

### 2026-11-18 — Add two-step login, password change and user deactivation

- **Branch:** `feat/mfa`
- **What:**
  - Contract: `/auth/mfa/setup`, `/auth/mfa/enable`, `/auth/mfa/verify`,
    `/auth/password`, `DELETE /tenants/{tenant}/users/{id}`,
    `/tenants/{tenant}/users/{id}/reset-mfa`; `mfa_enabled` and
    `mfa_verified` on users; the two-step rule in the description.
  - panel-api: migration `0002_mfa.sql`; `internal/mfa` (TOTP RFC 6238
    checked against the RFC vectors, ±1 step, replay refused; ten
    single-use recovery codes stored as SHA-256; secrets encrypted with
    `EQ_MFA_KEY`, user id as additional data); sessions carry `mfa_ok`, the
    API answers 403 `mfa_required` outside `/auth` until the second step,
    and the session id changes when it succeeds; throttle of wrong codes;
    password change ends the other sessions; tenant administrators
    deactivate people and reset authenticators (sessions end); audit of
    every step. MFA mandatory unless `EQ_MFA_REQUIRED=false`.
  - panel-ui: login steps (code from the app or a recovery code), first
    login with QR code (drawn in the browser) and manual key, recovery codes
    with copy and download, account page with password change, people page
    with authenticator status, reset and deactivate; activity sentences for
    the new actions.
  - Compose: `secrets/mfa.key` for panel-api. INSTALL updated.
- **Why:** the panel signs what the gateway lets through: a stolen password
  alone must not be enough to publish a map.
- **Status:** Go tests with the race detector green (TOTP vectors,
  replay, recovery code single use, throttle, password change logging out
  other browsers, deactivation, reset) against a real PostgreSQL. Driven in
  Chrome: first login forced to the authenticator setup, code accepted,
  recovery codes shown, panel opened; second login refuses a wrong code and
  accepts the next one; people and account pages. Not done: recovery code
  regeneration, WebAuthn keys, platform users management from the panel,
  forced password change on first login.
- **Next steps:**
  1. Test the whole stack on the VPS with the remote vulnerable server.
  2. Map diff between versions; form helpers for common map edits.
  3. Service status page; log search.
  4. Gateway serving several sites by host name.
  5. File uploads with AV/CDR scanning.

### 2026-11-18 — Add the web panel (panel-ui)

- **Branch:** `feat/panel-ui`
- **What:**
  - Contract: typed responses for tenants, users, apps, connectors, map
    versions and audit entries (the UI client is generated from them);
    `document` and `detail` declared as free objects.
  - `panel-ui/` (TypeScript, React 19, Vite, React Router, TanStack Query,
    Radix, Tailwind 4): login; tenants; protected sites with sidebar and
    status; site tabs Map (version list with the approval rail, version
    page with document and next action, new version from scratch or from a
    version with a YAML editor that checks syntax and shows the panel's
    format errors) and Connectors (add with the `docker run` command and the
    token shown once, rotate, remove); people with roles; activity in plain
    sentences. Generated client (`openapi-typescript` + `openapi-fetch`)
    with the CSRF header; error messages that say what to do.
  - Rules enforced in CI (`ui.yml`): generated client equal to the
    contract, module boundaries, files ≤ 200 lines, type check, build,
    tests.
  - Docker image on Caddy (static files, `/v1` to panel-api on the same
    origin, CSP); Compose `panel-ui` on `127.0.0.1:8088` (panel-api no
    longer published). panel-api can serve the build with `EQ_UI_DIR`
    (SPA fallback, cache for hashed assets); its CSP now allows inline
    style attributes.
  - INSTALL: first site through the web panel.
- **Why:** the panel API alone still meant `curl` on the VPS; now the whole
  first-site flow is done from the browser.
- **Status:** type check, unit tests and boundary check green. Driven in a
  real browser (Chrome via Playwright) against panel-api and PostgreSQL:
  login, protect a site, add a connector (command shown), editor writes
  version 2 from version 1 and sends it for review, administrator approves
  and publishes it (version 1 retired), activity page, no console errors.
  Not done: MFA, password change and user deactivation, map diff between
  versions, service status, logs, SIEM, LLM settings, import/export,
  dark mode.
- **Next steps:**
  1. Test the whole stack on the VPS with the remote vulnerable server.
  2. MFA (TOTP) and password change for panel users.
  3. Map diff between versions and form-based helpers for common map
     edits.
  4. Service status page; log search.
  5. Gateway serving several sites by host name.

### 2026-11-18 — Add panel-api: tenants, sites, connectors, maps

- **Branch:** `feat/panel-api`
- **What:**
  - Contract: OpenAPI updated with auth, tenants, users, apps, connectors
    (create, rotate, delete), map lifecycle (draft, submit, approve, reject,
    publish), internal endpoints for gateway and relay, error body; the
    `X-Eq-Panel` CSRF header. Planned sections kept.
  - `panel-api/` (new Go module): PostgreSQL (`lib/pq`), embedded
    migrations with an advisory lock; PBKDF2 passwords, sessions stored as
    SHA-256, login throttle, roles (platform admin/operator; tenant admin,
    editor, approver, auditor) with tenant isolation; maps validated against
    the embedded schema copy, panel-owned `meta`, Ed25519 signing at
    publication, one published version per site and approver ≠ author
    enforced by the database; audit log of every change;
    `create-admin` subcommand; health endpoints; Dockerfile.
  - Gateway: map source abstraction — file (as before) or panel
    (`EQ_MAP_URL`, `EQ_MAP_TOKEN[_FILE]`, ETag polling every 15 s, retry
    after a rejected map).
  - tunnel-relay: connectors from the panel (`EQ_CONNECTORS_URL`,
    `EQ_PANEL_TOKEN[_FILE]`), changes detected by digest; file kept as
    fallback.
  - Compose: `postgres` (16, volume, health check) and `panel-api`
    (`127.0.0.1:8088`); gateway reads the map of `EQ_APP` from the panel;
    relay reads connectors from the panel. **Fix:** services that read
    `secrets/` now run as `EQ_UID:EQ_GID` (the files are `0600`; as
    distroless `nonroot` the gateway could not read `form.key`).
  - CI: panel-api in the Go matrix (tests use the runner's PostgreSQL) and
    in image build/publish. `scripts/validate_schemas.py` checks embedded
    schema copies.
  - Docs: INSTALL (secrets, first administrator, panel workflow with
    `curl`), panel-api README, architecture.
- **Why:** without a panel every change meant files and commands on the
  VPS. Now sites, connectors and maps live in one place, with four-eyes
  approval and an audit trail.
- **Status:** tests green with the race detector, including the full API
  flow against a real PostgreSQL (isolation, roles, CSRF, author cannot
  approve, signature verified with the public key, relay view of
  connectors, audit) and login throttling. End to end with real binaries
  and PostgreSQL: admin created, tenant, site, user, connector, map drafted
  by an editor, approved and published by the admin; the gateway downloaded
  the signed map from the panel and served the legacy app through relay and
  connector, whose token came from the panel. **No web interface yet.** Not
  done: MFA, user deactivation and password change, change classes
  (automatic approval of narrowing changes), map diff, per-request
  semantic checks of maps (only the schema), gateway serving several sites.
- **Next steps:**
  1. panel-ui: login, sidebar with sites, site page (connectors, map
     versions with editor, approve and publish), users, audit.
  2. MFA (TOTP) for panel users.
  3. Service status page and log search.
  4. Test the whole stack on the VPS.
  5. Gateway serving several sites by host name.

### 2026-11-18 — Add rate limits and behaviour rules

- **Branch:** `feat/rate-limits`
- **What:**
  - `internal/limits`: fixed-window counters (Redis Lua `INCR`+`PEXPIRE`,
    sets for distinct values, marks with TTL; in-memory store as fallback).
    Rate limits from `defaults.rate_limit`, `routes[].rate_limit` and
    `forms[].rate_limit` (`per_ip`, `per_session`, `per_user`). Behaviour
    rules `distinct_param_values`, `failed_logins`, `request_rate` with
    actions `throttle` (`challenge` treated as throttle), `block`
    (site-wide), `alert`; `missing_predecessor` accepted and reported as not
    applied. `defaults.legacy_budget.max_rps` shared across replicas.
  - Gateway: checks before serving (assets only see blocks), 429 with
    `Retry-After` or 403, events `ratelimit.hit`, `request.blocked`
    (`behavior_block`), `behavior.alert`; enumeration observed on validated
    route values, failed logins observed on `session.start` forms; fail open
    when the store is unreachable (error logged at most every 10 s). Notes
    about unapplied map features logged at map load. The Redis client is
    shared with the session store.
  - Example map: `login_bruteforce` rule. Docs: map format, gateway README.
- **Why:** after login, brute force and record enumeration (IDOR) are the
  most common attacks on legacy portals; the map already described these
  limits but the gateway ignored them.
- **Status:** tests green with the race detector, including the limits
  engine against a real `redis-server` and HTTP tests: page limit with
  `Retry-After`, assets not counted, enumeration throttled at the third
  distinct id without reaching the legacy app, brute force blocked
  site-wide after three failed logins (even with the right password), no
  password in events. Not done: CAPTCHA challenge, `missing_predecessor`,
  sliding windows, per-tenant limits from the panel.
- **Next steps:**
  1. Test on the VPS with the remote vulnerable server (tunnel, login,
     limits).
  2. File uploads with AV/CDR scanning.
  3. Scripts by hash with CSP, URLs inside CSS, `external_links`.
  4. Prometheus metrics (gateway, relay).
  5. panel-api: tenants, maps, approval and signing.

### 2026-11-18 — Add pass-through login and sessions

- **Branch:** `feat/sessions`
- **What:**
  - Contract: `password` base type (requires rules, like `text`);
    `defaults.session` (`idle_minutes`, `absolute_hours`); `session` on
    forms (`start: <role>` or `end: true`) and routes (`end: true`); example
    map with login and logout, `roles.auth.provider: passthrough`.
  - `internal/session`: sessions keyed by the SHA-256 of a random id,
    content encrypted with AES-256-GCM (key derived from `EQ_FORM_KEY`, id
    as additional data), absolute and idle limits; Redis store (go-redis)
    and in-memory store.
  - `internal/legacyclient`: legacy `Set-Cookie` captured (still never sent
    to the client).
  - `internal/server`: session loaded per request, role from the session,
    legacy cookies added to rebuilt requests (routes, forms, assets),
    session created when the legacy app sets a cookie, login with a new id,
    `success.cookie` check, legacy expiry (`legacy.session_expired`) drops
    the role, logout, `Cache-Control: private, no-store` on session pages,
    `__Host-eq_session` cookie written once before the headers; session
    hash in events (`session`). Store down: visitors are anonymous, logins
    503.
  - Configuration `EQ_REDIS_URL`; Compose `redis` service (control network,
    no persistence, 256 MB, volatile-lru).
  - Docs: map format (login and sessions), gateway README, INSTALL.
- **Why:** most legacy apps worth protecting have a login. Their session
  cookies are a prime target (fixation, theft, weak flags): now the browser
  never sees them, and the gateway decides who may reach restricted pages.
- **Status:** tests green with the race detector, including a real
  `redis-server` test and an in-process login flow (pre-login legacy
  cookie, failed and successful login, new id, old id rejected, legacy
  expiry, logout). End to end with real binaries: two gateway replicas on
  one Redis, login on replica A, private page served by replica B; the
  stored blob does not contain the legacy cookie; Redis down → private page
  403, public page 200. Not done: cookie `Path`/`Domain`, OIDC, per-user
  legacy accounts, session listing/revocation from the panel, login rate
  limits (`rate_limit` is still ignored).
- **Next steps:**
  1. Test on the VPS with the remote vulnerable server (tunnel + login).
  2. Rate limits in Redis (routes, forms, login), then behavioural rules.
  3. File uploads with AV/CDR scanning.
  4. Scripts by hash with CSP, URLs inside CSS, `external_links`.
  5. Prometheus metrics; panel-api.

### 2026-11-18 — Add tunnel-relay and connector

- **Branch:** `feat/tunnel`
- **What:**
  - Contract: `schema/tunnel/tunnel.md` (`equalizer-tunnel/1`) replaces
    the gRPC draft `tunnel.proto`. The connector opens HTTPS on 443, the
    connection is upgraded and carries yamux streams with plain HTTP/1.1;
    the gateway → relay API on the control network is in the same document.
    Why not gRPC: its modules cannot be downloaded in the development
    environment, it would add a large dependency tree, and an upgraded
    HTTPS connection goes through Caddy on 443 with an automatic
    certificate (no extra port).
  - `tunnel-relay/` (new Go module): `connectors.json` registry (SHA-256
    of tokens, reloaded every 15 s and on SIGHUP; removed or rotated
    connectors lose their tunnels), upgrade and yamux server, session pool
    (max 8 per connector, round robin among sessions that declared the
    destination), gateway forwarding with a bearer token, errors in
    `X-Eq-Error`, admin `/healthz`, `/readyz`, `/v1/tunnels`, JSON logs,
    `connector.down` and `request.blocked` (`tunnel_unauthorized`) events.
  - `connector/` (new Go module): local allowlist `EQ_ALLOW` checked on the
    request and again in the dialer, executor without redirect following
    (30 s timeout), TLS options for legacy apps with self-signed
    certificates, reconnection with exponential backoff and jitter,
    `HTTPS_PROXY` support, Docker healthcheck while the tunnel is open.
  - Gateway: `transport.Relay` (destination checked against
    `legacy.reach.allow`, request id propagated, relay/connector errors
    mapped), `EQ_RELAY_URL` and `EQ_RELAY_TOKEN[_FILE]`, `site.BuildWith`
    with installation options.
  - `eqtool secret` (gateway ↔ relay token) and `eqtool connector`
    (registers a connector, prints its token once, `-rotate`).
  - Compose: `tunnel-relay` service (1 replica), Caddy site on
    `EQ_RELAY_DOMAIN` publishing only `/v1/tunnel`; port 8443 removed.
  - CI: Go matrix with the two new modules and their images;
    `publish.yml` pushes images to GHCR on every merge to `main`.
  - Docs: INSTALL (relay domain, connector registration and `docker run`),
    architecture, tenant manual, READMEs, CLAUDE.md decision.
- **Why:** the connector is what makes an already running vulnerable server
  protectable without exposing it: the legacy network needs only outbound
  HTTPS.
- **Status:** tests green with the race detector in all four modules. End
  to end with the real binaries (fake ISO-8859-1 legacy app, relay,
  connector, gateway in connector mode): 502 before the connector starts;
  then pages served, links rewritten, the form bound and submitted (the
  legacy app received `permitId=7&text=Caff%E8+ok`), 303 on success;
  relay restart → connector reconnects within seconds; a wrong connector
  token is refused; all 42 log and event lines validate against the
  schemas. **Not tested yet:** Caddy in front of the relay (upgrade through
  the real edge proxy) and the GHCR publication: both need the VPS and the
  first merge. Not done: mTLS enrolment from the panel, panel-signed
  allowlist, several relay replicas (tunnels are in memory), Prometheus
  metrics.
- **Next steps:**
  1. Test on the VPS: `docker compose up`, connector on the remote
     vulnerable server, map in connector mode (INSTALL steps 5–7).
  2. Pass-through login and sessions in Redis.
  3. File uploads with AV/CDR scanning; per-form rate limits.
  4. Scripts by hash with CSP, URLs inside CSS, `external_links`.
  5. Prometheus metrics (gateway, relay); panel-api.

### 2026-11-18 — Serve POST forms

- **Branch:** `feat/forms`
- **What:**
  - Contract: `hidden` placeholder namespace (legacy hidden inputs kept
    server-side) documented in the map schema; forms section in
    `docs/map-format.md`.
  - `internal/charset`: `FromUTF8` re-encodes values into ISO-8859-1,
    ISO-8859-15 or Windows-1252; unrepresentable characters are an error.
  - `internal/formtoken`: form state sealed with AES-256-GCM (form id,
    action parameters, kept hidden values, page charset, expiry, hash of the
    browser binding cookie). Stateless: works across replicas with a shared
    key.
  - `internal/formmap`: recognises a legacy `<form>` as a declared form by
    inverting its `legacy_request` (path, query, hidden inputs used as
    `{{params.x}}`), computes the public action and the input renames.
  - `internal/pipeline/forms`: new step before `rewrite`. Removes every
    hidden input of POST forms, sets the public action, renames inputs,
    drops `formaction`/`accept-charset`, adds the `_eq_token` input.
    Pipeline helpers `BaseURL`, `Attr`, `SetAttr`, `DelAttr`, `MarkFinal`.
  - `internal/legacyclient`: POST with an urlencoded body rebuilt from
    `legacy_request.form`.
  - `internal/site`: forms checked at load time (types, templates,
    `on_success.redirect`); forms with file fields marked unsupported (503).
  - `internal/server/form.go`: role, cross-site check (`Origin`,
    `Sec-Fetch-Site`), urlencoded only (415 otherwise), 1 MiB limit (413),
    token checks (missing, invalid, expired, other browser, other
    form/record → 403), field validation, re-encoding, success check
    (`status`, `body_contains`, `not_contains`, `redirect_to`), 303 to
    `on_success.redirect` or the legacy page rendered faithfully.
    `__Host-eq_form` cookie (Secure, HttpOnly, SameSite=Lax) set only on
    pages that contain a bound form.
  - Configuration: `EQ_FORM_KEY` / `EQ_FORM_KEY_FILE`; without it a random
    key and a warning (single replica only). `eqtool formkey`; Compose
    mounts `secrets/form.key`; INSTALL updated.
- **Why:** writes are the riskiest part of a legacy app. The browser never
  sees the legacy hidden fields, cannot replay a form from another browser
  or for another record, and every value reaches the legacy app typed and in
  its own charset.
- **Status:** tests green with the race detector, including an end-to-end
  test with a fake ISO-8859-1 legacy app: the page shows no internal hidden
  value, the rebuilt POST carries `permitId=42`, `text=Caf%E9 note`, the
  kept hidden value and the fixed value, and nothing else; ten rejection
  cases never reach the legacy app. Not done: file uploads (multipart), the
  `cookie` success check, per-form rate limits and CAPTCHA, re-encoding of
  GET query values for non-UTF-8 legacy apps, JavaScript that reads hidden
  inputs or input names (it would see the public names).
  **Requires on the VPS:** `eqtool formkey -out secrets/form.key` before
  `docker compose up`, otherwise the gateway container fails to mount it.
- **Next steps:**
  1. `tunnel-relay` and `connector`.
  2. Pass-through login and sessions in Redis (then the `cookie` check and
     per-user legacy accounts).
  3. File uploads with AV/CDR scanning; per-form rate limits.
  4. Scripts by hash with CSP, URLs inside CSS, `external_links`.
  5. Gateway Prometheus metrics; visual comparison with Playwright.

### 2026-11-18 — Translate the whole repository to English

- **Branch:** `chore/english`
- **What:**
  - Go code (gateway, eqtool): every comment, log message, error, HTTP error
    page and test translated; test data uses English entities (`guest`,
    `citizen`, `permit_id`, `/permits/{id}`). No logic changes.
  - Contracts: descriptions in the map, event and log schemas, OpenAPI
    summaries and descriptions, tunnel proto comments, metrics catalogue.
  - Example map renamed `portale-pratiche.yaml` → `permits-portal.yaml`
    and rewritten with English ids (`example-city`, `permits_list`,
    `permit_detail`, `permit_supplement`, ...).
  - Manuals renamed `amministratore.md` → `admin.md` and `cliente.md` →
    `tenant.md`; README, CLAUDE.md, INSTALL.md, architecture, map format,
    every service README, Compose files, `.env.example`, Caddy, Vector,
    Loki, CI workflows and Python scripts translated. This changelog's
    earlier entries translated as well.
  - `CLAUDE.md`: the language rule is now "everything in English",
    including commit messages; note on the vendored HTML parser and on
    opening PRs through the REST API. Stale lines fixed (INSTALL status,
    the duplicate "rendering pipeline: to do" row in the gateway README).
- **Why:** the project is international; Italian in comments and docs is a
  barrier for contributors.
- **Status:** `go vet` and `go test -race` green for gateway and tools;
  contracts validate; Compose validates. Identifiers were already English,
  so behaviour is unchanged.
- **Next steps:**
  1. Form POST: typed fields, CSRF token, legacy hidden fields held
     server-side, re-encoding into the legacy charset.
  2. `tunnel-relay` and `connector`.
  3. Pass-through login and sessions in Redis.
  4. Scripts by hash with CSP, URLs inside CSS, `external_links`.
  5. Gateway Prometheus metrics; visual comparison with Playwright.

### 2026-11-18 — Add faithful rendering and assets

- **Branch:** `feat/faithful-rendering`
- **What:**
  - Contract: `assets` section in the map (prefix, allowed types, maximum
    size, roles); prefixes containing `.` or `..` rejected; example updated.
  - `gateway/internal/thirdparty/xhtml`: copy of the HTML5 parser
    `golang.org/x/net/html` v0.40.0 (BSD licence), because the Go module
    proxy is not reachable from every development environment.
  - `internal/charset`: detection (header, `<meta>`, content) and
    conversion to UTF-8 of ISO-8859-1, ISO-8859-15, Windows-1252; other
    encodings rejected.
  - `internal/legacyurl`: translation of legacy URLs → public URLs from
    `links`, `compat` and the `legacy_request` of GET routes; assets keep
    their path; relative fallback without the internal host.
  - `internal/pipeline`: `Step` interface, `Context`, statistics; `clean`
    step (revealing comments and meta tags) and `rewrite` step (href, src,
    action, formaction, srcset, `<base>`, ...); charset declarations
    aligned to UTF-8.
  - `internal/server`: pipeline applied to HTML pages (fail closed on
    error), legacy redirects translated to public URLs (external ones
    blocked), assets checked on name, declared type and real type,
    sandboxed SVG, asset query never sent to the legacy application.
  - Router: asset prefixes (longest wins), after routes.
  - Documentation: gateway README, map format (links and assets).
- **Why:** the "100% identical interface" requirement: without assets the
  page loses its style; without rewriting, links lead to legacy URLs.
- **Status:** tests green with the race detector. Checked on sample pages:
  links, resources, forms and srcset rewritten, internal IP absent, comments
  and `generator` meta removed, ISO-8859-1 with entities converted without
  losing accents; assets with PHP content disguised as PNG or with a type not
  allowed are rejected; `../` and `//` in names rejected. Not done yet:
  scripts allowed by hash with CSP, URL rewriting inside CSS and `style`
  attributes, `external_links`, automatic visual comparison.
  **Note for forms:** pages are now output in UTF-8, so browsers will submit
  forms in UTF-8; the gateway must re-encode values into the legacy
  application's original charset before rebuilding POSTs.
- **Next steps:**
  1. Form POST: typed fields, CSRF token, legacy hidden fields held
     server-side, re-encoding into the legacy charset.
  2. `tunnel-relay` and `connector`.
  3. Pass-through login and sessions in Redis.
  4. Scripts by hash with CSP, URLs inside CSS, `external_links`.
  5. Gateway Prometheus metrics; visual comparison with Playwright.

### 2026-11-18 — Rebuild GET requests to the legacy application

- **Branch:** `feat/legacy-reconstruction`
- **What:**
  - Contract: `roles.anonymous` in the map schema (role of unauthenticated
    visitors) and in the example.
  - `gateway/internal/mapmodel`: map types moved to their own package and
    extended (types, defaults, field specs, `legacy_request`).
  - `internal/validate`: `int`, `enum`, `bool`, `date`, `string`, `email`,
    `text` types with constraints and the `no_html`, `no_sql_meta`,
    `no_template_meta` rules; undeclared query parameters dropped, repeated
    ones rejected; patterns and rules checked when the map loads.
  - `internal/placeholder`: `{{params.x}}`, `{{query.x}}`, ... placeholders.
  - `internal/legacyclient`: request built from scratch (path with encoded
    values, declared query, fixed headers), redirects not followed, response
    capped at 16 MiB, legacy headers filtered.
  - `internal/transport`: direct mode with a host:port allowlist in the
    dialer (also a defence against redirects and DNS).
  - `internal/budget`: concurrent requests to the legacy application, per
    replica.
  - `internal/site`: compiles a map version (router, validator, client,
    budget) and rejects inconsistent maps.
  - `internal/server/route.go`: anonymous role (403 on restricted routes),
    validation, rebuilding, generic pages for legacy 4xx/5xx (never stack
    traces), security headers, `request.served` events with split latency,
    `field.rejected` without the value, `legacy.error`.
  - Documentation: gateway README, map format (rules and anonymous role),
    INSTALL.
- **Why:** the first step in which the gateway really serves a site: the
  legacy application receives only rebuilt requests and validated values.
- **Status:** tests green with the race detector. End-to-end test with a fake
  legacy app: `/` and `/permits/7` served (the legacy app received only
  `GET /index.jsp` and `GET /detail.jsp?id=7`), `/permits/7abc` → 400,
  `/index.jsp` and `/../../etc/passwd` → 404. Known limits: legacy HTML
  passes through without faithful rendering (internal links still point to
  legacy URLs); legacy redirects become 502; forms, APIs, login and sessions
  are not served; `mode: connector` answers 502; the budget is not shared
  between replicas; map types are still written by hand.
- **Next steps:**
  1. Faithful rendering pipeline: HTML5 parser, cleanup, link rewriting
     (`links`, `compat`), redirect translation.
  2. Form POST with typed fields and CSRF token.
  3. `tunnel-relay` and `connector`.
  4. Pass-through login and sessions in Redis.
  5. Gateway Prometheus metrics.

### 2026-11-18 — Add base Compose, logging and gateway skeleton

- **Branch:** `feat/compose-gateway-skeleton`
- **What:**
  - `gateway/`: Go service (standard library only) with loading and
    verification of the Ed25519-signed map, hot reload (file and SIGHUP),
    allowlist router (404 unmapped, 405 method not allowed, OPTIONS and HEAD
    handled by the gateway, TRACE/CONNECT rejected), fail closed without a
    map (503), JSON logs conforming to the schema, `request_id` generated by
    the gateway and returned in `X-Eq-Request-Id`, `request.blocked` events,
    client IP only from trusted proxies, `/healthz` and `/readyz` on a
    separate port, `healthcheck` subcommand, distroless non-root Dockerfile,
    tests.
  - `tools/`: `eqtool keygen` and `eqtool sign` (map signing until
    panel-api exists), Dockerfile.
  - `scripts/map_to_json.py`: validates a YAML map and converts it to JSON.
  - `deploy/compose/`: `edge`, `control`, `lab` (internal) networks, Caddy
    with automatic TLS and dynamic balancing of replicas, gateway with 2
    replicas, Vector → Loki, Grafana on localhost only, `tools` profile,
    `.env.example`.
  - `SignedMap` contract (OpenAPI): the map travels as a base64 `payload`
    signed byte by byte instead of as a JSON object.
  - CI `go.yml`: gofmt, vet, tests with the race detector, Compose
    validation, image builds.
  - `INSTALL.md`: steps 4–6 made concrete (configuration, key and
    signing, start, logs in Grafana). `CLAUDE.md`: notes on the development
    environment.
- **Why:** to have from day one the basis for debugging (structured,
  searchable logs) and a gateway that already applies deny by default.
- **Status:** Go tests green; local end-to-end test with the signed example:
  `/` and `/permits/42` → 503 (mapped, no rebuilding yet), `/wp-admin` → 404,
  wrong method → 405, TRACE → 405, `/readyz` ready; the 12 log/event lines
  produced validate against the schemas. Compose is validated with
  `docker compose config` but **has not been started** (no Docker daemon in
  the development session): Caddy, Vector, Loki and Grafana must be tested on
  the VPS. Map types in the gateway are written by hand (a subset) because
  generation tools cannot be downloaded here: to be replaced.
- **Next steps:**
  1. Test the Compose stack on the VPS and fix it (especially the Vector
     configuration).
  2. Validation of parameter and field types in the gateway.
  3. `legacyclient` and `transport` (direct mode), then rebuilding of GET
     requests.
  4. Faithful rendering pipeline with an HTML5 parser (`golang.org/x/net/html`).
  5. `tunnel-relay` and `connector` on `tunnel.proto`.
  6. Gateway Prometheus metrics and type generation from the schemas.

### 2026-11-18 — Initialise repository, contracts and documentation

- **Branch:** `feat/bootstrap`
- **What:**
  - Monorepo layout with one folder per service and a README in each.
  - `schema/map/gateway-map.schema.json`: JSON Schema of the map (version
    `gateway-map/1.0`) with a validated example in `schema/map/examples/`.
  - `schema/api/panel-api.yaml`: OpenAPI 3.1 for panel-api (first version,
    brief bodies).
  - `schema/events/`: schemas of security events and log lines.
  - `schema/tunnel/tunnel.proto`: gRPC protocol between connector and relay.
  - `schema/metrics/catalog.md`: Prometheus metrics catalogue.
  - `scripts/validate_schemas.py` and the `contracts.yml` CI workflow.
  - `README.md`, `CLAUDE.md`, `INSTALL.md`, `docs/architecture.md`,
    `docs/map-format.md`, administrator and tenant manuals.
- **Why:** to fix the contracts between services before writing code, so
  services can be developed separately without refactoring; to make the
  project resumable by anyone.
- **Status:** contracts validate locally (schemas, example, OpenAPI, proto).
  The schema rejects the main bad cases: `text`/`file` fields without rules,
  non-`GET` routes, `template` rendering without extraction, connector
  without an id. In the map the authorisation key is called `target` (not
  `on`, which YAML reads as a boolean). No service is implemented yet:
  `INSTALL.md` and the manuals describe the planned behaviour.
- **Next steps:**
  1. `deploy/compose/`: base Compose with `edge`, `control`, `lab` networks
     and a log pipeline (Vector, Loki, Grafana).
  2. `gateway/`: HTTP server, loading and verification of the signed map,
     allowlist router, structured logs with `request_id`.
  3. `gateway/internal/pipeline/`: faithful rendering (parsing, cleanup, URL
     rewriting, forms) with tests on sample pages.
  4. `tunnel-relay/` and `connector/` on the `tunnel.proto` protocol.
  5. Type generation for Go from the schemas (`schema/` → `gen/`).
