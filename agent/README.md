# agent

Discovery agent in Python (standard library only). It takes discoveries from
panel-api, explores the site **read-only** through its connector (the same
way the gateway reaches it) and hands back a report and a proposed map,
which panel-api saves as a draft: it still needs someone else's approval.

The agent is being extended into a full study of the site, with a real
browser and two checks of its own map: see [docs/agent.md](../docs/agent.md).

## What a discovery does

1. **Crawl** (`crawler.py`): `/`, `robots.txt`, `sitemap.xml`, the template's
   pages, the paths real visitors asked for (reported by the gateway), then
   every same-site link, GET form and URL found in scripts. At most
   `max_pages`, 20 URLs per shape (calendars, searches), `EQ_RATE` requests
   per second. With test accounts (up to three), one more pass per account,
   each with a new session.
2. **Never opened** (`rules.py`, fixed): admin areas of each CMS, tooling
   (`phpmyadmin`, `server-status`…), backups and dot files, links that change
   something (logout, delete…). They are listed in the report.
3. **Fingerprint** (`fingerprint.py`): CMS and version, plugins, themes,
   libraries; components listed in `data/advisories.json` are flagged (the
   file ships empty: fill it from a vendor feed).
4. **Map** (`cluster.py`, `mapbuild.py`): varying segments become typed
   parameters (`{id}` int, `{slug}` with the admin names refused), query
   values give their types, POST forms become forms (hidden fields kept on
   the gateway), assets are grouped by folder. Form fields get the narrowest
   type their HTML declares (`fields.py`: choices, ranges, formats, RE2-safe
   patterns) and free text refuses HTML and template code; the report lists
   what is still open, with the reason. The base is the published map
   (continuous discovery: only additions, nothing removed) or the CMS
   template.
5. **AI** (`llm/`, optional): the tenant's own model reviews the structure
   (never values or page text; samples masked). It may rename, set the risk,
   exclude, and narrow a type when every observed value fits; anything else
   in its answer is ignored.

## Review aid

When a version is sent for review and the tenant has an AI model, the agent
gets the changes and the risks the panel found with fixed rules, and writes
a short summary and the questions to ask before approving. It never sees
the map's values or the site's pages, and it never approves.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `EQ_PANEL_URL` | `http://panel-api:8080` | panel-api on the control network |
| `EQ_INTERNAL_TOKEN` or `_FILE` | — | Service token of panel-api |
| `EQ_RELAY_URL` | `http://tunnel-relay:8082` | Relay's internal address |
| `EQ_RELAY_TOKEN` or `_FILE` | — | Token for the relay (the gateway's) |
| `EQ_DIRECT_DESTINATIONS` | — | `host:port` reachable in direct mode (lab) |
| `EQ_RATE` | `2` | Requests per second towards a site |
| `EQ_AGENT_ALLOW_PRIVATE_LLM` | `false` | `true` lets a tenant's local model live on a private network (on-prem) |
| `EQ_ADVISORIES_FILE` | bundled | Known vulnerable components |

AI settings belong to each tenant (panel → Settings); the platform has none.

## Tests

```bash
python3 -m unittest discover -s tests -t .
```

A small WordPress-like site runs in the test; the agent explores it in direct
mode and the proposed map is checked.
