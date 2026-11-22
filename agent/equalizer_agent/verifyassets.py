"""Files the pages load, checked through the gateway like the pages
(docs/agent.md, step 4).

A page can look the same through Equalizer while its fonts, images or
videos are refused: the text is there, the page is broken. Every file the
studied pages name (in their HTML and in their stylesheets) that an asset
prefix of the version serves is opened directly and asked through the
gateway (preview, HEAD: the verdict without the body). A file the site
serves and the gateway refuses is a broken page, and what would mend it
(the type the site sends, the size it needs) is kept for the correction."""

import urllib.parse

from . import css, extract
from .fetch import ReachError
from .gateway import TryError

MAX_FILES = 200
MAX_SHEETS = 30
SIZE_CEILING_KB = 4 << 20  # 4 GiB, as discovery
GENERIC = {"", "application/octet-stream", "binary/octet-stream", "application/unknown", "text/plain", "text/html"}


def names(body: bytes, url: str) -> list:
    """URLs of the files an HTML page names, and of its links (a document
    under an asset prefix is a file too: check() keeps those of a prefix)."""
    try:
        page = extract.parse(body.decode("utf-8", "replace"), url)
        return page.assets + page.links
    except Exception:  # a broken page names nothing we can read
        return []


def _prefix(doc: dict, path: str):
    best = None
    for a in doc.get("assets", []):
        p = a.get("prefix", "\0")
        if path.startswith(p) and (best is None or len(p) > len(best["prefix"])):
            best = a
    return best


def _paths(f, urls) -> list:
    """Same-site paths, in order, without repeats."""
    out = []
    for u in urls:
        if f.same_site(u):
            out.append(urllib.parse.urlsplit(u).path or "/")
    return list(dict.fromkeys(out))


def check(gw, f, doc: dict, m, urls) -> None:
    """Checks the files named by urls; adds to m.broken and m.asset_fixes."""
    paths = [p for p in _paths(f, urls) if _prefix(doc, p)]
    sheets = 0
    i = 0
    while i < len(paths) and i < MAX_FILES:
        path = paths[i]
        i += 1
        try:
            direct = f.get(path)
        except (ReachError, OSError, ValueError):
            continue
        if direct.status != 200:
            continue  # the site itself does not serve it: not Equalizer's to mend
        if direct.content_type == "text/css" and sheets < MAX_SHEETS:
            sheets += 1
            text = direct.body.decode("utf-8", "replace")
            paths += [p for p in _paths(f, css.refs(text, direct.url)) if p not in paths and _prefix(doc, p)]
        try:
            via = gw.preview(path, method="HEAD")
        except TryError:
            return
        ref = via.get("refusal")
        if not ref:
            continue
        reason = ref.get("reason", "")
        a = _prefix(doc, path)
        m.broken_files.add(path)
        m.broken.append({"path": path, "entry": via.get("entry"), "difference": "refused",
                         "detail": "%s (%s directly)" % (reason, direct.content_type or "no type")})
        fix = m.asset_fixes.setdefault(a["prefix"], {"types": [], "max_size_kb": 0})
        if reason == "asset_type_not_allowed" and direct.content_type not in GENERIC:
            if direct.content_type not in fix["types"]:
                fix["types"].append(direct.content_type)
        elif reason == "asset_too_large":
            size = direct.headers.get("content-length", "")
            if size.isdigit():
                need = -(-int(size) * 11 // 10240)  # size + 10%, in KiB
                if need <= SIZE_CEILING_KB:
                    fix["max_size_kb"] = max(fix["max_size_kb"], need)


def widen(doc: dict, m) -> list:
    """Applies m.asset_fixes to doc; the corrections. A page (not a file) an
    asset prefix refused means the prefix covers pages: it goes, and the
    site area serves its files."""
    out = []
    for b in m.broken:
        eid = b.get("entry") or ""
        if eid.startswith("asset:") and b["path"] not in m.broken_files:
            prefix = eid[len("asset:"):]
            if any(a.get("prefix") == prefix for a in doc.get("assets", [])):
                doc["assets"] = [a for a in doc["assets"] if a.get("prefix") != prefix]
                m.asset_fixes.pop(prefix, None)
                out.append({"entry": eid, "change": "removed", "detail": "back to the site area",
                            "reason": "%s is a page under %s, refused through Equalizer" % (b["path"], prefix)})
    for prefix, fix in sorted(m.asset_fixes.items()):
        a = next((x for x in doc.get("assets", []) if x.get("prefix") == prefix), None)
        if a is None:
            continue
        added = [t for t in fix["types"] if t not in a.get("types", [])]
        if added:
            a.setdefault("types", []).extend(added)
            out.append({"entry": "asset:" + prefix, "change": "widened", "detail": "+ " + ", ".join(added),
                        "reason": "files under %s were refused through Equalizer for their type" % prefix})
        if fix["max_size_kb"] > (a.get("max_size_kb") or 0):
            a["max_size_kb"] = fix["max_size_kb"]
            out.append({"entry": "asset:" + prefix, "change": "widened",
                        "detail": "max_size_kb %d" % fix["max_size_kb"],
                        "reason": "a file under %s was refused through Equalizer for its size" % prefix})
    return out
