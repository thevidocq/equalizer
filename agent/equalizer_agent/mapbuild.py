"""The proposed map: the published map (continuous discovery) or the CMS
template as a base, plus what the crawl found and the base does not cover.
Nothing is removed from a published map: that stays a human decision."""

import copy
import re
import urllib.parse

from . import cluster, fields as fieldspec, idor, rules

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
    if any(fits(doc, r, path) for r in doc.get("routes", [])):
        return True
    return any(path.startswith(a.get("prefix", "\0")) for a in doc.get("assets", []))


def _ensure_types(doc: dict):
    t = doc.setdefault("types", {})
    t.setdefault("slug", {"base": "string", "pattern": "^[A-Za-z0-9][A-Za-z0-9_-]{0,119}$", "deny": COMMON_DENY})
    t.setdefault("id", {"base": "int", "min": 0, "max": 999999999})


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
        # their parameters' names; the one already there wins
        if any(_shape(e.get("path", "")) == _shape(r.path) or ("{" not in r.path and fits(doc, e, r.path))
               for e in doc.get("routes", [])):
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
    have_assets = [a.get("prefix", "") for a in doc.get("assets", [])]
    added_assets = []
    for prefix, types in cluster.asset_prefixes(crawl.assets).items():
        if any(prefix.startswith(h) for h in have_assets):
            continue
        doc.setdefault("assets", []).append({"prefix": prefix, "types": types})
        added_assets.append({"prefix": prefix, "types": types})
    if not doc.get("forms"):
        doc.pop("forms", None)
    issues = idor.propose(doc)
    area = ensure_site_area(doc)

    after = sum(1 for p in paths if covered(doc, p))
    visitors = [u for u in job.get("unmapped", []) if u.get("method") == "GET" and not covered(doc, u.get("path", ""))
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
