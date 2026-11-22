"""What visitors asked for and the map refuses, turned into actions a person
can take with one click (docs/agent.md, "Visitor requests").

A visitor's request is never evidence: anyone can ask for any address. So
the agent never widens the map from one, and the report lists one only when
there is something to do about it, after checking it on the site itself:

- a file: asked by at least MIN_VISITORS distinct visitors; under an asset
  prefix of the map; of a passive type (assettypes) the prefix does not
  list yet; not named by the site's own pages (those the agent studies and
  mends on its own); and served by the site, as that type, when the agent
  opens it. A scanner asks for files that do not exist, or for scripts,
  pages and configuration files: none of them passes.
- a query key: asked by at least MIN_VISITORS distinct visitors on a GET
  route; a name the map can take on that route; and the page changes when
  the agent opens it with the key. A key the site ignores (?test=1) changes
  nothing and is left out.

Everything else (scanners, a single visitor, files that are not there, keys
that do nothing) stays out of the report: the gateway's logs keep every
refused request. Each item carries add, the panel's one-click change."""

import difflib
import urllib.parse

from . import assettypes, cluster, rules, verify, verifyassets
from .crawler import ASSET_EXT
from .fetch import ReachError
from .mapbuild import _is_dropped_key, _prefix_of, matches

MIN_VISITORS = 2  # distinct visitors (the gateway sends keyed hashes, never addresses)
MAX_FILES = 20  # files opened per discovery
MAX_KEYS = 10  # keys tried per discovery (three page loads each)
MAX_LISTED = 50


def files(doc: dict, unmapped: list, crawl) -> list:
    """The candidate files (not yet checked on the site), most visitors first."""
    named = set(crawl.assets) | {urllib.parse.urlsplit(u).path for u in crawl.urls}
    out = []
    for u in unmapped:
        p = u.get("path", "")
        if u.get("method") != "GET" or u.get("clients", 0) < MIN_VISITORS or "?" in p or "{" in p or p in named:
            continue
        a = _prefix_of(doc, p)
        name = p.rsplit("/", 1)[-1]
        kind = ASSET_EXT.get("." + name.rsplit(".", 1)[-1].lower(), "") if "." in name else ""
        if a is None or not assettypes.passive(kind, loaded=False) or kind in a.get("types", []):
            continue
        out.append({"path": p, "clients": u["clients"], "type": kind,
                    "add": {"prefix": a["prefix"], "type": kind}})
    return sorted(out, key=lambda x: (-x["clients"], x["path"]))


def keys(doc: dict, unmapped: list) -> list:
    """The candidate query keys (not yet tried on the site), most visitors first."""
    out = []
    for u in unmapped:
        if u.get("method") != "GET" or u.get("clients", 0) < MIN_VISITORS or not _is_dropped_key(doc, u.get("path", "")):
            continue
        path, _, q = u["path"].partition("?")
        key = q[:-1]
        e = next((r for r in doc.get("routes", []) if r.get("path") == path and r.get("method", "GET") == "GET"), None)
        lr = (e or {}).get("legacy_request") or {}
        if e is None or lr.get("method", "GET") != "GET" or not cluster.IDENT.match(key) or \
                key in (e.get("query") or {}) or key in (e.get("params") or {}) or key in (lr.get("query") or {}):
            continue
        out.append({"route": path, "route_id": e.get("id"), "key": key, "clients": u["clients"],
                    "add": {"route_id": e.get("id"), "key": key}})
    return sorted(out, key=lambda x: (-x["clients"], x["route"], x["key"]))


def _serves(f, path: str, kind: str) -> bool:
    """Whether the site serves path as a file of type kind."""
    try:
        r = f.request("HEAD", path)
        if r.status in (405, 501):  # no HEAD on this server: the file itself
            r = f.get(path)
    except (ReachError, OSError, ValueError):
        return False
    return r.status == 200 and _same_type(r.content_type, kind)


def _same_type(served: str, kind: str) -> bool:
    if served == kind:
        return True
    # a font or an image under another name of the same kind (font/woff, application/font-woff)
    top = kind.split("/")[0]
    return top in ("image", "font", "audio", "video") and served.split("/")[0] == top and "xml" not in served


def _page_for(crawl, route: str) -> str:
    """A studied page of the route, opened anonymously: the shortest."""
    pages = [pq for pq, s in crawl.pages.items()
             if "anonymous" in s.seen_by and s.status == 200 and s.ctype == "text/html"
             and matches(route, pq.partition("?")[0])]
    return min(pages, key=lambda pq: ("?" in pq, len(pq), pq), default="")


def _values(crawl, key: str) -> list:
    """Values the site's own links give key, else a plain one."""
    seen = []
    for pq in crawl.pages:
        for k, v in urllib.parse.parse_qsl(pq.partition("?")[2]):
            if k == key and v and v not in seen:
                seen.append(v)
    return seen[:2] or ["1"]


def _words(r) -> list:
    return verify._page(r.body, r.headers.get("content-type", "")).words if r.content_type == "text/html" else [r.body]


def _differs(a, b) -> float:
    """How far apart two answers are: 0 the same, 1 nothing in common."""
    if a.status != b.status or (300 <= a.status < 400 and a.location != b.location):
        return 1.0
    wa, wb = _words(a), _words(b)
    if wa == wb:
        return 0.0
    return 1 - difflib.SequenceMatcher(None, wa, wb, autojunk=False).ratio()


def _changes(f, page: str, key: str, values: list) -> bool:
    """Whether the page changes with key: further from itself than two loads
    of the page are from each other (dates, counters, tokens)."""
    try:
        a1, a2 = f.get(page), f.get(page)
        noise = _differs(a1, a2)
        if noise >= 1.0:
            return False  # the page itself is not stable: nothing can be told
        for v in values:
            sep = "&" if "?" in page else "?"
            url = page + sep + urllib.parse.urlencode({key: v})
            if rules.is_action(url):
                return False
            if _differs(a1, f.get(url)) > max(noise * 2, 1 - verify.TEXT_SIMILAR):
                return True
    except (ReachError, OSError, ValueError):
        return False
    return False


def confirm(report: dict, doc: dict, unmapped: list, crawl, f, cms=None) -> None:
    """Checks the candidates on the site and lists those that pass in
    report["files_refused"] and report["query_dropped"]. Opened anonymously:
    a test account's session is set aside meanwhile."""
    saved, f.cookies = f.cookies, {}
    try:
        listed = []
        for item in files(doc, unmapped, crawl)[:MAX_FILES]:
            p = item["path"]
            if rules.excluded(p, cms) or verifyassets._acts(p) or not _serves(f, p, item["type"]):
                continue
            item["why"] = "%d visitors asked for it; the site serves it as %s, which %s does not list" % (
                item["clients"], item["type"], item["add"]["prefix"])
            listed.append(item)
        report["files_refused"] = listed[:MAX_LISTED]
        listed = []
        for item in keys(doc, unmapped)[:MAX_KEYS]:
            page = _page_for(crawl, item["route"])
            if not page or not _changes(f, page, item["key"], _values(crawl, item["key"])):
                continue
            item["page"] = page
            item["why"] = "%d visitors used it; %s changes with it, and the map drops it" % (item["clients"], page)
            listed.append(item)
        report["query_dropped"] = listed[:MAX_LISTED]
    finally:
        f.cookies = saved
