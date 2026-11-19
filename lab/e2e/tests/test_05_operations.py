"""Running it: forgotten password by email, alerts when a site stops working
and when it works again."""
import time

from lab import Panel, mail_link, mails
import state


def test_forgotten_password_by_email():
    since = time.time()
    anon = Panel()
    anon.call("POST", "/auth/password-reset", {"email": "boss@acme.test"}, 202)
    anon.call("POST", "/auth/password-reset", {"email": "nobody@acme.test"}, 202)
    token = mail_link("boss@acme.test", "/reset/", since).rsplit("/", 1)[1]
    anon.call("POST", f"/auth/password-reset/{token}", {"password": "Boss-new-password-1"}, 204)
    again = Panel()
    again.login("boss@acme.test", "Boss-new-password-1", state.boss_secret)
    assert again.call("GET", "/auth/me", None, 200)["email"] == "boss@acme.test"
    state.boss = again
    assert not mails("nobody@acme.test", since)


def test_alert_when_the_connector_stops_and_recovers():
    since = time.time()
    state.connector.stop()
    deadline = time.time() + 9 * 60  # grace 3 minutes, checks every 2
    while time.time() < deadline and not mails("boss@acme.test", since):
        time.sleep(5)
    got = mails("boss@acme.test", since)
    assert got and "problem" in got[-1]["Subject"], [m["Subject"] for m in got]
    back = time.time()
    state.connector.start()
    deadline = time.time() + 5 * 60
    while time.time() < deadline and not mails("boss@acme.test", back):
        time.sleep(5)
    got = mails("boss@acme.test", back)
    assert got and "working again" in got[-1]["Subject"], [m["Subject"] for m in got]
