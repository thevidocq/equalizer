"""Compares the same pages seen directly on the legacy sites and through
Equalizer: screenshots pixel by pixel, visible text, failed resources and
console errors. Writes side-by-side images to .run/fidelity/."""
import difflib
import io
import os
import re
import sys

from PIL import Image, ImageChops
from playwright.sync_api import sync_playwright

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "tests"))
from lab import RUN  # noqa: E402  (also sets NO_PROXY)

OUT = os.path.join(RUN, "fidelity")
CHROME = "/usr/bin/google-chrome"
NO_MOTION = ("document.addEventListener('DOMContentLoaded',()=>{const s=document.createElement('style');"
             "s.textContent='*{animation:none!important;transition:none!important;caret-color:transparent!important}';"
             "document.head.appendChild(s)})")


def context(browser):
    ctx = browser.new_context(viewport={"width": 1280, "height": 900}, ignore_https_errors=True,
                              locale="en-US", timezone_id="Europe/Rome")
    ctx.add_init_script(NO_MOTION)
    return ctx


def fill_login(page, url, user, password, submit, done):
    page.goto(url)
    page.fill(user, "editor")
    page.fill(password, "Lab-editor-pass-1")
    page.locator(submit).first.click()
    try:
        page.wait_for_url(done)
    except Exception as e:  # say what the site answered, not only that it timed out
        raise AssertionError(f"login at {url} stuck on {page.url}: {page.inner_text('body')[:300]!r}") from e


# Each site: legacy port, public pages, how to log in, pages after login.
SITES = {
    "www.acme.test": (18090, ["/", "/about/", "/news-from-the-lab/", "/?s=lab", "/wp-login.php"],
                      lambda p, base: fill_login(p, base + "/wp-login.php", "#user_login", "#user_pass", "#wp-submit", "**/wp-admin/**"),
                      ["/wp-admin/", "/wp-admin/edit.php", "/wp-admin/upload.php", "/wp-admin/edit-comments.php",
                       "/wp-admin/options-general.php", "/wp-admin/profile.php", "/wp-admin/post-new.php"]),
    "news.acme.test": (18091, ["/", "/?option=com_finder&view=search&q=wombat", "/administrator/"],
                       lambda p, base: fill_login(p, base + "/administrator/", "#mod-login-username", "#mod-login-password",
                                                  "#btn-login-submit", "**/administrator/index.php**"),
                       ["/administrator/index.php", "/administrator/index.php?option=com_content&view=articles",
                        "/administrator/index.php?option=com_users&view=users",
                        "/administrator/index.php?option=com_config"]),
    "docs.acme.test": (18092, ["/", "/about", "/release-notes", "/search/node?keys=quokka", "/user/login"],
                       lambda p, base: fill_login(p, base + "/user/login", "#edit-name", "#edit-pass",
                                                  "#user-login-form [type=submit]", "**/user/1**"),
                       ["/admin/content", "/node/add/article", "/admin/people", "/admin/config", "/admin/structure/types"]),
    "intranet.acme.test": (18093, ["/", "/notice.php?id=1", "/search.php?q=canteen", "/login.php"],
                           lambda p, base: fill_login(p, base + "/login.php", "#user", "#password", "#login", base + "/"),
                           ["/admin/new.php"]),
}


def visit(page, url):
    problems = []
    # Failed loads are compared by their status below: Chrome does not log
    # every one of them (favicons) the same way.
    page.on("console", lambda m: problems.append("console: " + m.text[:160])
            if m.type == "error" and not m.text.startswith("Failed to load resource") else None)
    page.on("requestfailed", lambda r: problems.append("failed: " + r.url[:160]))
    page.on("response", lambda r: problems.append(f"{r.status}: {r.url[:160]}") if r.status >= 400 else None)
    page.goto(url)
    try:
        page.wait_for_load_state("networkidle", timeout=10000)
    except Exception:  # the block editor keeps polling
        page.wait_for_timeout(4000)
    page.wait_for_timeout(800)
    text = page.evaluate("() => document.body.innerText")
    shot = Image.open(io.BytesIO(page.screenshot(full_page=True))).convert("RGB")
    return text, shot, problems


def compare(host, path, a, b):
    ta, sa, pa = a
    tb, sb, pb = b
    shown = ta != ta.replace("http://" + host, "https://" + host)  # the page shows the site's address
    ta = ta.replace("http://" + host, "https://" + host)  # as visitors see it
    w, h = max(sa.width, sb.width), max(sa.height, sb.height)
    ca, cb = Image.new("RGB", (w, h), "white"), Image.new("RGB", (w, h), "white")
    ca.paste(sa)
    cb.paste(sb)
    diff = ImageChops.difference(ca, cb).convert("L").point(lambda v: 255 if v > 16 else 0)
    pct = 100 * sum(diff.histogram()[255:]) / (w * h)
    textdiff = [line for line in difflib.unified_diff(ta.splitlines(), tb.splitlines(), lineterm="", n=0)
                if line[:1] in "+-" and line[:3] not in ("+++", "---")]
    side = Image.new("RGB", (w * 2 + 20, h), "red")
    side.paste(ca, (0, 0))
    side.paste(cb, (w + 20, 0))
    name = host.split(".")[0] + "_" + (re.sub(r"[^A-Za-z0-9._-]+", "_", path.strip("/"))[:80] or "home")
    side.save(os.path.join(OUT, name + ".png"))
    if pct > 0:
        diff.save(os.path.join(OUT, name + ".diff.png"))
    print(f"{host + path:60} size {sa.size}→{sb.size}  pixels differ {pct:.2f}%  text lines differ {len(textdiff)}")
    for line in textdiff[:6]:
        print("    text", line[:150])
    only_gw = sorted(set(pb) - set(p.replace("http://" + host, "https://" + host) for p in pa))
    for p in only_gw[:6]:
        print("    gateway only", p)
    return pct, textdiff, only_gw, host + path, shown


def main(pw=None, sites=None):
    if pw is None:
        with sync_playwright() as p:
            return main(p, sites)
    os.makedirs(OUT, exist_ok=True)
    rules = ",".join(f"MAP {h}:80 127.0.0.1:{s[0]}" for h, s in SITES.items())
    results = []
    # Direct: the browser reaches the legacy sites themselves, as before Equalizer.
    direct = pw.chromium.launch(executable_path=CHROME, args=["--no-sandbox", "--host-resolver-rules=" + rules])
    gw = pw.chromium.launch(executable_path=CHROME, args=["--no-sandbox"])
    for host in sites or SITES:
        _, public, login, admin = SITES[host]
        d, g = "http://" + host, "https://" + host
        dctx, gctx = context(direct), context(gw)
        dp, gp = dctx.new_page(), gctx.new_page()
        for path in public:
            results.append(compare(host, path, visit(dp, d + path), visit(gp, g + path)))
        login(dp, d)
        login(gp, g)
        for path in admin:
            dp, gp = dctx.new_page(), gctx.new_page()
            results.append(compare(host, path, visit(dp, d + path), visit(gp, g + path)))
        dctx.close()
        gctx.close()
    direct.close()
    gw.close()
    worst = max(r[0] for r in results)
    print(f"\nworst pixel difference {worst:.2f}%; images in {OUT}")
    return results


if __name__ == "__main__":
    main(sites=sys.argv[1:] or None)
