"""Compares the same pages seen directly on the legacy WordPress and through
Equalizer: screenshots pixel by pixel, visible text, failed resources and
console errors. Writes side-by-side images to .run/fidelity/."""
import difflib
import io
import os
import sys

from PIL import Image, ImageChops
from playwright.sync_api import sync_playwright

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "tests"))
from lab import RUN  # noqa: E402  (also sets NO_PROXY)

PUBLIC = ["/", "/about/", "/news-from-the-lab/", "/?s=lab", "/wp-login.php"]
ADMIN = ["/wp-admin/", "/wp-admin/edit.php", "/wp-admin/upload.php", "/wp-admin/edit-comments.php",
         "/wp-admin/options-general.php", "/wp-admin/profile.php", "/wp-admin/post-new.php"]
OUT = os.path.join(RUN, "fidelity")
DIRECT, GATEWAY = "http://www.acme.test", "https://www.acme.test"
CHROME = "/usr/bin/google-chrome"
NO_MOTION = ("document.addEventListener('DOMContentLoaded',()=>{const s=document.createElement('style');"
             "s.textContent='*{animation:none!important;transition:none!important;caret-color:transparent!important}';"
             "document.head.appendChild(s)})")


def context(browser):
    ctx = browser.new_context(viewport={"width": 1280, "height": 900}, ignore_https_errors=True,
                              locale="en-US", timezone_id="Europe/Rome")
    ctx.add_init_script(NO_MOTION)
    return ctx


def login(page, base):
    page.goto(base + "/wp-login.php")
    page.fill("#user_login", "editor")
    page.fill("#user_pass", "Lab-editor-pass-1")
    page.click("#wp-submit")
    page.wait_for_url("**/wp-admin/**")


def visit(page, url):
    problems = []
    page.on("console", lambda m: problems.append("console: " + m.text[:160]) if m.type == "error" else None)
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


def compare(path, a, b):
    ta, sa, pa = a
    tb, sb, pb = b
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
    name = path.strip("/").replace("/", "_").replace("?", "_").replace("=", "") or "home"
    side.save(os.path.join(OUT, name + ".png"))
    if pct > 0:
        diff.save(os.path.join(OUT, name + ".diff.png"))
    print(f"{path:32} size {sa.size}→{sb.size}  pixels differ {pct:.2f}%  text lines differ {len(textdiff)}")
    for line in textdiff[:6]:
        print("    text", line[:150])
    only_gw = sorted(set(pb) - set(p.replace(DIRECT, GATEWAY) for p in pa))
    for p in only_gw[:6]:
        print("    gateway only", p)
    return pct, textdiff, only_gw, path


def main(pw=None):
    if pw is None:
        with sync_playwright() as p:
            return main(p)
    os.makedirs(OUT, exist_ok=True)
    results = []
    # Direct: the browser reaches WordPress itself, as before Equalizer.
    direct = pw.chromium.launch(executable_path=CHROME,
                                args=["--no-sandbox", "--host-resolver-rules=MAP www.acme.test:80 127.0.0.1:18090"])
    gw = pw.chromium.launch(executable_path=CHROME, args=["--no-sandbox"])
    dctx, gctx = context(direct), context(gw)
    dp, gp = dctx.new_page(), gctx.new_page()
    for path in PUBLIC:
        results.append(compare(path, visit(dp, DIRECT + path), visit(gp, GATEWAY + path)))
    login(dp, DIRECT)
    login(gp, GATEWAY)
    for path in ADMIN:
        dp, gp = dctx.new_page(), gctx.new_page()
        results.append(compare(path, visit(dp, DIRECT + path), visit(gp, GATEWAY + path)))
    direct.close()
    gw.close()
    worst = max(r[0] for r in results)
    print(f"\nworst pixel difference {worst:.2f}%; images in {OUT}")
    return results


if __name__ == "__main__":
    main()
