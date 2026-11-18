"""The proposed map: the published map (continuous discovery) or the CMS
template as a base, plus what the crawl found and the base does not cover.
Nothing is removed from a published map: that stays a human decision."""

import copy
import re
import urllib.parse

from . import cluster, rules

COMMON_DENY = ["admin", "administrator", "wp-admin", "wp-login", "phpmyadmin", "pma", "cpanel", "plesk", "webmail",
               "server-status", "server-info", "cgi-bin", "install", "setup", "backup", "backups", "config",
               "console", "manager", "jsonapi", "xmlrpc"]


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


def covered(doc: dict, path: str) -> bool:
    if any(matches(r.get("path", ""), path) for r in doc.get("routes", [])):
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
         "confidence": 0.9 if not r.query else 0.8, "risk": "medium" if any(q["type"] == "string" for q in r.query.values()) else "low",
         "legacy_request": {"method": "GET", "path": legacy_path}}
    if r.params:
        e["params"] = r.params
    if r.query:
        e["query"] = r.query
        e["legacy_request"]["query"] = {r.query_keys[k]: "{{query.%s}}" % k for k in r.query}
    return e


FIELD = {"email": {"type": "email"}, "number": {"type": "int", "min": -999999999, "max": 999999999},
         "password": {"type": "password", "max_length": 128, "rules": ["no_template_meta"]},
         "textarea": {"type": "text", "max_length": 5000, "rules": ["no_html", "no_template_meta"]},
         "date": {"type": "date"}}


def _form_entry(form, anon: str, taken: set):
    """None and a reason when the form cannot be mapped."""
    u = urllib.parse.urlsplit(form.action)
    if any(i.type == "file" for i in form.inputs):
        return None, "file_upload"
    fields, legacy, names = {}, {}, set()
    for i in form.visible():
        if i.name in legacy:
            continue
        name = cluster.ident(i.name, names)
        if i.type == "select":
            opts = [o for o in i.options if o and len(o) <= 100][:50]
            spec = {"type": "enum", "values": sorted(set(opts))} if opts else {"type": "string", "max_length": 100}
        elif i.type in FIELD:
            spec = dict(FIELD[i.type])
        else:
            spec = {"type": "string", "max_length": min(i.maxlength or 200, 1000)}
        spec["required"] = bool(i.required)
        fields[name], legacy[i.name] = spec, "{{fields.%s}}" % name
    if not fields:
        return None, "no_fields"
    for h in form.hidden():
        legacy.setdefault(h.name, "{{hidden.%s}}" % h.name)
    lits = [s for s in _segs(u.path)] or ["root"]
    e = {"id": cluster.ident("_".join(lits) + "_form", taken), "route": u.path or "/", "roles": [anon],
         "origin": "agent", "risk": "high" if any(f["type"] == "password" for f in fields.values()) else "medium",
         "fields": fields, "legacy_request": {"method": "POST", "path": u.path or "/", "form": legacy},
         "rate_limit": {"per_ip": "10/min"}}
    if u.query:
        e["legacy_request"]["query"] = dict(urllib.parse.parse_qsl(u.query))
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
    added_routes, added_forms, not_supported = [], [], []
    for r in cluster.routes(pages, loose_top=bool(fp.cms)):
        if any(covered(doc, p) for p in [r.path.replace("{", "x").replace("}", "")]) and not r.query:
            continue
        sample = [p for p in paths if matches(r.path, p)]
        if sample and all(covered(doc, p) for p in sample) and not r.query:
            continue
        if any(matches(e.get("path", ""), r.path) for e in doc.get("routes", [])):
            continue
        e = _route_entry(r, anon, taken)
        if r.logged_in_only:
            e["risk"] = "medium"
        doc.setdefault("routes", []).append(e)
        for k, vs in r.query_samples.items():
            samples[(e["id"], k)] = vs
        added_routes.append({"id": e["id"], "path": e["path"], "samples": r.samples, "logged_in_only": r.logged_in_only})
    have_forms = {f.get("route") for f in doc.get("forms", [])}
    for form in crawl.forms:
        if form.method != "POST":
            continue
        path = urllib.parse.urlsplit(form.action).path or "/"
        if path in have_forms or rules.excluded(path, fp.cms):
            continue
        e, why = _form_entry(form, anon, taken)
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
        "excluded": [{"path": p.split("?")[0], "reason": r} for p, r in sorted(crawl.excluded.items())][:200],
        "skipped_actions": len(crawl.skipped_actions), "not_supported": not_supported,
        "visitors_missing": [{"path": u["path"], "clients": u.get("clients", 0)} for u in visitors][:50],
        "not_seen_anymore": stale[:100], "login": crawl.login or None, "errors": crawl.errors,
        "base": "published map" if continuous else ("template" if job.get("template") else "empty map"),
    }
    return doc, report, samples


def _advisories():
    from .fingerprint import advisories
    return advisories()
