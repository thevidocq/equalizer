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
  route; a name the map can take on that route; a presentation key (a
  sort, an order, a page number, a filter, a language: PRESENTATION), since
  the agent opens the page with it and a GET may act (?add-to-cart=,
  ?doing_wp_cron=); and a page of the route (one that offers the key
  first), answering 200 as without it, shows other text with it. A key the site ignores (?test=1) changes nothing, a key
  that only redirects or errors (?author=1) is not a page option: both are
  left out.

Everything else (scanners, a single visitor, files that are not there, keys
that do nothing) stays out of the report: the gateway's logs keep every
refused request. Each item carries add, the panel's one-click change."""

import difflib
import re
import secrets
import urllib.parse

from . import assettypes, cluster, rules, verify, verifyassets
from .crawler import ASSET_EXT
from .fetch import ReachError
from .mapbuild import _is_dropped_key, _prefix_of, matches

MIN_VISITORS = 2  # distinct visitors (the gateway sends keyed hashes, never addresses)
MAX_FILES = 20  # files opened per discovery
MAX_KEYS = 10  # keys tried per discovery (on up to MAX_PAGES pages of the route each)
MAX_PAGES = 3  # pages of a route a key is tried on
MAX_SCAN = 6  # pages of a route read to find those that use the key
MAX_LISTED = 50

# Keys that change how a page presents what it shows, with values to try
# when no link of the site gives one. Only these are opened: a GET can act.
PRESENTATION = {
    "orderby": ["price", "date", "title", "popularity"], "order": ["desc", "asc"], "order_by": ["price", "date"],
    "sort": ["price", "date", "name"], "sortby": ["price", "date"], "sort_by": ["price", "date"],
    "dir": ["desc"], "direction": ["desc"], "page": ["2"], "paged": ["2"], "pg": ["2"],
    "per_page": ["5"], "perpage": ["5"], "limit": ["5"], "posts_per_page": ["5"], "product_count": ["5"],
    "view": ["list", "grid"], "display": ["list"], "layout": ["list"], "mode": ["list"],
    "lang": ["en", "it"], "language": ["en"], "locale": ["en"], "hl": ["en"], "currency": ["usd", "eur"],
    "q": ["test"], "s": ["test"], "search": ["test"], "query": ["test"], "keyword": ["test"], "keywords": ["test"],
    "category": ["1"], "cat": ["1"], "tag": ["1"], "year": ["2020"], "month": ["1"], "brand": ["1"],
    "color": ["red"], "colour": ["red"], "size": ["m"], "min_price": ["10"], "max_price": ["10"],
}
_FILTER = re.compile(r"^(filter_[a-z0-9_]+|pa_[a-z0-9_]+|min_[a-z_]+|max_[a-z_]+)$")

# A font's first bytes and the type the gateway knows it by (fontsig).
_FONTS = {b"wOFF": "font/woff", b"wOF2": "font/woff2", b"\x00\x01\x00\x00": "font/ttf", b"true": "font/ttf",
          b"OTTO": "font/otf"}


def presentation(key: str) -> bool:
    return key in PRESENTATION or bool(_FILTER.match(key))


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
        # a type the prefix lists may still be refused: the site sends it under another name
        if a is None or not assettypes.passive(kind, loaded=False):
            continue
        out.append({"path": p, "clients": u["clients"], "type": kind,
                    "add": {"prefix": a["prefix"], "type": kind, "prefix_types": list(a.get("types", []))}})
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
        if e is None or lr.get("method", "GET") != "GET" or not cluster.IDENT.match(key) or not presentation(key) or \
                key in (e.get("query") or {}) or key in (e.get("params") or {}) or key in (lr.get("query") or {}):
            continue
        out.append({"route": path, "route_id": e.get("id"), "key": key, "clients": u["clients"],
                    "add": {"route_id": e.get("id"), "key": key}})
    return sorted(out, key=lambda x: (-x["clients"], x["route"], x["key"]))


def _quoted(path: str) -> str:
    """The path as it goes on the wire: the gateway reports it decoded
    (spaces, accents)."""
    return urllib.parse.quote(path, safe="/%:@!$&'()*+,;=-._~")


def _served(f, path: str) -> str:
    """The type the site serves path as, "" when it does not serve it: HEAD
    first, the file itself when HEAD is not answered with the file (405,
    403 from a firewall) or for a font, judged by its first bytes as the
    gateway does (servers send fonts as application/octet-stream)."""
    path = _quoted(path)
    try:
        r = f.request("HEAD", path)
        font = r.content_type.startswith("font/") or "font" in r.content_type or path.lower().endswith(
            (".woff", ".woff2", ".ttf", ".otf"))
        if r.status != 200 or font:
            r = f.get(path)
    except (ReachError, OSError, ValueError):
        return ""
    if r.status != 200:
        return ""
    if r.body[:4] in _FONTS and (font or r.content_type in ("", "application/octet-stream")):
        return _FONTS[r.body[:4]]
    return r.content_type


def _kin(served: str, kind: str) -> bool:
    """Whether the site serves the file as what its name says: the same type,
    or another name of the same kind (image/vnd.microsoft.icon for .ico,
    audio/x-wav for .wav, font/ttf for application/x-font-ttf)."""
    font = lambda t: t.startswith("font/") or "font" in t
    return served == kind or (font(served) and font(kind)) or (
        served.split("/")[0] == kind.split("/")[0] in ("image", "audio", "video") and "xml" not in served)


def _pages_for(crawl, route: str) -> list:
    """Studied pages of the route, opened anonymously, shortest first: a
    pattern such as /{slug}/ covers the shop and the cart alike."""
    pages = [pq for pq, st in crawl.pages.items()
             if "anonymous" in st.seen_by and st.status == 200 and st.ctype == "text/html"
             and matches(route, pq.partition("?")[0])]
    return sorted(pages, key=lambda pq: ("?" in pq, len(pq), pq))[:MAX_SCAN]


def _uses(r, key: str) -> bool:
    """Whether a page offers key itself: a field (WooCommerce's <select
    name="orderby">) or a link with it."""
    text = r.body.decode("utf-8", "replace")
    return bool(re.search(r"""name=["']?%s["'\s>]|[?&;]%s=""" % (re.escape(key), re.escape(key)), text))


def _values(crawl, key: str) -> list:
    """Values the site's own links give key, else a plain one."""
    seen = []
    for pq in crawl.pages:
        for k, v in urllib.parse.parse_qsl(pq.partition("?")[2]):
            if k == key and v and v not in seen:
                seen.append(v)
    return seen[:2] or PRESENTATION.get(key, ["1"])[:2]


def _words(r) -> list:
    return verify._page(r.body, r.headers.get("content-type", "")).words if r.content_type == "text/html" else [r.body]


def _apart(a, b) -> float:
    """How far apart the visible text of two pages is: 0 the same."""
    wa, wb = _words(a), _words(b)
    if wa == wb:
        return 0.0
    return 1 - difflib.SequenceMatcher(None, wa, wb, autojunk=False).ratio()


def _page_ok(r) -> bool:
    return r.status == 200 and r.content_type == "text/html"


def _changes(f, page: str, a1, key: str, values: list, cms=None) -> bool:
    """Whether the page shows other text with key. Every answer must be the
    page itself (200, HTML): a redirect or an error is not a page option
    (?author=1 redirects to the author's page). The baseline is the page
    and the page with a key no site knows: a page cache serves the plain
    page and builds any page with a query, and the noise (dates, counters,
    rotating posts) is measured between the two. With no noise any other
    word is a change (a sort only reorders); with noise the change must be
    well beyond it."""
    sep = "&" if "?" in page else "?"
    try:
        a2 = f.get(page + sep + "eqprobe=" + secrets.token_hex(4))
        if not (_page_ok(a1) and _page_ok(a2)):
            return False
        noise = _apart(a1, a2)
        for v in values:
            url = page + sep + urllib.parse.urlencode({key: v})
            if rules.is_action(url) or rules.excluded(url, cms):
                return False
            b = f.get(url)
            if not _page_ok(b):
                continue
            d = _apart(a2, b)  # both built for a query: the cache does not stand between them
            if (noise == 0 and d > 0) or (noise > 0 and d > max(2 * noise, 1 - verify.TEXT_SIMILAR)):
                return True
    except (ReachError, OSError, ValueError):
        return False
    return False


def _key_changes(f, crawl, item: dict, cms=None) -> str:
    """The page of the route that changes with the key, or "": pages that
    offer the key themselves first, then the others, up to MAX_PAGES."""
    loaded = []
    for pq in _pages_for(crawl, item["route"]):
        try:
            loaded.append((pq, f.get(pq)))
        except (ReachError, OSError, ValueError):
            continue
    loaded.sort(key=lambda x: not _uses(x[1], item["key"]))
    values = _values(crawl, item["key"])
    for pq, a1 in loaded[:MAX_PAGES]:
        if _changes(f, pq, a1, item["key"], values, cms):
            return pq
    return ""


def confirm(report: dict, doc: dict, unmapped: list, crawl, f, cms=None) -> None:
    """Checks the candidates on the site and lists those that pass in
    report["files_refused"] and report["query_dropped"]. Opened anonymously:
    a test account's session is set aside meanwhile."""
    saved, f.cookies = f.cookies, {}
    try:
        listed, opened = [], 0
        for item in files(doc, unmapped, crawl):
            p = item["path"]
            if opened == MAX_FILES or len(listed) == MAX_LISTED:
                break
            if rules.excluded(p, cms) or verifyassets._acts(p):
                continue
            opened += 1
            served = _served(f, p)
            # what the gateway compares is the type the site sends: that is what a click adds
            types = item["add"].pop("prefix_types")
            if not _kin(served, item["type"]) or not assettypes.passive(served, loaded=False) or served in types:
                continue
            item["type"] = item["add"]["type"] = served
            item["why"] = "%d visitors asked for it; the site serves it as %s, which %s does not list" % (
                item["clients"], served, item["add"]["prefix"])
            listed.append(item)
        report["files_refused"] = listed
        listed, tried = [], 0
        for item in keys(doc, unmapped):
            if tried == MAX_KEYS or len(listed) == MAX_LISTED:
                break
            if not _pages_for(crawl, item["route"]):
                continue
            tried += 1
            page = _key_changes(f, crawl, item, cms)
            if not page:
                continue
            item["page"] = page
            item["why"] = "%d visitors used it; %s shows other content with it, and the map drops it" % (
                item["clients"], page)
            listed.append(item)
        report["query_dropped"] = listed
    finally:
        f.cookies = saved
