"""Files the pages load are verified through the gateway, and the agent
mends its draft when the gateway refuses them (docs/agent.md, step 4)."""
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import correct, verify, verifyassets  # noqa: E402
from equalizer_agent.fetch import Response  # noqa: E402

PAGE = b"""<html><head><link rel=stylesheet href="/t/css/s.css"></head><body>
<img src="/t/img/a.avif"><video src="/t/video/big.mp4"></video><img src="/up/photo.jpg">
<img src="https://cdn.elsewhere.test/x.png"></body></html>"""
FILES = {
    "/t/css/s.css": (200, "text/css", b"@font-face{src:url(../fonts/f.woff)} .x{background:url(/t/img/b.png)}", ""),
    "/t/img/a.avif": (200, "image/avif", b"", ""),
    "/t/video/big.mp4": (200, "video/mp4", b"", str(600 << 20)),
    "/t/fonts/f.woff": (200, "application/octet-stream", b"wOFF", ""),
    "/t/img/b.png": (200, "image/png", b"", ""),
    "/up/photo.jpg": (200, "image/jpeg", b"", ""),
}


class Fetcher:
    def same_site(self, url):
        return url.startswith("/") or url.startswith("http://site.test/")

    def get(self, path):
        status, ctype, body, size = FILES.get(path, (404, "text/html", b"", ""))
        h = {"content-type": ctype}
        if size:
            h["content-length"] = size
        return Response(status, h, body, "http://site.test" + path)


class Gateway:
    """Refuses what the draft would refuse: AVIF and big videos under /t/."""

    def __init__(self):
        self.asked = []

    def preview(self, path, headers=None, session=None, method="GET"):
        self.asked.append((method, path))
        refusal = {"/t/img/a.avif": "asset_type_not_allowed", "/t/video/big.mp4": "asset_too_large",
                   "/t/fonts/f.woff": "asset_type_not_allowed"}.get(path)
        return {"entry": "asset:/t/", "refusal": {"reason": refusal} if refusal else None}


class VerifyAssetsTest(unittest.TestCase):
    def doc(self):
        return {"assets": [{"prefix": "/t/", "types": ["text/css", "image/png", "video/mp4", "font/woff"]}],
                "areas": [{"id": "site", "prefix": "/"}]}

    def test_files_are_checked_and_mended(self):
        m, gw, doc = verify.Measure(), Gateway(), self.doc()
        files = verifyassets.names(PAGE, "http://site.test/")
        verifyassets.check(gw, Fetcher(), doc, m, files)
        asked = [p for _, p in gw.asked]
        # every file of an asset prefix, those of the stylesheet too, never
        # one the area serves (/up/) or another site's
        self.assertEqual(sorted(asked), sorted(["/t/css/s.css", "/t/img/a.avif", "/t/video/big.mp4",
                                                "/t/fonts/f.woff", "/t/img/b.png"]))
        self.assertTrue(all(meth == "HEAD" for meth, _ in gw.asked))
        self.assertEqual({b["path"] for b in m.broken}, {"/t/img/a.avif", "/t/video/big.mp4", "/t/fonts/f.woff"})
        fixes = correct.widen(doc, m)
        a = doc["assets"][0]
        self.assertIn("image/avif", a["types"])
        self.assertNotIn("application/octet-stream", a["types"])  # never a generic type
        self.assertEqual(a["max_size_kb"], -(-(600 << 20) * 11 // 10240))
        self.assertEqual({c["entry"] for c in fixes}, {"asset:/t/"})
        self.assertEqual(len(fixes), 2)

    def test_a_page_under_an_asset_prefix_removes_the_prefix(self):
        """/t/ also holds a page no route matches: the asset prefix refuses
        it, so the prefix goes and the site area serves the whole folder."""
        m = verify.Measure(broken=[{"path": "/t/news/", "entry": "asset:/t/", "difference": "refused"},
                                   {"path": "/t/img/a.avif", "entry": "asset:/t/", "difference": "refused"}],
                           broken_files={"/t/img/a.avif"}, asset_fixes={"/t/": {"types": ["image/avif"], "max_size_kb": 0}})
        doc = self.doc()
        fixes = correct.widen(doc, m)
        self.assertEqual(doc["assets"], [])
        self.assertEqual([(c["entry"], c["change"]) for c in fixes], [("asset:/t/", "removed")])

    def test_a_file_the_site_does_not_serve_is_not_broken(self):
        m = verify.Measure()
        verifyassets.check(Gateway(), Fetcher(), self.doc(), m, ["/t/img/missing.png"])
        self.assertEqual(m.broken, [])


if __name__ == "__main__":
    unittest.main()
