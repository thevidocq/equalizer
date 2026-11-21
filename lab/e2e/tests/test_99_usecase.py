"""WordPress use case, in pictures: the agent's draft published, attacks
against the protected site, and the panel's pages as the customer sees
them. Run after tests 01 and 06[blog] with Loki and Vector for the Logs page."""
import os
import time

import requests

import state
from lab import CA, PANEL, SITE
from test_03_attacks import raw_get

OUT = os.environ.get("EQ_USECASE_DIR", "/var/tmp/usecase")


def req(method, path, **kw):
    return requests.request(method, SITE + path, verify=CA, timeout=20, allow_redirects=False, **kw)


def test_publish_the_agents_draft():
    b, ann = state.boss, state.ann
    versions = b.call("GET", "/apps/blog/maps", None, 200)["items"]
    draft = max(v["version"] for v in versions if v["status"] == "draft")
    b.call("POST", f"/apps/blog/maps/{draft}/submit", None, 200)
    ann.call("POST", f"/apps/blog/maps/{draft}/approve", None, 200)
    ann.call("POST", f"/apps/blog/maps/{draft}/publish", None, 200)
    time.sleep(20)  # the gateways read new maps every 15 seconds
    assert req("GET", "/").status_code == 200
    b.call("PATCH", "/apps/blog", {"legacy_ip": "203.0.113.10"}, None)


def test_attacks():
    out = {}
    out["webshell"] = req("POST", "/wp-json/wp/v2/media", data=b"<?php system($_GET['c']); ?>",
                          headers={"Content-Type": "image/png", "Content-Disposition": 'attachment; filename="x.php.png"'}).status_code
    out["file_manager"] = req("GET", "/wp-content/plugins/wp-file-manager/lib/php/connector.minimal.php").status_code
    out["traversal"] = raw_get("/wp-content/../wp-config.php")[0]
    out["sqli"] = req("GET", "/?p=1%20UNION%20SELECT%20user_pass%20FROM%20wp_users").status_code
    out["xxe"] = req("POST", "/xmlrpc.php", data='<?xml version="1.0"?><!DOCTYPE r [<!ENTITY x SYSTEM "file:///etc/passwd">]>'
                     '<methodCall><methodName>&x;</methodName></methodCall>', headers={"Content-Type": "text/xml"}).status_code
    out["forged_comment"] = req("POST", "/wp-comments-post.php", data={"comment": "<script>alert(1)</script>", "comment_post_ID": "1"}).status_code
    out["brute"] = [req("POST", "/wp-login.php", data={"log": "admin", "pwd": f"guess{i}"}).status_code for i in range(40)]
    print("attacks:", out)
    assert out["file_manager"] == 404 and out["webshell"] in (403, 404, 415) and out["traversal"] in (400, 404)
    assert req("GET", "/about/").status_code == 200  # the site still works for everyone else
    time.sleep(15)  # Vector ships the events to Loki


def test_screens():
    from playwright.sync_api import sync_playwright
    os.makedirs(OUT, exist_ok=True)
    cookies = [{"name": c.name, "value": c.value, "domain": "panel.lab.test", "path": "/", "secure": True, "httpOnly": True}
               for c in state.boss.s.cookies]
    pages = [("01-sites", "/t/acme"), ("02-connectors", "/t/acme/connectors"), ("03-maps", "/t/acme/sites/blog/maps"),
             ("04-discovery", "/t/acme/sites/blog/discovery"), ("06-isolation", "/t/acme/sites/blog/isolation"),
             ("07-logs", "/t/acme/logs"), ("08-status", "/t/acme/status"), ("09-activity", "/t/acme/activity")]
    versions = state.boss.call("GET", "/apps/blog/maps", None, 200)["items"]
    agent_v = max(v["version"] for v in versions)
    pages.insert(4, ("05-version", f"/t/acme/sites/blog/maps/{agent_v}"))
    exe = "/usr/bin/google-chrome"
    with sync_playwright() as p:
        b = p.chromium.launch(executable_path=exe, args=["--no-sandbox"])
        ctx = b.new_context(ignore_https_errors=True, viewport={"width": 1440, "height": 900}, device_scale_factor=2)
        ctx.add_cookies(cookies)
        page = ctx.new_page()
        for name, path in pages:
            page.goto(PANEL + path)
            page.wait_for_load_state("networkidle")
            time.sleep(1.5)
            page.screenshot(path=f"{OUT}/{name}.png", full_page=True)
        site = b.new_context(ignore_https_errors=True, viewport={"width": 1440, "height": 900}, device_scale_factor=2)
        sp = site.new_page()
        sp.goto(SITE + "/")
        sp.screenshot(path=f"{OUT}/10-site.png")
        sp.goto(SITE + "/wp-content/plugins/wp-file-manager/lib/php/connector.minimal.php")
        sp.screenshot(path=f"{OUT}/11-blocked.png")
        b.close()
