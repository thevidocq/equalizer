"""The three measures of the study on one site, through its gateway:
normal visits (fidelity, which entry serves them), attacks on every input
point the map knows (what reaches the site), attacks on the protocol."""
import difflib
import html.parser
import itertools
import json
import re
import socket
import sys
import time
import urllib.parse

import requests

from attacks import PAYLOADS, decoded, verdict
from sites import ROOT

sys.path.insert(0, str(ROOT / "agent"))
from equalizer_agent import verify  # noqa: E402
from equalizer_agent.mapbuild import fits  # noqa: E402

_ips = (f"198.18.{a}.{b}" for a in range(0, 256) for b in range(1, 255))  # one visitor per request


class Site:
    def __init__(self, name, s, out):
        self.name, self.s, self.out = name, s, out
        self.direct = f"http://127.0.0.1:{s['legacy']}"
        self.gw = f"http://127.0.0.1:{s['gateway']}"
        self.log = out / "legacy.jsonl"
        self.gwlog = out / "gateway.log"

    def _size(self, p):
        return p.stat().st_size if p.exists() else 0

    def _tail(self, p, pos):
        with open(p, "rb") as f:
            f.seek(pos)
            data = f.read().decode("utf-8", "replace")
        return [json.loads(x) for x in data.splitlines() if x.startswith("{")]

    def send(self, base, method, path, data=None, session=None, ip=None):
        h = {"Host": self.s["host"], "X-Forwarded-For": ip or next(_ips)}
        try:
            return (session or requests).request(method, base + path, data=data, headers=h, allow_redirects=False,
                                                 timeout=60)
        except requests.RequestException:
            return None

    def through(self, method, path, data=None, session=None, ip=None):
        """(response, entry that served it, what the legacy site received)"""
        lp, gp = self._size(self.log), self._size(self.gwlog)
        r = self.send(self.gw, method, path, data, session, ip)
        time.sleep(0.05)
        legacy = self._tail(self.log, lp)
        entry = next((g.get("route") or "" for g in self._tail(self.gwlog, gp) if g.get("schema") == "event/1.0"), "")
        return r, entry, legacy


def _text(r):
    return verify._page(r.content, r.headers.get("content-type", "")).words


def visits(site: Site, report: dict) -> list:
    rows = []
    for u in sorted(set(report.get("pages", [])))[:150]:
        d = site.send(site.direct, "GET", u)
        r, entry, _ = site.through("GET", u)
        row = {"url": u, "entry": entry, "direct": d.status_code if d is not None else None,
               "via": r.status_code if r is not None else None}
        if d is not None and r is not None and d.status_code == r.status_code == 200 and \
                "html" in d.headers.get("content-type", ""):
            a, b = _text(d), _text(r)
            row["text"] = round(difflib.SequenceMatcher(None, a, b, autojunk=False).ratio(), 3) if a or b else 1.0
        rows.append(row)
    return rows


def _types(doc):
    return doc.get("types", {})


def _base(doc, spec):
    t = spec.get("type", "string")
    return _types(doc).get(t, {}).get("base", t), spec.get("pattern") or _types(doc).get(t, {}).get("pattern")


def kind_of(doc, spec) -> str:
    base, pattern = _base(doc, spec)
    if base in ("int", "enum", "bool", "date", "email"):
        return "typed"
    return "patterned" if pattern else "free"


def _sample(doc, report, route):
    """The shortest studied page the route really serves."""
    pages = [p for p in sorted({q.partition("?")[0] for q in report.get("pages", [])}) if fits(doc, route, p)]
    return min(pages, key=len, default=None)


class _Inputs(html.parser.HTMLParser):
    """The inputs of the form posting to action."""
    def __init__(self, action):
        super().__init__()
        self.action, self.inside, self.values = action, False, {}

    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        if tag == "form":
            self.inside = urllib.parse.urlsplit(a.get("action") or "").path == self.action
        elif self.inside and tag in ("input", "textarea", "select") and a.get("name"):
            self.values.setdefault(a["name"], a.get("value") or "")

    def handle_endtag(self, tag):
        if tag == "form":
            self.inside = False


def points(doc: dict, report: dict) -> list:
    """Every input point the map knows, with a way to send a value to it:
    (surface, point, send(site, value) -> (response, legacy))."""
    q = urllib.parse.quote
    out = []
    for r in doc.get("routes", []):
        if r.get("method", "GET") != "GET" or r.get("authz"):
            continue
        page = _sample(doc, report, r)
        if not page:
            continue
        sep = "&" if "?" in page else "?"
        for k, spec in (r.get("query") or {}).items():
            out.append((kind_of(doc, spec), f"{r['id']} ?{k}=",
                        lambda s, v, page=page, k=k, sep=sep: s.through("GET", page + sep + urllib.parse.urlencode({k: v}))))
            out.append(("extra_key:" + kind_of(doc, spec), f"{r['id']} ?{k}=…&zz=1",
                        lambda s, v, page=page, k=k, sep=sep: s.through("GET", page + sep + urllib.parse.urlencode({k: v, "zz": "1"}))))
        pattern = r["path"].strip("/").split("/")
        parts = page.strip("/").split("/")
        for i, seg in enumerate(pattern):
            if seg.startswith("{") and len(parts) == len(pattern):
                def send(s, v, i=i, parts=parts, page=page):
                    p = list(parts)
                    p[i] = q(v, safe="")
                    return s.through("GET", "/" + "/".join(p) + ("/" if page.endswith("/") else ""))
                out.append(("path", f"{r['id']} {seg}", send))
        out.append(("undeclared", f"{r['id']} ?xkey=",
                    lambda s, v, page=page, sep=sep: s.through("GET", page + sep + urllib.parse.urlencode({"xkey": v}))))
    for f in doc.get("forms", []):
        action = f.get("route")
        if not action or f.get("roles") and "guest" not in f.get("roles", []):
            continue
        for name, spec in f.get("fields", {}).items():
            if _base(doc, spec)[0] == "file":
                continue
            out.append((kind_of(doc, spec), f"form {f['id']}: {name}",
                        lambda s, v, f=f, name=name, action=action: _post(s, report, action, name, v)))
    area = [o["target"] for o in report.get("open", []) if o.get("reason") == "not_reached" and "?" not in o["target"]][:4]
    for t in area:
        out.append(("area", f"area {t} ?q=", lambda s, v, t=t: s.through("GET", t + "?" + urllib.parse.urlencode({"q": v}))))
    return out


_form_pages = {}


def _plain(name: str) -> str:
    """What a visitor types in an empty field of that name."""
    n = name.lower()
    if "mail" in n:
        return "anna@example.com"
    if "url" in n or "website" in n:
        return ""
    if any(w in n for w in ("comment", "message", "body", "text", "content")):
        return "A fine comment, thanks"
    return "Anna"


def _post(site, report, action, field, value):
    """Posts value in field of the form to action, from a page that shows it
    (found once, then reused: a new session each time)."""
    pages = [_form_pages[action]] if action in _form_pages else sorted(set(report.get("pages", [])), key=len)
    for page in pages:
        sess, ip = requests.Session(), next(_ips)
        r0 = site.send(site.gw, "GET", page, session=sess, ip=ip)
        if r0 is None or r0.status_code != 200:
            continue
        p = _Inputs(action)
        p.feed(r0.text)
        if field not in p.values:
            continue
        _form_pages[action] = page
        form = {k: v or _plain(k) for k, v in p.values.items()}
        form[field] = value
        return site.through("POST", action, data=form, session=sess, ip=ip)
    return None, "", []


def attacks(site: Site, doc: dict, report: dict) -> list:
    rows, n = [], itertools.count(1000)
    for surface, point, send in points(doc, report):
        if point.startswith("form"):
            # a plain value must reach the site, or the point is not measured
            marker = f"mk{next(n)}z"
            plain = {"email": marker + "@example.com", "url": "https://example.com/" + marker}
            r, _, legacy = send(site, next((v for k, v in plain.items() if k in point.lower()), marker + " hello"))
            if not any(marker in decoded(e) for e in legacy):
                rows.append({"surface": surface, "point": point, "class": "control", "entry": "",
                             "status": r.status_code if r is not None else None, "verdict": "unmeasured"})
                continue
        for cls, plist in PAYLOADS.items():
            for p in plist:
                marker = f"MK{next(n)}Z"
                r, entry, legacy = send(site, marker + p)
                rows.append({"surface": surface, "point": point, "class": cls, "entry": entry,
                             "status": r.status_code if r is not None else None,
                             "verdict": verdict(r.status_code if r is not None else None, legacy, marker, p, cls)})
    return rows


PROTOCOL = {
    "traversal ../": b"GET /x/../../etc/passwd HTTP/1.1\r\n",
    "traversal %2e%2e": b"GET /%2e%2e/%2e%2e/etc/passwd HTTP/1.1\r\n",
    "traversal ..%2f": b"GET /x/..%2f..%2fetc/passwd HTTP/1.1\r\n",
    "traversal backslash": b"GET /x\\..\\..\\etc\\passwd HTTP/1.1\r\n",
    "null byte": b"GET /index.php%00.png HTTP/1.1\r\n",
    "TRACE": b"TRACE / HTTP/1.1\r\n",
    "header 64 KB": b"GET / HTTP/1.1\r\nX-Big: " + b"A" * 65536 + b"\r\n",
    "another host": None,
    "smuggling CL+TE": b"POST / HTTP/1.1\r\nContent-Length: 6\r\nTransfer-Encoding: chunked\r\n",
    "XXE": b"POST / HTTP/1.1\r\nContent-Type: text/xml\r\n",
    "PHP as PNG": b"POST / HTTP/1.1\r\nContent-Type: image/png\r\n",
    "unknown binary": b"POST / HTTP/1.1\r\nContent-Type: application/x-foo\r\n",
    "forged headers": b"GET / HTTP/1.1\r\nX-HTTP-Method-Override: DELETE\r\nX-Original-URL: /admin/\r\nX-Remote-User: admin\r\n",
}
BODIES = {"smuggling CL+TE": b"0\r\n\r\nG",
          "XXE": b'<?xml version="1.0"?><!DOCTYPE r [<!ENTITY x SYSTEM "file:///etc/passwd">]><r>&x;</r>',
          "PHP as PNG": b"<?php system($_GET['c']); ?>", "unknown binary": b"\x00\x01\x02MKBIN"}


def protocol(site: Site) -> list:
    rows = []
    for name, head in PROTOCOL.items():
        if head is None:
            req = b"GET / HTTP/1.1\r\nHost: evil.test\r\nConnection: close\r\n\r\n"
        else:
            body = BODIES.get(name, b"")
            req = head + b"Host: " + site.s["host"].encode() + b"\r\nConnection: close\r\n"
            if body and name != "smuggling CL+TE":
                req += b"Content-Length: %d\r\n" % len(body)
            req += b"\r\n" + body
        lp = site._size(site.log)
        c = socket.create_connection(("127.0.0.1", site.s["gateway"]), timeout=10)
        c.sendall(req)
        data = b""
        try:
            while chunk := c.recv(65536):
                data += chunk
        except socket.timeout:
            pass
        c.close()
        time.sleep(0.1)
        legacy = site._tail(site.log, lp)
        status = int(data.split(b" ", 2)[1]) if data.startswith(b"HTTP/") else None
        forged = any(k in e["h"] for e in legacy for k in ("HTTP_X_HTTP_METHOD_OVERRIDE", "HTTP_X_ORIGINAL_URL", "HTTP_X_REMOTE_USER"))
        bad = b"passwd" in b"".join(e["u"].encode() for e in legacy) or forged or \
            any(b"ENTITY" in __import__("base64").b64decode(e["b"]) or b"<?php" in __import__("base64").b64decode(e["b"]) for e in legacy)
        rows.append({"test": name, "status": status, "reached": bool(legacy),
                     "verdict": "passed" if bad else "neutralised" if legacy else "blocked"})
    return rows
