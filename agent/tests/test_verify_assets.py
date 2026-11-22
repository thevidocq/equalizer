"""Files the pages load are verified through the gateway, and the agent
mends its draft when the gateway refuses them, with passive types only
(docs/agent.md, step 4)."""
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import assettypes, correct, verify, verifyassets  # noqa: E402
from equalizer_agent.fetch import Response  # noqa: E402

PAGE = b"""<html><head><link rel=stylesheet href="/t/css/s.css"><script src="/t/js/app.js"></script></head><body>
<img src="/t/img/a.avif"><video src="/t/video/big.mp4"></video><img src="/up/photo.jpg">
<a href="/t/news/">a page under the prefix</a><a href="/t/feed.xhtml">xhtml</a><a href="/t/export.csv">csv</a>
<a href="/t/backup.zip">zip</a><a href="/t/logout">action</a><script src="/t/lib/dropzone/dropzone.min.js"></script><a href="/t/account/logout/bye.png">action folder</a><a href="/t/account/log-out/bye.png">x</a><a href="/t/sign-out/x.png">x</a><img src="/t/img/delete-icon.png"><a href="/t/js/linked.js">linked script</a>
<img src="https://cdn.elsewhere.test/x.png"></body></html>"""
FILES = {
    "/t/css/s.css": (200, "text/css", b"@font-face{src:url(../fonts/f.woff)} .x{background:url(/t/img/b.png)}", ""),
    "/t/js/app.js": (200, "application/javascript", b"", ""),
    "/t/js/linked.js": (200, "application/javascript", b"", ""),
    "/t/img/a.avif": (200, "image/avif", b"", ""),
    "/t/video/big.mp4": (200, "video/mp4", b"", str(600 << 20)),
    "/t/fonts/f.woff": (200, "application/octet-stream", b"wOFF", ""),
    "/t/img/b.png": (200, "image/png", b"", ""),
    "/up/photo.jpg": (200, "image/jpeg", b"", ""),
    "/t/news/": (200, "text/html", b"<html>news</html>", ""),
    "/t/feed.xhtml": (200, "application/xhtml+xml", b"", ""),
    "/t/export.csv": (200, "text/csv", b"", ""),
    "/t/backup.zip": (200, "application/zip", b"", ""),
    "/t/logout": (200, "image/png", b"", ""),
    "/t/lib/dropzone/dropzone.min.js": (200, "application/javascript", b"", ""),
    "/t/account/logout/bye.png": (200, "image/png", b"", ""),
    "/t/account/log-out/bye.png": (200, "image/png", b"", ""),
    "/t/sign-out/x.png": (200, "image/png", b"", ""),
    "/t/img/delete-icon.png": (200, "image/png", b"", ""),
}


class Fetcher:
    def __init__(self):
        self.asked = []

    def same_site(self, url):
        return url.startswith("/") or url.startswith("http://site.test/")

    def get(self, path):
        self.asked.append(path)
        status, ctype, body, size = FILES.get(path, (404, "text/html", b"", ""))
        h = {"content-type": ctype}
        if size:
            h["content-length"] = size
        return Response(status, h, body, "http://site.test" + path)


class Gateway:
    """The draft refuses everything under /t/ but stylesheets, PNG and MP4 of normal size."""

    def __init__(self):
        self.asked = []

    def preview(self, path, headers=None, session=None, method="GET"):
        self.asked.append((method, path))
        ctype = FILES[path][1]
        if path == "/t/video/big.mp4":
            reason = "asset_too_large"
        elif ctype in ("text/css", "image/png"):
            reason = None
        else:
            reason = "asset_type_not_allowed"
        return {"entry": "asset:/t/", "refusal": {"reason": reason} if reason else None}


def doc():
    return {"assets": [{"prefix": "/t/", "types": ["text/css", "image/png", "video/mp4", "font/woff"]}],
            "areas": [{"id": "site", "prefix": "/"}]}


class VerifyAssetsTest(unittest.TestCase):
    def run_check(self, d):
        m, gw, f = verify.Measure(), Gateway(), Fetcher()
        verifyassets.check(gw, f, d, m, verifyassets.names(PAGE, "http://site.test/"))
        return m, gw, f

    def test_files_are_checked_and_mended_with_passive_types_only(self):
        d = doc()
        m, gw, f = self.run_check(d)
        self.assertTrue(all(meth == "HEAD" for meth, _ in gw.asked))
        self.assertNotIn("/up/photo.jpg", f.asked)  # the area serves it: nothing to check
        self.assertNotIn("/t/backup.zip", f.asked)  # sensitive: never opened
        self.assertNotIn("/t/logout", f.asked)  # an action: never opened
        for p in ("/t/account/logout/bye.png", "/t/account/log-out/bye.png", "/t/sign-out/x.png"):
            self.assertNotIn(p, f.asked)  # a file in an action's folder neither
        self.assertIn("/t/img/delete-icon.png", f.asked)  # a file named delete-icon is no action, as in the crawl
        self.assertIn("/t/lib/dropzone/dropzone.min.js", f.asked)  # nor a folder named dropzone
        self.assertNotIn("/t/news/", [p for _, p in gw.asked])  # a page: verify compares it, not this
        fixes = correct.widen(d, m)
        a = d["assets"][0]
        self.assertIn("image/avif", a["types"])
        self.assertIn("application/javascript", a["types"])  # a page loads it
        for never in ("application/octet-stream", "application/xhtml+xml", "text/csv", "application/zip"):
            self.assertNotIn(never, a["types"])
        self.assertEqual(a["max_size_kb"], -(-(600 << 20) * 11 // 10240))
        self.assertEqual({c["entry"] for c in fixes}, {"asset:/t/"})
        self.assertEqual(len({b["path"] for b in m.broken}), len(m.broken))  # each file once

    def test_a_page_refused_by_a_prefix_never_removes_it(self):
        d = doc()
        m = verify.Measure(broken=[{"path": "/t/news/", "entry": "asset:/t/", "difference": "refused"}])
        correct.widen(d, m)
        self.assertEqual([a["prefix"] for a in d["assets"]], ["/t/"])

    def test_passive(self):
        for t in ("image/avif", "image/svg+xml", "font/woff2", "video/mp4", "audio/x-wav", "text/css", "application/pdf"):
            self.assertTrue(assettypes.passive(t), t)
        for t in ("text/plain", "text/csv", "text/html", "application/xhtml+xml", "text/xml", "application/xml",
                  "application/zip", "application/sql", "application/json", "image/*", "application/x-font_ttf",
                  "image/svg+xml; charset=utf-8"):
            self.assertFalse(assettypes.passive(t), t)
        self.assertTrue(assettypes.passive("application/javascript", loaded=True))
        self.assertFalse(assettypes.passive("application/javascript", loaded=False))

    def test_a_media_prefix_keeps_its_media_limit(self):
        """A 30 MiB image on a prefix with videos: the limit rises for it,
        never below the 512 MiB the videos had."""
        from equalizer_agent import mapbuild
        need, _ = mapbuild.size_limit({"prefix": "/t/", "types": ["image/png", "video/mp4"]}, "/t/a.png", 30 << 20)
        self.assertEqual(need, mapbuild.MEDIA_MAX_KB)
        need, _ = mapbuild.size_limit({"prefix": "/t/", "types": ["application/pdf", "video/mp4"]}, "/t/a.pdf", 30 << 20)
        self.assertGreaterEqual(need, 30 << 10)


if __name__ == "__main__":
    unittest.main()
