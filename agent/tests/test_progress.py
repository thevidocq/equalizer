import unittest

from equalizer_agent import crawler


class SayTest(unittest.TestCase):
    """The panel shows what the discovery is doing next to the page count."""

    def test_says_and_throttles(self):
        got = []
        c = crawler.Crawler(fetcher=None, cms=None, max_pages=10, log=None, progress=got.append)
        c.say("building the map")
        c.say("exploring", "/a/", force=False)  # within 2 s of the last: skipped
        c.say("the AI is studying the structure")
        self.assertEqual([p["doing"] for p in got], ["building the map", "the AI is studying the structure"])
        self.assertEqual(got[0]["pages"], 0)

    def test_path_is_cut(self):
        got = []
        c = crawler.Crawler(fetcher=None, cms=None, max_pages=10, log=None, progress=got.append)
        c.say("exploring", "/" + "x" * 300)
        self.assertEqual(len(got[0]["path"]), 100)

    def test_progress_failure_does_not_stop_the_study(self):
        def boom(_):
            raise OSError("panel down")
        c = crawler.Crawler(fetcher=None, cms=None, max_pages=10, log=None, progress=boom)
        c.say("exploring")  # no exception


if __name__ == "__main__":
    unittest.main()
