"""The study's first step: several test accounts, and form fields typed
from what their HTML declares (docs/agent.md)."""
import io
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from equalizer_agent import config, discovery, extract, fields  # noqa: E402
from equalizer_agent.log import Logger  # noqa: E402
from tests.fakesite import Site  # noqa: E402
from tests.test_discovery import FakePanel  # noqa: E402


class AccountsTest(unittest.TestCase):
    def setUp(self):
        self.site = Site()
        dest = f"127.0.0.1:{self.site.port}"
        self.cfg = config.Config(panel_token="x" * 32, direct={dest}, rate=0)
        self.job = {"id": 8, "tenant": "acme", "app": "news", "domain": "news.acme.test", "kind": None,
                    "base_url": "http://news.acme.test", "max_pages": 100,
                    "reach": {"mode": "direct", "address": dest, "allow": [dest]}, "llm": None, "unmapped": [],
                    "accounts": [{"label": "member", "username": "tester", "password": "pw"},
                                 {"label": "editor", "username": "editor", "password": "pw2"},
                                 {"label": "broken", "username": "nobody", "password": "x"}]}

    def tearDown(self):
        self.site.close()

    def run_job(self, job):
        panel = FakePanel()
        discovery.run(job, self.cfg, panel, Logger("test", "warn", io.StringIO()))
        return panel.results[0]

    def test_each_account_has_its_own_pass(self):
        res = self.run_job(self.job)
        rep, doc = res["report"], res["document"]
        self.assertEqual(rep["accounts"], [{"label": "member", "login": "ok"}, {"label": "editor", "login": "ok"},
                                           {"label": "broken", "login": "failed"}])
        routes = {r["path"]: r for r in doc["routes"]}
        self.assertIn("/members/area/profile", routes)
        self.assertIn("/editors/desk", routes)
        # the map does not restrict pages by account: the legacy site decides
        self.assertEqual(routes["/editors/desk"]["roles"], ["guest"])
        # an account never borrows another's session: the editor's desk was
        # only asked with the editor's cookie
        posts = [b for m, p, b in self.site.requests if m == "POST" and p == "/wp-login.php"]
        self.assertEqual(len(posts), 3)

    def test_older_panel_single_account(self):
        job = dict(self.job, accounts=None, account={"username": "tester", "password": "pw"})
        rep = self.run_job(job)["report"]
        self.assertEqual(rep["accounts"], [{"label": "account", "login": "ok"}])
        self.assertEqual(rep["login"], "ok")


class FieldsTest(unittest.TestCase):
    def form(self, html):
        return extract.parse("<form method=post action=/f>" + html + "</form>", "http://x/").forms[0]

    def test_html_attributes_give_tight_types(self):
        f = self.form('''<input name="first_name" maxlength="60"><input name="nickname"><input type="tel" name="phone">
            <input name="zip" pattern="[0-9]{5}"><input name="bad" pattern="(?=a)b"><input name="q">
            <input type="number" name="age" min="18" max="120"><input type="number" name="weight" step="0.1">
            <input type="radio" name="plan" value="free"><input type="radio" name="plan" value="pro">
            <input type="checkbox" name="news"><select name="country"><option value="it"><option value="fr"></select>
            <input type="url" name="site"><textarea name="bio" maxlength="300"></textarea>''')
        got = {i.name: fields.spec(i) for i in f.visible()}
        self.assertEqual(got["first_name"], ({"type": "string", "max_length": 60,
                                              "rules": ["no_html", "no_template_meta", "no_sql_meta"]}, ""))
        self.assertEqual(got["nickname"][1], "accepts_sql")
        self.assertEqual(got["q"][1], "accepts_sql")  # a search may hold quotes
        self.assertEqual(got["zip"][0]["pattern"], "^(?:[0-9]{5})$")
        self.assertNotIn("pattern", got["bad"][0])  # lookahead: not RE2, not kept
        big = self.form('<input name="code" pattern="[a-z]{1,2000}">').visible()[0]
        self.assertNotIn("pattern", fields.spec(big)[0])  # RE2: at most 1000 repeats
        self.assertEqual(got["age"][0], {"type": "int", "min": 18, "max": 120})
        self.assertEqual(got["weight"][0]["type"], "string")
        self.assertEqual(got["plan"][0], {"type": "enum", "values": ["free", "pro"]})
        self.assertEqual(got["news"][0], {"type": "enum", "values": ["on"]})
        self.assertEqual(got["country"][0], {"type": "enum", "values": ["fr", "it"]})
        self.assertTrue(got["phone"][0]["pattern"].startswith("^[0-9+"))
        self.assertTrue(got["site"][0]["pattern"].startswith("^https?://"))
        self.assertEqual((got["bio"][0]["type"], got["bio"][0]["max_length"], got["bio"][1]), ("text", 300, "accepts_sql"))

    def test_still_open_in_the_report(self):
        site = Site()
        try:
            dest = f"127.0.0.1:{site.port}"
            cfg = config.Config(panel_token="x" * 32, direct={dest}, rate=0)
            job = {"id": 9, "tenant": "acme", "app": "news", "domain": "news.acme.test", "kind": None,
                   "base_url": "http://news.acme.test", "max_pages": 100, "llm": None, "unmapped": [],
                   "reach": {"mode": "direct", "address": dest, "allow": [dest]}}
            panel = FakePanel()
            discovery.run(job, cfg, panel, Logger("test", "warn", io.StringIO()))
            rep, doc = panel.results[0]["report"], panel.results[0]["document"]
        finally:
            site.close()
        signup = next(f for f in doc["forms"] if f["route"] == "/signup/")
        self.assertFalse(signup["fields"]["news"]["required"])
        opened = {(o["kind"], o["target"], o["reason"]) for o in rep["open"]}
        self.assertIn(("input", signup["id"] + ".bio", "accepts_sql"), opened)
        self.assertNotIn(("input", signup["id"] + ".zip", "accepts_sql"), opened)
        self.assertIn(("path", "/wp-admin/", "not_reached"), opened)
        self.assertIn(("path", "/logout", "action_not_performed"), opened)


if __name__ == "__main__":
    unittest.main()


class ReadOnlyTest(unittest.TestCase):
    def test_rules(self):
        from equalizer_agent import rules
        # the administration: only a test account opens it
        self.assertEqual(rules.excluded("/admin/content", "drupal"), "admin")
        self.assertEqual(rules.excluded("/admin/content", "drupal", logged_in=True), "")
        # WordPress writes when its administration is only viewed: closed
        self.assertEqual(rules.excluded("/wp-admin/edit.php", "wordpress", logged_in=True), "admin")
        # never, not even logged in: jobs, installers, updates, exports
        for p in ("/wp-cron.php", "/wp-admin/update-core.php", "/wp-admin/export.php", "/update.php", "/cron/abc",
                  "/administrator/index.php?option=com_installer", "/admin/reports/updates",
                  "/wp-admin/post-new.php"):  # WordPress saves an auto-draft when it opens
            self.assertEqual(rules.excluded(p, None, logged_in=True), "never", p)
        # links that act: verbs as words, one-time tokens
        for p in ("/wp-admin/post.php?post=5&action=trash&_wpnonce=ab12", "/wp-admin/plugins.php?action=deactivate",
                  "/administrator/index.php?option=com_content&task=articles.publish",
                  "/admin/content?0123456789abcdef0123456789abcdef=1", "/node/3/delete", "/user/logout?token=x",
                  "/wp-admin/edit-comments.php?c=2&action=spam",
                  "/administrator/index.php?option=com_content&view=article&layout=edit&id=1"):
            self.assertTrue(rules.is_action(p), p)
        for p in ("/wp-admin/edit.php?post_type=page", "/running-tips/", "/administrator/index.php?option=com_content&view=articles",
                  "/admin/content", "/nuclear-power/"):
            self.assertFalse(rules.is_action(p), p)

    def test_fetcher_never_writes(self):
        from equalizer_agent.fetch import Fetcher, ReadOnlyError
        f = Fetcher("http://x.test", {"mode": "direct", "allow": ["x.test:80"]}, "t", direct={"x.test:80"})
        for m in ("POST", "PUT", "PATCH", "DELETE"):
            with self.assertRaises(ReadOnlyError):
                f.request(m, "/")
        with self.assertRaises(ReadOnlyError):
            f.request("PUT", "/", login=True)


CHROME = "/usr/bin/google-chrome" if os.path.exists("/usr/bin/google-chrome") else ""


@unittest.skipUnless(__import__("equalizer_agent.browser", fromlist=["x"]).available(), "Playwright is not installed")
class BrowserTest(unittest.TestCase):
    def test_scripts_requests_are_studied_behind_the_guard(self):
        site = Site()
        try:
            dest = f"127.0.0.1:{site.port}"
            cfg = config.Config(panel_token="x" * 32, direct={dest}, rate=0, browser="on", browser_path=CHROME)
            job = {"id": 10, "tenant": "acme", "app": "news", "domain": "news.acme.test", "kind": None,
                   "base_url": f"http://{dest}", "max_pages": 100, "llm": None, "unmapped": [],
                   "reach": {"mode": "direct", "address": dest, "allow": [dest]}}
            panel = FakePanel()
            discovery.run(job, cfg, panel, Logger("test", "warn", io.StringIO()))
            rep, doc = panel.results[0]["report"], panel.results[0]["document"]
        finally:
            site.close()
        self.assertGreater(rep["browser"]["pages"], 0)
        # the script's GET was studied and mapped
        self.assertIn("/data/items?page=1", rep["browser"]["script_requests"])
        # a JSON answer stays with the site area (map APIs need their shapes), said in the report
        opened = {(o["target"], o["reason"]) for o in rep["open"]}
        self.assertIn(("/data/items", "other"), opened)
        # files the pages load are assets, not pages
        self.assertFalse(any(p.endswith(".css") or ".css?" in p for p in rep["browser"]["script_requests"]))
        # the POST never reached the site; the action link neither
        sent = [(m, p) for m, p, *_ in site.requests]
        self.assertNotIn(("POST", "/api/track"), sent)
        self.assertNotIn(("GET", "/logout"), sent)
        self.assertIn({"method": "POST", "path": "/api/track"}, rep["browser"]["kept_in"])
        self.assertIn(("/api/track", "action_not_performed"), opened)
