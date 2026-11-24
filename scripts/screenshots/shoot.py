"""Takes the panel's screenshots used by the manuals (docs/screenshots),
from a panel filled by seed.py, at the size the manuals use.

    python3 scripts/screenshots/shoot.py <panel URL> <accounts.json> <out dir>"""

import json
import os
import sys

from playwright.sync_api import sync_playwright

SIZE = {"width": 1360, "height": 900}

# name, who, path, what to wait for, what to do before the shot
ADMIN = [
    ("sites", "/t/acme", "Customer portal", None),
    ("discovery", "/t/acme/sites/blog/discovery", "From the source code", None),
    ("maps", "/t/acme/sites/portal/maps", "Version", None),
    ("new-version", "/t/acme/sites/portal/maps/new", "Save", None),
    ("verification", "/t/acme/sites/portal/maps/2", "Corrections", None),
    ("review", "/t/acme/sites/blog/maps/2", "Review aid", None),
    ("isolation", "/t/acme/sites/blog/isolation", "Isolation", None),
    ("certificate", "/t/acme/sites/blog/certificate", "Certificate", None),
    ("login", "/t/acme/sites/blog/login", "Login", None),
    ("source", "/t/acme/sites/blog/source", "Last read", None),
    ("connectors", "/t/acme/connectors", "cn-web01", None),
    ("people", "/t/acme/people", "Jamie Lee", None),
    ("activity", "/t/acme/activity", "Alex Morgan", None),
    ("logs", "/t/acme/logs", "blog", None),
    ("status", "/t/acme/status", "Company blog", None),
    ("settings-ai", "/t/acme/settings", "AI model", None),
    ("rename", "/t/acme/sites/portal/discovery", "Customer portal", "rename"),
]
ROOT = [
    ("platform-tenants", "/tenants", "Acme", None),
    ("platform-status", "/status", "Status", None),
]


def login(page, base, email, password):
    page.goto(base + "/login")
    page.get_by_label("Email").fill(email)
    page.get_by_label("Password").fill(password)
    page.get_by_role("button", name="Log in").click()
    page.wait_for_url(lambda u: "/login" not in u, timeout=15000)
    print("logged in as", email, "at", page.url, "cookies:", [c["name"] for c in page.context.cookies()])


failed = []


def shoot(page, base, out, shots):
    for name, path, wait, action in shots:
        page.goto(base + path)
        try:
            page.get_by_text(wait).first.wait_for(timeout=15000)
        except Exception:
            failed.append(name)
            print("error:", name, "did not show", repr(wait), "at", page.url, "|", page.inner_text("body")[:300].replace("\n", " / "))
        if action == "rename":
            try:
                page.get_by_role("button", name="Rename the site").click(timeout=10000)
                page.get_by_role("dialog").wait_for(timeout=10000)
            except Exception as e:
                print("warning: rename:", e)
        page.wait_for_timeout(600)  # fonts and the last request
        page.screenshot(path=os.path.join(out, name + ".png"), full_page=True)
        print("took", name)


def main(base, accounts, out):
    with open(accounts) as f:
        acc = json.load(f)
    os.makedirs(out, exist_ok=True)
    with sync_playwright() as p:
        browser = p.chromium.launch()
        ctx = browser.new_context(viewport=SIZE, device_scale_factor=1, timezone_id="Europe/Rome", locale="en-GB")
        page = ctx.new_page()
        page.goto(base + "/login")
        page.get_by_label("Email").wait_for()
        page.wait_for_timeout(400)
        page.screenshot(path=os.path.join(out, "panel-login.png"))
        login(page, base, acc["admin"], acc["password"])
        shoot(page, base, out, ADMIN)
        ctx.close()
        ctx = browser.new_context(viewport=SIZE, device_scale_factor=1, timezone_id="Europe/Rome", locale="en-GB")
        page = ctx.new_page()
        login(page, base, acc["root"], acc["password"])
        shoot(page, base, out, ROOT)
        browser.close()
    if failed:
        sys.exit("pages that did not load: " + ", ".join(failed))


if __name__ == "__main__":
    main(*sys.argv[1:4])
