"""Read-only exploration of the site: anonymous first, then with the test
account if there is one. Admin areas, sensitive files and links that change
something are recorded, never opened."""

import re
import urllib.parse
from collections import deque
from dataclasses import dataclass, field

from . import extract, rules
from .fetch import Fetcher, ReachError

ASSET_EXT = {".css": "text/css", ".js": "application/javascript", ".png": "image/png", ".jpg": "image/jpeg",
             ".jpeg": "image/jpeg", ".gif": "image/gif", ".svg": "image/svg+xml", ".webp": "image/webp",
             ".ico": "image/x-icon", ".woff": "font/woff", ".woff2": "font/woff2", ".ttf": "font/ttf",
             ".pdf": "application/pdf", ".mp4": "video/mp4"}
PER_SHAPE = 20  # a calendar or a search must not eat the whole budget
MAX_SCRIPTS = 30


@dataclass
class Seen:
    status: int
    ctype: str
    title: str = ""
    logged_in: bool = False


@dataclass
class CrawlResult:
    pages: dict = field(default_factory=dict)  # path?query -> Seen
    forms: list = field(default_factory=list)  # extract.Form (same site)
    assets: dict = field(default_factory=dict)  # path -> content type
    excluded: dict = field(default_factory=dict)  # path?query -> reason
    skipped_actions: set = field(default_factory=set)
    generator: str = ""
    urls: list = field(default_factory=list)  # every same-site URL seen, for fingerprinting
    login: str = ""  # outcome of the test account login
    errors: int = 0


def path_query(url: str) -> str:
    u = urllib.parse.urlsplit(url)
    q = urllib.parse.urlencode(sorted(urllib.parse.parse_qsl(u.query, keep_blank_values=True)))
    return (u.path or "/") + ("?" + q if q else "")


def shape(pq: str) -> str:
    path, _, q = pq.partition("?")
    segs = ["9" if s.isdigit() else ("x" if len(s) > 30 else s) for s in path.split("/")]
    keys = sorted({k for k, _ in urllib.parse.parse_qsl(q, keep_blank_values=True)})
    return "/".join(segs) + "?" + "&".join(keys)


class Crawler:
    def __init__(self, fetcher: Fetcher, cms: str | None, max_pages: int, log, progress=None):
        self.f, self.cms, self.max, self.log, self.progress = fetcher, cms, max_pages, log, progress
        self.r = CrawlResult()
        self.shapes: dict = {}
        self.scripts = 0

    def anonymous(self, seeds: list) -> CrawlResult:
        self._crawl(seeds, logged_in=False)
        return self.r

    def logged_in(self, account: dict) -> CrawlResult:
        """Second pass with the test account (call after the CMS is known)."""
        if self._login(account):
            self._crawl(["/"] + [p for p in list(self.r.pages)[:50]], logged_in=True)
        return self.r

    def _crawl(self, seeds, logged_in):
        queue = deque(dict.fromkeys(seeds))
        visited = set(self.r.pages) if not logged_in else set()
        while queue and len(self.r.pages) < self.max:
            pq = queue.popleft()
            if pq in visited:
                continue
            visited.add(pq)
            if not self._allowed(pq):
                continue
            try:
                resp = self.f.get(pq)
            except (ReachError, OSError, ValueError) as e:
                self.r.errors += 1
                if self.r.errors > 50 and not self.r.pages:
                    raise ReachError("the site does not answer through the connector: " + str(e)[:100])
                continue
            if pq in self.r.pages and not logged_in:
                continue
            first = pq not in self.r.pages
            if first or logged_in:
                self.r.pages.setdefault(pq, Seen(resp.status, resp.content_type, logged_in=logged_in))
            if resp.status in (301, 302, 303, 307, 308) and resp.location:
                self._enqueue(queue, urllib.parse.urljoin(resp.url, resp.location))
                continue
            if resp.status != 200:
                continue
            if pq.startswith("/sitemap") and "xml" in resp.content_type:
                for loc in extract.sitemap_urls(resp.body.decode("utf-8", "replace")):
                    self._enqueue(queue, loc)
                continue
            if pq == "/robots.txt":
                for line in resp.body.decode("utf-8", "replace").splitlines():
                    k, _, v = line.partition(":")
                    if k.strip().lower() == "sitemap":
                        self._enqueue(queue, v.strip())
                continue
            if resp.content_type not in ("text/html", "application/xhtml+xml"):
                continue
            page = extract.parse(resp.body.decode("utf-8", "replace"), resp.url)
            self.r.pages[pq].title = page.title[:120]
            if page.generator and not self.r.generator:
                self.r.generator = page.generator
            for link in page.links:
                self._enqueue(queue, link)
            for a in page.assets:
                self._asset(a)
            for s in page.scripts:
                self._asset(s)
                self._script(queue, s)
            for form in page.forms:
                if self.f.same_site(form.action):
                    self.r.forms.append(form)
                    if form.method == "GET":
                        self._enqueue(queue, form.action)
            if self.progress and len(self.r.pages) % 20 == 0:
                self.progress({"pages": len(self.r.pages), "forms": len(self.r.forms), "logged_in": logged_in})

    def _allowed(self, pq: str) -> bool:
        reason = rules.excluded(pq, self.cms)
        if reason:
            self.r.excluded[pq] = reason
            return False
        if rules.is_action(pq):
            self.r.skipped_actions.add(pq)
            return False
        s = shape(pq)
        self.shapes[s] = self.shapes.get(s, 0) + 1
        return self.shapes[s] <= PER_SHAPE

    def _enqueue(self, queue, url):
        if not self.f.same_site(url):
            return
        self.r.urls.append(url)
        pq = path_query(url)
        ext = "." + pq.split("?")[0].rsplit(".", 1)[-1].lower() if "." in pq.split("?")[0].rsplit("/", 1)[-1] else ""
        if ext in ASSET_EXT:
            self._asset(url)
            return
        queue.append(pq)

    def _asset(self, url):
        if not self.f.same_site(url):
            return
        self.r.urls.append(url)
        path = urllib.parse.urlsplit(url).path
        ext = "." + path.rsplit(".", 1)[-1].lower() if "." in path.rsplit("/", 1)[-1] else ""
        if ext in ASSET_EXT and not rules.excluded(path, self.cms):
            self.r.assets[path] = ASSET_EXT[ext]

    def _script(self, queue, url):
        if not self.f.same_site(url) or self.scripts >= MAX_SCRIPTS:
            return
        self.scripts += 1
        try:
            resp = self.f.get(path_query(url))
        except (ReachError, OSError, ValueError):
            return
        if resp.status == 200:
            for u in extract.script_urls(resp.body.decode("utf-8", "replace")[:500_000], resp.url):
                if not re.search(r"\.(js|css|map)(\?|$)", u):
                    self._enqueue(queue, u)

    def _login(self, account: dict) -> bool:
        """Posts the first form with a password field, as a browser would."""
        for form in self.r.forms:
            pw = [i for i in form.inputs if i.type == "password"]
            user = [i for i in form.visible() if i.type in ("text", "email")]
            if not pw or not user or form.method != "POST":
                continue
            data = {i.name: "" for i in form.inputs if i.type not in ("submit", "button", "image", "reset")}
            # hidden values (tokens) come from a fresh copy of the page
            page_pq = path_query(form.action)
            try:
                fresh = self.f.get(page_pq)
                for f2 in extract.parse(fresh.body.decode("utf-8", "replace"), fresh.url).forms:
                    if path_query(f2.action) == page_pq:
                        data.update(_hidden_values(fresh.body.decode("utf-8", "replace"), [h.name for h in f2.hidden()]))
            except (ReachError, OSError, ValueError):
                pass
            data[user[0].name] = account.get("username", "")
            data[pw[0].name] = account.get("password", "")
            try:
                resp = self.f.request("POST", page_pq, data)
            except (ReachError, OSError, ValueError):
                self.r.login = "failed"
                return False
            ok = resp.status in (200, 302, 303) and bool(self.f.cookies)
            self.r.login = "ok" if ok else "failed"
            self.log.info("test account login", outcome=self.r.login, form=page_pq.split("?")[0])
            return ok
        known = rules.LOGIN.get(self.cms or "")
        if known and not getattr(self, "_tried_known", False):
            self._tried_known = True
            try:
                resp = self.f.get(known)
                if resp.status == 200:
                    self.r.forms.extend(f for f in extract.parse(resp.body.decode("utf-8", "replace"), resp.url).forms
                                        if self.f.same_site(f.action))
                    return self._login(account)
            except (ReachError, OSError, ValueError):
                pass
        self.r.login = "no_login_form"
        return False


def _hidden_values(html: str, names: list) -> dict:
    out = {}
    for n in names:
        m = re.search(r"""<input[^>]+name=["']%s["'][^>]*value=["']([^"']*)["']""" % re.escape(n), html, re.I) or \
            re.search(r"""<input[^>]+value=["']([^"']*)["'][^>]*name=["']%s["']""" % re.escape(n), html, re.I)
        if m:
            out[n] = m.group(1)
    return out
