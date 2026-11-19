"""What an attacker tries first must not reach WordPress as written, and
must not break the site for everyone else."""
import http.client
import ssl
from urllib.parse import urlsplit

import requests

from lab import CA, SITE


def req(method, path, **kw):
    return requests.request(method, SITE + path, verify=CA, timeout=20, allow_redirects=False, **kw)


def raw_get(path):
    """Sends the path exactly as written: requests would clean "../" first."""
    conn = http.client.HTTPSConnection(urlsplit(SITE).hostname, context=ssl.create_default_context(cafile=CA), timeout=20)
    try:
        conn.request("GET", path)
        res = conn.getresponse()
        return res.status, res.read()
    finally:
        conn.close()


def test_path_traversal_refused():
    for path in ["/wp-content/../wp-config.php", "/%2e%2e/%2e%2e/etc/passwd", "/wp-includes/..%2f..%2fwp-config.php"]:
        status, body = raw_get(path)
        assert status in (400, 404), (path, status)
        assert b"DB_NAME" not in body and b"root:" not in body, path


def test_reflected_script_escaped():
    r = req("GET", "/?s=%3Cscript%3Ealert(1)%3C/script%3E")
    assert r.status_code in (200, 400) and "<script>alert(1)</script>" not in r.text


def test_xml_entities_refused():
    body = '<?xml version="1.0"?><!DOCTYPE r [<!ENTITY x SYSTEM "file:///etc/passwd">]><methodCall><methodName>&x;</methodName></methodCall>'
    assert req("POST", "/xmlrpc.php", data=body, headers={"Content-Type": "text/xml"}).status_code == 400


def test_php_disguised_as_picture_refused():
    r = req("POST", "/wp-json/wp/v2/media", data=b"<?php system($_GET['c']); ?>",
            headers={"Content-Type": "image/png", "Content-Disposition": 'attachment; filename="x.php.png"'})
    assert r.status_code == 415


def test_unknown_binary_body_refused():
    assert req("POST", "/wp-admin/admin-ajax.php", data=b"\x00\x01\x02", headers={"Content-Type": "application/x-foo"}).status_code == 415


def test_forged_proxy_headers_ignored():
    # Method override and a forged identity never reach WordPress: the
    # request stays a GET from an anonymous visitor.
    r = req("GET", "/wp-admin/", headers={"X-HTTP-Method-Override": "DELETE", "X-Remote-User": "admin",
                                          "X-Original-URL": "/wp-admin/options.php"})
    assert r.status_code == 302 and "wp-login.php" in r.headers.get("Location", "")


def test_internal_headers_hidden():
    r = req("GET", "/")
    assert "X-Powered-By" not in r.headers and "PHP" not in r.headers.get("Server", "")


def test_site_still_works_after_attacks():
    r = req("GET", "/about/")
    assert r.status_code == 200 and "Acme makes things" in r.text
