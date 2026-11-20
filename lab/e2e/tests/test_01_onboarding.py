"""From the first platform administrator to a published WordPress site, as
on a new installation: real emails, authenticator codes, a real connector."""
import time

import requests

import pytest

from lab import CA, HTTP_SITES, LAB, SITE, SITES, Connector, Panel, mail_link, site_certificate, totp
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
    assert items[0]["online"] and items[0]["versions"] == [(LAB / ".." / ".." / "VERSION").read_text().strip()] and not items[0]["outdated"], items


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
    # Published without a certificate: the panel says so, visitors get no
    # HTTPS (no certificate is ever made up for a protected site).
    assert b.call("GET", "/apps/blog", None, 200)["tls"]["status"] == "missing"
    time.sleep(1)
    assert handshake("www.acme.test") is None
    # The legacy site's own certificate, uploaded by the administrator.
    upload(b, "blog", "www.acme.test", "acme.test", "old.acme.test")
    for _ in range(60):
        try:
            r = requests.get(SITE + "/", verify=CA, timeout=10)
            if r.status_code == 200 and "Acme Blog" in r.text:
                break
        except requests.RequestException:
            pass
        time.sleep(0.5)
    assert r.status_code == 200 and "Acme Blog" in r.text, r.status_code


def publish(app, name, domain, kind, address, scheme="https"):
    b = state.boss
    b.call("POST", "/tenants/acme/apps", {"id": app, "name": name, "public_domain": domain, "kind": kind}, 201)
    if scheme == "http":
        b.call("PATCH", f"/apps/{app}", {"public_scheme": "http"}, 200)
    else:
        upload(b, app, domain)
    doc = b.call("POST", f"/apps/{app}/map-template", {"template": kind, "connector_id": "cn-web",
                                                       "address": address, "scheme": "http", "max_upload_mb": 20}, 200)["document"]
    v = b.call("POST", f"/apps/{app}/maps", {"document": doc, "note": f"From the {kind} template"}, 201)["version"]
    b.call("POST", f"/apps/{app}/maps/{v}/submit", None, 200)
    state.ann.call("POST", f"/apps/{app}/maps/{v}/approve", None, 200)
    state.ann.call("POST", f"/apps/{app}/maps/{v}/publish", None, 200)


@pytest.mark.parametrize("kind", list(SITES))
def test_other_sites_published(kind):
    domain, app, address, marker = SITES[kind]
    scheme = "http" if domain in HTTP_SITES else "https"
    publish(app, marker, domain, kind, address, scheme)
    r = None
    for _ in range(60):
        try:
            r = requests.get(f"{scheme}://{domain}/", verify=CA, timeout=10)
            if r.status_code == 200 and marker in r.content.decode(r.encoding or "utf-8", "replace"):
                break
        except requests.RequestException:
            pass
        time.sleep(0.5)
    assert r is not None and r.status_code == 200 and marker in r.text, (kind, r and r.status_code)


def upload(panel, app, *names):
    tls = panel.call("PUT", f"/apps/{app}/certificate", {"pem": site_certificate(*names)}, 200)["tls"]
    assert tls["status"] == "ok" and tls["certificate"]["domains"] == list(names), tls
    return tls


def handshake(name):
    """The certificate the site shows to visitors, or None without one."""
    import socket
    import ssl
    ctx = ssl.create_default_context(cafile=CA)
    try:
        with socket.create_connection(("127.0.0.1", 443), timeout=10) as raw, ctx.wrap_socket(raw, server_hostname=name) as s:
            return s.getpeercert(binary_form=True)
    except (ssl.SSLError, OSError):
        return None


def test_visitors_see_the_sites_own_certificate():
    import hashlib
    b = state.boss
    want = b.call("GET", "/apps/blog", None, 200)["tls"]["certificate"]["fingerprint_sha256"]
    for name in ("www.acme.test", "acme.test", "old.acme.test"):
        der = None
        for _ in range(30):  # the gateways read new certificates within seconds
            der = handshake(name)
            if der:
                break
            time.sleep(1)
        assert der and hashlib.sha256(der).hexdigest() == want, name
    # http: an https site is sent to https; the http site is served in clear
    # and has no certificate at all.
    r = requests.get("http://www.acme.test/about/?x=1", timeout=10, allow_redirects=False)
    assert r.status_code == 301 and r.headers["Location"] == "https://www.acme.test/about/?x=1"
    assert handshake("intranet.acme.test") is None
    r = requests.get("http://intranet.acme.test/", timeout=10)
    assert r.status_code == 200 and "Acme Intranet" in r.text and r.url.startswith("http://")
    # Another site's certificate is refused at upload.
    assert b.s.put("https://panel.lab.test/v1/apps/news/certificate", json={"pem": site_certificate("www.acme.test")},
                   timeout=30).status_code == 422
