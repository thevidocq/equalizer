"""A real WordPress behind Equalizer, used with a real browser: what a
visitor and an editor do every day must work as on the legacy site."""
import re

import requests

from lab import CA, SITE

EDITOR = ("editor", "Lab-editor-pass-1")


def get(path, **kw):
    return requests.get(SITE + path, verify=CA, timeout=20, allow_redirects=False, **kw)


def test_home_and_links_point_to_equalizer(page):
    page.goto(SITE + "/")
    assert "Acme Blog" in page.title()
    assert "127.0.0.1:18090" not in page.content(), "legacy address leaked"
    # Every link, script, style, image and form points to Equalizer.
    urls = page.evaluate("""() => [...document.querySelectorAll('a[href],link[href],script[src],img[src],form[action]')]
        .map(e => e.getAttribute('href') || e.getAttribute('src') || e.getAttribute('action'))""")
    legacy = [u for u in urls if u.startswith("http://")]
    assert urls and not legacy, legacy


def test_post_with_accents(page):
    page.goto(SITE + "/news-from-the-lab/")
    assert "Città, perché, über, ñandú" in page.content()


def test_page_and_search(page):
    page.goto(SITE + "/about/")
    assert "Acme makes things" in page.content()
    page.goto(SITE + "/?s=lab")
    assert "News from the lab" in page.content()


def test_comment(page):
    page.goto(SITE + "/news-from-the-lab/")
    page.fill("#comment", "Great post, è tutto ok.")
    page.fill("#author", "Visitor One")
    page.fill("#email", "visitor@example.org")
    page.click("#submit")
    page.wait_for_load_state()
    assert "Great post, è tutto ok." in page.content()


def test_other_domains_redirect():
    r = get("/about/?x=1", headers={})
    assert r.status_code == 200
    r = requests.get("https://acme.test/about/?x=1", verify=CA, timeout=20, allow_redirects=False)
    assert r.status_code == 301 and r.headers["Location"] == "https://www.acme.test/about/?x=1", (r.status_code, r.headers)


def wp_login(page, remember=False):
    page.goto(SITE + "/wp-login.php")
    page.fill("#user_login", EDITOR[0])
    page.fill("#user_pass", EDITOR[1])
    if remember:
        page.check("#rememberme")
    page.click("#wp-submit")
    page.wait_for_url(re.compile(r"/wp-admin/"))


def test_login_and_dashboard(page):
    wp_login(page)
    assert "Dashboard" in page.content()
    names = {c["name"] for c in page.context.cookies()}
    assert not any(n.startswith("wordpress_logged_in") for n in names), f"legacy login cookie in the browser: {names}"


def test_remember_me_keeps_the_session(page):
    wp_login(page, remember=True)
    session = [c for c in page.context.cookies() if c["name"].startswith("__Host-eq")]
    assert session and session[0]["expires"] > 0, f"session cookie not persistent: {session}"


def test_rest_api_with_nonce(page):
    wp_login(page)
    page.goto(SITE + "/wp-admin/post-new.php")
    nonce = page.evaluate("() => (window.wpApiSettings || {}).nonce")
    assert nonce, "no REST nonce on the editor page"
    res = page.evaluate("""async (nonce) => {
        const r = await fetch('/wp-json/wp/v2/posts', {method: 'POST', credentials: 'same-origin',
          headers: {'Content-Type': 'application/json', 'X-WP-Nonce': nonce},
          body: JSON.stringify({title: 'Written through Equalizer', content: 'Hello from the editor', status: 'publish'})});
        return [r.status, await r.text()];
    }""", nonce)
    assert res[0] == 201, res
    page.goto(SITE + "/written-through-equalizer/")
    assert "Hello from the editor" in page.content()


PNG = bytes.fromhex("89504e470d0a1a0a0000000d4948445200000001000000010806000000"
                    "1f15c4890000000d49444154789c6360f8cf00000301010018dd8db00000000049454e44ae426082")


def test_media_upload_raw_body(page):
    wp_login(page)
    page.goto(SITE + "/wp-admin/post-new.php")
    nonce = page.evaluate("() => (window.wpApiSettings || {}).nonce")
    res = page.evaluate("""async ([nonce, bytes]) => {
        const r = await fetch('/wp-json/wp/v2/media', {method: 'POST', credentials: 'same-origin',
          headers: {'X-WP-Nonce': nonce, 'Content-Type': 'image/png', 'Content-Disposition': 'attachment; filename="pixel.png"'},
          body: new Uint8Array(bytes)});
        return [r.status, await r.text()];
    }""", [nonce, list(PNG)])
    assert res[0] == 201, res
    url = re.search(r'"source_url":"([^"]+)"', res[1]).group(1).replace("\\/", "/")
    assert url.startswith("https://www.acme.test/"), url
    assert get(url.replace("https://www.acme.test", "")).content[:8] == PNG[:8]


def test_admin_has_no_mixed_content(page):
    wp_login(page)
    errors = []
    page.on("console", lambda m: errors.append(m.text) if "Mixed Content" in m.text else None)
    page.goto(SITE + "/wp-admin/post-new.php")
    page.wait_for_load_state("networkidle")
    assert not errors, errors[:3]
    assert "http://www.acme.test" not in page.content()


def test_media_upload_form(page):
    wp_login(page)
    page.goto(SITE + "/wp-admin/media-new.php?browser-uploader")
    page.set_input_files("#async-upload", files=[{"name": "photo.png", "mimeType": "image/png", "buffer": PNG}])
    page.click("#html-upload")
    page.wait_for_url(re.compile(r"upload\.php"))
    assert "photo" in page.content()
