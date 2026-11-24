"""Read-only exploration of the site: anonymous first, then with the test
account if there is one. Admin areas, sensitive files and links that change
something are recorded, never opened."""

import re
import http.client
import time
import urllib.parse
from collections import deque
from dataclasses import dataclass, field

from . import browser, css, extract, rules, webapps
from .fetch import Fetcher, ReachError
from .login import LoginMixin, path_query

ASSET_EXT = {".css": "text/css", ".js": "application/javascript", ".png": "image/png", ".jpg": "image/jpeg",
             ".jpeg": "image/jpeg", ".gif": "image/gif", ".svg": "image/svg+xml", ".webp": "image/webp",
             ".ico": "image/x-icon", ".woff": "font/woff", ".woff2": "font/woff2", ".ttf": "font/ttf",
             ".otf": "font/otf", ".pdf": "application/pdf", ".mp4": "video/mp4", ".m4v": "video/mp4",
             ".webm": "video/webm", ".ogv": "video/ogg", ".mp3": "audio/mpeg", ".m4a": "audio/mp4",
             ".ogg": "audio/ogg", ".wav": "audio/wav", ".avif": "image/avif", ".bmp": "image/bmp",
             ".mov": "video/quicktime", ".flac": "audio/flac", ".aac": "audio/aac", ".opus": "audio/ogg",
             ".mjs": "application/javascript", ".webmanifest": "application/manifest+json", ".wasm": "application/wasm",
             ".txt": "text/plain", ".csv": "text/csv", ".rtf": "application/rtf",
             ".doc": "application/msword", ".xls": "application/vnd.ms-excel", ".ppt": "application/vnd.ms-powerpoint",
             ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
             ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
             ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
             ".odt": "application/vnd.oasis.opendocument.text", ".ods": "application/vnd.oasis.opendocument.spreadsheet",
             ".odp": "application/vnd.oasis.opendocument.presentation", ".epub": "application/epub+zip"}
# Archives, logs and dumps are never studied (rules.SENSITIVE), even when linked.
PER_SHAPE = 20  # a calendar or a search must not eat the whole budget
MAX_SCRIPTS = 30
MAX_STYLESHEETS = 60  # read for the fonts and images they name
BROWSER_PAGES = 40  # pages opened in the browser per pass


@dataclass
class Seen:
    status: int
    ctype: str
    title: str = ""
    logged_in: bool = False
    seen_by: set = field(default_factory=set)  # "anonymous" and test account labels


@dataclass
class CrawlResult:
    pages: dict = field(default_factory=dict)  # path?query -> Seen
    forms: list = field(default_factory=list)  # extract.Form (same site)
    assets: dict = field(default_factory=dict)  # path -> content type
    sizes: dict = field(default_factory=dict)  # path -> bytes, for the files the agent opened
    served: set = field(default_factory=set)  # paths whose type is the one the site sent: never guessed over
    excluded: dict = field(default_factory=dict)  # path?query -> reason
    skipped_actions: set = field(default_factory=set)
    generator: str = ""
    urls: list = field(default_factory=list)  # every same-site URL seen, for fingerprinting
    login: str = ""  # outcome of the first test account's login
    logins: dict = field(default_factory=dict)  # account label -> ok, failed, no_login_form
    errors: int = 0
    script_requests: set = field(default_factory=set)  # GET path?query made by the pages' scripts
    script_refused: set = field(default_factory=set)  # (method, path) the read-only guard kept in
    browser_pages: int = 0


def shape(pq: str) -> str:
    path, _, q = pq.partition("?")
    segs = ["9" if s.isdigit() else ("x" if len(s) > 30 else s) for s in path.split("/")]
    keys = sorted({k for k, _ in urllib.parse.parse_qsl(q, keep_blank_values=True)})
    return "/".join(segs) + "?" + "&".join(keys)


class Crawler(LoginMixin):
    def __init__(self, fetcher: Fetcher, cms: str | None, max_pages: int, log, progress=None):
        self.f, self.cms, self.max, self.log, self.progress = fetcher, cms, max_pages, log, progress
        self.r = CrawlResult()
        self.shapes: dict = {}
        self.scripts = 0
        self.sheets: dict = {}  # stylesheet path -> read already
        self.label = "anonymous"
        self.browser = None  # {"executable": path} to study pages in a real browser

    def say(self, doing: str, path: str | None = None, force: bool = True):
        """Tells the panel what the discovery is doing now, in a few words
        (shown next to the page count); at most every 2 seconds unless
        it starts something new."""
        if not self.progress:
            return
        now = time.monotonic()
        if not force and now - getattr(self, "_said", 0.0) < 2:
            return
        self._said = now
        p = {"pages": len(self.r.pages), "forms": len(self.r.forms), "logged_in": getattr(self, "logged_pass", False),
             "doing": doing[:80]}
        if path:
            p["path"] = path[:100]
        try:
            self.progress(p)
        except Exception:  # noqa: BLE001 - progress is a courtesy, the study goes on
            pass

    def anonymous(self, seeds: list) -> CrawlResult:
        self.label = "anonymous"
        self._crawl(seeds, logged_in=False)
        self._browse(logged_in=False)
        self._stylesheets()
        return self.r

    def _browse(self, logged_in: bool):
        """Opens this pass's pages in the browser (when there is one) and
        studies what their scripts asked for like any other page."""
        if not self.browser:
            return
        pages = [pq for pq, s in self.r.pages.items()
                 if self.label in s.seen_by and s.status < 400 and s.ctype == "text/html"][:BROWSER_PAGES]
        host = urllib.parse.urlsplit(self.f.base_url).netloc
        self.say(f"opening {len(pages)} pages in the browser to see what their scripts load")
        seen = browser.study(self.f, host, pages, self.log, self._allowed_quiet, self.browser.get("executable", ""))
        self.r.browser_pages += seen.pages
        self.r.script_refused |= seen.refused
        new = []
        for pq in sorted(seen.requested - set(self.r.pages) - set(pages)):
            last = pq.split("?")[0].rsplit("/", 1)[-1]
            if "." in last and "." + last.rsplit(".", 1)[-1].lower() in ASSET_EXT or self._fragment(pq.split("?")[0]):
                self._asset(self.f.base_url.rstrip("/") + pq)  # a file the page loads
            else:
                new.append(pq)
        self.r.script_requests |= set(new)
        if new:
            self._crawl(new, logged_in)

    def _allowed_quiet(self, pq: str) -> bool:
        """The crawler's rules for the browser's requests."""
        reason = rules.excluded(pq, self.cms, logged_in=getattr(self, "logged_pass", False))
        if reason:
            self.r.excluded.setdefault(pq, reason)
            return False
        last = pq.split("?")[0].rsplit("/", 1)[-1]
        static = "." in last and "." + last.rsplit(".", 1)[-1].lower() in ASSET_EXT
        # a script file named block-library/… acts on nothing; but a file
        # that ends the session (/logout.png on Rails) or sits in such a
        # folder is never opened
        if (static and (webapps.ends_session(pq) or webapps.STRONG.search(pq.rsplit("/", 1)[0] + "/"))) or \
                (not static and rules.is_action(pq)):
            self.r.skipped_actions.add(pq)
            return False
        return True

    def logged_in(self, account: dict, label: str = "account") -> CrawlResult:
        """One pass per test account (call after the CMS is known). Each
        starts from a new session: what one account sees never comes from
        another's cookies."""
        self.f.cookies = {}
        self._tried_known = False
        self.after_login = []
        self.label = label
        ok = self._login(account)
        self.r.logins[label] = self.r.login
        if len(self.r.logins) > 1:
            self.r.login = next(iter(self.r.logins.values()))
        if ok:
            prof = webapps.get(self.cms)
            admin = list(prof.admin_home) if prof.admin_read_only else []
            if prof.admin_login and admin:
                self._admin_login(account)
            seeds = getattr(self, "after_login", []) + ["/"] + admin
            self._crawl(seeds + [p for p in list(self.r.pages)[:50]], logged_in=True)
            self._browse(logged_in=True)
            self._stylesheets()
        self.f.cookies = {}
        return self.r

    def login_only(self, account: dict, pages: list) -> bool:
        """Logs a test account in without studying anything (a verification):
        the login forms come from the first pages, or the profile's login page."""
        self.f.cookies = {}
        self._tried_known = False
        for pq in (["/"] + [p for p in pages if "login" in p.lower()] + pages)[:8]:
            try:
                resp = self.f.get(pq)
            except (ReachError, OSError, ValueError):
                continue
            if resp.status == 200 and resp.content_type == "text/html":
                self.r.forms.extend(f for f in extract.parse(resp.body.decode("utf-8", "replace"), resp.url).forms
                                    if self.f.same_site(f.action))
            if any(i.type == "password" for f in self.r.forms for i in f.inputs):
                break
        ok = self._login(account)
        prof = webapps.get(self.cms)
        if ok and prof.admin_read_only and prof.admin_login:
            self._admin_login(account)
        return ok

    def _crawl(self, seeds, logged_in):
        queue = deque(dict.fromkeys(seeds))
        visited = set(self.r.pages) if not logged_in else set()
        # each pass has its own budget: the logged-in pass must not find it
        # spent by the anonymous one
        self.shapes = {}
        self.logged_pass = logged_in
        fetched = 0
        while queue and fetched < self.max:
            pq = queue.popleft()
            if pq in visited:
                continue
            visited.add(pq)
            if self._static(pq) and pq.split("?")[0] != "/robots.txt":
                self._known_file(pq)
                continue
            if not self._allowed(pq):
                continue
            fetched += 1
            self.say("exploring" if self.label == "anonymous" else f"exploring as {self.label}", pq.split("?")[0], force=False)
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
            self.r.pages[pq].seen_by.add(self.label)
            if resp.status in (301, 302, 303, 307, 308) and resp.location:
                self._enqueue(queue, urllib.parse.urljoin(resp.url, resp.location))
                continue
            if resp.status != 200:
                continue
            if "xml" in resp.content_type and extract.is_sitemap(resp.body):
                # whatever its name: /sitemap.xml, WordPress's /wp-sitemap.xml,
                # those robots.txt and sitemap indexes list
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

    def _allowed(self, pq: str) -> bool:
        reason = rules.excluded(pq, self.cms, logged_in=getattr(self, "logged_pass", False))
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
        if ext in ASSET_EXT or self._fragment(pq.split("?")[0]):
            self._asset(url)
            return
        if getattr(self, "logged_pass", False) and pq not in self.r.pages:
            queue.appendleft(pq)  # logged in: pages only members see come first
        else:
            queue.append(pq)

    def _asset(self, url, served: str = ""):
        """served is the type the site sent, when the agent opened the file:
        an image, audio or video type of the same family as the extension's
        is kept (audio/x-wav for a .wav), since the gateway checks the type
        the site sends. Fonts need no such care: the gateway reads them."""
        if not self.f.same_site(url):
            return
        self.r.urls.append(url)
        path = urllib.parse.urlsplit(url).path
        ext = "." + path.rsplit(".", 1)[-1].lower() if "." in path.rsplit("/", 1)[-1] else ""
        kind = "text/html" if self._fragment(path) else ASSET_EXT.get(ext)
        family = (kind or "").split("/")[0]
        exact = bool(kind and family in ("image", "audio", "video") and served.startswith(family + "/"))
        if exact:
            kind = served
        elif path in self.r.served:
            return  # the type the site sent stays: a later link guesses nothing over it
        if kind and not rules.excluded(path, self.cms):
            self.r.assets[path] = kind
            if exact:
                self.r.served.add(path)
            if kind == "text/css":
                self.sheets.setdefault(path, False)

    @staticmethod
    def _static(pq: str) -> bool:
        last = pq.split("?")[0].rsplit("/", 1)[-1]
        return "." in last and "." + last.rsplit(".", 1)[-1].lower() in ASSET_EXT

    GENERIC = ("", "application/octet-stream", "binary/octet-stream", "application/unknown")

    def _fits_extension(self, path: str, served: str) -> bool:
        """Whether what the site sent can be the file its name says: a page
        (a front controller, a soft 404) answering /members/x.png is not an
        image, and must not make /members/ an asset prefix."""
        ext = "." + path.rsplit(".", 1)[-1].lower() if "." in path.rsplit("/", 1)[-1] else ""
        kind = ASSET_EXT.get(ext, "")
        if not kind or self._fragment(path):
            return bool(kind)
        if served == kind or served in self.GENERIC:
            return True
        family = kind.split("/")[0]
        if family in ("image", "audio", "video"):
            return served.startswith(family + "/")
        if family == "font" or "font" in served:
            return "font" in served
        return served == kind or (served.split("/")[0] == family and "html" not in served)

    def _known_file(self, pq: str):
        """A file named outside any page (a visitor asked for it and the
        gateway refused it): studied only if the site really serves it, and
        never opened when the crawler's rules keep it out."""
        if rules.excluded(pq, self.cms, logged_in=getattr(self, "logged_pass", False)) or rules.is_action(pq):
            self.r.excluded.setdefault(pq, "never")
            return
        try:
            resp = self.f.get(pq.split("?")[0])
        except (ReachError, OSError, ValueError, http.client.HTTPException):
            return
        if resp.status == 200 and self._fits_extension(pq.split("?")[0], resp.content_type):
            path = pq.split("?")[0]
            self._asset(self.f.base_url.rstrip("/") + path, served=resp.content_type)
            size = resp.headers.get("content-length", "")
            if path in self.r.assets and size.isdigit():
                self.r.sizes[path] = int(size)  # a file too large for the gateway's default limit

    def _stylesheets(self):
        """Reads the stylesheets found so far, and those they import, for the
        fonts and images they name."""
        self.say("reading the stylesheets for the files they load")
        read = 0
        while read < MAX_STYLESHEETS:
            path = next((p for p, done in self.sheets.items() if not done), None)
            if path is None:
                return
            self.sheets[path] = True
            read += 1
            try:
                resp = self.f.get(path)
            except (ReachError, OSError, ValueError):
                continue
            if resp.status != 200:
                continue
            for url in css.refs(resp.body.decode("utf-8", "replace"), resp.url):
                self._asset(url)

    def _fragment(self, path: str) -> bool:
        return webapps.is_fragment(webapps.get(self.cms), path)

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
