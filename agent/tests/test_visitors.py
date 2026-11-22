"""What visitors asked for reaches the report only as an action a person can
take, after the agent has checked it on the site (docs/agent.md, "Visitor
requests"): never a scanner's guess, never a single visitor, never a file
that is not there or a key that changes nothing."""
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import crawler, visitors  # noqa: E402
from equalizer_agent.fetch import Response  # noqa: E402


def page(words: str, status: int = 200) -> Response:
    return Response(status, {"content-type": "text/html; charset=utf-8"},
                    ("<html><body><p>%s</p></body></html>" % words).encode(), "")


class FakeSite:
    """The legacy site: a path?query answers what the table says, else 404."""

    def __init__(self, table: dict):
        self.table, self.cookies, self.asked = table, {"session": "test-account"}, []

    def request(self, method: str, path_query: str, **_):
        self.asked.append((method, path_query, dict(self.cookies)))
        r = self.table.get(path_query)
        if callable(r):
            r = r()
        return r or page("not found", 404)

    def get(self, path_query: str):
        return self.request("GET", path_query)


def doc():
    return {"assets": [{"prefix": "/uploads/", "types": ["image/png"]},
                       {"prefix": "/wp-includes/", "types": ["text/css", "application/javascript"]}],
            "routes": [{"id": "home", "path": "/", "method": "GET", "legacy_request": {"method": "GET", "path": "/"},
                        "query": {"s": {"type": "string"}}},
                       {"id": "shop", "path": "/shop/", "method": "GET", "legacy_request": {"method": "GET", "path": "/shop/"}},
                       {"id": "item", "path": "/items/{id}", "method": "GET", "params": {"id": {"type": "int"}},
                        "legacy_request": {"method": "GET", "path": "/items/{{params.id}}"}}]}


def crawl():
    c = crawler.CrawlResult()
    for pq in ("/", "/shop/", "/shop/?page=2"):
        c.pages[pq] = crawler.Seen(200, "text/html", seen_by={"anonymous"})
    c.assets["/uploads/logo.png"] = "image/png"
    c.assets["/uploads/linked.pdf"] = "application/pdf"  # the site's own: the agent mends it itself
    return c


def get(path, n=2):
    return {"method": "GET", "path": path, "clients": n}


class FilesTest(unittest.TestCase):
    def test_only_real_passive_files_asked_by_several(self):
        pdf = Response(200, {"content-type": "application/pdf"}, b"%PDF-1.4", "")
        site = FakeSite({"/uploads/price-list.pdf": pdf,
                         "/uploads/brochure.pdf": Response(200, {"content-type": "text/html"}, b"<html>", "")})
        unmapped = [get("/uploads/price-list.pdf", 3),
                    get("/uploads/once.pdf", 1),  # a single visitor
                    get("/uploads/ghost.pdf", 5),  # not on the site: a scanner's guess
                    get("/uploads/brochure.pdf", 4),  # the site answers a page, not a PDF
                    get("/uploads/backup.zip", 9), get("/uploads/debug.log", 9),  # never passive
                    get("/uploads/evil.js", 9), get("/wp-includes/js/wp-embed.min.js", 9),  # scripts
                    get("/uploads/linked.pdf", 9),  # named by the site's pages
                    get("/uploads/logo.png", 9),  # its type is already allowed
                    get("/CFIDE/administrator", 9), get("/wp-login.php", 9),  # not files of an asset prefix
                    {"method": "POST", "path": "/uploads/x.pdf", "clients": 9}]
        report = {}
        visitors.confirm(report, doc(), unmapped, crawl(), site)
        self.assertEqual([(i["path"], i["add"]) for i in report["files_refused"]],
                         [("/uploads/price-list.pdf", {"prefix": "/uploads/", "type": "application/pdf"})])
        opened = {p for _, p, _ in site.asked}
        for never in ("/uploads/once.pdf", "/uploads/backup.zip", "/uploads/evil.js", "/CFIDE/administrator"):
            self.assertNotIn(never, opened)  # not even opened
        self.assertTrue(all(c == {} for _, _, c in site.asked))  # anonymously
        self.assertEqual(site.cookies, {"session": "test-account"})  # the account's session given back

    def test_sensitive_names_never_opened(self):
        site = FakeSite({})
        visitors.confirm({}, doc(), [get("/uploads/logout/x.pdf", 9)], crawl(), site)
        self.assertEqual(site.asked, [])


class KeysTest(unittest.TestCase):
    def test_only_keys_that_change_the_page(self):
        site = FakeSite({
            "/": page("Home welcome posts"), "/?prova=1": page("Home welcome posts"),  # ignored by the site
            "/shop/": page("Shop apple banana cherry"),
            "/shop/?orderby=1": page("Shop cherry banana apple"),  # sorted: the page changes
            "/shop/?view=1": page("Shop apple banana cherry"),
        })
        unmapped = [get("/?prova=", 2), get("/shop/?orderby=", 3), get("/shop/?view=", 4),
                    get("/shop/?solo=", 1),  # a single visitor: not even tried
                    get("/?s=", 9),  # declared already
                    get("/items/{id}?id=", 9)]  # a path parameter
        report = {}
        visitors.confirm(report, doc(), unmapped, crawl(), site)
        self.assertEqual([(i["route"], i["key"], i["add"]) for i in report["query_dropped"]],
                         [("/shop/", "orderby", {"route_id": "shop", "key": "orderby"})])
        self.assertNotIn("/shop/?solo=1", {p for _, p, _ in site.asked})

    def test_an_unstable_page_tells_nothing(self):
        n = iter(range(1000))
        site = FakeSite({"/shop/": lambda: page("Shop %d" % next(n), status=200 + next(n) % 2)})
        report = {}
        visitors.confirm(report, doc(), [get("/shop/?orderby=", 5)], crawl(), site)
        self.assertEqual(report["query_dropped"], [])

    def test_noise_is_not_a_change(self):
        n = iter(range(1000))
        words = " ".join("word%d" % i for i in range(200))
        site = FakeSite({"/shop/": lambda: page(words + " visit %d" % next(n)),
                         "/shop/?ref2=1": lambda: page(words + " visit %d" % next(n))})
        report = {}
        visitors.confirm(report, doc(), [get("/shop/?ref2=", 5)], crawl(), site)
        self.assertEqual(report["query_dropped"], [])

    def test_action_keys_never_opened(self):
        site = FakeSite({"/shop/": page("Shop")})
        visitors.confirm({}, doc(), [get("/shop/?logout=", 5), get("/shop/?delete=", 5)], crawl(), site)
        self.assertFalse([p for _, p, _ in site.asked if "logout" in p or "delete" in p])


if __name__ == "__main__":
    unittest.main()
