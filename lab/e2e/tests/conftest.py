import pytest
from playwright.sync_api import sync_playwright

from lab import RUN


@pytest.fixture(scope="session")
def playwright():
    with sync_playwright() as p:
        yield p


@pytest.fixture(scope="session")
def browser(playwright):
    exe = "/usr/bin/google-chrome"
    b = playwright.chromium.launch(executable_path=exe, args=["--no-sandbox"]) if __import__("os").path.exists(exe) else playwright.chromium.launch()
    yield b
    b.close()


@pytest.fixture()
def page(browser):
    ctx = browser.new_context(ignore_https_errors=True, viewport={"width": 1280, "height": 900})
    pg = ctx.new_page()
    yield pg
    ctx.close()


def pytest_report_header(config):
    return f"lab: {RUN}"
