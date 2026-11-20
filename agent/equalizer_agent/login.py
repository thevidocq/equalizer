"""How the agent logs in with a test account, as a person does with the
site's own form (docs/agent.md, "How it logs in")."""

import json
import re
import time
import urllib.parse

from . import extract, webapps
from .fetch import ReachError


def path_query(url: str) -> str:
    u = urllib.parse.urlsplit(url)
    q = urllib.parse.urlencode(sorted(urllib.parse.parse_qsl(u.query, keep_blank_values=True)))
    return (u.path or "/") + ("?" + q if q else "")


class LoginMixin:
    """The Crawler's logins: self.f (Fetcher), self.r (CrawlResult),
    self.cms and self.log come from it."""

    def _admin_login(self, account: dict) -> bool:
        """An administration with its own login (Joomla): its form, posted
        once like the site's. Its outcome does not change the report's."""
        page = webapps.get(self.cms).admin_login
        try:
            resp = self.f.get(page)
        except (ReachError, OSError, ValueError):
            return False
        html = resp.body.decode("utf-8", "replace")
        for form in extract.parse(html, resp.url).forms:
            pw = [i for i in form.inputs if i.type == "password"]
            user = [i for i in form.visible() if i.type in ("text", "email")]
            if form.method != "POST" or not pw or not user or not self.f.same_site(form.action):
                continue
            data = {i.name: "" for i in form.inputs if i.type not in ("submit", "button", "image", "reset")}
            data.update({h.name: h.value for h in form.hidden()})
            data[user[0].name], data[pw[0].name] = account.get("username", ""), account.get("password", "")
            before = dict(self.f.cookies)
            try:
                r = self.f.request("POST", path_query(form.action), data, login=True)
            except (ReachError, OSError, ValueError):
                return False
            ok = _logged_in(r, before, self.f.cookies)
            self.log.info("test account administration login", outcome="ok" if ok else "failed", form=page)
            return ok
        return False

    def _login(self, account: dict) -> bool:
        """Posts the site's login form, as a browser would: of the forms with
        a password field, a login page's first and one with a single
        password before a sign-up form (OpenCart's register form has one)."""
        for form in sorted(self.r.forms, key=_login_rank):
            pw = [i for i in form.inputs if i.type == "password"]
            user = [i for i in form.visible() if i.type in ("text", "email")]
            if not pw or not user or form.method != "POST":
                continue
            user.sort(key=lambda i: 0 if i.type == "email" or USER_NAME.search(i.name or "") else 1)
            data = {i.name: "" for i in form.inputs if i.type not in ("submit", "button", "image", "reset")}
            # A fresh copy of the page that shows the form, in this session:
            # its hidden values and the tokens some sites put in the action
            # (OpenCart's login_token) belong to the session that saw them.
            page_pq = path_query(form.action)
            shown = path_query(form.page) if form.page else page_pq
            try:
                fresh = self.f.get(shown)
                html = fresh.body.decode("utf-8", "replace")
                for f2 in extract.parse(html, fresh.url).forms:
                    pw2 = [i for i in f2.inputs if i.type == "password"]
                    user2 = [i for i in f2.visible() if i.type in ("text", "email")]
                    if path_query(f2.action).split("?")[0] == page_pq.split("?")[0] and pw2 and user2:
                        # this session's copy: its names and values win
                        form, pw, user, page_pq = f2, pw2, user2, path_query(f2.action)
                        user.sort(key=lambda i: 0 if i.type == "email" or USER_NAME.search(i.name or "") else 1)
                        data = {i.name: "" for i in form.inputs if i.type not in ("submit", "button", "image", "reset")}
                        data.update({h.name: h.value for h in f2.hidden()})
                        break
            except (ReachError, OSError, ValueError):
                pass
            data[user[0].name] = account.get("username", "")
            data[pw[0].name] = account.get("password", "")
            # A person takes a moment: phpBB refuses a form sent within the
            # second it was shown.
            time.sleep(LOGIN_PAUSE)
            # the button a person clicks is sent too (phpBB checks for it)
            button = next((i for i in form.inputs if i.type in ("submit", "image") and i.name), None)
            if button is not None and button.name not in data:
                data[button.name] = button.value
            before = dict(self.f.cookies)
            try:
                resp = self.f.request("POST", page_pq, data, login=True)
            except (ReachError, OSError, ValueError):
                self.r.login = "failed"
                return False
            if resp.status == 200 and resp.body[:1] in (b"{", "{"):
                # A login made in script answers where to go next (OpenCart)
                # or what went wrong: the answer decides, not the cookies.
                ok = self._json_redirect(resp)
            else:
                ok = _logged_in(resp, before, self.f.cookies)
                if not ok and self.f.cookies == before and resp.status == 200:
                    ok = self._form_gone(shown)  # the same session, now logged in
            self.r.login = "ok" if ok else "failed"
            self.log.info("test account login", outcome=self.r.login, form=page_pq.split("?")[0])
            return ok
        known = webapps.get(self.cms).login
        if known and not getattr(self, "_tried_known", False):
            self._tried_known = True
            try:
                resp = self.f.get(known)
                if resp.status == 200:
                    self.r.forms.extend(f for f in extract.parse(resp.body.decode("utf-8", "replace"), resp.url).forms
                                        if self.f.same_site(f.action))
                    return self._login(account)
            except (ReachError, OSError, ValueError):
                pass
        self.r.login = "no_login_form"
        return False

    def _json_redirect(self, resp) -> bool:
        """A script login's answer such as {"redirect": "…account…"}: it
        worked when it sends to a page of the site that is not the login,
        which the logged-in pass then opens first."""
        try:
            got = json.loads(resp.body)
        except ValueError:
            return False
        to = next((got[k] for k in ("redirect", "location", "url") if isinstance(got, dict) and
                   isinstance(got.get(k), str)), "")
        if not to or not self.f.same_site(urllib.parse.urljoin(resp.url, to)) or "login" in to.lower():
            return False
        self.after_login = [path_query(urllib.parse.urljoin(resp.url, to))]
        return True

    def _form_gone(self, shown: str) -> bool:
        """Some sites keep their session cookie when someone logs in: the
        page that showed the login form, opened again, tells. Logged in, it
        sends elsewhere or no longer asks for a password."""
        try:
            resp = self.f.get(shown)
        except (ReachError, OSError, ValueError):
            return False
        if resp.status in (301, 302, 303, 307, 308):
            return "login" not in (resp.location or "").lower()
        body = resp.body.decode("utf-8", "replace") if isinstance(resp.body, bytes) else str(resp.body)
        return resp.status == 200 and not re.search(r"""<input[^>]+type=["']?password""", body, re.I)


LOGIN_PAUSE = 1.2  # seconds between showing a login form and sending it
LOGIN_WORDS = re.compile(r"log_?in|sign_?in|logon|authentication|userlogin|ucp\.php\?mode=login", re.I)
USER_NAME = re.compile(r"user|login|e-?mail|^log$|^name$|account|identifier", re.I)


def _login_rank(form) -> tuple:
    where = (form.page or "") + " " + form.action
    pw = sum(1 for i in form.inputs if i.type == "password")
    return (0 if LOGIN_WORDS.search(where) else 1, 0 if pw == 1 else 1, len(form.visible()))



def _logged_in(resp, before: dict, after: dict) -> bool:
    """A login worked when the site set or changed a cookie (a new session)
    and did not show the login form again: most sites already set a session
    cookie on the first visit, so a cookie alone proves nothing."""
    if after == before or not after:
        return False
    if resp.status in (301, 302, 303, 307, 308):
        loc = (resp.location or "").lower()
        return "login" not in loc or "testsession" in loc  # Moodle: login/index.php?testsession=…
    if resp.status != 200:
        return False
    body = resp.body.decode("utf-8", "replace") if isinstance(resp.body, bytes) else str(resp.body)
    return not re.search(r"""<input[^>]+type=["']?password""", body, re.I)
