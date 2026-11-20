"""The organisation's identity provider in front of the admin only: the
public site stays open, /wp-admin asks for the provider first."""
import json
import re
import time

import requests

from lab import CA, SITE
import state


def test_publish_sso_on_admin_paths():
    b = state.boss
    pub = b.call("GET", "/apps/blog/maps", None, 200)["items"]
    current = next(v for v in pub if v["status"] == "published")["version"]
    doc = b.call("GET", f"/apps/blog/maps/{current}", None, 200)["document"]
    doc.pop("meta", None)
    role = next(iter(doc["roles"]["list"]))
    doc["roles"]["auth"] = {"provider": "oidc", "issuer": "https://idp.lab.test", "client_id": "acme-blog",
                            "roles": [{"group": "Blog editors", "role": role}],
                            "paths": ["/wp-admin/", "/wp-login.php", "/xmlrpc.php"]}
    v = b.call("POST", "/apps/blog/maps", {"document": doc, "note": "SSO on the admin"}, 201)["version"]
    b.call("POST", f"/apps/blog/maps/{v}/publish", None, 200)
    for _ in range(60):
        r = requests.get(SITE + "/wp-login.php", verify=CA, timeout=10, allow_redirects=False)
        if r.status_code == 302 and "idp.lab.test" in r.headers.get("Location", ""):
            break
        time.sleep(0.5)
    assert "idp.lab.test" in r.headers.get("Location", ""), (r.status_code, r.headers)


def test_public_site_needs_no_login(page):
    page.goto(SITE + "/about/")
    assert "Acme makes things" in page.content()


def test_admin_through_the_provider(page):
    page.goto(SITE + "/wp-admin/")
    assert "idp.lab.test" in page.url
    page.click("#as-alice")
    page.wait_for_url(re.compile(r"www\.acme\.test/wp-"))
    # Back on the site: WordPress's own login comes next, as agreed.
    assert "user_login" in page.content() or "Dashboard" in page.content()


def test_user_outside_the_groups_refused(page):
    page.goto(SITE + "/wp-login.php")
    page.click("#as-bob")
    page.wait_for_load_state()
    assert "www.acme.test" in page.url and "user_login" not in page.content()


def test_tricks_still_ask_for_the_provider():
    for path in ["/WP-ADMIN/", "//wp-admin/", "/x/../wp-admin/", "/wp-login.php."]:
        r = requests.get(SITE + path, verify=CA, timeout=10, allow_redirects=False)
        assert r.status_code in (302, 400) and "user_login" not in r.text, (path, r.status_code)
        if r.status_code == 302:
            assert "idp.lab.test" in r.headers["Location"], path


def test_application_password_needs_the_provider():
    r = requests.get(SITE + "/wp-json/wp/v2/users/me", verify=CA, timeout=10, auth=("editor", "Lab-editor-pass-1"))
    assert r.status_code == 401, (r.status_code, r.text[:200])
