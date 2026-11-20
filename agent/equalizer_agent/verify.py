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

from . import attacks, mapbuild
from .fetch import Fetcher, ReachError
from .gateway import Gateway, TryError, cookie_header

WORDS = {"accepts_sql": "SQL", "accepts_markup": "HTML", "accepts_template": "template code", "accepts_path": "paths",
         "accepts_shell": "shell characters", "other": "out-of-type values"}
TEXT_SIMILAR = 0.97  # dates, counters and tokens differ between two loads


class _Page(HTMLParser):
    """Visible words, internal link paths and forms of a page."""

    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.words, self.links, self.forms, self._skip, self._form = [], set(), [], 0, None

    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        if tag in ("script", "style", "noscript", "template"):
            self._skip += 1
        elif tag == "a" and a.get("href"):
            u = urllib.parse.urlsplit(a["href"])
            if not u.netloc or u.scheme in ("http", "https"):
                self.links.add(u.path or "/")
        elif tag == "form":
            self._form = {"action": urllib.parse.urlsplit(a.get("action") or "").path, "method": (a.get("method") or "get").upper(),
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


def _page(body: bytes, content_type: str = "") -> _Page:
    p = _Page()
    try:
        text = body.decode(_charset(content_type), "replace")
    except LookupError:
        text = body.decode("utf-8", "replace")
    try:
        p.feed(text)
    except Exception:  # a broken page is compared by its status alone
        pass
    return p


def compare(direct, via) -> tuple:
    """(difference, detail) between the legacy answer and the gateway's, or
    ("", "") when they are the same for a visitor."""
    if via.get("refusal"):
        return "refused", via["refusal"].get("reason", "")
    resp = via.get("response") or {}
    if resp.get("status") != direct.status:
        return "status", "%s directly, %s through Equalizer" % (direct.status, resp.get("status"))
    if 300 <= direct.status < 400:  # the same place, wherever each says it is
        there = urllib.parse.urlsplit(direct.location).path
        here = urllib.parse.urlsplit((resp.get("headers") or {}).get("Location", "")).path
        return ("", "") if there == here else ("status", "redirects to %s instead of %s" % (here, there))
    if direct.content_type != "text/html":
        return "", ""
    a = _page(direct.body, direct.headers.get("content-type", ""))
    b = _page(resp.get("body") or b"", (resp.get("headers") or {}).get("Content-Type", ""))
    ratio = difflib.SequenceMatcher(None, a.words, b.words, autojunk=False).ratio() if a.words or b.words else 1
    if ratio < TEXT_SIMILAR:
        return "text", "%d%% of the visible text is the same" % int(ratio * 100)
    lost = sorted(a.links - b.links)
    if lost:
        return "links", "links missing: " + ", ".join(lost[:5])
    if len(a.forms) != len(b.forms):
        return "forms", "%d forms directly, %d through Equalizer" % (len(a.forms), len(b.forms))
    return "", ""


class Run:
    def __init__(self, job: dict, cfg, log):
        self.job, self.log, self.doc = job, log, job["document"]
        self.f = Fetcher(job["base_url"], job["reach"], job["tenant"], cfg.relay_url, cfg.relay_token, cfg.direct, cfg.rate)
        self.gw = Gateway(cfg.gateway_url, cfg.panel_token, job["try_map"])
        self.broken, self.open, self.rendered = [], [], {}  # rendered: page -> (_Page, cookies)
        self.identical = self.compared = self.area = 0
        self.inputs = self.protected = 0

    # --- 1. the site still works -------------------------------------------
    def pages(self):
        for pq in self.job["pages"]:
            try:
                direct = self.f.get(pq)
                via = self.gw.preview(pq)
            except (ReachError, OSError, ValueError):
                continue  # not reachable now: neither identical nor broken
            self.compared += 1
            self.area += 1 if via.get("area") else 0
            diff, detail = compare(direct, via)
            if diff:
                self.broken.append({"path": pq, "entry": via.get("entry"), "difference": diff, "detail": detail[:300]})
                continue
            self.identical += 1
            resp = via.get("response") or {}
            if direct.content_type == "text/html":
                self.rendered[pq] = (_page(resp.get("body") or b"", (resp.get("headers") or {}).get("Content-Type", "")),
                                     via.get("cookies") or [])

    # --- 2. attacks are refused ----------------------------------------------
    def _tried(self, target: str, send) -> None:
        """Sends every attack with send(value) -> verdict; counts the input."""
        let_through = []
        for reason, value in attacks.values():
            try:
                if send(value).get("accepted"):
                    let_through.append(reason)
            except TryError:
                return
        self.inputs += 1
        if not let_through:
            self.protected += 1
            return
        others = [WORDS[r] for r in dict.fromkeys(let_through) if r != let_through[0]]
        self.open.append({"kind": "input", "target": target, "reason": let_through[0],
                          "detail": "also " + ", ".join(others) if others else ""})

    def routes(self):
        for r in self.doc.get("routes", []):
            if r.get("method", "GET") != "GET":
                continue
            sample = next((pq for pq in self.job["pages"] if mapbuild.fits(self.doc, r, pq.split("?")[0])), None)
            if sample is None and "{" in r.get("path", ""):
                continue  # no page to start from
            path, _, q = (sample or r["path"]).partition("?")
            query = dict(urllib.parse.parse_qsl(q, keep_blank_values=True))
            for name, spec in (r.get("query") or {}).items():
                base = dict(query)
                base.setdefault(name, attacks.valid(spec))

                def send(v, name=name, base=base, path=path):
                    return self.gw.dry_run("GET", path + "?" + urllib.parse.urlencode(dict(base, **{name: v})))
                self._tried(r["id"] + "." + name, send)
            segs = path.split("/")
            for i, pat in enumerate(r.get("path", "").split("/")):
                if pat.startswith("{") and i < len(segs):
                    def send(v, i=i):
                        s = list(segs)
                        s[i] = urllib.parse.quote(v, safe="")
                        return self.gw.dry_run("GET", "/".join(s) + ("?" + q if q else ""))
                    self._tried(r["id"] + "." + pat.strip("{}"), send)

    def forms(self):
        for fm in self.doc.get("forms", []):
            fields = fm.get("fields") or {}
            found = next(((pq, form, cookies) for pq, (page, cookies) in self.rendered.items() for form in page.forms
                          if form["method"] == "POST" and "multipart" not in form["enctype"]
                          and set(fields) <= set(form["inputs"])), None)
            if found is None:
                for name in fields:
                    self.open.append({"kind": "input", "target": fm["id"] + "." + name, "reason": "other",
                                      "detail": "not tried: no studied page shows this form"})
                continue
            pq, form, cookies = found
            action = form["action"] or pq.split("?")[0]
            headers = {"Cookie": cookie_header(cookies), "Content-Type": "application/x-www-form-urlencoded"}
            good = dict(form["inputs"], **{n: attacks.valid(s) for n, s in fields.items()})

            def send(values, action=action, headers=headers):
                return self.gw.dry_run("POST", action, headers, urllib.parse.urlencode(values).encode())
            try:
                baseline = send(good)
            except TryError:
                continue
            if not baseline.get("accepted"):
                if any(s.get("pattern") for s in fields.values()):
                    self.open.append({"kind": "input", "target": fm["id"], "reason": "other",
                                      "detail": "not tried: no valid submission could be built"})
                else:
                    ref = baseline.get("refusal") or {}
                    self.broken.append({"path": pq, "entry": fm["id"], "difference": "refused",
                                        "detail": ("a valid submission is refused: %s %s" % (ref.get("reason", ""), ref.get("field") or "")).strip()})
                continue
            for name in fields:
                self._tried(fm["id"] + "." + name, lambda v, name=name, good=good: send(dict(good, **{name: v})))

    def score(self) -> dict:
        return {"functions_total": self.compared, "functions_identical": self.identical,
                "inputs_total": self.inputs, "inputs_protected": self.protected,
                "area_percent": round(100.0 * self.area / self.compared, 1) if self.compared else 0}


def run(job: dict, cfg, panel, log) -> None:
    try:
        r = Run(job, cfg, log)
    except ReachError as e:
        panel.verification_result(job["id"], error=str(e))
        return
    try:
        r.pages()
        r.routes()
        r.forms()
    except TryError as e:
        panel.verification_result(job["id"], error=str(e))
        return
    log.info("verification finished", verification=job["id"], app=job["app"], version=job["version"], **r.score())
    panel.verification_result(job["id"], score=r.score(), broken=r.broken, still_open=r.open)

