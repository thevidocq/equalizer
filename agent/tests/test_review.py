import io
import json
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import config, review  # noqa: E402
from equalizer_agent.log import Logger  # noqa: E402


class Panel:
    def __init__(self):
        self.out = None

    def review_result(self, jid, summary="", questions=None, error=None):
        self.out = {"summary": summary, "questions": questions, "error": error}


class LLM:
    def __init__(self, text):
        self.text, self.user = text, ""

    def complete(self, system, user):
        self.user = user
        return self.text


class ReviewTest(unittest.TestCase):
    job = {"id": 3, "app": "portal", "version": 2, "note": "add backups",
           "review": {"base_version": 1, "changes": [{"section": "routes", "entry": "bk", "change": "added", "detail": "/backup/{f}"}],
                      "findings": [{"severity": "high", "entry": "routes.bk", "rule": "sensitive_path", "message": "backup"}]},
           "llm": {"provider": "anthropic", "model": "m", "api_key": "k"}}

    def run_with(self, text):
        llm, panel = LLM(text), Panel()
        orig = review.provider
        review.provider = lambda cfg, allow: llm
        try:
            review.run(self.job, config.Config(panel_token="x" * 32), panel, Logger("t", "warn", io.StringIO()))
        finally:
            review.provider = orig
        return llm, panel.out

    def test_summary_and_questions(self):
        llm, out = self.run_with('Sure! {"summary": "Opens /backup/{f} to everyone.", "questions": ["Why?", 3]}')
        self.assertEqual(out["summary"], "Opens /backup/{f} to everyone.")
        self.assertEqual(out["questions"], ["Why?"])
        self.assertIn("sensitive_path", llm.user)
        self.assertNotIn("api_key", llm.user)

    def test_no_summary_is_a_failure(self):
        _, out = self.run_with("I cannot help with that")
        self.assertTrue(out["error"])


if __name__ == "__main__":
    unittest.main()
