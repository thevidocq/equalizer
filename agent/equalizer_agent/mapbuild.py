"""The proposed map: the published map (continuous discovery) or the CMS
template as a base, plus what the crawl found and the base does not cover.
Nothing is removed from a published map: that stays a human decision."""

import copy
import re
import urllib.parse

from . import assettypes, cluster, fields as fieldspec, idor, rules
from .crawler import ASSET_EXT

COMMON_DENY = ["admin", "administrator", "wp-admin", "wp-login", "phpmyadmin", "pma", "cpanel", "plesk", "webmail",
               "server-status", "server-info", "cgi-bin", "install", "setup", "backup", "backups", "config",
               "console", "manager", "jsonapi", "xmlrpc"]


# What a map path may hold (schema/map/map.schema.json).
MAP_PATH = re.compile(r"^/[A-Za-z0-9_./{}\-]*$")


def skeleton(base_url: str, reach: dict) -> dict:
    return {
        "meta": {"schema": "gateway-map/1.0", "tenant": "x", "app": "x", "version": 1, "status": "draft"},
        "legacy": {"base_url": base_url, "reach": reach, "timeout_ms": 10000},
        "roles": {"anonymous": "guest", "list": {"guest": {}}},
        "defaults": {"rate_limit": {"per_ip": "120/min"}, "legacy_budget": {"max_concurrency": 10},
                     "string": {"max_length": 200}},
        "types": {}, "routes": [], "assets": [],
    }


def _segs(path: str) -> list:
    return [s for s in path.strip("/").split("/") if s] if path.strip("/") else []


def matches(pattern: str, path: str) -> bool:
    a, b = _segs(pattern), _segs(path)
    return len(a) == len(b) and all(x.startswith("{") or x == y for x, y in zip(a, b))


def _shape(path: str) -> str:
    return re.sub(r"\{[^}]*\}", "{}", path)


def fits(doc: dict, route: dict, path: str) -> bool:
    """Whether the gateway would serve path with route: the segments match
    and each value fits its parameter's type (the deny list of a slug keeps
    /administrator/index.php out of /{section}/{slug})."""
    if not matches(route.get("path", ""), path):
        return False
    types = doc.get("types", {})
    for pat, val in zip(_segs(route.get("path", "")), _segs(path)):
        if not pat.startswith("{"):
            continue
        spec = (route.get("params") or {}).get(pat.strip("{}"), {})
        t = types.get(spec.get("type", ""), {})
        base = t.get("base", spec.get("type", ""))
        if base in ("int", "id") and not val.isdigit():
            return False
        deny = {d.lower() for d in (t.get("deny") or []) + (spec.get("deny") or [])}
        if val.lower() in deny:
            return False
        rx = spec.get("pattern") or t.get("pattern")
        if rx:
            try:
                if not re.search(rx, val):
                    return False
            except re.error:
                return False
    return True


def covered(doc: dict, path: str) -> bool:
    """Whether the map serves path precisely. A path with a query (a key the
    gateway reported as dropped: /shop/?orderby=) also needs its keys."""
    path, _, q = path.partition("?")
    if q:
        keys = {k for k, _ in urllib.parse.parse_qsl(q, keep_blank_values=True)}
        r = gateway_route(doc, path)
        return r is not None and keys <= set(r.get("query") or {})  # the names a visitor sends
    return _covered(doc, path)


def gateway_route(doc: dict, path: str):
    """The route the gateway would serve path with: a route whose pattern is
    path itself, else the one that matches with the fewest parameters (a
    literal /search before /{slug})."""
    routes = [r for r in doc.get("routes", []) if r.get("method", "GET") == "GET"]
    same = next((r for r in routes if r.get("path") == path), None)
    if same is not None:
        return same
    # as the gateway's router: by structure, fewest parameters, then map order
    # (it checks the values after choosing)
    fit = [r for r in routes if matches(r.get("path", ""), path)]
    return min(fit, key=lambda r: r.get("path", "").count("{"), default=None)


def _covered(doc: dict, path: str) -> bool:
    if any(fits(doc, r, path) for r in doc.get("routes", [])):
        return True
    return any(path.startswith(a.get("prefix", "\0")) for a in doc.get("assets", []))


def _ensure_types(doc: dict):
    t = doc.setdefault("types", {})
    t.setdefault("slug", {"base": "string", "pattern": "^[A-Za-z0-9][A-Za-z0-9_-]{0,119}$", "deny": COMMON_DENY})
    t.setdefault("id", {"base": "int", "min": 0, "max": 999999999})


def _merge_query(e: dict, r: cluster.Route) -> list:
    """Adds to route e the query keys of r it does not declare yet: keys the
    site's own links use, with the values seen there, typed as for a new
    route. GET routes whose legacy request is a GET only, names a map can
    hold only, never a key seen without a value (a visitor's report is no
    evidence). Returns the keys added."""
    lr = e.get("legacy_request") or {}
    if e.get("method", "GET") != "GET" or lr.get("method", "GET") != "GET":
        return []
    have = set((lr.get("query") or {}).keys())
    taken = set(e.get("params") or {}) | set(e.get("query") or {})
    added = []
    for name, key in sorted(r.query_keys.items()):
        if key in have or key in taken or not cluster.IDENT.match(key):
            continue  # a visitor sends the key itself: only a name the map can hold
        if not [v for v in r.query_samples.get(name, []) if v]:
            continue
        taken.add(key)
        e.setdefault("query", {})[key] = dict(r.query[name])
        lr.setdefault("query", {})[key] = "{{query.%s}}" % key
        e["legacy_request"] = lr
        added.append(key)
    return added


def _is_dropped_key(doc: dict, p: str) -> bool:
    """A report of a key a route dropped: the route's own path (pattern or
    literal), then ?key= with no value."""
    path, _, q = p.partition("?")
    return bool(q) and q.endswith("=") and "&" not in q and any(e.get("path") == path for e in doc.get("routes", []))


def _prefix_of(doc: dict, path: str):
    best = None
    for a in doc.get("assets", []):
        p = a.get("prefix", "\0")
        if path.startswith(p) and (best is None or len(p) > len(best.get("prefix", ""))):
            best = a
    return best


def _dropped_keys(doc: dict, unmapped: list, crawl) -> list:
    """Query keys the gateway dropped on a route (reported with the route's
    pattern, /permits/{id}?orderby=, never a value), for the person: never
    added by the agent. Each says whether the site's own links use the key
    on that route, and what adding it would do (add: the panel's one-click
    change, or None when the map cannot take it)."""
    used = {}
    for pq in crawl.pages:
        path, _, q = pq.partition("?")
        for k, _ in urllib.parse.parse_qsl(q, keep_blank_values=True):
            used.setdefault(k, set()).add(path)
    out = []
    for u in unmapped:
        path, _, q = u.get("path", "").partition("?")
        if u.get("method") != "GET" or not _is_dropped_key(doc, u.get("path", "")):
            continue
        key = q[:-1]
        e = next((r for r in doc.get("routes", []) if r.get("path") == path and r.get("method", "GET") == "GET"), None)
        if e is None or key in (e.get("query") or {}):
            continue  # no GET route there, or this draft declares it already (the site's links use it)
        site = any(matches(path, p) for p in used.get(key, ()))
        lr = e.get("legacy_request") or {}
        item = {"route": path, "route_id": e.get("id"), "key": key, "clients": u.get("clients", 0),
                "verdict": "site" if site else "suspect", "add": None}
        if lr.get("method", "GET") != "GET" or not cluster.IDENT.match(key) or \
                key in (e.get("params") or {}) or key in (lr.get("query") or {}):
            item["why"] = "the map cannot take this key on this route (a path parameter, a key the site already " \
                          "gets under this name, or a route that posts to the site)"
        else:
            item["add"] = {"route_id": e.get("id"), "key": key}
            item["why"] = ("a link of the site uses it on this page" if site else
                           "no link of the site uses it: perhaps someone trying parameters; adding it lets its "
                           "value reach the site, checked as text without markup")
        out.append(item)
    return sorted(out, key=lambda x: (x["verdict"] != "site", -x["clients"]))[:50]


def _files_refused(doc: dict, unmapped: list, crawl) -> list:
    """Files under an asset prefix the gateway refused visitors, for the
    person: never added by the agent (anyone can ask for any file) and never
    opened for them. Each says whether a page of the site names the file,
    and what adding it would do (add: the panel's one-click change)."""
    named = set(crawl.assets) | {urllib.parse.urlsplit(u).path for u in crawl.urls}
    out = []
    for u in unmapped:
        p = u.get("path", "")
        a = _prefix_of(doc, p) if u.get("method") == "GET" and "?" not in p and "{" not in p else None
        if a is None:
            continue
        site = p in named
        ext = "." + p.rsplit(".", 1)[-1].lower() if "." in p.rsplit("/", 1)[-1] else ""
        kind = crawl.assets.get(p) or ASSET_EXT.get(ext, "")
        item = {"path": p, "clients": u.get("clients", 0), "verdict": "site" if site else "suspect", "add": None}
        if not assettypes.VALID.match(kind) or kind in a.get("types", []):
            item["why"] = "its type is unknown, or already allowed (then the file is refused for its size or content)"
        elif kind in assettypes.SCRIPTS:
            item["why"] = "a script no page of the site loads: never added on a visitor's request"
        else:
            item["add"] = {"prefix": a["prefix"], "type": kind}
            why = "a page of the site links to it" if site else \
                "no page of the site links to it: perhaps a leftover file, or someone trying addresses"
            if not assettypes.passive(kind, loaded=False):
                why += "; adding %s serves every %s file under %s" % (kind, kind, a["prefix"])
            item["why"] = why
        out.append(item)
    return sorted(out, key=lambda x: (x["verdict"] != "site", -x["clients"]))[:50]


def _route_entry(r: cluster.Route, anon: str, taken: set) -> dict:
    lits = [s for s in _segs(r.path) if not s.startswith("{")]
    rid = cluster.ident("_".join(lits) + ("_item" if r.params else "") if lits else ("home" if r.path == "/" else "page"), taken)
    legacy_path = re.sub(r"\{([a-z0-9_]+)\}", r"{{params.\1}}", r.path)
    e = {"id": rid, "path": r.path, "method": "GET", "roles": [anon], "render": "faithful", "origin": "agent",
         "confidence": 0.9 if not r.query else 0.8,
         "legacy_request": {"method": "GET", "path": legacy_path}}
    if r.params:
        e["params"] = r.params
    if r.query:
        e["query"] = r.query
        e["legacy_request"]["query"] = {r.query_keys[k]: "{{query.%s}}" % k for k in r.query}
    return e


# A value of the action's query that belongs to one session or one page
# (phpBB's sid, OpenCart's login_token): kept server-side like a hidden
# input, so the form is recognised in every session.
SESSION_KEY = re.compile(r"(^|_)(sid|sess|session|sessionid|token|nonce|hash|key|csrf)$", re.I)
SESSION_VALUE = re.compile(r"^([0-9a-f]{16,}|[A-Za-z0-9_\-]{24,})$")


def _per_session(key: str, value: str) -> bool:
    return bool(re.match(r"^[A-Za-z_][A-Za-z0-9_]*$", key)) and bool(SESSION_KEY.search(key) or SESSION_VALUE.match(value))


def _form_entry(form, anon: str, taken: set, still_open: list):
    """None and a reason when the form cannot be mapped. Fields that must
    still accept characters an attack needs go to still_open."""
    u = urllib.parse.urlsplit(form.action)
    if any(i.type == "file" for i in form.inputs):
        return None, "file_upload"
    fields, legacy, names, opened = {}, {}, set(), []
    for i in form.visible():
        if i.name in legacy:
            continue
        name = cluster.ident(i.name, names)
        spec, why = fieldspec.spec(i)
        # a checkbox left unticked is not sent at all
        spec["required"] = bool(i.required) and i.type != "checkbox"
        fields[name], legacy[i.name] = spec, "{{fields.%s}}" % name
        if why:
            opened.append((name, why))
    if not fields:
        return None, "no_fields"
    for h in form.hidden():
        legacy.setdefault(h.name, "{{hidden.%s}}" % h.name)
    lits = [s for s in _segs(u.path)] or ["root"]
    e = {"id": cluster.ident("_".join(lits) + "_form", taken), "route": u.path or "/", "roles": [anon],
         "origin": "agent",
         "fields": fields, "legacy_request": {"method": "POST", "path": u.path or "/", "form": legacy},
         "rate_limit": {"per_ip": "10/min"}}
    if u.query:
        e["legacy_request"]["query"] = {k: "{{hidden.%s}}" % k if _per_session(k, v) else v
                                        for k, v in urllib.parse.parse_qsl(u.query)}
    still_open.extend({"kind": "input", "target": e["id"] + "." + n, "reason": why} for n, why in opened)
    return e, ""


def _widen_assets(doc: dict, assets: dict) -> list:
    """Adds to the asset prefixes already in the map the types of the files
    the site serves under them (a video under /site/templates/ when the
    prefix only listed images): the customer never edits types by hand.
    Returns [{"prefix", "types": the added ones, "widened": True}]."""
    added = {}
    entries = sorted(doc.get("assets", []), key=lambda a: len(a.get("prefix", "")), reverse=True)
    for path, ctype in sorted(assets.items()):
        a = next((a for a in entries if path.startswith(a.get("prefix", "\0"))), None)
        if a is None or ctype in a.get("types", []) or not assettypes.passive(ctype):
            continue  # only passive types: a prefix opens every file of a type it lists
        a.setdefault("types", []).append(ctype)
        added.setdefault(a["prefix"], []).append(ctype)
    return [{"prefix": p, "types": t, "widened": True} for p, t in sorted(added.items())]


# The gateway's default size limits (gateway/internal/server/asset*.go).
ASSET_MAX_KB, MEDIA_MAX_KB, SIZE_CEILING_KB = 10 << 10, 512 << 10, 4 << 20


MEDIA_EXT = (".mp4", ".m4v", ".mov", ".webm", ".ogv", ".mp3", ".m4a", ".aac", ".oga", ".ogg", ".opus", ".wav", ".flac")


def size_limit(a: dict, path: str, size: int) -> tuple:
    """(max_size_kb to set on asset prefix a so that the file at path fits,
    or 0 if it already does or never may; the limit in force). The gateway
    streams media files (by extension, under a prefix listing an audio or
    video type) up to 512 MiB by default, other files 10 MiB; max_size_kb,
    once set, is the limit of both, so a prefix with media never goes below
    the media default."""
    media_prefix = any(t.startswith(("audio/", "video/")) for t in a.get("types", []))
    media_file = media_prefix and path.lower().endswith(MEDIA_EXT)
    now = a.get("max_size_kb") or (MEDIA_MAX_KB if media_file else ASSET_MAX_KB)
    need = -(-size * 11 // 10240)  # ceil(size * 1.1 / 1024)
    if need <= now or need > SIZE_CEILING_KB:
        return 0, now
    if media_prefix:
        need = max(need, a.get("max_size_kb") or MEDIA_MAX_KB)
    return need, now


def _fit_sizes(doc: dict, sizes: dict) -> list:
    """Raises an asset prefix's max_size_kb to fit the largest file the site
    serves under it (with 10% room), up to 4 GiB. Returns the changes."""
    out = {}
    entries = sorted(doc.get("assets", []), key=lambda a: len(a.get("prefix", "")), reverse=True)
    for path, size in sizes.items():
        a = next((a for a in entries if path.startswith(a.get("prefix", "\0"))), None)
        if a is None:
            continue
        need, now = size_limit(a, path, size)
        if need:
            a["max_size_kb"] = need
            out[a["prefix"]] = need
    return [{"prefix": p, "max_size_kb": kb, "widened": True} for p, kb in sorted(out.items())]


def build(job: dict, crawl, fp) -> tuple:
    """Returns (document, report, samples): samples are the query values
    seen per (route id, key), to check what the AI suggests."""
    samples = {}
    base = job.get("current_map") or job.get("template")
    doc = copy.deepcopy(base) if base else skeleton(job["base_url"], job["reach"])
    continuous = bool(job.get("current_map"))
    _ensure_types(doc)
    anon = doc.get("roles", {}).get("anonymous") or "guest"
    pages = {pq: s for pq, s in crawl.pages.items() if s.status < 400 and s.ctype in ("text/html", "application/xhtml+xml", "")}
    paths = sorted({pq.split("?")[0] for pq in pages})
    before = sum(1 for p in paths if covered(doc, p))

    taken = {r["id"] for r in doc.get("routes", [])} | {f["id"] for f in doc.get("forms", [])}
    added_routes, added_forms, not_supported, still_open = [], [], [], []
    query_added = []
    for r in cluster.routes(pages, loose_top=bool(fp.cms)):
        if not MAP_PATH.match(r.path):  # Magento's base64 referers (…~/): left to the area
            not_supported.append({"path": r.path, "reason": "characters a map path cannot hold"})
            continue
        if any(covered(doc, p) for p in [r.path.replace("{", "x").replace("}", "")]) and not r.query:
            continue
        sample = [p for p in paths if matches(r.path, p)]
        if sample and all(covered(doc, p) for p in sample) and not r.query:
            continue
        # one route per shape: the gateway refuses two that differ only by
        # their parameters' names; the one already there wins, and gets the
        # query keys it lacked (the gateway drops undeclared ones silently)
        there = next((e for e in doc.get("routes", []) if _shape(e.get("path", "")) == _shape(r.path)
                      and e.get("method", "GET") == "GET"), None)
        if there is None and "{" not in r.path:
            there = gateway_route(doc, r.path)
        if there is not None:
            keys = _merge_query(there, r)
            if keys:
                query_added.append({"id": there.get("id"), "keys": keys})
            continue
        e = _route_entry(r, anon, taken)
        doc.setdefault("routes", []).append(e)
        for k, vs in r.query_samples.items():
            samples[(e["id"], k)] = vs
        for k, q in r.query.items():
            if q["type"] == "string":
                still_open.append({"kind": "input", "target": e["id"] + "." + k, "reason": "accepts_sql"})
        added_routes.append({"id": e["id"], "path": e["path"], "samples": r.samples, "logged_in_only": r.logged_in_only})
    have_forms = {f.get("route") for f in doc.get("forms", [])}
    for form in crawl.forms:
        if form.method != "POST":
            continue
        path = urllib.parse.urlsplit(form.action).path or "/"
        if path in have_forms or rules.excluded(path, fp.cms):
            continue
        if not MAP_PATH.match(path):
            have_forms.add(path)
            not_supported.append({"path": path, "reason": "characters a map path cannot hold"})
            continue
        e, why = _form_entry(form, anon, taken, still_open)
        have_forms.add(path)
        if e is None:
            not_supported.append({"path": path, "reason": why})
            continue
        doc.setdefault("forms", []).append(e)
        added_forms.append({"id": e["id"], "route": e["route"], "fields": sorted(e["fields"])})
    added_assets = _widen_assets(doc, crawl.assets)
    have_assets = [a.get("prefix", "") for a in doc.get("assets", [])]
    # An asset prefix takes everything under it that no route matches, POSTs
    # included: never one over pages, forms or script requests the study saw
    # (they stay with the site area, files included).
    # everything the study knows is not a file: pages, forms, script requests,
    # links it skipped or kept out, same-site URLs it did not get to open
    pages = {pq.split("?")[0] for pq in crawl.pages} | {urllib.parse.urlsplit(f.action).path for f in crawl.forms}
    pages |= {pq.split("?")[0] for pq in list(crawl.skipped_actions) + list(crawl.excluded) + list(crawl.script_requests)}
    pages |= {p for _, p in crawl.script_refused}
    pages |= {urllib.parse.urlsplit(u).path for u in crawl.urls} - set(crawl.assets)
    for prefix, types in cluster.asset_prefixes(crawl.assets).items():
        if any(prefix.startswith(h) for h in have_assets):
            continue
        if any(p.startswith(prefix) and p not in crawl.assets for p in pages):
            continue
        doc.setdefault("assets", []).append({"prefix": prefix, "types": types})
        added_assets.append({"prefix": prefix, "types": types})
    added_assets += _fit_sizes(doc, getattr(crawl, "sizes", {}))
    if not doc.get("forms"):
        doc.pop("forms", None)
    issues = idor.propose(doc)
    area = ensure_site_area(doc)

    after = sum(1 for p in paths if covered(doc, p))
    visitors = [u for u in job.get("unmapped", []) if u.get("method") == "GET" and "{" not in u.get("path", "")
                and not _is_dropped_key(doc, u.get("path", ""))
                and not covered(doc, u.get("path", ""))
                and not rules.excluded(u.get("path", ""), fp.cms)]
    stale = [r["path"] for r in (job.get("current_map") or {}).get("routes", []) if not any(matches(r["path"], p) for p in paths)]
    report = {
        **fp.as_report(_advisories()),
        "mode": "continuous" if continuous else "first",
        "pages_found": len(paths), "covered_before": before, "covered_after": after,
        "coverage": round(100 * after / len(paths)) if paths else 0,
        "routes_added": added_routes, "forms_added": added_forms, "assets_added": added_assets,
        "issues_proposed": issues,
        "site_area": area,
        "excluded": [{"path": p.split("?")[0], "reason": r} for p, r in sorted(crawl.excluded.items())][:200],
        "skipped_actions": len(crawl.skipped_actions), "not_supported": not_supported,
        "query_added": query_added,
        "query_dropped": _dropped_keys(doc, job.get("unmapped", []), crawl),
        "files_refused": _files_refused(doc, job.get("unmapped", []), crawl),
        "visitors_missing": [{"path": u["path"], "clients": u.get("clients", 0)} for u in visitors][:50],
        "not_seen_anymore": stale[:100], "login": crawl.login or None, "errors": crawl.errors,
        "accounts": [{"label": k, "login": v} for k, v in crawl.logins.items()],
        "pages": [pq for pq, s in crawl.pages.items()
                  if "anonymous" in s.seen_by and s.status < 400 and s.ctype == "text/html"][:300],
        "pages_by_account": {label: [pq for pq, s in crawl.pages.items()
                                     if label in s.seen_by and "anonymous" not in s.seen_by
                                     and s.status < 400 and s.ctype == "text/html"][:150]
                             for label in crawl.logins},
        "browser": {"pages": crawl.browser_pages, "script_requests": sorted(crawl.script_requests)[:200],
                    "kept_in": [{"method": m, "path": p} for m, p in sorted(crawl.script_refused)][:200]},
        "open": _still_open(crawl, still_open, not_supported),
        "base": "published map" if continuous else ("template" if job.get("template") else "empty map"),
    }
    return doc, report, samples


def _still_open(crawl, inputs: list, not_supported: list) -> list:
    """What the precise entries leave to the site area, and the fields that
    still accept characters an attack needs, each with its reason."""
    out = []
    for pq, reason in sorted(crawl.excluded.items()):
        out.append({"kind": "path", "target": pq.split("?")[0], "reason": "not_reached",
                    "detail": "never opened: " + ("administration" if reason == "admin" else "sensitive file")})
    for pq in sorted(crawl.skipped_actions):
        out.append({"kind": "path", "target": pq.split("?")[0], "reason": "action_not_performed"})
    for pq, s in sorted(crawl.pages.items()):
        if s.status >= 500:
            out.append({"kind": "path", "target": pq.split("?")[0], "reason": "legacy_error", "detail": "HTTP %d" % s.status})
    for pq in sorted(crawl.script_requests):
        seen = crawl.pages.get(pq)
        if seen and seen.status < 400 and seen.ctype not in ("text/html", "application/xhtml+xml", ""):
            out.append({"kind": "path", "target": pq.split("?")[0], "reason": "other",
                        "detail": "a script's %s request, served by the site area" % (seen.ctype or "data")})
    for m, p in sorted(crawl.script_refused):
        out.append({"kind": "path", "target": p, "reason": "action_not_performed",
                    "detail": "a script's %s, kept in by the read-only guard" % m})
    for n in not_supported:
        out.append({"kind": "path", "target": n["path"], "reason": "other", "detail": n["reason"]})
    out.extend(inputs)
    seen, dedup = set(), []
    for o in out:
        k = (o["kind"], o["target"], o["reason"])
        if k not in seen:
            seen.add(k)
            dedup.append(o)
    return dedup[:1000]


ALL_METHODS = ["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"]


def ensure_site_area(doc: dict) -> str:
    """The whole site is always served: whatever the precise entries do not
    cover goes through the site area, admin included. Returns "added" or
    "present"."""
    if any(a.get("prefix") == "/" for a in doc.get("areas", [])):
        return "present"
    ids = {a.get("id") for a in doc.get("areas", [])}
    doc.setdefault("areas", []).append({"id": "site" if "site" not in ids else "whole_site", "prefix": "/",
                                        "methods": ALL_METHODS, "origin": "agent"})
    return "added"


def _advisories():
    from .fingerprint import advisories
    return advisories()
