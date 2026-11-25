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

    def test_the_ai_knows_equalizer_decisions(self):
        rep = dict(REPORT, open=[{"target": "contact.message", "reason": "accepts_sql", "detail": "a quote in /contact?x=1"}])
        item = next(it for it in study.evidence(DOC, rep).items.values() if it.get("target") == "contact.message")
        self.assertIn("kept free on purpose", item["means"])
        self.assertNotIn("detail", item)  # the reason only
        self.assertIn("administration included", study.EQUALIZER)
        self.assertIn("no_sql_meta", study.EQUALIZER)
        # the old actions against those decisions are gone
        for gone in ("close_area", "add_role", "exclude_entry"):
            self.assertNotIn(gone, study.ACTION)
        self.assertIn("on_the_site", study.ACTION)
        llm = FakeLLM([{"findings": [finding(action="close_area")]}, {"summary": "s"}])
        self.assertEqual(study.study(llm, DOC, REPORT)["findings"][0]["action"], "none")  # kept, without that advice

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

    def test_an_answer_cut_keeps_its_whole_findings(self):
        whole = json.dumps({"findings": [finding(), finding(title="Second", severity="high")]})
        cut = whole[:-60]  # the provider stopped inside the second finding
        n = len(study.parts(study.evidence(DOC, REPORT)))
        out = study.study(FakeLLM([cut] + ['{"findings": []}'] * (n - 1) + ["{}"]), DOC, REPORT)
        self.assertEqual([f["title"] for f in out["findings"]], ["Public upload through the file manager"])

    def test_odd_answers_never_raise(self):
        n = len(study.parts(study.evidence(DOC, REPORT)))
        odd = [finding(refs=3), finding(refs=True), finding(refs=None), "x", None, 7]
        out = study.study(FakeLLM([{"findings": odd}] + [{"findings": [finding(), finding(title="B")]}] * (n - 1)
                                  + [{"duplicates": [[0, 1], [1, 0]], "order": "x", "summary": 5, "steps": "x"}]), DOC, REPORT)
        self.assertEqual(len(out["findings"]), 1 + (n - 2) * 2 if n > 2 else 1)  # one of the pair stays

    def test_visitor_keys_and_post_only_entries(self):
        report = {**REPORT, "visitors_added": [{"kind": "key", "route": "/shop/", "route_id": "shop", "key": "color"}],
                  "open": [{"target": "/x", "reason": "browser_skipped", "detail": "net::ERR at https://x/?token=abc"}],
                  "source": {"unseen": [{"path": "/shop/", "origin": "rest_route", "methods": ["POST"]},
                                        {"path": "/shop/", "origin": "php_file", "methods": ["GET"]}]}}
        ev = study.evidence(DOC, report)
        self.assertIn("/shop/?color", ev.labels.values())
        self.assertNotIn("token", json.dumps(ev.items))
        maps = [it["map"] for it in ev.items.values() if it.get("kind") == "code_only"]
        self.assertEqual(maps, ["area", "precise"])  # a POST to a page the map serves for GET goes through the area

    def test_run_hands_the_study_back(self):
        class Panel:
            def __init__(self):
                self.got = None

            def study_result(self, i, sec):
                self.got = (i, sec)
                return True

        class Log:
            def info(self, *a, **k): pass
            def error(self, *a, **k): pass
            def warn(self, *a, **k): pass

        class Cfg:
            allow_private_llm = False
        p = Panel()
        orig = study.study
        study.study = lambda llm, doc, rep: (_ for _ in ()).throw(RuntimeError("x"))
        import equalizer_agent.llm as llm_mod
        orig_p = llm_mod.provider
        llm_mod.provider = lambda cfg, allow: object()
        try:
            study.run({"id": 4, "app": "a", "version": 7, "document": DOC, "report": REPORT, "llm": {"provider": "x"}}, Cfg(), p, Log())
        finally:
            study.study, llm_mod.provider = orig, orig_p
        self.assertEqual(p.got, (4, {"used": False, "error": "the study could not be completed", "version": 7}))
        study.run({"id": 5, "version": 1, "llm": None}, Cfg(), p, Log())
        self.assertEqual(p.got[1]["reason"], "no AI model configured for this tenant")

        # the panel down for a moment: the study is sent again, not lost
        class Flaky(Panel):
            calls = 0

            def study_result(self, i, sec):
                Flaky.calls += 1
                if Flaky.calls < 3:
                    raise OSError("down")
                return super().study_result(i, sec)
        f, wait = Flaky(), study.RESULT_WAIT
        study.RESULT_WAIT = 0
        try:
            study.run({"id": 6, "version": 2, "llm": None}, Cfg(), f, Log())
        finally:
            study.RESULT_WAIT = wait
        self.assertEqual((Flaky.calls, f.got[0]), (3, 6))

    def test_parts(self):
        ev = study.evidence(DOC, REPORT)
        parts = study.parts(ev)
        self.assertTrue(all(i[0] in "EIOCVX" for i in parts[0]))
        self.assertIn("R1", parts[-1])


if __name__ == "__main__":
    unittest.main()
