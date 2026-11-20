"""Requests towards the legacy site, as the gateway makes them: through the
relay and the connector, or directly (a site at a public IP address that
accepts only Equalizer's address, or a lab destination the platform
allows). Only destinations in the map's reach are used; redirects are
returned, never followed; bodies are bounded."""

import http.client
import ipaddress
import socket
import ssl
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


def _public_ip(host: str) -> bool:
    """A public IP address written as such: a name could resolve to the
    platform's own network, an address cannot."""
    try:
        return ipaddress.ip_address(host.strip("[]")).is_global
    except ValueError:
        return False


class _Pinned(urllib.request.HTTPHandler, urllib.request.HTTPSHandler):
    """Connects every request to one host:port while the URL's host stays in
    Host and in TLS SNI, and the certificate is checked against it: a
    virtual host reached at its address, as the gateway does."""

    def __init__(self, dest: str):
        urllib.request.HTTPHandler.__init__(self)
        urllib.request.HTTPSHandler.__init__(self, context=ssl.create_default_context())
        host, _, port = dest.rpartition(":")
        self.addr = (host.strip("[]"), int(port))

    def http_open(self, req):
        addr = self.addr

        class Conn(http.client.HTTPConnection):
            def connect(self):
                self.sock = socket.create_connection(addr, self.timeout)
        return self.do_open(Conn, req)

    def https_open(self, req):
        addr, ctx = self.addr, self._context

        class Conn(http.client.HTTPSConnection):
            def connect(self):
                raw = socket.create_connection(addr, self.timeout)
                self.sock = ctx.wrap_socket(raw, server_hostname=self.host)
        return self.do_open(Conn, req)


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
        if mode == "direct" and self.dest not in self.direct and not _public_ip(self.dest.rpartition(":")[0]):
            raise ReachError("direct mode reaches a public IP address, or a destination in EQ_DIRECT_DESTINATIONS")
        if mode not in ("direct", "connector"):
            raise ReachError("unknown legacy.reach.mode")
        if mode == "direct":
            self.opener = urllib.request.build_opener(NoRedirect(), _Pinned(self.dest))
        else:
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
        else:  # connected to dest by _Pinned; the site's own name in Host and SNI
            target = f"{self.base.scheme}://{self.base.netloc}{path_query}"
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
