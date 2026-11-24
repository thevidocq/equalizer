"""A real browser for the study, behind the same read-only guard.

Headless Chromium (Playwright) opens the pages the crawler found, so their
scripts run and the requests they make are seen. The browser reaches
nothing by itself: every request it makes is intercepted and answered by
the agent's Fetcher (the relay, the connector, the read-only guard, the
test account's session) when it is a GET or HEAD for the site itself that
the crawler's rules allow; everything else is refused and recorded. Its
network otherwise points at a closed port, so nothing the interception
does not cover (a WebSocket, say) gets out. The browser never clicks,
types or submits.

Optional: without Playwright installed (EQ_BROWSER=off, or an image
without it) the study runs as before, with plain requests."""

import time
import urllib.parse
from dataclasses import dataclass, field

from .fetch import ReachError

try:  # an optional dependency: the agent works without it
    from playwright.sync_api import Error as PlaywrightError
    from playwright.sync_api import sync_playwright
except ImportError:  # pragma: no cover - depends on the image
    sync_playwright = None
    PlaywrightError = Exception

PAGE_SECONDS = 20  # a page in the browser at most, loading and settling; then it is skipped
SETTLE_MS = 1500
CLOSED_PORT = "http://127.0.0.1:9"  # discard: nothing listens
DROPPED = {"connection", "keep-alive", "transfer-encoding", "set-cookie", "content-length", "content-encoding",
           "strict-transport-security", "alt-svc"}


def available() -> bool:
    return sync_playwright is not None


@dataclass
class Seen:
    """What the pages asked for while they loaded."""
    requested: set = field(default_factory=set)  # GET path?query, served through the guard
    refused: set = field(default_factory=set)  # (method, path) the guard kept in
    pages: int = 0
    errors: int = 0
    # pages left before their scripts were seen: {path, reason, seconds}
    skipped: list = field(default_factory=list)


class _Guard:
    """Answers the browser's requests through the Fetcher, or refuses."""

    def __init__(self, fetcher, host: str, seen: Seen, allowed):
        self.f, self.host, self.seen, self.allowed = fetcher, host.lower(), seen, allowed

    def target(self, url: str) -> str:
        u = urllib.parse.urlsplit(url)
        if u.scheme not in ("http", "https") or not self.f.same_site(url):
            return ""  # another site; the site's own other names (www., public domain) are its own
        return (u.path or "/") + ("?" + u.query if u.query else "")

    def __call__(self, route):
        req = route.request
        if req.url.startswith(("data:", "blob:")):  # the page's own memory
            return route.continue_()
        pq = self.target(req.url)
        if not pq:  # another site (fonts, analytics): the study stays on this one
            return route.abort("blockedbyclient")
        if req.method not in ("GET", "HEAD") or not self.allowed(pq):
            self.seen.refused.add((req.method, pq.split("?")[0]))
            return route.abort("blockedbyclient")
        try:
            resp = self.f.request(req.method, pq)
        except (ReachError, OSError, ValueError):
            self.seen.errors += 1
            return route.abort("failed")
        self.seen.requested.add(pq)
        headers = {k: v for k, v in resp.headers.items() if k.lower() not in DROPPED}
        route.fulfill(status=resp.status, headers=headers, body=resp.body)


def _close(ctx, b) -> None:
    """Closes the browser without leaving route handlers running: a handler
    still waiting when the context closes is cancelled, and Playwright prints
    that as a traceback outside the JSON logs."""
    if ctx is not None:
        try:
            ctx.unroute_all(behavior="ignoreErrors")
        except (PlaywrightError, TypeError):  # older Playwright: no behaviour argument
            pass
        try:
            ctx.close()
        except PlaywrightError:
            pass
    b.close()


def study(fetcher, host: str, pages: list, log, allowed, executable: str = "", page_seconds: int = PAGE_SECONDS,
          progress=None) -> Seen:
    """Opens each page in the browser and returns what it asked for. host is
    the site's host[:port] as in its base URL; the Fetcher's session (a test
    account's, or none) is the browser's. A page gets page_seconds at most:
    then it is skipped, and Seen.skipped says which, why and after how long.
    progress(done, total, path) is told before each page."""
    seen = Seen()
    if not available() or not pages:
        return seen
    base = urllib.parse.urlsplit(fetcher.base_url)
    origin = "%s://%s" % (base.scheme, base.netloc)
    guard = _Guard(fetcher, host, seen, allowed)
    try:
        with sync_playwright() as pw:
            opts = {"headless": True, "proxy": {"server": CLOSED_PORT},
                    "args": ["--no-sandbox", "--disable-dev-shm-usage", "--proxy-bypass-list=<-loopback>"]}
            if executable:
                opts["executable_path"] = executable
            b = pw.chromium.launch(**opts)
            ctx = None
            try:
                ctx = b.new_context(service_workers="block", accept_downloads=False, ignore_https_errors=True)
                ctx.route("**/*", guard)
                page = ctx.new_page()
                page.on("dialog", lambda d: d.dismiss())
                for n, pq in enumerate(pages):
                    if progress:
                        progress(n, len(pages), pq)
                    started = time.monotonic()
                    try:
                        page.goto(origin + pq, wait_until="load", timeout=page_seconds * 1000)
                        page.wait_for_timeout(min(SETTLE_MS, max(0, page_seconds * 1000 - int((time.monotonic() - started) * 1000))))
                        seen.pages += 1
                    except PlaywrightError as e:
                        seen.errors += 1
                        took = round(time.monotonic() - started, 1)
                        timeout = "Timeout" in type(e).__name__ or "timeout" in str(e).lower()
                        seen.skipped.append({"path": pq, "seconds": took,
                                             "reason": "time_limit" if timeout else "error",
                                             "detail": "" if timeout else _first_line(e)})
                        if not timeout:
                            page = _fresh(ctx, page)
                    if (n + 1) % 10 == 0:
                        log.info("browser progress", done=n + 1, total=len(pages), skipped=len(seen.skipped))
            finally:
                _close(ctx, b)
    except PlaywrightError as e:
        log.warn("browser unavailable", error=str(e)[:200])
        done = {s["path"] for s in seen.skipped}
        for pq in pages[seen.pages + len(seen.skipped):]:
            if pq not in done:
                seen.skipped.append({"path": pq, "seconds": 0, "reason": "browser_failed", "detail": _first_line(e)})
    return seen


def _fresh(ctx, page):
    """A new tab after a page broke the old one (a crash, a closed target)."""
    try:
        page.close()
    except PlaywrightError:
        pass
    p = ctx.new_page()
    p.on("dialog", lambda d: d.dismiss())
    return p


def _first_line(e: Exception) -> str:
    """The first line of an error, or its kind when it says nothing."""
    return (str(e).splitlines() or [type(e).__name__])[0][:160]
