"""The source summary in a discovery (docs/agent.md): free-text keys and
fields the code always reads as a number, an e-mail or one of a few values
get that type, entry points never opened are reported, nothing else
changes."""
import copy
import json
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import source  # noqa: E402

EXAMPLE = os.path.join(os.path.dirname(__file__), "..", "..", "schema", "source", "examples", "wordpress.json")


def summary():
    with open(EXAMPLE) as f:
        s = json.load(f)
    s["routes"].append({"path": "/wp-json/demo/v1/status", "methods": ["GET"], "origin": "rest_route", "public": True,
                        "where": {"file": "wp-content/plugins/demo/demo.php", "line": 40}})
    s["routes"].append({"path": "/wp-json/demo/v1/private", "methods": ["GET"], "origin": "rest_route",
                        "where": {"file": "wp-content/plugins/demo/demo.php", "line": 41}})
    # read as an int in one place and raw in another: left alone
    s["inputs"] += [{"name": "cat", "source": "query", "type": "int", "where": {"file": "a.php", "line": 1}},
                    {"name": "cat", "source": "request", "type": "unknown", "where": {"file": "b.php", "line": 2}}]
    return s


def draft():
    text = {"type": "string", "max_length": 200, "rules": ["no_html", "no_template_meta"]}
    return {"routes": [
        {"id": "list", "path": "/list/", "origin": "agent", "query": {"id": dict(text), "order": dict(text), "cat": dict(text),
                                                                         "raw": dict(text)},
         "legacy_request": {"method": "GET", "path": "/list/",
                            "query": {"id": "{{query.id}}", "order": "{{query.order}}", "cat": "{{query.cat}}", "raw": "{{query.raw}}"}}},
        {"id": "mine", "path": "/mine/", "origin": "human", "query": {"id": dict(text)},
         "legacy_request": {"method": "GET", "path": "/mine/", "query": {"id": "{{query.id}}"}}}],
        "forms": [{"id": "contact_form", "route": "/contact", "origin": "agent",
                   "fields": {"email": dict(text, required=True), "q": dict(text)},
                   "legacy_request": {"method": "POST", "path": "/contact", "form": {"email": "{{fields.email}}", "q": "{{fields.q}}"}}}]}


class SourceTest(unittest.TestCase):
    def test_types(self):
        doc = draft()
        report = {"open": [{"kind": "input", "target": "list.id", "reason": "accepts_sql"},
                           {"kind": "input", "target": "list.raw", "reason": "accepts_sql"}]}
        job = {"source_status": "used", "source": summary()}
        source.apply(report, doc, job, {("list", "id"): ["3", "12"], ("list", "order"): ["asc"]},
                     {"/", "/list/"})
        q = doc["routes"][0]["query"]
        self.assertEqual(q["id"]["type"], "int")
        self.assertEqual(q["order"], {"type": "enum", "values": ["asc", "desc"]})
        self.assertEqual(q["cat"]["type"], "string", "two readings of cat")
        self.assertEqual(q["raw"]["type"], "string", "read raw")
        self.assertEqual(doc["routes"][1]["query"]["id"]["type"], "string", "a person's entry")
        f = doc["forms"][0]["fields"]
        self.assertEqual(f["email"], {"type": "email", "required": True})
        self.assertEqual(f["q"]["type"], "string", "sanitize_text_field is text")
        self.assertEqual([o["target"] for o in report["open"]], ["list.raw"])
        s = report["source"]
        self.assertEqual((s["status"], s["stack"]), ("used", "wordpress"))
        self.assertEqual({(t["field"], t["type"]) for t in s["typed"]}, {("id", "int"), ("order", "enum"), ("email", "email")})
        self.assertIn({"kind": "include", "input": "page", "route": "/wp-content/plugins/demo/direct.php",
                       "where": "wp-content/plugins/demo/direct.php:3"}, s["risks"])
        self.assertIn({"path": "/wp-content/plugins/demo/direct.php", "origin": "php_file", "action": None, "public": True},
                      s["unseen"])
        self.assertIn({"path": "/wp-json/demo/v1/status", "origin": "rest_route", "action": None, "public": True}, s["unseen"])

    def test_values_seen_must_fit(self):
        doc = draft()
        report = {"open": []}
        source.apply(report, doc, {"source_status": "used", "source": summary()},
                     {("list", "id"): ["3", "abc"], ("list", "order"): ["price"]}, set())
        q = doc["routes"][0]["query"]
        self.assertEqual((q["id"]["type"], q["order"]["type"]), ("string", "string"))

    def test_failed_or_late(self):
        doc, report = draft(), {}
        before = copy.deepcopy(doc)
        source.apply(report, doc, {"source_status": "late", "source_error": "the source reader did not finish"}, {}, set())
        self.assertEqual(report["source"], {"status": "late", "error": "the source reader did not finish"})
        self.assertEqual(doc, before)
        report = {}
        source.apply(report, doc, {}, {}, set())
        self.assertNotIn("source", report)


if __name__ == "__main__":
    unittest.main()
