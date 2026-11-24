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
import time
import urllib.parse
from html.parser import HTMLParser

from dataclasses import dataclass, field

from . import attacks, correct, crawler, mapbuild, verifyassets
from .fetch import Fetcher, ReachError
from .gateway import Gateway, Jar, TryError

WORDS = {"accepts_sql": "SQL", "accepts_markup": "HTML", "accepts_template": "template code", "accepts_path": "paths",
         "accepts_shell": "shell characters", "other": "out-of-type values"}
TEXT_SIMILAR = 0.97  # dates, counters and tokens differ between two loads
SELF_FLOOR = 0.8  # two loads less alike than this are not the same page
AGAIN = 3  # more loads of a page that differs, to tell what the site varies by itself


class _Page(HTMLParser):
    """Visible words, internal link paths and forms of a page."""

    def __init__(self, base: str = "/"):
        super().__init__(convert_charrefs=True)
        self.base = base  # the page's own path: relative links resolve against it
        self.words, self.links, self.forms, self._skip, self._form = [], set(), [], 0, None

    def _path(self, href: str) -> str:
        """The path a link leads to, decoded: %ce%b5 and %CE%B5 are the same
        address for a browser and for the site."""
        u = urllib.parse.urlsplit(href)
        if u.netloc:  # the legacy host or the public one: the path is what counts
            return _decoded(u.path or "/")
        return _decoded(urllib.parse.urlsplit(urllib.parse.urljoin(self.base, href)).path or "/")

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


def _decoded(path: str) -> str:
    """A path as the site reads it: %ce%b5, %CE%B5 and the raw letter are
    the same address."""
    return urllib.parse.unquote(path, errors="surrogateescape")


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


def _stable(a, others: list) -> tuple:
    """What the site does not vary across its loads (a and the others):
    (its words, the links every load shows, the links some loads show, how
    many of those each load shows at least), or None when a load is too far
    from a to be the same page (a login wall, a challenge). Several loads: a
    shop that shows four related products at random out of a few dozen
    repeats one now and then between two loads, rarely across four."""
    if any(_ratio(a.words, c.words) < SELF_FLOOR for c in others):
        return None
    words = a.words
    for c in others:
        m = difflib.SequenceMatcher(None, words, c.words, autojunk=False)
        words = [w for blk in m.get_matching_blocks() for w in words[blk.a:blk.a + blk.size]]
    sets = [_unslashed(a.links)] + [_unslashed(c.links) for c in others]
    every, some = set.intersection(*sets), set.union(*sets)
    return words, every, some, min(len(x - every) for x in sets)


def _parent(path: str) -> str:
    return path.rstrip("/").rsplit("/", 1)[0]


def _kept(stable: list, words: list) -> float:
    """Share of the site's stable words the gateway's page still has, in order."""
    if not stable:
        return 1.0
    m = difflib.SequenceMatcher(None, stable, words, autojunk=False)
    return sum(blk.size for blk in m.get_matching_blocks()) / len(stable)


def _ratio(a: list, b: list) -> float:
    return difflib.SequenceMatcher(None, a, b, autojunk=False).ratio() if a or b else 1


def _slash_redirect(direct) -> bool:
    """The site's answer only sends to the same path with or without its
    final slash."""
    if not 300 <= direct.status < 400 or not direct.location:
        return False
    there = _decoded(urllib.parse.urlsplit(direct.location).path)
    here = _decoded(urllib.parse.urlsplit(direct.url).path)
    return there != here and there.rstrip("/") == here.rstrip("/")


def compare(direct, via, hosts: tuple = (), again=None) -> tuple:
    """(difference, detail) between the legacy answer and the gateway's, or
    ("", "") when they are the same for a visitor. again, more answers of
    the site to the same request (one or a list), excuses what the site
    varies by itself:
    the gateway may differ from the first answer as much as the site
    differs from itself in text, and lose only links the site itself
    shows or hides from one load to the next."""
    if via.get("refusal"):
        return "refused", via["refusal"].get("reason", "")
    resp = via.get("response") or {}
    if _slash_redirect(direct) and resp.get("status") == 200:
        return "", ""  # the page itself, without the site's detour to its other spelling
    if resp.get("status") != direct.status:
        return "status", "%s directly, %s through Equalizer" % (direct.status, resp.get("status"))
    if 300 <= direct.status < 400:  # the same place, wherever each says it is
        there = _decoded(urllib.parse.urlsplit(direct.location).path)
        here = _decoded(urllib.parse.urlsplit((resp.get("headers") or {}).get("Location", "")).path)
        return ("", "") if there == here else ("status", "redirects to %s instead of %s" % (here, there))
    if direct.content_type != "text/html":
        return "", ""
    base = urllib.parse.urlsplit(direct.url).path or "/"
    a = _page(direct.body, direct.headers.get("content-type", ""), base)
    b = _page(resp.get("body") or b"", (resp.get("headers") or {}).get("Content-Type", ""), base)
    a.words, b.words = _same_site(a.words, hosts), _same_site(b.words, hosts)
    site = None
    if again:
        others = []
        for r in again if isinstance(again, list) else [again]:
            c = _page(r.body, r.headers.get("content-type", ""), base)
            c.words = _same_site(c.words, hosts)
            others.append(c)
        site = _stable(a, others)
    ratio = _ratio(a.words, b.words)
    if ratio < TEXT_SIMILAR and (site is None or _kept(site[0], b.words) < TEXT_SIMILAR):
        return "text", "%d%% of the visible text is the same%s" % (int(ratio * 100), _words_apart(a.words, b.words))
    la, lb = _unslashed(a.links), _unslashed(b.links)
    lost = la - lb
    if lost and site is not None and site[1] != site[2]:
        _, every, some, shown = site
        new = lb - la
        # links the site shows or hides by itself are excused when the
        # gateway's page shows as many of that kind as the site does: other
        # random products, not a block that went missing. Either the lost
        # links are among those the site varies, or each has a stand-in
        # beside it (another /product/... where a /product/... went)
        varied = lost <= some - every and len(lb - every) >= shown
        swapped = len(new) >= len(lost) and all(any(_parent(n) == _parent(x) for n in new) for x in lost)
        if varied or swapped:
            lost = set()
    lost = sorted(lost)
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
    changing: int = 0  # identical once what the site varies by itself is set aside
    compared: int = 0
    area: int = 0
    inputs: int = 0
    protected: int = 0
    accounts: list = field(default_factory=list)  # labels whose pages were verified
    tried: set = field(default_factory=set)  # inputs already tried, in any session
    files: list = field(default_factory=list)  # URLs of the files the pages name
    asset_fixes: dict = field(default_factory=dict)  # asset prefix -> {"types", "max_size_kb"} that would mend it
    broken_files: set = field(default_factory=set)  # broken paths that are files, not pages

    def score(self) -> dict:
        return {"functions_total": self.compared, "functions_identical": self.identical,
                "inputs_total": self.inputs, "inputs_protected": self.protected,
                "area_percent": round(100.0 * self.area / self.compared, 1) if self.compared else 0,
                "functions_changing": self.changing}


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
    def __init__(self, job: dict, cfg, log, progress=None):
        self.job, self.log, self.cfg = job, log, cfg
        self.progress = progress  # progress(step, done, total, account): the panel's view of where it is
        self.step = "pages"
        self._told = 0.0
        self.f = Fetcher(job["base_url"], job["reach"], job["tenant"], cfg.relay_url, cfg.relay_token, cfg.direct, cfg.rate)
        meta = job["document"].get("meta") or {}
        self.hosts = tuple(h for h in (urllib.parse.urlsplit(job["base_url"]).netloc, meta.get("public_domain"),
                                       *(meta.get("aliases") or [])) if h)
        self.f.add_hosts(self.hosts)

    def tell(self, step: str, done: int, total: int, account=None, force: bool = False) -> None:
        """Tells the panel where the check is, at most every 2 seconds; the
        log every 50 pages."""
        if step == "pages" and done and done % 50 == 0:
            self.log.info("verification progress", verification=self.job["id"], step=step, done=done, total=total,
                          account=account)
        now = time.monotonic()
        if self.progress and (force or now - self._told >= 2):
            self._told = now
            self.progress("correcting" if self.step == "correcting" and step == "pages" else step, done, total, account)

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
            rendered = self._pages(gw, m, pages, jar, session, label)
            self.tell("attacks", 0, len(doc.get("routes", [])) + len(doc.get("forms", [])), label, force=True)
            self._routes(gw, doc, m, pages, jar, label)
            self._forms(gw, doc, m, rendered, label)
            self.f.cookies = {}
        self.tell("files", 0, 1, None, force=True)
        verifyassets.check(gw, self.f, doc, m, m.files, self.job.get("kind"))  # public files, as an anonymous visitor
        return m

    # --- 1. the site still works -------------------------------------------
    def _pages(self, gw, m, pages, jar, session, label=None) -> dict:
        rendered = {}
        for n, pq in enumerate(pages):
            self.tell("pages", n, len(pages), label, force=n == 0)
            try:
                direct = self.f.get(pq)
                via = gw.preview(pq, jar.header(), session=session if not jar.cookies else None)
            except (ReachError, OSError, ValueError):
                continue  # not reachable now: neither identical nor broken
            jar.keep(via.get("cookies"))
            if session is None and direct.status == 200 and direct.content_type == "text/html":
                m.files += verifyassets.names(direct.body, direct.url)
            m.compared += 1
            m.area += 1 if via.get("area") else 0
            diff, detail = compare(direct, via, self.hosts)
            if diff in ("text", "links"):
                again = self._again(pq, direct)
                if again and compare(direct, _as_via(again[0]), self.hosts)[0]:
                    # the site answers differently each time (random products):
                    # only what it varies by itself is excused
                    diff, detail = compare(direct, via, self.hosts, again)
                    m.changing += 0 if diff else 1
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

    def _again(self, pq, direct) -> list:
        """More answers of the site to the same request (AGAIN of them), or
        [] when one is not the same kind of answer (an error, a redirect to
        the login, a rate limit): then nothing is excused. The first is
        fetched alone: a site that answers the same twice needs no more."""
        out = []
        for n in range(AGAIN):
            try:
                again = self.f.get(pq)
            except (ReachError, OSError, ValueError):
                return []
            if again.status != direct.status or again.content_type != direct.content_type:
                return []
            out.append(again)
            if n == 0 and not compare(direct, _as_via(again), self.hosts)[0]:
                return out  # the site does not vary: one load says so
        return out

    # --- 2. attacks are refused ----------------------------------------------
    def _tried(self, m, target: str, send, base: str = "") -> None:
        """Sends every attack on an input of that base type with
        send(value) -> verdict; counts the input."""
        if target in m.tried:
            return  # already tried in another session
        let_through = []
        for reason, value in attacks.values(base):
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

    def _routes(self, gw, doc, m, pages, jar, label=None):
        total = len(doc.get("routes", [])) + len(doc.get("forms", []))
        for n, r in enumerate(doc.get("routes", [])):
            self.tell("attacks", n, total, label)
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
                self._tried(m, r["id"] + "." + name, send, _base(doc, spec))
            segs = path.split("/")
            for i, pat in enumerate(r.get("path", "").split("/")):
                if pat.startswith("{") and i < len(segs):
                    def send(v, i=i):
                        s = list(segs)
                        s[i] = urllib.parse.quote(v, safe="")
                        return gw.dry_run("GET", "/".join(s) + ("?" + q if q else ""), jar.header())
                    self._tried(m, r["id"] + "." + pat.strip("{}"), send,
                                _base(doc, (r.get("params") or {}).get(pat.strip("{}"), {})))

    def _forms(self, gw, doc, m, rendered, label=None):
        routes = len(doc.get("routes", []))
        for n, fm in enumerate(doc.get("forms", [])):
            self.tell("attacks", routes + n, routes + len(doc.get("forms", [])), label)
            fields = fm.get("fields") or {}
            # the page's form that posts to this entry (another form with the
            # same field names, such as a search, carries another token)
            found = next(((pq, form, cookies) for pq, (page, cookies) in rendered.items() for form in page.forms
                          if form["method"] == "POST" and "multipart" not in form["enctype"]
                          and _action_path(pq, form["action"]).rstrip("/") == (fm.get("route") or "/").rstrip("/")
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
                self._tried(m, fm["id"] + "." + name, lambda v, name=name, good=good: send(dict(good, **{name: v})),
                            _base(doc, fields[name]))


def _base(doc: dict, spec: dict) -> str:
    """The base type of a field: its own, or that of the type it names."""
    t = spec.get("type", "string")
    return ((doc.get("types") or {}).get(t) or {}).get("base", t)


def _as_via(r) -> dict:
    """A direct answer shaped like the gateway's preview, to compare the site
    with itself."""
    h = {"Content-Type": r.headers.get("content-type", "")}
    if r.location:
        h["Location"] = r.location
    return {"accepted": True, "refusal": None, "response": {"status": r.status, "headers": h, "body": r.body}}


def _action_path(page_pq: str, action: str) -> str:
    return _decoded(urllib.parse.urlsplit(urllib.parse.urljoin(page_pq, action or page_pq)).path or "/")


def _public(m: Measure) -> dict:
    return {"score": m.score(), "broken": m.broken, "still_open": m.open}


def run(job: dict, cfg, panel, log) -> None:
    log.info("verification started", verification=job["id"], app=job["app"], version=job["version"],
             pages=len(job.get("pages") or []), accounts=len(job.get("accounts") or []))
    try:
        r = Run(job, cfg, log, lambda step, done, total, account: panel.verification_progress(job["id"], step, done, total, account))
    except ReachError as e:
        panel.verification_result(job["id"], error=str(e))
        return
    try:
        doc, tm = job["document"], job["try_map"]
        first = r.measure(doc, tm)
        last, corrections = first, []
        if job.get("correct"):
            r.step = "correcting"
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
