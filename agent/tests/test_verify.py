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
