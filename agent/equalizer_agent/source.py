"""What the site's source code adds to a discovery.

The source reader sends a summary of the code (schema/source/summary.
schema.json): entry points, the type the code gives each request key, the
calls where a request value may do harm, components. Never code, never
values. The discovery uses it for:

- types: a query key or form field the agent added as free text, whose
  name the code always reads as an integer, an e-mail or one of a few
  values (in_array), gets that type instead. A key read raw anywhere, or
  with two different types, is left as it is: the code accepts anything
  there. The values seen on the site must fit, or nothing changes. Only
  entries the agent writes are touched; a person's never are.
- the report: entry points the exploration never opened (REST routes,
  AJAX actions, PHP files that run when opened), the calls where a request
  value may do harm, the components. Nothing found only in the code is
  opened: a file no link points to may act when opened, and the
  administration paths stay closed to the exploration.
"""

import re

from .fields import INT_LIMIT

NARROWING = ("int", "email", "enum")
_INT = re.compile(r"^-?[0-9]{1,9}$")
_EMAIL = re.compile(r"^[^\s@<>\"'`]{1,64}@[^\s@<>\"'`]{1,190}$")
_IDENT = re.compile(r"^[A-Za-z_][A-Za-z0-9_\-\[\]]{0,63}$")


def _code_types(summary: dict) -> dict:
    """{(name, "query"|"body"): (type, values, where, route)} for names the
    code reads with one narrowing type everywhere; route is the path the
    reading belongs to, or None when it serves any."""
    seen = {}
    for i in summary.get("inputs") or []:
        name = i.get("name") or ""
        if not _IDENT.match(name):
            continue
        sources = {"query": ["query"], "body": ["body"], "request": ["query", "body"]}.get(i.get("source"), [])
        for src in sources:
            seen.setdefault((name, src), []).append(i)
    out = {}
    for key, items in seen.items():
        kinds = {(i.get("type"), tuple(i.get("values") or [])) for i in items}
        if len(kinds) != 1:
            continue
        kind, values = next(iter(kinds))
        if kind not in NARROWING or (kind == "enum" and not values):
            continue
        routes = {i.get("route") for i in items}
        w = items[0].get("where") or {}
        out[key] = (kind, list(values), "%s:%s" % (w.get("file", "?"), w.get("line", "?")),
                    next(iter(routes)) if len(routes) == 1 else None)
    return out


def _spec(kind: str, values: list):
    if kind == "int":
        return {"type": "int", "min": -INT_LIMIT, "max": INT_LIMIT}
    if kind == "email":
        return {"type": "email"}
    vals = sorted({v for v in values if isinstance(v, str) and v != "" and len(v) <= 100})[:100]
    return {"type": "enum", "values": vals} if vals else None


def _fits(kind: str, values: list, seen: list) -> bool:
    for v in seen:
        if v == "":
            continue
        if kind == "int" and not _INT.match(v) or kind == "email" and not _EMAIL.match(v) \
                or kind == "enum" and v not in values:
            return False
    return True


def _narrowable(spec: dict) -> bool:
    return spec.get("type") == "string" and not spec.get("pattern")


def apply(report: dict, doc: dict, job: dict, samples: dict, opened: set) -> None:
    """Narrows the agent's free-text keys and fields to the code's types
    and writes report["source"]; opened are the paths the exploration
    opened."""
    status = job.get("source_status")
    if not status:
        return
    out = {"status": status, "error": job.get("source_error")}
    summary = job.get("source") if status == "used" else None
    if not summary:
        report["source"] = out
        return
    out.update(stack=summary.get("stack", "unknown"), files=(summary.get("files") or {}).get("read", 0),
               routes=len(summary.get("routes") or []), typed=[])
    types = _code_types(summary)

    def code_type(name, src, path):
        t = types.get((name, src))
        if t is None or t[3] not in (None, path):
            return None
        return t

    for e in doc.get("routes", []):
        if e.get("origin") != "agent":
            continue
        lr = e.get("legacy_request") or {}
        for legacy, tmpl in (lr.get("query") or {}).items():
            m = re.fullmatch(r"\{\{query\.([A-Za-z0-9_]+)\}\}", str(tmpl))
            q = (e.get("query") or {}).get(m.group(1)) if m else None
            t = code_type(legacy, "query", lr.get("path"))
            if q is None or t is None or not _narrowable(q):
                continue
            new = _spec(t[0], t[1])
            if new is None or not _fits(t[0], t[1], samples.get((e["id"], m.group(1)), [])):
                continue
            new.update({k: q[k] for k in ("required",) if k in q})
            e["query"][m.group(1)] = new
            out["typed"].append({"route": e.get("path", ""), "field": legacy, "type": t[0], "where": t[2]})
            _close(report, e["id"] + "." + m.group(1))
    for f in doc.get("forms", []):
        if f.get("origin") != "agent":
            continue
        lr = f.get("legacy_request") or {}
        for legacy, tmpl in (lr.get("form") or {}).items():
            m = re.fullmatch(r"\{\{fields\.([A-Za-z0-9_]+)\}\}", str(tmpl))
            spec = (f.get("fields") or {}).get(m.group(1)) if m else None
            t = code_type(legacy, "body", lr.get("path"))
            if spec is None or t is None or not _narrowable(spec):
                continue
            new = _spec(t[0], t[1])
            if new is None:
                continue
            new["required"] = spec.get("required", False)
            f["fields"][m.group(1)] = new
            out["typed"].append({"route": f.get("route", ""), "field": legacy, "type": t[0], "where": t[2]})
            _close(report, f["id"] + "." + m.group(1))
    out["typed"] = out["typed"][:300]
    out["risks"] = [{"kind": r.get("kind"), "input": r.get("input"), "route": r.get("route"),
                     "where": "%s:%s" % ((r.get("where") or {}).get("file", "?"), (r.get("where") or {}).get("line", "?"))}
                    for r in (summary.get("risks") or [])][:100]
    out["components"] = [{"name": c.get("name"), "kind": c.get("kind"), "version": c.get("version")}
                         for c in (summary.get("components") or [])][:200]
    out["unseen"] = _unseen(summary, opened)
    report["source"] = out


def _unseen(summary: dict, opened: set) -> list:
    out, seen = [], set()
    for r in summary.get("routes") or []:
        p = r.get("path") or ""
        key = (p, r.get("origin"), r.get("action"))
        if key in seen or "{" not in p and p in opened and not r.get("action"):
            continue
        seen.add(key)
        out.append({"path": p, "origin": r.get("origin"), "action": r.get("action"), "public": bool(r.get("public"))})
    return out[:200]


def _close(report: dict, target: str) -> None:
    """A field the code types no longer accepts what an attack needs."""
    report["open"] = [o for o in report.get("open", []) if not (o.get("kind") == "input" and o.get("target") == target)]
