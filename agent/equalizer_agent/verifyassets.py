"""Files the pages load, checked through the gateway like the pages
(docs/agent.md, step 4).

A page can look the same through Equalizer while its fonts, images or
videos are refused: the text is there, the page is broken. Every file the
studied pages name (in their HTML and in their stylesheets) that an asset
prefix of the version serves is opened directly and asked through the
gateway (preview, HEAD: the verdict without the body). A file the site
serves and the gateway refuses is broken, and what would mend it is kept
for the correction: a passive type the site sends (assettypes), or the
size it needs. Nothing else is ever widened, and a prefix is never removed:
a page an asset prefix refuses stays broken, for a person to decide."""

import urllib.parse

from . import assettypes, crawler, css, extract, mapbuild, rules, webapps
from .fetch import ReachError
from .gateway import TryError

MAX_FILES = 200
MAX_SHEETS = 30


def names(body: bytes, url: str) -> list:
    """(URL, loaded) of the files an HTML page names: loaded for what the
    page loads (src, stylesheets), not for what it only links to."""
    try:
        page = extract.parse(body.decode("utf-8", "replace"), url)
    except Exception:  # a broken page names nothing we can read
        return []
    return [(u, True) for u in page.assets + page.scripts] + [(u, False) for u in page.links]


def _acts(path: str) -> bool:
    """Whether opening path could act on the site. A file (by its name) is
    judged by its folders, and only against the strong words (logout,
    delete, unsubscribe…): reset.css, /lib/dropzone/…js, /assets/update/x.png
    are files; /account/logout/bye.png is not opened."""
    if crawler.Crawler._static(path):
        return bool(webapps.STRONG.search(path.rsplit("/", 1)[0] + "/")) or webapps.ends_session(path)
    return rules.is_action(path)


def _prefix(doc: dict, path: str):
    best = None
    for a in doc.get("assets", []):
        p = a.get("prefix", "\0")
        if path.startswith(p) and (best is None or len(p) > len(best["prefix"])):
            best = a
    return best


def check(gw, f, doc: dict, m, files, cms=None) -> None:
    """Checks the files named in files ((url, loaded) pairs); adds to m.broken
    and m.asset_fixes."""
    todo, seen = [], {}
    for u, loaded in files:
        if not f.same_site(u):
            continue
        path = urllib.parse.urlsplit(u).path or "/"
        if path not in seen:
            todo.append(path)
        seen[path] = seen.get(path, False) or loaded
    already = {b.get("path") for b in m.broken}
    sheets, i = 0, 0
    while i < len(todo) and i < MAX_FILES:
        path = todo[i]
        i += 1
        a = _prefix(doc, path)
        # the crawler's rules hold here too: never an action, never a sensitive file
        # (a file's name such as reset.css or delete-icon.svg is not an action, as in the crawl)
        if a is None or path in already or rules.excluded(path, cms) or _acts(path):
            continue
        try:
            direct = f.get(path)
        except (ReachError, OSError, ValueError):
            continue
        if direct.status != 200 or "html" in direct.content_type:
            continue  # not served, or a page: pages are compared by verify, not here
        if direct.content_type == "text/css" and sheets < MAX_SHEETS:
            sheets += 1
            for ref in css.refs(direct.body.decode("utf-8", "replace"), direct.url):
                if f.same_site(ref):
                    p = urllib.parse.urlsplit(ref).path or "/"
                    if p not in seen:
                        todo.append(p)
                    seen[p] = True
        try:
            via = gw.preview(path, method="HEAD")
        except TryError:
            return
        ref = via.get("refusal")
        if not ref:
            continue
        reason = ref.get("reason", "")
        m.broken_files.add(path)
        m.broken.append({"path": path, "entry": via.get("entry"), "difference": "refused",
                         "detail": "%s (%s directly)" % (reason, direct.content_type or "no type")})
        already.add(path)
        fix = m.asset_fixes.setdefault(a["prefix"], {"types": [], "max_size_kb": 0})
        kind = direct.content_type
        if reason == "asset_type_not_allowed" and assettypes.passive(kind, loaded=seen[path]):
            if kind not in fix["types"]:
                fix["types"].append(kind)
        elif reason == "asset_too_large":
            size = direct.headers.get("content-length", "")
            if size.isdigit():
                need, _ = mapbuild.size_limit(a, path, int(size))
                fix["max_size_kb"] = max(fix["max_size_kb"], need)


def widen(doc: dict, m) -> list:
    """Applies m.asset_fixes to doc; the corrections."""
    out = []
    for prefix, fix in sorted(m.asset_fixes.items()):
        a = next((x for x in doc.get("assets", []) if x.get("prefix") == prefix), None)
        if a is None:
            continue
        added = [t for t in fix["types"] if t not in a.get("types", [])]
        if added:
            a.setdefault("types", []).extend(added)
            out.append({"entry": "asset:" + prefix, "change": "widened", "detail": "+ " + ", ".join(added),
                        "reason": "files the pages load under %s were refused through Equalizer for their type" % prefix})
        if fix["max_size_kb"] > (a.get("max_size_kb") or 0):
            a["max_size_kb"] = fix["max_size_kb"]
            out.append({"entry": "asset:" + prefix, "change": "widened",
                        "detail": "max_size_kb %d" % fix["max_size_kb"],
                        "reason": "a file under %s was refused through Equalizer for its size" % prefix})
    return out
