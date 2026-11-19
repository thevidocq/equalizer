"""Joomla, Drupal and a custom old-style PHP application used through
Equalizer as their visitors and editors do: read a page, search, log in,
publish an article, attach a file."""
import re
import time

import requests

from lab import CA

NEWS, DOCS, INTRANET = "https://news.acme.test", "https://docs.acme.test", "http://intranet.acme.test"  # the intranet in http
PNG = bytes.fromhex("89504e470d0a1a0a0000000d4948445200000001000000010806000000"
                    "1f15c4890000000d49444154789c6360f8cf00000301010018dd8db00000000049454e44ae426082")


def get(url, **kw):
    return requests.get(url, verify=CA, timeout=20, allow_redirects=False, **kw)


def no_legacy_cookies(page, *prefixes):
    names = {c["name"] for c in page.context.cookies()}
    assert not any(n.startswith(prefixes) for n in names), f"legacy cookie in the browser: {names}"


# --- Joomla ---------------------------------------------------------------

def test_joomla_home_and_article(page):
    page.goto(NEWS + "/")
    assert "Welcome to Acme News" in page.content() and "città, perché, Größe" in page.content()
    page.get_by_role("link", name="Welcome to Acme News").first.click()
    page.wait_for_load_state()
    assert page.url.startswith(NEWS + "/") and "wombat" in page.content()


def test_joomla_search(page):
    page.goto(NEWS + "/?option=com_finder&view=search&q=wombat")
    assert page.locator("#q").input_value() == "wombat"  # the results page, not the home page
    assert page.locator(".result__title", has_text="Welcome to Acme News").count() == 1


def joomla_admin(page):
    page.goto(NEWS + "/administrator/")
    page.fill("#mod-login-username", "editor")
    page.fill("#mod-login-password", "Lab-editor-pass-1")
    page.click("#btn-login-submit")
    page.wait_for_selector("text=Home Dashboard", timeout=20000)


def test_joomla_admin_publishes_an_article(page):
    joomla_admin(page)
    assert len(page.context.cookies()) <= 3, page.context.cookies()  # Equalizer only: Joomla names its cookies by hash
    title = f"Joomla article {time.time_ns()}"
    page.goto(NEWS + "/administrator/index.php?option=com_content&view=articles")
    page.locator("joomla-toolbar-button button.button-new, .button-new").first.click()  # "New", as a user does
    page.fill("#jform_title", title)
    page.frame_locator("#jform_articletext_ifr").locator("body").fill("Written through Equalizer: novità.")
    page.get_by_role("button", name="Save & Close").click()
    page.wait_for_selector("text=Article saved.")
    link = page.locator("a", has_text=title).first.get_attribute("href")
    aid = re.search(r"id=(\d+)", link).group(1)
    r = get(f"{NEWS}/?option=com_content&view=article&id={aid}&catid=2")
    assert r.status_code == 200 and title in r.text and "novità" in r.text, r.status_code


# --- Drupal ---------------------------------------------------------------

def test_drupal_page_and_search(page):
    page.goto(DOCS + "/about")
    assert "About Acme Docs" in page.content() and "città, perché, Größe" in page.content()
    page.goto(DOCS + "/search/node?keys=quokka")
    page.get_by_role("link", name="Release notes from the lab").click()
    page.wait_for_url(DOCS + "/release-notes")


def drupal_login(page):
    page.goto(DOCS + "/user/login")
    page.fill("#edit-name", "editor")
    page.fill("#edit-pass", "Lab-editor-pass-1")
    page.locator("#user-login-form").get_by_role("button", name="Log in").click()  # not the search form's
    page.wait_for_url(re.compile(r"/user/1"))


def test_drupal_editor_publishes_an_article(page):
    drupal_login(page)
    no_legacy_cookies(page, "SESS", "SSESS")
    title = f"Drupal article {time.time_ns()}"
    page.goto(DOCS + "/node/add/article")
    page.fill("#edit-title-0-value", title)
    page.locator(".ck-editor__editable").first.fill("Written through Equalizer: novità.")
    page.click("#edit-submit")
    page.wait_for_selector(f"text=Article {title} has been created.")
    r = get(page.url)
    assert r.status_code == 200 and title in r.text and "novità" in r.text


def test_drupal_admin_pages(page):
    drupal_login(page)
    for path, text in [("/admin/content", "Content"), ("/admin/people", "People"), ("/admin/config", "Configuration")]:
        page.goto(DOCS + path)
        assert page.locator("h1").first.inner_text().strip() == text, path


# --- Acme Intranet (custom, ISO-8859-1) -----------------------------------

def test_intranet_accents_and_search(page):
    page.goto(INTRANET + "/")
    page.get_by_role("link", name="Canteen opening hours").click()
    assert "Specialità del giorno: lasagne, caffè." in page.content()
    page.fill("input[name=q]", "caffè")  # sent to the legacy app in its own charset
    page.click("input[value=Go]")
    assert page.get_by_role("link", name="Canteen opening hours").count() == 1


def intranet_login(page):
    page.goto(INTRANET + "/admin/new.php")  # redirected to the login page, then back
    page.fill("#user", "editor")
    page.fill("#password", "Lab-editor-pass-1")
    page.click("#login")
    page.wait_for_url(INTRANET + "/admin/new.php")


def test_intranet_notice_with_attachment(page):
    intranet_login(page)
    no_legacy_cookies(page, "PHPSESSID")
    title = f"Novità in mensa {time.time_ns()}"
    page.fill("#title", title)
    page.fill("#body", "Da lunedì: più verdure, così è.")
    page.set_input_files("#file", files=[{"name": "menu.png", "mimeType": "image/png", "buffer": PNG}])
    page.click("#save")
    page.wait_for_url(re.compile(r"/notice\.php\?id=\d+"))
    assert title in page.content() and "così è" in page.content()
    href = page.get_by_role("link", name="menu.png").get_attribute("href")
    assert get(INTRANET + href).content[:8] == PNG[:8]


def test_intranet_wrong_password(page):
    page.goto(INTRANET + "/login.php")
    page.fill("#user", "editor")
    page.fill("#password", "wrong")
    page.click("#login")
    assert "Wrong user name or password." in page.content()


def test_intranet_refuses_a_script_as_attachment(page):
    """The application stores any attachment and its server runs .php files:
    a classic hole of old applications. Equalizer must not let it through."""
    intranet_login(page)
    page.fill("#title", "Harmless notice")
    page.set_input_files("#file", files=[{"name": "shell.php", "mimeType": "image/png",
                                          "buffer": b"<?php echo 'PWNED-' . (6*7); ?>"}])
    page.click("#save")
    r = get(INTRANET + "/uploads/shell.php")
    assert b"PWNED-42" not in r.content, r.status_code
