"""The agent studies every file the site loads: those named in HTML
attributes, in stylesheets, and those visitors asked for and the gateway
refused (docs/agent.md)."""
import io
import os
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import config, css, discovery, extract  # noqa: E402
from equalizer_agent.log import Logger  # noqa: E402
from tests.test_discovery import FakePanel  # noqa: E402

HOME = """<html><head><title>Liquid</title>
<link rel="stylesheet" href="/site/templates/front/css/styles.css">
<style>.hero{background:url('/site/templates/front/img/hero.webp')}</style></head><body>
<div style="background-image: url(/site/templates/front/img/band.jpg)"></div>
<img src="data:image/gif;base64,R0lGOD" data-src="/site/assets/files/1/lazy.jpg"
     srcset="/site/assets/files/1/a-400.jpg 400w, /site/assets/files/1/a-800.jpg 800w">
<video poster="/site/templates/front/img/poster.jpg"><source src="/site/templates/front/video/intro.mp4"></video>
<a href="/blog/2024/hello/">Hello</a><img src="/blog/2024/hello.jpg">
<a href="/site/assets/files/2/brochure.docx">Brochure</a>
</body></html>"""
STYLES = """@import url("fonts.css");
/* url(/commented/out.png) */
.logo{background:url(../img/logo.svg)}"""
FONTS = """@font-face{font-family:G;src:url('../fonts/g.woff2') format('woff2'),url("../fonts/g.woff") format('woff')}"""
FILES = {"/site/templates/front/css/styles.css": (STYLES, "text/css"),
         "/site/templates/front/css/fonts.css": (FONTS, "text/css"),
         "/site/templates/front/video/late.webm": ("\x1aE\xdf\xa3", "video/webm"),
         "/site/templates/front/audio/jingle.wav": ("RIFF", "audio/x-wav"),
         "/site/assets/files/9/manual.pdf": ("%PDF", "application/pdf")}


class Site:
    def __init__(self):
        class H(BaseHTTPRequestHandler):
            def log_message(self, *a):
                pass

            def do_GET(self):
                p = self.path.split("?")[0]
                body, ctype = (HOME, "text/html") if p == "/" else FILES.get(p, ("", ""))
                if p == "/blog/2024/hello/":
                    body, ctype = "<html><title>Hello</title><body>post</body></html>", "text/html"
                if not ctype:
                    status = 200 if p.startswith("/site/") and "missing" not in p else 404
                    body, ctype = "x", "application/octet-stream"
                else:
                    status = 200
                self.send_response(status)
                self.send_header("Content-Type", ctype)
                if p.endswith("manual.pdf"):
                    self.send_header("Content-Length", str(30 << 20))  # announced, not sent
                    self.end_headers()
                    return
                self.end_headers()
                self.wfile.write(body.encode("latin-1"))

        self.srv = ThreadingHTTPServer(("127.0.0.1", 0), H)
        self.port = self.srv.server_address[1]
        threading.Thread(target=self.srv.serve_forever, daemon=True).start()

    def close(self):
        self.srv.shutdown()


class CssTest(unittest.TestCase):
    def test_refs(self):
        got = css.refs(STYLES + FONTS + "a{b:url(data:image/png;base64,x)}", "http://x/a/css/s.css")
        self.assertEqual(got, ["http://x/a/css/fonts.css", "http://x/a/img/logo.svg",
                               "http://x/a/fonts/g.woff2", "http://x/a/fonts/g.woff"])


class ExtractTest(unittest.TestCase):
    def test_srcset(self):
        self.assertEqual(extract.srcset("/a-400.jpg 400w, /a-800.jpg 800w"), ["/a-400.jpg", "/a-800.jpg"])
        # a CDN URL with commas stays whole
        self.assertEqual(extract.srcset("https://c.test/w_400,h_300/a.jpg 1x,https://c.test/w_800,h_600/a.jpg 2x"),
                         ["https://c.test/w_400,h_300/a.jpg", "https://c.test/w_800,h_600/a.jpg"])
        self.assertEqual(extract.srcset(" "), [])

    def test_every_file_a_page_names(self):
        got = {a.split("x.test", 1)[1] for a in extract.parse(HOME, "http://x.test/").assets}
        for p in ("/site/templates/front/img/hero.webp", "/site/templates/front/img/band.jpg",
                  "/site/assets/files/1/lazy.jpg", "/site/assets/files/1/a-400.jpg", "/site/assets/files/1/a-800.jpg",
                  "/site/templates/front/img/poster.jpg", "/site/templates/front/video/intro.mp4"):
            self.assertIn(p, got)
        self.assertFalse([a for a in got if "base64" in a])


class StudyTest(unittest.TestCase):
    def setUp(self):
        self.site = Site()
        dest = f"127.0.0.1:{self.site.port}"
        self.cfg = config.Config(panel_token="x" * 32, direct={dest}, rate=0)
        self.job = {"id": 1, "tenant": "acme", "app": "liquid", "domain": "liquid.test", "kind": None,
                    "base_url": "http://liquid.test", "max_pages": 20,
                    "reach": {"mode": "direct", "address": dest, "allow": [dest]}, "llm": None,
                    "unmapped": [{"method": "GET", "path": "/site/templates/front/video/late.webm", "clients": 1},
                                 {"method": "GET", "path": "/site/templates/front/audio/jingle.wav", "clients": 1},
                                 {"method": "GET", "path": "/site/assets/files/9/manual.pdf", "clients": 1},
                                 {"method": "GET", "path": "/site/templates/front/video/missing.ogv", "clients": 1}]}

    def tearDown(self):
        self.site.close()

    def test_the_map_covers_every_file(self):
        panel = FakePanel()
        discovery.run(self.job, self.cfg, panel, Logger("test", "warn", io.StringIO()))
        doc = panel.results[0]["document"]
        types = {t for a in doc["assets"] if a["prefix"] == "/site/templates/" for t in a["types"]}
        # fonts from a stylesheet another stylesheet imports, images from CSS,
        # the video and its poster, the file a visitor asked for
        for t in ("text/css", "font/woff", "font/woff2", "image/svg+xml", "image/webp", "image/jpeg",
                  "video/mp4", "video/webm"):
            self.assertIn(t, types)
        self.assertIn("audio/x-wav", types)  # the type the site sends, which the gateway checks
        self.assertNotIn("audio/wav", types)
        self.assertNotIn("video/ogg", types)  # the site does not serve it
        files = next(a for a in doc["assets"] if a["prefix"] == "/site/assets/")
        self.assertIn("application/pdf", files["types"])
        self.assertIn("application/vnd.openxmlformats-officedocument.wordprocessingml.document", files["types"])
        # images next to pages: no asset prefix over the pages, the area serves both
        self.assertNotIn("/blog/2024/", {a["prefix"] for a in doc["assets"]})
        self.assertEqual(files["max_size_kb"], -(-(30 << 20) * 11 // 10240))  # 30 MiB + 10%


if __name__ == "__main__":
    unittest.main()
