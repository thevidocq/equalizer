"""Requests towards the legacy site, as the gateway makes them: through the
relay and the connector (or directly, for lab destinations the platform
allows). Only destinations in the map's reach are used; redirects are
returned, never followed; bodies are bounded."""

import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from dataclasses import dataclass, field
from http.cookies import SimpleCookie

MAX_BODY = 2 << 20
TIMEOUT = 20


@dataclass
class Response:
    status: int
    headers: dict
    body: bytes
    url: str  # legacy URL, as base_url sees it

    @property
    def content_type(self) -> str:
        return self.headers.get("content-type", "").split(";")[0].strip().lower()

    @property
    def location(self) -> str:
        return self.headers.get("location", "")


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None


class ReachError(Exception):
    pass


def _dest(u: urllib.parse.SplitResult) -> str:
    port = u.port or (443 if u.scheme == "https" else 80)
    return f"{(u.hostname or '').lower()}:{port}"


class ReadOnlyError(ValueError):
    """A request that could change the site."""


@dataclass
class Fetcher:
    base_url: str
    reach: dict
    tenant: str
    relay_url: str = ""
    relay_token: str = ""
    direct: set = field(default_factory=set)
    rate: float = 2.0
    cookies: dict = field(default_factory=dict)
    _last: float = 0.0

    def __post_init__(self):
        self.base = urllib.parse.urlsplit(self.base_url)
        self.allow = {a.lower() for a in self.reach.get("allow", [])}
        self.address = (self.reach.get("address") or "").lower()
        self.dest = self.address or _dest(self.base)
        if self.dest not in self.allow:
            raise ReachError("the destination is not in legacy.reach.allow")
        mode = self.reach.get("mode")
        if mode == "direct" and self.dest not in self.direct:
            raise ReachError("direct mode is limited to EQ_DIRECT_DESTINATIONS")
        if mode not in ("direct", "connector"):
            raise ReachError("unknown legacy.reach.mode")
        self.opener = urllib.request.build_opener(NoRedirect())

    def same_site(self, url: str) -> bool:
        u = urllib.parse.urlsplit(url)
        return u.scheme in ("http", "https") and (u.netloc.lower() == self.base.netloc.lower())

    def get(self, path_query: str) -> Response:
        return self.request("GET", path_query)

    def request(self, method: str, path_query: str, form: dict | None = None, login: bool = False) -> Response:
        """Read-only: GET and HEAD, and one POST to log a test account in
        (login=True). Nothing else ever leaves the agent."""
        if not path_query.startswith("/"):
            raise ValueError("path must start with /")
        if method not in ("GET", "HEAD") and not (login and method == "POST"):
            raise ReadOnlyError(method + " refused: the agent never writes to the site")
        self._throttle()
        data = urllib.parse.urlencode(form).encode() if form is not None else None
        headers = {"User-Agent": "equalizer-discovery/0.1", "Accept": "text/html,application/xhtml+xml,*/*;q=0.8"}
        if data is not None:
            headers["Content-Type"] = "application/x-www-form-urlencoded"
        if self.cookies:
            headers["Cookie"] = "; ".join(f"{k}={v}" for k, v in self.cookies.items())
        if self.reach.get("mode") == "connector":
            target = self.relay_url + path_query
            headers.update({
                "Authorization": "Bearer " + self.relay_token, "X-Eq-Connector": self.reach.get("connector_id", ""),
                "X-Eq-Tenant": self.tenant, "X-Eq-Destination": self.dest, "X-Eq-Scheme": self.base.scheme,
                "X-Eq-Request-Id": "d-" + uuid.uuid4().hex[:16]})
            if self.address:
                headers["X-Eq-Host"] = self.base.netloc.lower()
        else:
            target = f"{self.base.scheme}://{self.dest}{path_query}"
            headers["Host"] = self.base.netloc
        req = urllib.request.Request(target, data=data, method=method, headers=headers)
        try:
            resp = self.opener.open(req, timeout=TIMEOUT)
        except urllib.error.HTTPError as e:
            resp = e
        with resp:
            body = resp.read(MAX_BODY + 1)[:MAX_BODY]
            hdrs = {k.lower(): v for k, v in resp.headers.items()}
            if hdrs.get("x-eq-error"):
                raise ReachError("relay: " + hdrs["x-eq-error"][:64])
            for raw in resp.headers.get_all("Set-Cookie") or []:
                c = SimpleCookie()
                try:
                    c.load(raw)
                except Exception:
                    continue
                for k, m in c.items():
                    self.cookies[k] = m.value
            return Response(resp.status, hdrs, body, self.base_url.rstrip("/") + path_query)

    def _throttle(self):
        if self.rate <= 0:
            return
        wait = self._last + 1.0 / self.rate - time.monotonic()
        if wait > 0:
            time.sleep(wait)
        self._last = time.monotonic()
