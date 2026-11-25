import json
import unittest

from equalizer_agent.llm import study

DOC = {
    "routes": [{"id": "shop", "path": "/shop/", "method": "GET", "query": {"orderby": {"type": "enum", "values": ["date", "price"]},
                                                                           "s": {"type": "string", "max_length": 200, "rules": ["no_html"]}}},
               {"id": "order", "path": "/order/{id}", "params": {"id": {"type": "int", "min": 1, "max": 99999}}}],
    "forms": [{"id": "upload", "route": "/upload", "fields": {"doc": {"type": "file"}, "note": {"type": "text", "max_length": 500, "rules": ["no_html"]}}}],
    "areas": [{"id": "site", "prefix": "/", "methods": ["GET", "POST"]}],
}
REPORT = {
    "cms": "wordpress", "version": "6.4.1",
    "components": [{"name": "plugin:wp-file-manager", "version": "6.0"}],
    "known_issues": [{"component": "plugin:wp-file-manager", "version": "6.0", "fixed": "6.9"}],
    "source": {"unseen": [{"path": "/wp-admin/admin-ajax.php", "origin": "wp_ajax_nopriv", "query": {"action": "fm_upload"},
                           "public": True, "owner": "plugin wp-file-manager", "where": "wp-content/plugins/wp-file-manager/x.php:12"},
                          {"path": "/shop/", "origin": "php_file", "public": True, "where": "shop.php:1"}],
               "risks": [{"kind": "file_write", "input": "file", "route": "/wp-admin/admin-ajax.php",
                          "where": "wp-content/plugins/wp-file-manager/x.php:40", "owner": "plugin wp-file-manager"}],
               "unseen_total": 2, "risks_total": 1},
    "open": [{"target": "shop.s", "reason": "free_text"}],
    "issues_proposed": [{"list": "/orders", "detail": "/order/{id}", "field": "id"}],
}


class FakeLLM:
    def __init__(self, answers):
        self.answers, self.calls = list(answers), []

    def complete(self, system, user):
        self.calls.append((system, json.loads(user)))
        a = self.answers.pop(0)
        if isinstance(a, Exception):
            raise a
        return a if isinstance(a, str) else json.dumps(a)


def finding(**kw):
    base = {"severity": "critical", "category": "file_upload", "title": "Public upload through the file manager",
            "refs": ["E5", "R1", "C1"], "why": "a nopriv action writes a file", "attack": "upload a script",
            "coverage": "open", "coverage_why": "served by the site area", "action": "update_component",
            "action_detail": "update wp-file-manager"}
    base.update(kw)
    return base


class StudyTest(unittest.TestCase):
    def test_evidence_says_how_the_map_serves_each_entry(self):
        ev = study.evidence(DOC, REPORT)
        by_path = {it.get("path"): it for it in ev.items.values() if it.get("kind") == "code_only"}
        self.assertEqual(by_path["/wp-admin/admin-ajax.php"]["map"], "area")
        self.assertEqual(by_path["/shop/"]["map"], "precise")
        page = next(it for it in ev.items.values() if it.get("path") == "/shop/" and it.get("kind") == "page")
        self.assertEqual(page["query"], {"orderby": "enum(2)", "s": "string<=200 no_html"})
        self.assertTrue(next(it for it in ev.items.values() if it.get("name") == "plugin:wp-file-manager")["known_issue"])

    def test_only_names_and_types_reach_the_ai(self):
        llm = FakeLLM([{"findings": []}, {"summary": ""}])
        study.study(llm, DOC, REPORT)
        sent = json.dumps([c[1] for c in llm.calls])
        self.assertNotIn("values", sent)  # an enum's values stay out: only how many
        self.assertNotIn("price", sent)

    def test_findings_are_checked_and_ranked(self):
        answers = [{"findings": [finding(),
                                 finding(refs=["E99"], title="invented"),       # cites nothing given
                                 finding(severity="urgent", title="bad"),       # not a severity
                                 finding(severity="low", coverage="blocked", title="Order ids", refs=["E2", "I1"],
                                         category="access_control", action="owner_check"),
                                 finding(title="Same upload again")]}]
        answers += [{"findings": []}] * (len(study.parts(study.evidence(DOC, REPORT))) - 1)
        answers.append({"summary": "A shop with a public file manager.", "order": [1, 0], "duplicates": [[0, 2]],
                        "steps": ["Update wp-file-manager", ""]})
        llm = FakeLLM(answers)
        out = study.study(llm, DOC, REPORT)
        self.assertTrue(out["used"])
        self.assertEqual([f["title"] for f in out["findings"]], ["Order ids", "Public upload through the file manager"])
        self.assertEqual(out["steps"], ["Update wp-file-manager"])
        self.assertEqual(out["counts"]["critical"], 1)
        self.assertEqual(out["findings"][1]["refs"][0], {"id": "E5", "label": "/wp-admin/admin-ajax.php?action=fm_upload"})
        self.assertEqual(out["surface"]["uploads"], 1)
        self.assertEqual(out["surface"]["code_entries_public"], 2)

    def test_a_provider_that_fails(self):
        out = study.study(FakeLLM([RuntimeError("down")] * 10), DOC, REPORT)
        self.assertEqual(out["used"], False)
        part = study.study(FakeLLM([{"findings": [finding()]}, RuntimeError("down")] + [RuntimeError("x")] * 10), DOC, REPORT)
        self.assertTrue(part["used"] and part["partial"])
        self.assertEqual(len(part["findings"]), 1)

    def test_prose_and_junk_answers(self):
        n = len(study.parts(study.evidence(DOC, REPORT)))
        llm = FakeLLM(["I think the site is fine."] * (n - 1) + ["```json\n" + json.dumps({"findings": [finding(why="x" * 5000)]}) + "\n```"]
                      + ["not json"])
        out = study.study(llm, DOC, REPORT)
        self.assertEqual(len(out["findings"][0]["why"]), 600)
        self.assertEqual(out["summary"], "")

    def test_parts(self):
        ev = study.evidence(DOC, REPORT)
        parts = study.parts(ev)
        self.assertTrue(all(i[0] in "EIOCVX" for i in parts[0]))
        self.assertIn("R1", parts[-1])


if __name__ == "__main__":
    unittest.main()
