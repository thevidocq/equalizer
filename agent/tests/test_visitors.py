"""What visitors asked for enters the agent's draft only once checked on the
site (docs/agent.md, "Visitor requests"): never a scanner's guess, never a
single visitor, never a file that is not there or a key that changes
nothing, and a key only with the values the site itself offers."""
import os
import re
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import crawler, visitors  # noqa: E402
from equalizer_agent.fetch import ReachError, Response  # noqa: E402


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
    for pq in ("/", "/shop/"):
        c.pages[pq] = crawler.Seen(200, "text/html", seen_by={"anonymous"})
    c.assets["/uploads/logo.png"] = "image/png"
    c.assets["/uploads/linked.pdf"] = "application/pdf"  # the site's own: the study adds it itself
    return c


def get(path, n=2):
    return {"method": "GET", "path": path, "clients": n}


def run(unmapped, site, d=None, c=None, cms=None):
    """(report, draft) after the check."""
    d = d or doc()
    report = {}
    visitors.apply(report, d, unmapped, c or crawl(), site, cms)
    return report, d


def files_added(report):
    return sorted((i["path"], i["type"]) for i in report["visitors_added"] if i["kind"] == "file")


def keys_added(report):
    return [(i["route"], i["key"]) for i in report["visitors_added"] if i["kind"] == "key"]


SORT = '<select name="orderby"><option value="menu_order">Default</option><option value="price">Price</option></select>'


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
                    get("/uploads/shell.php", 50),  # a page or a script, however many ask
                    get("/uploads/linked.pdf", 9),  # named by the site's pages
                    get("/uploads/logo.png", 9),  # its type is already allowed
                    get("/CFIDE/administrator", 9), get("/wp-login.php", 9),  # not files of an asset prefix
                    {"method": "POST", "path": "/uploads/x.pdf", "clients": 9}]
        report, d = run(unmapped, site)
        self.assertEqual(files_added(report), [("/uploads/price-list.pdf", "application/pdf")])
        self.assertEqual(d["assets"][0]["types"], ["image/png", "application/pdf"])  # in the draft
        self.assertIn("3 visitors", report["visitors_added"][0]["why"])
        opened = {p for _, p, _ in site.asked}
        for never in ("/uploads/once.pdf", "/uploads/backup.zip", "/uploads/evil.js", "/uploads/shell.php",
                      "/CFIDE/administrator"):
            self.assertNotIn(never, opened)  # not even opened
        self.assertTrue(all(c == {} for _, _, c in site.asked))  # anonymously
        self.assertEqual(site.cookies, {"session": "test-account"})  # the account's session given back

    def test_the_type_the_site_sends_is_what_is_added(self):
        ico = Response(200, {"content-type": "image/vnd.microsoft.icon"}, b"\x00\x00\x01\x00", "")
        woff2 = Response(200, {"content-type": "application/octet-stream"}, b"wOF2rest", "")
        denied = Response(403, {"content-type": "text/html"}, b"", "")
        pdf = Response(200, {"content-type": "application/pdf"}, b"%PDF", "")
        site = FakeSite({"/uploads/fav.ico": ico, "/uploads/font.woff2": woff2, "/uploads/doc.pdf": pdf,
                         "/uploads/fake.woff": Response(200, {"content-type": "application/octet-stream"}, b"MZ..", "")})
        heads = {"/uploads/doc.pdf": denied}  # a firewall refusing HEAD: the file itself
        real = site.request
        site.request = lambda m, p, **k: heads[p] if m == "HEAD" and p in heads else real(m, p, **k)
        report, _ = run([get("/uploads/fav.ico"), get("/uploads/font.woff2"), get("/uploads/doc.pdf"),
                         get("/uploads/fake.woff")], site)
        self.assertEqual(files_added(report),
                         [("/uploads/doc.pdf", "application/pdf"), ("/uploads/fav.ico", "image/vnd.microsoft.icon"),
                          ("/uploads/font.woff2", "font/woff2")])

    def test_spaces_and_types_known_under_another_name(self):
        wav = Response(200, {"content-type": "audio/x-wav"}, b"RIFF....WAVE", "")
        d = doc()
        d["assets"][0]["types"].append("audio/wav")  # listed, yet the site sends audio/x-wav
        site = FakeSite({"/uploads/my%20song.wav": wav})
        report, _ = run([get("/uploads/my song.wav")], site, d)
        self.assertEqual(files_added(report), [("/uploads/my song.wav", "audio/x-wav")])

    def test_session_given_back_after_a_failure(self):
        class Down(FakeSite):
            def request(self, *a, **k):
                raise ReachError("down")
        site = Down({})
        report, d = run([get("/uploads/a.pdf")], site)
        self.assertEqual(report["visitors_added"], [])
        self.assertEqual(d, doc())
        self.assertEqual(site.cookies, {"session": "test-account"})

    def test_sensitive_names_never_opened(self):
        site = FakeSite({})
        run([get("/uploads/logout/x.pdf", 9)], site)
        self.assertEqual(site.asked, [])


class KeysTest(unittest.TestCase):
    def test_only_presentation_keys_with_the_values_the_site_offers(self):
        site = FakeSite({
            "/": page("Home welcome posts"),
            "/shop/": page(SORT + " apple banana cherry"),
            "/shop/?orderby=price": page(SORT + " cherry banana apple"),  # sorted: the page changes
            "/shop/?view=list": page(SORT + " apple banana cherry"),
        })
        unmapped = [get("/?prova=", 9),  # not a presentation key: never opened
                    get("/shop/?orderby=", 3),
                    get("/shop/?view=", 4),  # the site offers no values for it: never free text
                    get("/shop/?sort=", 1),  # a single visitor: not even tried
                    get("/?s=", 9),  # declared already
                    get("/items/{id}?id=", 9),  # a path parameter
                    get("/shop/?add-to-cart=", 9), get("/?doing_wp_cron=", 9)]  # a GET that acts: never opened
        report, d = run(unmapped, site, cms="wordpress")
        self.assertEqual(keys_added(report), [("/shop/", "orderby")])
        shop = next(r for r in d["routes"] if r["id"] == "shop")
        # only what the site offers passes: anything else, an attack included, is refused by the gateway
        self.assertEqual(shop["query"]["orderby"], {"type": "enum", "values": ["menu_order", "price"], "required": False})
        self.assertEqual(shop["legacy_request"]["query"], {"orderby": "{{query.orderby}}"})
        opened = {p for _, p, _ in site.asked}
        for never in ("/?prova=1", "/shop/?sort=price", "/shop/?add-to-cart=1", "/?doing_wp_cron=1", "/shop/?view=list"):
            self.assertNotIn(never, opened)

    def test_numbers_for_page_numbers_and_prices(self):
        site = FakeSite({"/shop/": page("Page one items"), "/shop/?paged=2": page("Page two items"),
                         "/shop/?min_price=10": page("Cheap items")})
        report, d = run([get("/shop/?paged=", 3), get("/shop/?min_price=", 3)], site)
        shop = next(r for r in d["routes"] if r["id"] == "shop")
        self.assertEqual(shop["query"]["paged"], {"type": "int", "min": 1, "max": 100000, "required": False})
        self.assertEqual(shop["query"]["min_price"]["type"], "int")
        self.assertEqual(sorted(k for _, k in keys_added(report)), ["min_price", "paged"])

    def test_a_redirect_or_an_error_is_not_a_page_option(self):
        moved = Response(301, {"location": "/author/admin/", "content-type": "text/html"}, b"", "")
        offer = '<a href="/shop/?orderby=price">p</a><a href="/shop/?orderby=date">d</a>'
        site = FakeSite({"/": page("Home"), "/?paged=2": moved,
                         "/shop/": page(offer), "/shop/?orderby=price": page("Forbidden", 403),
                         "/shop/?orderby=date": page("Oops", 500)})
        report, d = run([get("/?paged=", 9), get("/shop/?orderby=", 9)], site, cms="wordpress")
        self.assertEqual(report["visitors_added"], [])
        self.assertEqual(d, doc())  # the draft is untouched

    def test_an_unstable_page_tells_nothing(self):
        n = iter(range(1000))
        site = FakeSite({"/shop/": lambda: page(SORT + " %d" % next(n), status=200 + next(n) % 2)})
        report, _ = run([get("/shop/?orderby=", 5)], site)
        self.assertEqual(report["visitors_added"], [])

    def test_noise_is_not_a_change(self):
        n = iter(range(1000))
        words = SORT + " ".join("word%d" % i for i in range(200))
        noisy = lambda: page(words + " visit %d" % next(n))  # noqa: E731
        site = FakeSite({"/shop/": noisy, "/shop/?orderby=price": noisy, "/shop/?orderby=menu_order": noisy})
        report, _ = run([get("/shop/?orderby=", 5)], site)
        self.assertEqual(report["visitors_added"], [])

    def test_the_page_that_offers_the_key(self):
        # /{slug}/ covers the cart and the shop: the shop offers ?orderby= and sorts
        d = doc()
        d["routes"].append({"id": "page", "path": "/{slug}/", "method": "GET", "params": {"slug": {"type": "slug"}},
                            "legacy_request": {"method": "GET", "path": "/{{params.slug}}/"}})
        c = crawler.CrawlResult()
        for pq in ("/cart/", "/about/", "/shop/"):
            c.pages[pq] = crawler.Seen(200, "text/html", seen_by={"anonymous"})
        site = FakeSite({"/cart/": page("Cart empty"), "/about/": page("About us"),
                         "/shop/": page(SORT + " apple banana cherry"),
                         "/shop/?orderby=price": page(SORT + " cherry banana apple")})
        report, _ = run([get("/{slug}/?orderby=", 3)], site, d, c)
        self.assertEqual([(i["route"], i["page"]) for i in report["visitors_added"]], [("/{slug}/", "/shop/")])

    def test_a_page_cache_is_not_a_change(self):
        # the plain page comes from a cache; any page with a query is built, with a random widget
        n = iter(range(1000))
        words = SORT + " ".join("word%d" % i for i in range(200))
        site = FakeSite({"/shop/": page(words)})
        real = site.request
        site.request = lambda m, p, **k: page(words + " random%d" % next(n)) if "?" in p else real(m, p, **k)
        report, _ = run([get("/shop/?orderby=", 5)], site)
        self.assertEqual(report["visitors_added"], [])

    def test_a_sort_on_a_long_stable_page_is_a_change(self):
        items = ["product%d alpha beta gamma" % i for i in range(60)]
        site = FakeSite({"/shop/": page(SORT + " ".join(items)),
                         "/shop/?orderby=price": page(SORT + " ".join(items[1:] + items[:1]))})
        report, _ = run([get("/shop/?orderby=", 5)], site)
        self.assertEqual(keys_added(report), [("/shop/", "orderby")])

    def test_offered_values_are_plain(self):
        body = ('<select name="orderby"><option value="price">a</option><option value="x&quot;&gt;">b</option>'
                '</select><a href="/shop/?orderby=date&amp;x=1">d</a><a href="/shop/?orderby={{7*7}}">t</a>').encode()
        self.assertEqual(visitors._offered([body], crawler.CrawlResult(), "orderby"), ["price", "date"])
        self.assertIsNone(visitors.spec("view", []))


if __name__ == "__main__":
    unittest.main()
