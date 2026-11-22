"""What visitors asked for reaches the report only as an action a person can
take, after the agent has checked it on the site (docs/agent.md, "Visitor
requests"): never a scanner's guess, never a single visitor, never a file
that is not there or a key that changes nothing."""
import os
import re
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
        # a key no site knows (the agent's baseline) is ignored, as sites do
        path_query = re.sub(r"[?&]eqprobe=[0-9a-f]+$", "", path_query)
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

    def test_the_type_the_site_sends_is_what_a_click_adds(self):
        ico = Response(200, {"content-type": "image/vnd.microsoft.icon"}, b"\x00\x00\x01\x00", "")
        woff2 = Response(200, {"content-type": "application/octet-stream"}, b"wOF2rest", "")
        denied = Response(403, {"content-type": "text/html"}, b"", "")
        pdf = Response(200, {"content-type": "application/pdf"}, b"%PDF", "")
        site = FakeSite({"/uploads/fav.ico": ico, "/uploads/font.woff2": woff2, "/uploads/doc.pdf": pdf,
                         "/uploads/fake.woff": Response(200, {"content-type": "application/octet-stream"}, b"MZ..", "")})
        heads = {"/uploads/doc.pdf": denied}  # a firewall refusing HEAD: the file itself
        real = site.request
        site.request = lambda m, p, **k: heads[p] if m == "HEAD" and p in heads else real(m, p, **k)
        report = {}
        visitors.confirm(report, doc(), [get("/uploads/fav.ico"), get("/uploads/font.woff2"), get("/uploads/doc.pdf"),
                                         get("/uploads/fake.woff")], crawl(), site)
        self.assertEqual(sorted((i["path"], i["add"]["type"]) for i in report["files_refused"]),
                         [("/uploads/doc.pdf", "application/pdf"), ("/uploads/fav.ico", "image/vnd.microsoft.icon"),
                          ("/uploads/font.woff2", "font/woff2")])
        self.assertTrue(all("prefix_types" not in i["add"] for i in report["files_refused"]))

    def test_spaces_and_types_known_under_another_name(self):
        wav = Response(200, {"content-type": "audio/x-wav"}, b"RIFF....WAVE", "")
        d = doc()
        d["assets"][0]["types"].append("audio/wav")  # listed, yet the site sends audio/x-wav
        site = FakeSite({"/uploads/my%20song.wav": wav})
        report = {}
        visitors.confirm(report, d, [get("/uploads/my song.wav")], crawl(), site)
        self.assertEqual([(i["path"], i["add"]["type"]) for i in report["files_refused"]],
                         [("/uploads/my song.wav", "audio/x-wav")])

    def test_session_given_back_after_a_failure(self):
        from equalizer_agent.fetch import ReachError

        class Down(FakeSite):
            def request(self, *a, **k):
                raise ReachError("down")
        site = Down({})
        visitors.confirm({}, doc(), [get("/uploads/a.pdf")], crawl(), site)
        self.assertEqual(site.cookies, {"session": "test-account"})

    def test_sensitive_names_never_opened(self):
        site = FakeSite({})
        visitors.confirm({}, doc(), [get("/uploads/logout/x.pdf", 9)], crawl(), site)
        self.assertEqual(site.asked, [])


class KeysTest(unittest.TestCase):
    def test_only_presentation_keys_that_change_the_page(self):
        site = FakeSite({
            "/": page("Home welcome posts"),
            "/shop/": page("Shop apple banana cherry"),
            "/shop/?orderby=price": page("Shop cherry banana apple"),  # sorted: the page changes
            "/shop/?view=list": page("Shop apple banana cherry"), "/shop/?view=grid": page("Shop apple banana cherry"),
        })
        unmapped = [get("/?prova=", 9),  # not a presentation key: never opened
                    get("/shop/?orderby=", 3), get("/shop/?view=", 4),  # view changes nothing here
                    get("/shop/?sort=", 1),  # a single visitor: not even tried
                    get("/?s=", 9),  # declared already
                    get("/items/{id}?id=", 9),  # a path parameter
                    get("/shop/?add-to-cart=", 9), get("/?doing_wp_cron=", 9)]  # a GET that acts: never opened
        report = {}
        visitors.confirm(report, doc(), unmapped, crawl(), site, "wordpress")
        self.assertEqual([(i["route"], i["key"], i["add"]) for i in report["query_dropped"]],
                         [("/shop/", "orderby", {"route_id": "shop", "key": "orderby"})])
        opened = {p for _, p, _ in site.asked}
        for never in ("/?prova=1", "/shop/?sort=price", "/shop/?add-to-cart=1", "/?doing_wp_cron=1"):
            self.assertNotIn(never, opened)

    def test_a_redirect_or_an_error_is_not_a_page_option(self):
        moved = Response(301, {"location": "/author/admin/", "content-type": "text/html"}, b"", "")
        site = FakeSite({"/": page("Home"), "/?paged=2": moved,
                         "/shop/": page("Shop"), "/shop/?orderby=price": page("Forbidden", 403),
                         "/shop/?orderby=date": page("Oops", 500), "/shop/?orderby=title": page("x", 404),
                         "/shop/?orderby=popularity": moved})
        report = {}
        visitors.confirm(report, doc(), [get("/?paged=", 9), get("/shop/?orderby=", 9)], crawl(), site, "wordpress")
        self.assertEqual(report["query_dropped"], [])

    def test_an_unstable_page_tells_nothing(self):
        n = iter(range(1000))
        site = FakeSite({"/shop/": lambda: page("Shop %d" % next(n), status=200 + next(n) % 2)})
        report = {}
        visitors.confirm(report, doc(), [get("/shop/?orderby=", 5)], crawl(), site)
        self.assertEqual(report["query_dropped"], [])

    def test_noise_is_not_a_change(self):
        n = iter(range(1000))
        words = " ".join("word%d" % i for i in range(200))
        noisy = lambda: page(words + " visit %d" % next(n))  # noqa: E731
        site = FakeSite({"/shop/": noisy, "/shop/?orderby=price": noisy, "/shop/?orderby=date": noisy})
        report = {}
        visitors.confirm(report, doc(), [get("/shop/?orderby=", 5)], crawl(), site)
        self.assertEqual(report["query_dropped"], [])

    def test_the_page_that_offers_the_key(self):
        # /{slug}/ covers the cart and the shop: the shop offers ?orderby= and sorts
        d = doc()
        d["routes"].append({"id": "page", "path": "/{slug}/", "method": "GET", "params": {"slug": {"type": "slug"}},
                            "legacy_request": {"method": "GET", "path": "/{{params.slug}}/"}})
        c = crawler.CrawlResult()
        for pq in ("/cart/", "/about/", "/shop/"):
            c.pages[pq] = crawler.Seen(200, "text/html", seen_by={"anonymous"})
        shop = '<select name="orderby"></select> apple banana cherry'
        site = FakeSite({"/cart/": page("Cart empty"), "/about/": page("About us"), "/shop/": page(shop),
                         "/shop/?orderby=price": page('<select name="orderby"></select> cherry banana apple')})
        report = {}
        visitors.confirm(report, d, [get("/{slug}/?orderby=", 3)], c, site)
        self.assertEqual([(i["route"], i["page"]) for i in report["query_dropped"]], [("/{slug}/", "/shop/")])

    def test_a_page_cache_is_not_a_change(self):
        # the plain page comes from a cache; any page with a query is built, with a random widget
        n = iter(range(1000))
        words = " ".join("word%d" % i for i in range(200))
        site = FakeSite({"/shop/": page(words)})
        real = site.request
        site.request = lambda m, p, **k: page(words + " random%d" % next(n)) if "?" in p else real(m, p, **k)
        report = {}
        visitors.confirm(report, doc(), [get("/shop/?view=", 5)], crawl(), site)
        self.assertEqual(report["query_dropped"], [])

    def test_a_sort_on_a_long_stable_page_is_a_change(self):
        items = ["product%d alpha beta gamma" % i for i in range(60)]
        site = FakeSite({"/shop/": page(" ".join(items)),
                         "/shop/?orderby=price": page(" ".join(items[1:] + items[:1]))})
        report = {}
        visitors.confirm(report, doc(), [get("/shop/?orderby=", 5)], crawl(), site)
        self.assertEqual([i["key"] for i in report["query_dropped"]], ["orderby"])


if __name__ == "__main__":
    unittest.main()
