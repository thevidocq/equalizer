"""How much Equalizer adds to a visit: the same pages of the lab sites opened
directly on the legacy server and through Equalizer (front, gateway, relay,
connector), on a running lab after the onboarding test (tests 01).

    python3 perf.py [--rounds 30] [--browser] [--load SECONDS] > perf.json

1. pages: each page fetched by a client that keeps its connection, the two
   ways alternated, `rounds` times; median and p95 in milliseconds.
2. --browser: the page loaded whole in Chrome (images, styles, scripts),
   cold (new profile) and warm (same profile), until the load event.
3. --load: 10 clients at once on each site's home page for SECONDS, the two
   ways one after the other: pages per second and median time.

Everything runs on one machine: this measures Equalizer's own cost; a real
deployment adds the network between the gateway and the customer's server."""

import argparse
import concurrent.futures
import json
import os
import re
import statistics
import sys
import threading
import time
import urllib.parse

import requests
from requests.adapters import HTTPAdapter

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "tests"))
from lab import CA, HTTP_SITES, RUN, SITES  # noqa: E402

ALL = dict(SITES, wordpress=("www.acme.test", "blog", "127.0.0.1:18090", "Acme Blog"))
SKIP = re.compile(r"logout|login|wp-admin|administrator|/admin|delete|cart|checkout|wishlist|compare|"
                  r"action=|token|\.(css|js|png|jpe?g|gif|svg|ico|woff2?|xml|json|pdf)(\?|$)", re.I)


def public(domain: str) -> str:
    return ("http" if domain in HTTP_SITES else "https") + "://" + domain


def pages(domain: str, address: str, n: int = 6) -> list:
    """The home page and the first internal pages it links to."""
    s = requests.Session()
    r = s.get(f"http://{address}/", headers={"Host": domain}, timeout=60, allow_redirects=False)
    out = ["/"]
    if r.is_redirect:  # PrestaShop's / → /index.php, MediaWiki's / → /index.php/Main_Page
        first = urllib.parse.urlsplit(r.headers["location"])
        out = [(first.path or "/") + ("?" + first.query if first.query else "")]
        r = s.get(f"http://{address}{out[0]}", headers={"Host": domain}, timeout=60, allow_redirects=False)
    for href in re.findall(r'href="([^"#]+)"', r.text):
        u = urllib.parse.urlsplit(urllib.parse.urljoin(f"http://{domain}/", href.replace("&amp;", "&")))
        pq = (u.path or "/") + ("?" + u.query if u.query else "")
        if u.netloc != domain or pq in out or SKIP.search(pq):
            continue
        try:
            a = s.get(f"http://{address}{pq}", headers={"Host": domain}, timeout=60, allow_redirects=False)
            b = requests.get(public(domain) + pq, verify=CA, timeout=60, allow_redirects=False)
        except requests.RequestException:
            continue
        if a.status_code == 200 and b.status_code == 200 and "text/html" in a.headers.get("content-type", ""):
            out.append(pq)
        if len(out) >= n:
            break
    return out


class FromAddress(HTTPAdapter):
    """Connections from one 127.0.0.x address: Equalizer limits each visitor
    address (120 pages a minute in the templates), and a benchmark is many
    visitors, not one that would be slowed down on purpose."""

    def __init__(self, ip: str):
        self.ip = ip
        super().__init__()

    def init_poolmanager(self, *a, **kw):
        kw["source_address"] = (self.ip, 0)
        super().init_poolmanager(*a, **kw)


_next_ip = [2]
_ip_lock = threading.Lock()


def visitor() -> requests.Session:
    """A new visitor: its own address and connections."""
    with _ip_lock:
        n = _next_ip[0]
        _next_ip[0] = n + 1 if n < 64000 else 2
    s = requests.Session()
    s.verify = CA
    a = FromAddress(f"127.0.{n // 250}.{n % 250 + 2}")
    s.mount("https://", a)
    s.mount("http://", a)
    return s


def timed(session, url, **kw) -> tuple:
    t = time.perf_counter()
    r = session.get(url, timeout=60, allow_redirects=False, **kw)
    return (time.perf_counter() - t) * 1000, r.status_code, len(r.content)


def stats(xs: list) -> dict:
    xs = sorted(xs)
    return {"median": round(statistics.median(xs), 1), "p95": round(xs[max(0, int(len(xs) * 0.95) - 1)], 1)}


def measure_pages(rounds: int) -> dict:
    out = {}
    for kind, (domain, _, address, _) in ALL.items():
        site = {}
        for pq in pages(domain, address):
            direct, via = requests.Session(), visitor()
            timed(direct, f"http://{address}{pq}", headers={"Host": domain})  # warm both
            timed(via, public(domain) + pq)
            d, v, sizes, other = [], [], None, 0
            for _ in range(rounds):
                a = timed(direct, f"http://{address}{pq}", headers={"Host": domain})
                b = timed(via, public(domain) + pq)
                if a[1] != b[1]:
                    other += 1  # not the same answer: not compared
                    continue
                d.append(a[0])
                v.append(b[0])
                sizes = (a[2], b[2])
            if not d:
                continue
            site[pq] = {"direct": stats(d), "equalizer": stats(v), "bytes": sizes, "samples": len(d),
                        "different_answers": other, "added_ms": round(statistics.median(v) - statistics.median(d), 1)}
        out[domain] = site
        print(domain, "done", file=sys.stderr)
    return out


def measure_browser(reps: int = 5) -> dict:
    from playwright.sync_api import sync_playwright
    exe = "/usr/bin/google-chrome"
    rules = ", ".join(f"MAP {d} {a}" for d, _, a, _ in ALL.values())
    out = {}
    with sync_playwright() as p:
        ways = {"direct": p.chromium.launch(executable_path=exe, args=["--no-sandbox", f"--host-resolver-rules={rules}"]),
                "equalizer": p.chromium.launch(executable_path=exe, args=["--no-sandbox"])}
        for kind, (domain, _, address, _) in ALL.items():
            for pq in pages(domain, address, 3):
                row = {}
                for way, browser in ways.items():
                    base = f"http://{domain}" if way == "direct" else public(domain)
                    cold, warm = [], []
                    for _ in range(reps):
                        ctx = browser.new_context(ignore_https_errors=True)
                        page = ctx.new_page()
                        for bucket in (cold, warm):
                            page.goto(base + pq, wait_until="load", timeout=120000)
                            bucket.append(page.evaluate(
                                "() => { const n = performance.getEntriesByType('navigation')[0];"
                                " return n.loadEventEnd - n.startTime; }"))
                        ctx.close()
                    row[way] = {"cold": stats(cold), "warm": stats(warm)}
                row["added_ms_cold"] = round(row["equalizer"]["cold"]["median"] - row["direct"]["cold"]["median"], 1)
                out.setdefault(domain, {})[pq] = row
            print(domain, "browser done", file=sys.stderr)
        for b in ways.values():
            b.close()
    return out


def measure_load(seconds: int, clients: int = 10) -> dict:
    out = {}
    for domain, address in (("www.acme.test", "127.0.0.1:18090"), ("cart.acme.test", "127.0.0.1:18101"),
                            ("wiki.acme.test", "127.0.0.1:18104")):
        row = {}
        for way in ("direct", "equalizer"):
            path = "/index.php/Main_Page" if domain == "wiki.acme.test" else "/"
            url = (f"http://{address}" if way == "direct" else public(domain)) + path
            times, errors, stop = [], [0], time.time() + seconds
            lock = threading.Lock()

            def client():
                # through Equalizer: 25 visitors per client, in turn, each
                # well under its address's limit
                pool = [visitor() for _ in range(25)] if way == "equalizer" else [requests.Session()]
                n = 0
                while time.time() < stop:
                    s = pool[n % len(pool)]
                    n += 1
                    try:
                        ms, status, _ = timed(s, url, headers={"Host": domain} if way == "direct" else {})
                        with lock:
                            (times.append(ms) if status == 200 else errors.__setitem__(0, errors[0] + 1))
                    except requests.RequestException:
                        with lock:
                            errors[0] += 1
            with concurrent.futures.ThreadPoolExecutor(clients) as ex:
                for _ in range(clients):
                    ex.submit(client)
            row[way] = {"pages_per_second": round(len(times) / seconds, 1), "errors": errors[0], **stats(times or [0])}
        out[domain] = row
        print(domain, "load done", file=sys.stderr)
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--rounds", type=int, default=30)
    ap.add_argument("--browser", action="store_true")
    ap.add_argument("--load", type=int, default=0)
    a = ap.parse_args()
    os.environ["NO_PROXY"] = os.environ["no_proxy"] = "127.0.0.1,localhost,.lab.test,.acme.test,acme.test"
    res = {"lab": str(RUN), "cpus": os.cpu_count(), "pages": measure_pages(a.rounds)}
    if a.browser:
        res["browser"] = measure_browser()
    if a.load:
        res["load"] = measure_load(a.load)
    json.dump(res, sys.stdout, indent=1)


if __name__ == "__main__":
    main()
