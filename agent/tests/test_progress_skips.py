"""Pages skipped in the browser are accounted for, and a verification says
where it is (docs/agent.md)."""
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import crawler, mapbuild, verify  # noqa: E402


class _Log:
    def __init__(self):
        self.lines = []

    def info(self, msg, **kw):
        self.lines.append((msg, kw))


class SkippedPagesTest(unittest.TestCase):
    def test_in_report_and_still_open(self):
        crawl = crawler.CrawlResult()
        crawl.browser_page_seconds = 20
        crawl.browser_skipped = [{"path": "/shop/?x=1", "reason": "time_limit", "seconds": 20.0, "detail": "", "account": "anonymous"},
                                 {"path": "/cart/", "reason": "error", "seconds": 1.2, "detail": "net::ERR_ABORTED", "account": "anonymous"}]
        out = mapbuild._still_open(crawl, [], [])
        skipped = [o for o in out if o["reason"] == "browser_skipped"]
        self.assertEqual([o["target"] for o in skipped], ["/shop/", "/cart/"])
        self.assertIn("time limit reached after 20.0s", skipped[0]["detail"])
        self.assertIn("net::ERR_ABORTED", skipped[1]["detail"])


class ProgressTest(unittest.TestCase):
    def test_told_and_logged(self):
        told, log = [], _Log()
        r = verify.Run.__new__(verify.Run)
        r.job, r.log, r.progress, r.step, r._told = {"id": 7}, log, lambda *a: told.append(a), "pages", 0.0
        for n in range(120):
            r.tell("pages", n, 349, None, force=n == 0)
        self.assertEqual(told[0], ("pages", 0, 349, None))
        self.assertLessEqual(len(told), 2, "at most every 2 seconds")
        self.assertEqual([kw["done"] for _, kw in log.lines], [50, 100])
        r.step = "correcting"
        r.tell("pages", 1, 10, "editor", force=True)
        self.assertEqual(told[-1], ("correcting", 1, 10, "editor"))


if __name__ == "__main__":
    unittest.main()
