"""The comparison of a page with and without Equalizer (docs/agent.md, step 4)."""
import io
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import attacks, verify  # noqa: E402
from equalizer_agent.fetch import Response  # noqa: E402
from equalizer_agent.log import Logger  # noqa: E402

PAGE = b'<html><body><h1>Caf\xe9 news</h1><a href="http://legacy:8080/about/">About</a><form method=post><input name=q></form></body></html>'


def direct(body=PAGE, status=200, ctype="text/html; charset=iso-8859-1", location=""):
    h = {"content-type": ctype}
    if location:
        h["location"] = location
    return Response(status, h, body, "http://legacy:8080/")


def via(body, status=200, headers=None, refusal=None):
    return {"accepted": refusal is None, "refusal": refusal,
            "response": {"status": status, "headers": headers or {"Content-Type": "text/html; charset=utf-8"}, "body": body}}


class CompareTest(unittest.TestCase):
    def test_same_page_in_another_charset_and_address(self):
        rebuilt = PAGE.decode("iso-8859-1").replace("http://legacy:8080/about/", "/about/").encode("utf-8")
        self.assertEqual(verify.compare(direct(), via(rebuilt)), ("", ""))

    def test_differences(self):
        self.assertEqual(verify.compare(direct(), via(b"", refusal={"reason": "route_not_mapped"}))[0], "refused")
        self.assertEqual(verify.compare(direct(), via(PAGE, status=500))[0], "status")
        self.assertEqual(verify.compare(direct(), via(b"<html><body>Something else entirely</body></html>"))[0], "text")
        no_link = PAGE.replace(b'<a href="http://legacy:8080/about/">About</a>', b"About")
        self.assertEqual(verify.compare(direct(), via(no_link.decode("iso-8859-1").encode()))[0], "links")
        no_form = PAGE.replace(b"<form method=post><input name=q></form>", b"")
        self.assertEqual(verify.compare(direct(), via(no_form.decode("iso-8859-1").encode()))[0], "forms")

    def test_relative_links_resolve_against_the_page(self):
        d = Response(200, {"content-type": "text/html"}, b'<a href="index.php?x=1">A</a>', "http://legacy:8080/administrator/index.php")
        self.assertEqual(verify.compare(d, via(b'<a href="/administrator/index.php?x=1">A</a>')), ("", ""))

    def test_escaping_case_is_the_same_link(self):
        # WordPress writes %ce%b5, a rebuilt link %CE%B5: the same address
        d = Response(200, {"content-type": "text/html"}, b'<a href="/tag/%ce%b5%cf%80/">T</a>', "http://legacy:8080/")
        self.assertEqual(verify.compare(d, via(b'<a href="/tag/%CE%B5%CF%80/">T</a>')), ("", ""))
        self.assertEqual(verify.compare(d, via(b'<a href="/tag/other/">T</a>'))[0], "links")
        r = direct(b"", 301, location="http://legacy:8080/tag/%ce%b5/")
        self.assertEqual(verify.compare(r, via(b"", 301, {"Location": "/tag/%CE%B5/"})), ("", ""))

    def test_only_what_the_site_varies_by_itself_is_excused(self):
        # WooCommerce's related products are random: Belt one time, Hoodie the next
        words = (b"<h1>Cap</h1><p>" + b" ".join(b"w%d" % i for i in range(60)) + b"</p>" +
                 b"".join(b"<a href='/menu/%d'>M</a>" % i for i in range(10)))

        def page(rel, extra=b""):
            return Response(200, {"content-type": "text/html"}, words + extra +
                            b"<p>Related: %s</p><a href='/product/%s'>R</a><a href='/cart'>C</a>" % (rel, rel),
                            "http://legacy:8080/product/cap/")
        first, again = page(b"belt"), page(b"hoodie")
        self.assertEqual(verify.compare(first, via(page(b"beanie").body))[0], "links")
        # the gateway's answer differs only where the site differs from itself
        self.assertEqual(verify.compare(first, via(page(b"beanie").body), (), again), ("", ""))
        # a link the site always shows is still missed
        lost = page(b"beanie").body.replace(b"<a href='/cart'>C</a>", b"")
        self.assertEqual(verify.compare(first, via(lost), (), again)[0], "links")
        # text lost far beyond the site's own variation is still a difference
        self.assertEqual(verify.compare(first, via(b"<h1>Cap</h1>"), (), again)[0], "text")

    def test_a_second_load_that_is_another_page_excuses_nothing(self):
        # a login wall or a challenge served as 200 text/html
        body = b"<h1>Cap</h1><p>" + b" ".join(b"w%d" % i for i in range(60)) + b"</p><a href='/a'>A</a><a href='/b'>B</a>"
        first = Response(200, {"content-type": "text/html"}, body, "http://legacy:8080/p")
        wall = Response(200, {"content-type": "text/html"}, b"<p>Please log in</p><a href='/login'>L</a>", "http://legacy:8080/p")
        third = b"<h1>Cap</h1><p>" + b" ".join(b"w%d" % i for i in range(40)) + b"</p><a href='/a'>A</a>"
        self.assertNotEqual(verify.compare(first, via(third), (), wall)[0], "")

    def test_the_stable_half_must_survive(self):
        # the site randomises half the page; the gateway loses the other half
        stable = b" ".join(b"s%d" % i for i in range(50))

        def load(tag):
            return Response(200, {"content-type": "text/html"},
                            b"<p>" + stable + b"</p><p>" + b" ".join(tag + b"%d" % i for i in range(8)) + b"</p>",
                            "http://legacy:8080/p")
        first, again = load(b"x"), load(b"y")
        lost = b"<p>" + b" ".join(b"s%d" % i for i in range(25)) + b"</p><p>" + b" ".join(b"z%d" % i for i in range(8)) + b"</p>"
        self.assertEqual(verify.compare(first, via(lost), (), again)[0], "text")
        kept = load(b"z").body
        self.assertEqual(verify.compare(first, via(kept), (), again), ("", ""))

    def test_random_related_products(self):
        """WooCommerce shows four related products at random out of a few:
        two loads share some by chance, the gateway's load shows others."""
        import random
        rnd = random.Random(7)
        names = [b"hoodie", b"polo", b"beanie", b"belt", b"cap", b"tshirt", b"sunglasses", b"album"]
        main = b"<h1>T-shirt with logo</h1><p>" + b" ".join(b"desc%d" % i for i in range(120)) + b"</p><a href='/cart'>Cart</a>"

        def load():
            rel = b"".join(b"<li><a href='/product/%s/'>%s</a> <span>%d.00</span> <a href='/?add-to-cart=%s'>Add to cart</a></li>"
                           % (n, n, 10 + i * 3, n) for i, n in enumerate(rnd.sample(names, 4)))
            return Response(200, {"content-type": "text/html"}, main + b"<ul>" + rel + b"</ul>", "http://legacy:8080/product/t-shirt-with-logo/")
        broken = 0
        for _ in range(40):
            first, through = load(), load()
            diff = verify.compare(first, via(through.body), (), [load() for _ in range(verify.AGAIN)])[0]
            broken += 1 if diff else 0
        self.assertEqual(broken, 0)
        # a related block that went missing is not another random one
        gone = load().body.split(b"<ul>")[0]
        self.assertNotEqual(verify.compare(load(), via(gone), (), [load() for _ in range(verify.AGAIN)])[0], "")
        # a link the site always shows, broken where random products move,
        # is not excused by an extra product
        moved = load().body.replace(b"<a href='/cart'>Cart</a>", b"<a href='/wrong'>Cart</a>")
        self.assertEqual(verify.compare(load(), via(moved), (), [load() for _ in range(verify.AGAIN)])[0], "links")
        # what the site always shows is still checked
        lost = load().body.replace(b"<a href='/cart'>Cart</a>", b"")
        self.assertEqual(verify.compare(load(), via(lost), (), [load() for _ in range(verify.AGAIN)])[0], "links")

    def test_a_second_answer_that_is_an_error_excuses_nothing(self):
        first = Response(200, {"content-type": "text/html"}, b"<a href='/a'>A</a>", "http://legacy:8080/x")
        run = verify.Run.__new__(verify.Run)
        for status in (429, 503, 302, 500):
            run.f = type("F", (), {"get": lambda self, pq, s=status: Response(s, {"content-type": "text/html"}, b"", "http://legacy:8080/x")})()
            self.assertEqual(run._again("/x", first), [], status)
        run.hosts = ()
        run.f = type("F", (), {"get": lambda self, pq: first})()
        self.assertEqual(run._again("/x", first), [first], "the same answer: one load is enough")

    def test_long_numbers_are_not_attacks_on_text(self):
        from equalizer_agent import attacks
        self.assertIn(attacks.OVERFLOW, [v for _, v in attacks.values("int")])
        self.assertNotIn(attacks.OVERFLOW, [v for _, v in attacks.values("string")])

    def test_a_final_slash_is_the_same_page(self):
        """Magento links to /page/ and answers 301 to /page; the gateway
        writes the link in its route's spelling and serves the page."""
        d = Response(301, {"content-type": "text/html", "location": "http://legacy:8080/privacy"}, b"",
                     "http://legacy:8080/privacy/")
        self.assertEqual(verify.compare(d, via(b"<p>Privacy</p>")), ("", ""))
        other = Response(301, {"content-type": "text/html", "location": "http://legacy:8080/elsewhere"}, b"",
                         "http://legacy:8080/privacy/")
        self.assertEqual(verify.compare(other, via(b"<p>Privacy</p>"))[0], "status")
        page = Response(200, {"content-type": "text/html"}, b'<a href="/privacy/">P</a>', "http://legacy:8080/")
        self.assertEqual(verify.compare(page, via(b'<a href="/privacy">P</a>')), ("", ""))

    def test_the_sites_own_address_in_the_text(self):
        d = Response(200, {"content-type": "text/html"}, b"<p>Alias: http://legacy:8080/about</p>", "http://legacy:8080/a")
        self.assertEqual(verify.compare(d, via(b"<p>Alias: https://www.acme.test/about</p>"), ("legacy:8080", "www.acme.test")), ("", ""))
        self.assertEqual(verify.compare(d, via(b"<p>Alias: https://evil.test/about</p>"), ("legacy:8080", "www.acme.test"))[0], "text")

    def test_redirects_compare_where_they_go(self):
        d = direct(b"", 302, location="http://legacy:8080/search/node")
        self.assertEqual(verify.compare(d, via(b"", 302, {"Location": "/search/node"})), ("", ""))
        self.assertEqual(verify.compare(d, via(b"", 302, {"Location": "/elsewhere"}))[0], "status")


class AttacksTest(unittest.TestCase):
    def test_valid_values_fit_their_specs(self):
        self.assertEqual(attacks.valid({"type": "enum", "values": ["asc", "desc"]}), "asc")
        self.assertEqual(attacks.valid({"type": "int", "min": 18, "max": 120}), "18")
        self.assertEqual(attacks.valid({"type": "email"}), "agent@example.com")
        self.assertTrue(all(v for _, v in attacks.values()))


if __name__ == "__main__":
    unittest.main()


class CorrectTest(unittest.TestCase):
    def doc(self):
        return {"roles": {"anonymous": "guest"}, "areas": [{"id": "site", "prefix": "/"}],
                "routes": [{"id": "export", "path": "/export", "method": "GET", "query": {"format": {"type": "int", "min": 1, "max": 9}}},
                           {"id": "list", "path": "/list", "method": "GET", "query": {"page": {"type": "string", "max_length": 50},
                                                                                       "sort": {"type": "string", "max_length": 50},
                                                                                       "q": {"type": "string", "max_length": 50}}},
                           {"id": "weird", "path": "/weird", "method": "GET"}],
                "forms": []}

    def test_widen_mends_what_broke(self):
        from equalizer_agent import correct
        m = verify.Measure(broken=[{"path": "/export?format=csv", "entry": "export", "difference": "refused"},
                                   {"path": "/weird", "entry": "weird", "difference": "text", "detail": "80% of the text"},
                                   {"path": "/x", "entry": "site", "difference": "status"}],
                           refused_fields={"export": {"format"}})
        d = self.doc()
        got = correct.widen(d, m)
        self.assertEqual([(c["entry"], c["change"]) for c in got], [("export.format", "widened"), ("weird", "removed")])
        self.assertEqual(d["routes"][0]["query"]["format"]["type"], "string")
        self.assertNotIn("weird", [r["id"] for r in d["routes"]])

    def test_a_refused_form_alone_is_mended_too(self):
        """A form whose valid submission the gateway refuses breaks no page
        view: removing it mends fewer broken entries, not more pages."""
        from equalizer_agent import correct
        doc = dict(self.doc(), forms=[{"id": "login_form", "route": "/login", "fields": {}}])
        first = verify.Measure(broken=[{"path": "/", "entry": "login_form", "difference": "refused",
                                        "detail": "a valid submission is refused: form_token_missing"}], identical=5)

        class Run:
            def measure(self, d, tm):
                return verify.Measure(identical=5)

        class Panel:
            def try_map(self, job_id, d):
                return {"signed": True}

        out, m, corr = correct.loop({"id": 1, "pages": []}, doc, first, Run(), Panel(), Logger("t", "warn", io.StringIO()))
        self.assertEqual([(c["entry"], c["change"]) for c in corr], [("login_form", "removed")])
        self.assertEqual(out["forms"], [])

    def test_narrow_only_from_values_seen(self):
        from equalizer_agent import correct
        m = verify.Measure(open=[{"kind": "input", "target": "list.page", "reason": "accepts_sql"},
                                 {"kind": "input", "target": "list.sort", "reason": "accepts_sql"},
                                 {"kind": "input", "target": "list.q", "reason": "accepts_sql"}])
        pages = ["/list?page=1&sort=date&q=red shoes", "/list?page=2&sort=title&q=o'neil", "/list?page=3&sort=date-desc"]
        d = self.doc()
        got = {c["entry"]: c["detail"] for c in correct.narrow(d, m, pages)}
        self.assertEqual(got["list.page"], "string → int")
        self.assertIn("letters", got["list.sort"])
        self.assertNotIn("list.q", got)  # free text seen: not narrowed
        self.assertEqual(d["routes"][1]["query"]["page"]["type"], "int")
