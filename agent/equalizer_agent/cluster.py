"""From URLs to routes: segments that vary become typed parameters, query
values give their types. Deterministic: same pages, same routes."""

import re
import urllib.parse
from dataclasses import dataclass, field

SLUG = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_\-]{0,119}$")
IDENT = re.compile(r"^[a-z][a-z0-9_]{0,63}$")
DATE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
CODE = re.compile(r"^[a-z][a-z0-9_\-]{0,30}$")


@dataclass
class Route:
    path: str  # public = legacy path, with {param}
    params: dict = field(default_factory=dict)  # name -> spec
    query: dict = field(default_factory=dict)  # public name -> spec
    query_keys: dict = field(default_factory=dict)  # public name -> legacy key
    query_samples: dict = field(default_factory=dict)  # public name -> observed values
    samples: int = 0
    logged_in_only: bool = False


def ident(raw: str, taken: set) -> str:
    base = re.sub(r"[^a-z0-9_]+", "_", raw.lower()).strip("_") or "x"
    if not base[0].isalpha():
        base = "p_" + base
    base = base[:56]
    name, n = base, 2
    while name in taken:
        name, n = f"{base}_{n}", n + 1
    taken.add(name)
    return name


def _is_value(v: str) -> bool:
    return v.isdigit() or bool(SLUG.match(v))


def patterns(paths: list, loose_top: bool = False) -> dict:
    """Groups concrete paths into patterns: {pattern: [paths]}. Top-level
    pages (/about, /contact) become /{slug} only for a known CMS, whose
    template refuses admin names; otherwise they stay one route each."""
    split = {p: p.strip("/").split("/") if p.strip("/") else [] for p in set(paths)}
    groups = {}
    for p, segs in split.items():
        groups.setdefault((len(segs), p.endswith("/") and p != "/"), []).append(p)
    out = {}
    for (n, slash), members in groups.items():
        rows = {p: list(split[p]) for p in members}
        for i in reversed(range(n)):
            buckets = {}
            for p, segs in rows.items():
                key = tuple(s if j != i else None for j, s in enumerate(segs))
                buckets.setdefault(key, []).append(p)
            for key, ps in buckets.items():
                vals = {rows[p][i] for p in ps}
                numeric = all(v.isdigit() for v in vals)
                wide = len(vals) >= 3 and all(_is_value(v) for v in vals) and (i > 0 or loose_top)
                if (numeric and len(vals) >= 2) or wide:
                    kind = "{int}" if numeric else "{slug}"
                    for p in ps:
                        rows[p][i] = kind
        for p, segs in rows.items():
            pat = "/" + "/".join(segs) + ("/" if slash else "")
            out.setdefault(pat, []).append(p)
    return out


def infer(values: list) -> dict:
    """The narrowest type every observed value fits."""
    vals = [v for v in values if v != ""]
    if not vals:
        return {"type": "string", "max_length": 50, "required": False}
    if all(v.isdigit() for v in vals) and all(len(v) < 10 for v in vals):
        return {"type": "int", "min": 0, "max": 999999999, "required": False}
    if all(DATE.match(v) for v in vals):
        return {"type": "date", "required": False}
    distinct = sorted(set(vals))
    if len(vals) >= 5 and len(distinct) <= 8 and all(CODE.match(v) for v in distinct):
        return {"type": "enum", "values": distinct, "required": False}
    longest = max(len(v) for v in vals)
    # no markup or template code in a value seen in links; SQL delimiters
    # may be part of a search and stay allowed (reported as still open)
    return {"type": "string", "max_length": min(200, max(50, longest * 2)), "required": False,
            "rules": ["no_html", "no_template_meta"]}


def routes(pages: dict, loose_top: bool = False) -> list:
    """pages: path?query -> Seen. Only pages that answered 200 or redirected."""
    good = {pq: s for pq, s in pages.items() if s.status < 400}
    by_path = {}
    for pq, s in good.items():
        path, _, q = pq.partition("?")
        by_path.setdefault(path, []).append((urllib.parse.parse_qsl(q, keep_blank_values=True), s))
    out = []
    for pat, members in sorted(patterns(list(by_path), loose_top).items()):
        taken = set()
        segs = pat.strip("/").split("/") if pat.strip("/") else []
        params, named = {}, []
        for i, sgm in enumerate(segs):
            if sgm in ("{int}", "{slug}"):
                prev = segs[i - 1] if i and segs[i - 1] not in ("{int}", "{slug}") else ""
                stem = re.sub(r"s$", "", re.sub(r"[^a-z0-9]+", "_", prev.lower()).strip("_"))
                name = ident((stem + "_" if stem else "") + ("id" if sgm == "{int}" else "slug"), taken)
                params[name] = {"type": "id" if sgm == "{int}" else "slug"}
                named.append("{" + name + "}")
            else:
                named.append(sgm)
        path = "/" + "/".join(named) + ("/" if pat.endswith("/") and pat != "/" else "")
        r = Route(path=path, params=params)
        qvals = {}
        seen = []
        for m in members:
            for qs, s in by_path[m]:
                seen.append(s)
                for k, v in qs:
                    qvals.setdefault(k, []).append(v)
        for k in sorted(qvals):
            name = k if IDENT.match(k) and k not in params else ident(k, taken | set(params))
            r.query[name], r.query_keys[name] = infer(qvals[k]), k
            r.query_samples[name] = qvals[k][:50]
        r.samples = len(seen)
        r.logged_in_only = all(s.logged_in for s in seen)
        out.append(r)
    return out


def asset_prefixes(assets: dict) -> dict:
    """path -> type, grouped by their first two folders: {prefix: types}."""
    out = {}
    for path, ctype in assets.items():
        parts = path.split("/")[1:-1]
        if not parts:
            continue
        prefix = "/" + "/".join(parts[:2]) + "/"
        if not re.match(r"^/([A-Za-z0-9_.\-]+/)*$", prefix) or "/../" in prefix:
            continue
        out.setdefault(prefix, set()).add(ctype)
    return {p: sorted(t) for p, t in sorted(out.items())}
