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


class WebappsTest(unittest.TestCase):
    def test_compatibility_matrix_is_up_to_date(self):
        from equalizer_agent.webapps import matrix
        path = os.path.join(os.path.dirname(__file__), "..", "..", "docs", "compatibility.md")
        with open(path, encoding="utf-8") as f:
            self.assertEqual(f.read(), matrix.render(),
                             "docs/compatibility.md is stale: python3 -m equalizer_agent.webapps.matrix > ../docs/compatibility.md")

    def test_profiles(self):
        import re
        from equalizer_agent import fingerprint, rules, webapps
        names = webapps.names()
        self.assertEqual(len(names), len(set(names)))
        for p in webapps.ALL:  # every rule compiles; a profile opens an administration only if tested
            for rx in p.admin + p.never:
                re.compile(rx)
            self.assertFalse(p.admin_read_only and not p.tested, p.name)
        self.assertIs(webapps.get("nothing"), webapps.GENERIC)
        # recognition from the generator tag, for the new ones too
        for gen, cms in (("TYPO3 CMS 12.4", "typo3"), ("MediaWiki 1.39.5", "mediawiki"), ("PrestaShop", "prestashop"),
                         ("Joomla! - Open Source Content Management", "joomla")):
            fp = fingerprint.Fingerprint()
            fingerprint.detect(fp, gen, [])
            self.assertEqual(fp.cms, cms, gen)
        fp = fingerprint.Fingerprint()
        fingerprint.detect(fp, "", ["http://x/static/version1700000000/frontend/Magento/luma/en_US/Magento_Theme/x.js"])
        self.assertEqual(fp.cms, "magento")
        # pages that act when opened, from the new profiles
        for pq, cms in (("/checkout/cart/add/product/5", "magento"), ("/index.php?route=account/logout", "opencart"),
                        ("/index.php?title=Main&action=edit", "mediawiki"), ("/posting.php?mode=reply&t=3", "phpbb"),
                        ("/typo3/install.php", "typo3"), ("/admin/index.php", "moodle")):
            self.assertEqual(rules.excluded(pq, cms, logged_in=True), "never", pq)


class SessionKeptLoginTest(unittest.TestCase):
    """A login that keeps the session cookie and answers in JSON, with a
    token in the form's action that belongs to the session which saw the
    form, and a sign-up form seen first (OpenCart's): the agent logs in all
    the same, with the login form."""

    def setUp(self):
        import itertools
        import threading
        from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
        ids, logged = itertools.count(1), set()

        class H(BaseHTTPRequestHandler):
            def log_message(self, *a):
                pass

            def _send(self, status, body="", headers=None, ctype="text/html"):
                self.send_response(status)
                self.send_header("Content-Type", ctype)
                for k, v in (headers or {}).items():
                    self.send_header(k, v)
                self.end_headers()
                self.wfile.write(body.encode())

            def _sid(self):
                c = self.headers.get("Cookie") or ""
                return c.split("sid=")[1].split(";")[0] if "sid=" in c else ""

            def do_GET(self):
                sid = self._sid() or str(next(ids))
                cookie = {"Set-Cookie": f"sid={sid}; Path=/"}
                p = self.path.split("?")[0]
                if p == "/":
                    member = '<a href="/account/orders">Orders</a>' if sid in logged else ""
                    return self._send(200, f'<html><a href="/register">Sign up</a> <a href="/login">Log in</a>{member}</html>',
                                      cookie)
                if p == "/register":  # a sign-up form first: never the one used to log in
                    return self._send(200, '<html><form method="post" action="/register.do"><input name="firstname">'
                                           '<input type="email" name="email"><input type="password" name="password">'
                                           '</form></html>', cookie)
                if p == "/login":
                    if sid in logged:
                        return self._send(302, headers={"Location": "/account/orders", **cookie})
                    return self._send(200, f'<html><form method="post" action="/login.do?token=t{sid}">'
                                           '<input type="email" name="email"><input type="password" name="password">'
                                           '<input type="hidden" name="redirect" value=""></form></html>', cookie)
                if p == "/account/orders" and sid in logged:
                    return self._send(200, "<html><title>Orders</title></html>", cookie)
                return self._send(404, "<html>not found</html>", cookie)

            def do_POST(self):
                body = self.rfile.read(int(self.headers.get("Content-Length") or 0)).decode()
                sid = self._sid()
                ok = self.path == f"/login.do?token=t{sid}" and "email=a%40acme.test" in body and "password=pw" in body
                if ok:
                    logged.add(sid)
                # a refusal sets a cookie of its own: the answer decides, not the cookies
                return self._send(200, '{"redirect": "/account/orders"}' if ok else '{"error": "no"}',
                                  {"Set-Cookie": f"sid={sid}; Path=/" if ok else "flash=error; Path=/"},
                                  "application/json")

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), H)
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        dest = f"127.0.0.1:{self.server.server_address[1]}"
        self.cfg = config.Config(panel_token="x" * 32, direct={dest}, rate=0)
        self.job = {"id": 9, "tenant": "acme", "app": "cart", "domain": "cart.acme.test", "kind": None,
                    "base_url": "http://cart.acme.test", "max_pages": 50,
                    "reach": {"mode": "direct", "address": dest, "allow": [dest]}, "llm": None, "unmapped": [],
                    "accounts": [{"label": "customer", "username": "a@acme.test", "password": "pw"},
                                 {"label": "wrong", "username": "a@acme.test", "password": "nope"}]}

    def tearDown(self):
        self.server.shutdown()

    def test_logs_in_and_studies_member_pages(self):
        panel = FakePanel()
        discovery.run(self.job, self.cfg, panel, Logger("test", "warn", io.StringIO()))
        res = panel.results[0]
        self.assertEqual(res["report"]["accounts"], [{"label": "customer", "login": "ok"},
                                                     {"label": "wrong", "login": "failed"}])
        self.assertIn("/account/orders", {r["path"] for r in res["document"]["routes"]})


class MapPathTest(unittest.TestCase):
    def test_paths_a_map_cannot_hold_stay_in_the_area(self):
        """Magento puts a base64 referer in the path (…/referer/aHR0…~/):
        such a page is not proposed and says why."""
        from equalizer_agent import crawler, fingerprint, mapbuild
        crawl = crawler.CrawlResult()
        odd = "/index.php/customer/account/login/referer/aHR0cDovL3N0b3Jl~/"
        for pq in ("/", "/about/", odd):
            crawl.pages[pq] = crawler.Seen(200, "text/html")
        crawl.forms.append(extract.parse('<form method="post" action="%s"><input name="q"></form>' % odd.replace(
            "/login/", "/loginPost/"), "http://x.test/").forms[0])
        job = {"base_url": "http://x.test", "reach": {"mode": "direct", "address": "127.0.0.1:1"}, "unmapped": []}
        doc, rep, _ = mapbuild.build(job, crawl, fingerprint.Fingerprint())
        self.assertNotIn(odd, {r["path"] for r in doc["routes"]})
        self.assertFalse([f for f in doc.get("forms", []) if "~" in f["route"]])
        self.assertIn(odd, {o["target"] for o in rep["open"] if o["reason"] == "other"})


class FragmentsTest(unittest.TestCase):
    def test_templates_are_assets_under_a_stable_prefix(self):
        """Magento's knockout templates are HTML files its scripts load:
        assets of type text/html, under a prefix that does not name the
        deployment (/static/version1795173710/ changes with the next one)."""
        from equalizer_agent import cluster, crawler
        from equalizer_agent.fetch import Fetcher
        c = crawler.Crawler(Fetcher("http://store.test", {"mode": "direct", "address": "127.0.0.1:1",
                                                          "allow": ["127.0.0.1:1"]}, "acme", "", "", {"127.0.0.1:1"}, 0),
                            "magento", 10, Logger("t", "warn", io.StringIO()))
        tpl = "/static/version1795173710/frontend/Magento/luma/en_US/Magento_Ui/templates/modal/modal-popup.html"
        c._asset("http://store.test" + tpl)
        c._asset("http://store.test/static/version1795173710/frontend/Magento/luma/en_US/css/styles-m.css")
        self.assertEqual(c.r.assets[tpl], "text/html")
        self.assertEqual(cluster.asset_prefixes(c.r.assets), {"/static/": ["text/css", "text/html"]})
        # elsewhere an .html file is a page
        g = crawler.Crawler(c.f, "wordpress", 10, Logger("t", "warn", io.StringIO()))
        g._asset("http://store.test/static/page.html")
        self.assertNotIn("/static/page.html", g.r.assets)


class FormQueryTest(unittest.TestCase):
    def test_a_session_value_in_the_action_is_kept_server_side(self):
        """phpBB's login posts to ucp.php?mode=login&sid=…: the mode stays a
        fixed value, the session id is taken from each page, like a hidden
        input, so the form is recognised in every session."""
        from equalizer_agent import mapbuild
        form = extract.parse('<form method="post" action="/ucp.php?mode=login&amp;sid=92fb00cb054461322fd0914ca501fb49">'
                             '<input name="username"><input type="password" name="password"></form>', "http://x.test/").forms[0]
        e, why = mapbuild._form_entry(form, "guest", set(), [])
        self.assertEqual(why, "")
        self.assertEqual(e["legacy_request"]["query"], {"mode": "login", "sid": "{{hidden.sid}}"})


class AssetTypesTest(unittest.TestCase):
    def test_existing_prefix_gets_the_types_the_site_serves(self):
        """A video under a prefix already in the map (that listed only images)
        widens that prefix in the next discovery: nobody edits types by hand."""
        from equalizer_agent import crawler, fingerprint, mapbuild
        crawl = crawler.CrawlResult()
        crawl.pages["/"] = crawler.Seen(200, "text/html")
        crawl.assets["/site/templates/front/metaliquid/video/homepage-video.mp4"] = "video/mp4"
        crawl.assets["/site/templates/front/metaliquid/logo.png"] = "image/png"
        crawl.assets["/new/files/a.css"] = "text/css"
        current = {"meta": {"version": 2}, "legacy": {}, "roles": {"anonymous": "guest"},
                   "routes": [{"id": "home", "path": "/", "method": "GET"}],
                   "assets": [{"prefix": "/site/templates/", "types": ["image/png", "font/woff"]}]}
        job = {"base_url": "http://x.test", "reach": {"mode": "direct", "address": "127.0.0.1:1"},
               "unmapped": [], "current_map": current}
        doc, rep, _ = mapbuild.build(job, crawl, fingerprint.Fingerprint())
        tpl = next(a for a in doc["assets"] if a["prefix"] == "/site/templates/")
        self.assertEqual(tpl["types"], ["image/png", "font/woff", "video/mp4"])
        self.assertIn({"prefix": "/site/templates/", "types": ["video/mp4"], "widened": True}, rep["assets_added"])
        self.assertIn({"prefix": "/new/files/", "types": ["text/css"]}, rep["assets_added"])
        self.assertEqual(current["assets"][0]["types"], ["image/png", "font/woff"])  # the job's map is untouched
