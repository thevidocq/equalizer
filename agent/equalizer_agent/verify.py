"""The two checks of a map version (docs/agent.md, steps 4 and 5).

1. Does the site still work? Every studied page is opened directly
   through the connector and through the gateway with the version
   (preview), and the two are compared: status, visible text, links and
   forms. Forms are never submitted: a valid submission is tried in
   dry-run.
2. Are attacks refused? Every parameter and field of the version gets the
   attack set in dry-run. Nothing of this step reaches the legacy site.

The result is a score measured on the gateway itself, the pages that
broke and the inputs still open, each with its reason."""

import difflib
import re
import urllib.parse
from html.parser import HTMLParser

from dataclasses import dataclass, field

from . import attacks, correct, crawler, mapbuild
from .fetch import Fetcher, ReachError
from .gateway import Gateway, Jar, TryError

WORDS = {"accepts_sql": "SQL", "accepts_markup": "HTML", "accepts_template": "template code", "accepts_path": "paths",
         "accepts_shell": "shell characters", "other": "out-of-type values"}
TEXT_SIMILAR = 0.97  # dates, counters and tokens differ between two loads


class _Page(HTMLParser):
    """Visible words, internal link paths and forms of a page."""

    def __init__(self, base: str = "/"):
        super().__init__(convert_charrefs=True)
        self.base = base  # the page's own path: relative links resolve against it
        self.words, self.links, self.forms, self._skip, self._form = [], set(), [], 0, None

    def _path(self, href: str) -> str:
        u = urllib.parse.urlsplit(href)
        if u.netloc:  # the legacy host or the public one: the path is what counts
            return u.path or "/"
        return urllib.parse.urlsplit(urllib.parse.urljoin(self.base, href)).path or "/"

    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        if tag in ("script", "style", "noscript", "template"):
            self._skip += 1
        elif tag == "a" and a.get("href"):
            u = urllib.parse.urlsplit(a["href"])
            if not u.netloc or u.scheme in ("http", "https"):
                self.links.add(self._path(a["href"]))
        elif tag == "form":
            self._form = {"action": self._path(a["action"]) if a.get("action") else "", "method": (a.get("method") or "get").upper(),
                          "enctype": (a.get("enctype") or "").lower(), "inputs": {}}
            self.forms.append(self._form)
        elif tag in ("input", "textarea", "select") and self._form is not None and a.get("name"):
            self._form["inputs"].setdefault(a["name"], a.get("value") or "")

    def handle_endtag(self, tag):
        if tag in ("script", "style", "noscript", "template") and self._skip:
            self._skip -= 1
        elif tag == "form":
            self._form = None

    def handle_data(self, data):
        if not self._skip:
            self.words.extend(data.split())


def _charset(content_type: str) -> str:
    m = re.search(r"charset=([\w.:-]+)", content_type or "", re.I)
    return m.group(1) if m else "utf-8"


def _page(body: bytes, content_type: str = "", base: str = "/") -> _Page:
    p = _Page(base)
    try:
        text = body.decode(_charset(content_type), "replace")
    except LookupError:
        text = body.decode("utf-8", "replace")
    try:
        p.feed(text)
    except Exception:  # a broken page is compared by its status alone
        pass
    return p


def _same_site(words: list, hosts: tuple) -> list:
    """The site's own address in the text (http://legacy, https://public)
    is the same place: written without scheme and host on both sides."""
    if not hosts:
        return words
    rx = re.compile(r"^https?://(%s)(?=/|$)" % "|".join(re.escape(h) for h in hosts), re.I)
    return [rx.sub("", w) or "/" for w in words]


def _unslashed(links: set) -> set:
    """Links the same but for a final slash are the same link: the gateway
    writes each in the spelling of its route, and the site sends the other
    spelling there (Magento: /page/ answers 301 to /page)."""
    return {p.rstrip("/") or "/" for p in links}


def _slash_redirect(direct) -> bool:
    """The site's answer only sends to the same path with or without its
    final slash."""
    if not 300 <= direct.status < 400 or not direct.location:
        return False
    there = urllib.parse.urlsplit(direct.location).path
    here = urllib.parse.urlsplit(direct.url).path
    return there != here and there.rstrip("/") == here.rstrip("/")


def compare(direct, via, hosts: tuple = ()) -> tuple:
    """(difference, detail) between the legacy answer and the gateway's, or
    ("", "") when they are the same for a visitor."""
    if via.get("refusal"):
        return "refused", via["refusal"].get("reason", "")
    resp = via.get("response") or {}
    if _slash_redirect(direct) and resp.get("status") == 200:
        return "", ""  # the page itself, without the site's detour to its other spelling
    if resp.get("status") != direct.status:
        return "status", "%s directly, %s through Equalizer" % (direct.status, resp.get("status"))
    if 300 <= direct.status < 400:  # the same place, wherever each says it is
        there = urllib.parse.urlsplit(direct.location).path
        here = urllib.parse.urlsplit((resp.get("headers") or {}).get("Location", "")).path
        return ("", "") if there == here else ("status", "redirects to %s instead of %s" % (here, there))
    if direct.content_type != "text/html":
        return "", ""
    base = urllib.parse.urlsplit(direct.url).path or "/"
    a = _page(direct.body, direct.headers.get("content-type", ""), base)
    b = _page(resp.get("body") or b"", (resp.get("headers") or {}).get("Content-Type", ""), base)
    a.words, b.words = _same_site(a.words, hosts), _same_site(b.words, hosts)
    ratio = difflib.SequenceMatcher(None, a.words, b.words, autojunk=False).ratio() if a.words or b.words else 1
    if ratio < TEXT_SIMILAR:
        return "text", "%d%% of the visible text is the same%s" % (int(ratio * 100), _words_apart(a.words, b.words))
    lost = sorted(_unslashed(a.links) - _unslashed(b.links))
    if lost:
        return "links", "links missing: " + ", ".join(lost[:5])
    if len(a.forms) != len(b.forms):
        return "forms", "%d forms directly, %d through Equalizer" % (len(a.forms), len(b.forms))
    return "", ""


@dataclass
class Measure:
    """What one round of checks found."""
    broken: list = field(default_factory=list)
    open: list = field(default_factory=list)
    refused_fields: dict = field(default_factory=dict)  # entry -> field the gateway refused on a studied page
    identical: int = 0
    compared: int = 0
    area: int = 0
    inputs: int = 0
    protected: int = 0
    accounts: list = field(default_factory=list)  # labels whose pages were verified
    tried: set = field(default_factory=set)  # inputs already tried, in any session

    def score(self) -> dict:
        return {"functions_total": self.compared, "functions_identical": self.identical,
                "inputs_total": self.inputs, "inputs_protected": self.protected,
                "area_percent": round(100.0 * self.area / self.compared, 1) if self.compared else 0}


def _words_apart(a: list, b: list) -> str:
    """A few words that differ, so a person can tell noise from a fault."""
    only_a, only_b = [], []
    for op, i1, i2, j1, j2 in difflib.SequenceMatcher(None, a, b, autojunk=False).get_opcodes():
        if op in ("replace", "delete"):
            only_a += a[i1:i2]
        if op in ("replace", "insert"):
            only_b += b[j1:j2]
    cut = lambda ws: " ".join(w[:30] for w in ws[:6])
    return "; directly: %s; through Equalizer: %s" % (cut(only_a) or "-", cut(only_b) or "-")


class Run:
    def __init__(self, job: dict, cfg, log):
        self.job, self.log, self.cfg = job, log, cfg
        self.f = Fetcher(job["base_url"], job["reach"], job["tenant"], cfg.relay_url, cfg.relay_token, cfg.direct, cfg.rate)
        meta = job["document"].get("meta") or {}
        self.hosts = tuple(h for h in (urllib.parse.urlsplit(job["base_url"]).netloc, meta.get("public_domain"),
                                       *(meta.get("aliases") or [])) if h)

    def measure(self, doc: dict, try_map: dict) -> Measure:
        """Both checks of one version: public pages, then each test account's."""
        m = Measure()
        gw = Gateway(self.cfg.gateway_url, self.cfg.panel_token, try_map)
        self.f.cookies = {}
        sessions = [(None, self.job["pages"], Jar(), None)]
        for acc in self.job.get("accounts") or []:
            c = crawler.Crawler(self.f, self.job.get("kind"), 10, self.log)
            c.cms = self.job.get("kind")
            if not c.login_only(acc, self.job["pages"]):
                m.open.append({"kind": "path", "target": "(pages of %s)" % acc["label"], "reason": "other",
                               "detail": "not verified: the test account could not log in"})
                continue
            role = (doc.get("roles") or {}).get("anonymous", "guest")
            sessions.append((acc["label"], (self.job.get("pages_by_account") or {}).get(acc["label"], []), Jar(),
                             {"role": role, "legacy_cookies": dict(self.f.cookies)}))
            m.accounts.append(acc["label"])
            self.f.cookies = {}
        for label, pages, jar, session in sessions:
            if session is not None:  # log in again: each account's checks run with its own session
                c = crawler.Crawler(self.f, self.job.get("kind"), 10, self.log)
                c.cms = self.job.get("kind")
                c.login_only(next(a for a in self.job["accounts"] if a["label"] == label), self.job["pages"])
                session = dict(session, legacy_cookies=dict(self.f.cookies))
            rendered = self._pages(gw, m, pages, jar, session)
            self._routes(gw, doc, m, pages, jar)
            self._forms(gw, doc, m, rendered)
            self.f.cookies = {}
        return m

    # --- 1. the site still works -------------------------------------------
    def _pages(self, gw, m, pages, jar, session) -> dict:
        rendered = {}
        for pq in pages:
            try:
                direct = self.f.get(pq)
                via = gw.preview(pq, jar.header(), session=session if not jar.cookies else None)
            except (ReachError, OSError, ValueError):
                continue  # not reachable now: neither identical nor broken
            jar.keep(via.get("cookies"))
            m.compared += 1
            m.area += 1 if via.get("area") else 0
            diff, detail = compare(direct, via, self.hosts)
            if diff:
                m.broken.append({"path": pq, "entry": via.get("entry"), "difference": diff, "detail": detail[:300]})
                field_ = ((via.get("refusal") or {}).get("field"))
                if via.get("entry"):
                    m.refused_fields.setdefault(via["entry"], set()).add(field_ or "")
                continue
            m.identical += 1
            resp = via.get("response") or {}
            if direct.content_type == "text/html":
                rendered[pq] = (_page(resp.get("body") or b"", (resp.get("headers") or {}).get("Content-Type", ""), pq.split("?")[0]),
                                dict(jar.cookies))
        return rendered

    # --- 2. attacks are refused ----------------------------------------------
    def _tried(self, m, target: str, send) -> None:
        """Sends every attack with send(value) -> verdict; counts the input."""
        if target in m.tried:
            return  # already tried in another session
        let_through = []
        for reason, value in attacks.values():
            try:
                if send(value).get("accepted"):
                    let_through.append(reason)
            except TryError:
                return
        m.inputs += 1
        m.tried.add(target)
        if not let_through:
            m.protected += 1
            return
        others = [WORDS[r] for r in dict.fromkeys(let_through) if r != let_through[0]]
        m.open.append({"kind": "input", "target": target, "reason": let_through[0],
                       "detail": "also " + ", ".join(others) if others else ""})

    def _routes(self, gw, doc, m, pages, jar):
        for r in doc.get("routes", []):
            if r.get("method", "GET") != "GET":
                continue
            sample = next((pq for pq in pages if mapbuild.fits(doc, r, pq.split("?")[0])), None)
            if sample is None and "{" in r.get("path", ""):
                continue  # no page to start from
            if sample is None and pages is not self.job["pages"]:
                continue  # literal routes are tried once, in the public session
            path, _, q = (sample or r["path"]).partition("?")
            query = dict(urllib.parse.parse_qsl(q, keep_blank_values=True))
            for name, spec in (r.get("query") or {}).items():
                base = dict(query)
                base.setdefault(name, attacks.valid(spec))

                def send(v, name=name, base=base, path=path):
                    return gw.dry_run("GET", path + "?" + urllib.parse.urlencode(dict(base, **{name: v})), jar.header())
                self._tried(m, r["id"] + "." + name, send)
            segs = path.split("/")
            for i, pat in enumerate(r.get("path", "").split("/")):
                if pat.startswith("{") and i < len(segs):
                    def send(v, i=i):
                        s = list(segs)
                        s[i] = urllib.parse.quote(v, safe="")
                        return gw.dry_run("GET", "/".join(s) + ("?" + q if q else ""), jar.header())
                    self._tried(m, r["id"] + "." + pat.strip("{}"), send)

    def _forms(self, gw, doc, m, rendered):
        for fm in doc.get("forms", []):
            fields = fm.get("fields") or {}
            found = next(((pq, form, cookies) for pq, (page, cookies) in rendered.items() for form in page.forms
                          if form["method"] == "POST" and "multipart" not in form["enctype"]
                          and set(fields) <= set(form["inputs"])), None)
            if found is None:
                continue
            pq, form, cookies = found
            action = form["action"] or pq.split("?")[0]
            headers = {"Cookie": "; ".join(f"{k}={v}" for k, v in cookies.items()),
                       "Content-Type": "application/x-www-form-urlencoded"}
            good = dict(form["inputs"], **{n: attacks.valid(s) for n, s in fields.items()})

            def send(values, action=action, headers=headers):
                return gw.dry_run("POST", action, headers, urllib.parse.urlencode(values).encode())
            try:
                baseline = send(good)
            except TryError:
                continue
            if not baseline.get("accepted"):
                if any(s.get("pattern") for s in fields.values()):
                    m.open.append({"kind": "input", "target": fm["id"], "reason": "other",
                                   "detail": "not tried: no valid submission could be built"})
                else:
                    ref = baseline.get("refusal") or {}
                    m.broken.append({"path": pq, "entry": fm["id"], "difference": "refused",
                                     "detail": ("a valid submission is refused: %s %s" % (ref.get("reason", ""), ref.get("field") or "")).strip()})
                    m.refused_fields.setdefault(fm["id"], set()).add(ref.get("field") or "")
                continue
            for name in fields:
                self._tried(m, fm["id"] + "." + name, lambda v, name=name, good=good: send(dict(good, **{name: v})))


def _public(m: Measure) -> dict:
    return {"score": m.score(), "broken": m.broken, "still_open": m.open}


def run(job: dict, cfg, panel, log) -> None:
    try:
        r = Run(job, cfg, log)
    except ReachError as e:
        panel.verification_result(job["id"], error=str(e))
        return
    try:
        doc, tm = job["document"], job["try_map"]
        first = r.measure(doc, tm)
        last, corrections = first, []
        if job.get("correct"):
            doc, last, corrections = correct.loop(job, doc, first, r, panel, log)
    except TryError as e:
        panel.verification_result(job["id"], error=str(e))
        return
    log.info("verification finished", verification=job["id"], app=job["app"], version=job["version"],
             corrections=len(corrections), accounts=len(first.accounts), **last.score())
    if corrections:
        panel.verification_result(job["id"], document=doc, corrections=corrections, before=first.score(), **_public(last))
    else:
        panel.verification_result(job["id"], **_public(last))
