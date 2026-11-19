"""From the first platform administrator to a published WordPress site, as
on a new installation: real emails, authenticator codes, a real connector."""
import time

import requests

from lab import CA, SITE, Connector, Panel, mail_link, totp
import state


def test_platform_admin_first_login():
    root = Panel()
    root.login("root@lab.test", "Lab-root-password-1")
    me = root.call("GET", "/auth/me", None, 200)
    assert me["platform_role"] == "admin"
    state.root_secret = root.enrol()
    state.root = root


def test_tenant_and_invitation_email():
    since = time.time()
    state.root.call("POST", "/tenants", {"id": "acme", "name": "Acme Ltd", "admin_email": "boss@acme.test", "admin_name": "Boss Acme"}, 201)
    link = mail_link("boss@acme.test", "/invite/", since)
    token = link.rsplit("/", 1)[1]
    boss = Panel()
    boss.call("GET", f"/invitations/{token}", None, 200)
    boss.call("POST", f"/invitations/{token}/accept", {"password": "Boss-password-123"}, 201)
    state.boss_secret = boss.enrol()
    state.boss = boss
    # The platform sees the tenant, not its configuration.
    assert "acme" in str(state.root.call("GET", "/platform/tenants", None, 200))
    state.root.call("GET", "/tenants/acme/connectors", None, 403)


def test_second_person_to_approve():
    since = time.time()
    state.boss.call("POST", "/tenants/acme/invitations", {"email": "ann@acme.test", "name": "Ann Approver", "role": "approver"}, 201)
    token = mail_link("ann@acme.test", "/invite/", since).rsplit("/", 1)[1]
    ann = Panel()
    ann.call("POST", f"/invitations/{token}/accept", {"password": "Ann-password-1234"}, 201)
    ann.enrol()
    state.ann = ann


def test_connector_online():
    created = state.boss.call("POST", "/tenants/acme/connectors", {"id": "cn-web"}, 201)
    state.connector = Connector("cn-web", created["token"])
    state.connector.start()
    for _ in range(60):
        items = state.boss.call("GET", "/tenants/acme/connectors", None, 200)["items"]
        if items and items[0].get("online"):
            break
        time.sleep(0.5)
    assert items[0]["online"] and items[0]["versions"] == ["0.1.0"] and not items[0]["outdated"], items


def test_site_from_template_published():
    b = state.boss
    b.call("POST", "/tenants/acme/apps", {"id": "blog", "name": "Acme Blog", "public_domain": "www.acme.test", "kind": "wordpress"}, 201)
    b.call("PUT", "/apps/blog/aliases", {"aliases": ["acme.test", "old.acme.test"]}, 200)
    doc = b.call("POST", "/apps/blog/map-template", {"template": "wordpress", "connector_id": "cn-web",
                                                      "address": "127.0.0.1:18090", "scheme": "http", "max_upload_mb": 20}, 200)["document"]
    v = b.call("POST", "/apps/blog/maps", {"document": doc, "note": "From the WordPress template"}, 201)["version"]
    b.call("POST", f"/apps/blog/maps/{v}/submit", None, 200)
    b.call("POST", f"/apps/blog/maps/{v}/approve", None, 403)  # never one's own version
    state.ann.call("POST", f"/apps/blog/maps/{v}/approve", None, 200)
    state.ann.call("POST", f"/apps/blog/maps/{v}/publish", None, 200)
    for _ in range(60):
        try:
            r = requests.get(SITE + "/", verify=CA, timeout=10)
            if r.status_code == 200 and "Acme Blog" in r.text:
                break
        except requests.RequestException:
            pass
        time.sleep(0.5)
    assert r.status_code == 200 and "Acme Blog" in r.text, r.status_code
