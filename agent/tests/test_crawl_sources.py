"""Where the crawler finds pages: sitemaps by any name, robots.txt, links
written with another name of the site; and what it never opens."""
import io
import os
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import config, discovery  # noqa: E402
from equalizer_agent.fetch import Fetcher  # noqa: E402
from equalizer_agent.log import Logger  # noqa: E402
from tests.test_discovery import FakePanel  # noqa: E402

PAGES = {
    "/": ("text/html", '<html><title>Home</title><a href="https://www.shop.test/linked/">www link</a>'
                       '<a href="https://public.test/public-name/">public link</a></html>'),
    "/robots.txt": ("text/plain", "User-agent: *\nSitemap: http://legacy.test/wp-sitemap.xml\n"),
    "/wp-sitemap.xml": ("application/xml", '<?xml version="1.0"?><sitemapindex><sitemap><loc>http://legacy.test/wp-sitemap-posts-post-1.xml</loc></sitemap></sitemapindex>'),
    "/wp-sitemap-posts-post-1.xml": ("application/xml", '<?xml version="1.0"?><urlset><url><loc>http://legacy.test/from-sitemap/</loc></url></urlset>'),
    "/from-sitemap/": ("text/html", "<html><title>S</title></html>"),
    "/linked/": ("text/html", "<html><title>L</title></html>"),
    "/public-name/": ("text/html", "<html><title>P</title></html>"),
    "/backup/report.pdf": ("application/pdf", "%PDF"),
}


class Site:
    def __init__(self):
        self.asked = []
        site = self

        class H(BaseHTTPRequestHandler):
            def log_message(self, *a):
                pass

            def do_GET(self):
                p = self.path.split("?")[0]
                site.asked.append(p)
                ctype, body = PAGES.get(p, ("text/html", "not found"))
                self.send_response(200 if p in PAGES else 404)
                self.send_header("Content-Type", ctype)
                self.end_headers()
                self.wfile.write(body.encode())

        self.srv = ThreadingHTTPServer(("127.0.0.1", 0), H)
        self.port = self.srv.server_address[1]
        threading.Thread(target=self.srv.serve_forever, daemon=True).start()

    def close(self):
        self.srv.shutdown()


class SourcesTest(unittest.TestCase):
    def test_sitemaps_names_and_rules(self):
        site = Site()
        self.addCleanup(site.close)
        dest = f"127.0.0.1:{site.port}"
        cfg = config.Config(panel_token="x" * 32, direct={dest}, rate=0)
        job = {"id": 1, "tenant": "acme", "app": "shop", "domain": "public.test", "kind": None,
               "base_url": "http://legacy.test", "max_pages": 50,
               "reach": {"mode": "direct", "address": dest, "allow": [dest]}, "llm": None,
               "current_map": {"meta": {"aliases": ["shop.test"]}},
               "unmapped": [{"method": "GET", "path": "/backup/report.pdf", "clients": 3}]}
        panel = FakePanel()
        discovery.run(job, cfg, panel, Logger("test", "warn", io.StringIO()))
        for p in ("/wp-sitemap.xml", "/wp-sitemap-posts-post-1.xml", "/from-sitemap/", "/linked/", "/public-name/"):
            self.assertIn(p, site.asked)
        self.assertNotIn("/backup/report.pdf", site.asked)  # never opened, whoever asked for it

    def test_hosts(self):
        f = Fetcher("https://shop.test", {"mode": "direct", "address": "127.0.0.1:1", "allow": ["127.0.0.1:1"]},
                    "acme", "", "", {"127.0.0.1:1"}, 0)
        f.add_hosts(["public.test", "10.0.0.1:8080", ""])
        self.assertEqual(f.hosts, {"shop.test", "www.shop.test", "public.test", "www.public.test", "10.0.0.1:8080"})
        self.assertTrue(f.same_site("https://www.public.test/x"))
        self.assertFalse(f.same_site("https://elsewhere.test/x"))
        self.assertFalse(f.same_site("javascript:alert(1)"))


if __name__ == "__main__":
    unittest.main()


class BrowserGuardTest(unittest.TestCase):
    def test_the_sites_other_names_are_its_own(self):
        from equalizer_agent import browser
        f = Fetcher("https://shop.test", {"mode": "direct", "address": "127.0.0.1:1", "allow": ["127.0.0.1:1"]},
                    "acme", "", "", {"127.0.0.1:1"}, 0)
        g = browser._Guard(f, "shop.test", browser.Seen(), lambda pq: True)
        self.assertEqual(g.target("https://www.shop.test/a?b=1"), "/a?b=1")
        self.assertEqual(g.target("https://cdn.elsewhere.test/a"), "")


class SitemapEscapingTest(unittest.TestCase):
    def test_entities_and_cdata(self):
        from equalizer_agent import extract
        xml = ("<urlset><url><loc>http://s.test/c.php?cat=1&amp;lang=en</loc></url>"
               "<url><loc><![CDATA[http://s.test/d?x=1&y=2]]></loc></url></urlset>")
        self.assertEqual(extract.sitemap_urls(xml), ["http://s.test/c.php?cat=1&lang=en", "http://s.test/d?x=1&y=2"])


class MalformedAnswerTest(unittest.TestCase):
    def test_a_broken_answer_is_a_reach_error(self):
        """An IncompleteRead on one stylesheet is one file lost, not the study."""
        import http.client
        from equalizer_agent.fetch import ReachError
        f = Fetcher("http://s.test", {"mode": "direct", "address": "127.0.0.1:1", "allow": ["127.0.0.1:1"]},
                    "acme", "", "", {"127.0.0.1:1"}, 0)

        def broken(req, path_query):
            raise http.client.IncompleteRead(b"x")
        f._send = broken
        with self.assertRaises(ReachError):
            f.get("/a.css")


class MapbuildGuardsTest(unittest.TestCase):
    def test_no_asset_prefix_over_what_the_study_knows_and_the_right_route(self):
        from equalizer_agent import crawler, fingerprint, mapbuild
        crawl = crawler.CrawlResult()
        crawl.pages["/"] = crawler.Seen(200, "text/html")
        crawl.pages["/search?q=shoes"] = crawler.Seen(200, "text/html")
        crawl.assets["/members/avatar.png"] = "image/png"
        crawl.skipped_actions.add("/members/logout")
        crawl.assets["/app/logo.png"] = "image/png"
        crawl.script_refused.add(("POST", "/app/save"))
        crawl.assets["/files/a.png"] = "image/png"
        current = {"meta": {"version": 1}, "legacy": {}, "roles": {"anonymous": "guest"},
                   "routes": [{"id": "page", "path": "/{slug}", "method": "GET", "roles": ["guest"],
                               "legacy_request": {"method": "GET", "path": "/{{params.slug}}"},
                               "params": {"slug": {"type": "string", "max_length": 100, "required": True}}},
                              {"id": "search", "path": "/search", "method": "GET", "roles": ["guest"],
                               "legacy_request": {"method": "GET", "path": "/search"}}]}
        job = {"base_url": "http://x.test", "reach": {"mode": "direct", "address": "127.0.0.1:1"},
               "unmapped": [], "current_map": current}
        doc, rep, _ = mapbuild.build(job, crawl, fingerprint.Fingerprint())
        prefixes = {a["prefix"] for a in doc.get("assets", [])}
        self.assertNotIn("/members/", prefixes)  # over a link it skipped
        self.assertNotIn("/app/", prefixes)  # over a script's refused POST
        self.assertIn("/files/", prefixes)
        search = next(r for r in doc["routes"] if r["id"] == "search")
        page = next(r for r in doc["routes"] if r["id"] == "page")
        self.assertIn("q", search.get("query", {}))  # the route the gateway serves /search with
        self.assertNotIn("q", page.get("query", {}))
