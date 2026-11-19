"""The relay and the panel face the Internet too: what a hostile client
tries on them. Only against the lab."""
import requests

from lab import CA, PANEL

RELAY_DIRECT = "http://127.0.0.1:18081/v1/tunnel"  # as behind Caddy: the address is in X-Forwarded-For


def tunnel(token: str, attacker: str) -> int:
    r = requests.get(RELAY_DIRECT, timeout=10, headers={
        "Connection": "Upgrade", "Upgrade": "equalizer-tunnel/1", "X-Eq-Connector": "cn-web",
        "Authorization": "Bearer " + token, "X-Forwarded-For": attacker})
    return r.status_code


def test_relay_guessing_tokens_is_cut_short():
    codes = [tunnel(f"guess-{i}", "198.51.100.9") for i in range(25)]
    assert codes[:20] == [401] * 20 and set(codes[20:]) == {429}, codes
    # Another address is not affected.
    assert tunnel("guess-x", "203.0.113.50") == 401


def test_panel_wrong_passwords_lock_one_account_not_everyone():
    s = requests.Session()
    s.verify = CA
    s.headers["X-Eq-Panel"] = "1"
    for _ in range(6):
        s.post(PANEL + "/v1/auth/login", json={"email": "nobody@lab.test", "password": "wrong-password-1"}, timeout=10)
    assert s.post(PANEL + "/v1/auth/login", json={"email": "nobody@lab.test", "password": "x"},
                  timeout=10).status_code == 429
    # From the same address, the platform administrator still logs in.
    r = s.post(PANEL + "/v1/auth/login", json={"email": "root@lab.test", "password": "Lab-root-password-1"}, timeout=10)
    assert r.status_code == 200, r.status_code


def test_panel_refuses_cross_site_writes_and_hides_internal_api():
    r = requests.post(PANEL + "/v1/auth/login", json={"email": "root@lab.test", "password": "x"}, verify=CA, timeout=10)
    assert r.status_code == 403  # no X-Eq-Panel header: a form from another site
    assert requests.get(PANEL + "/v1/internal/maps", verify=CA, timeout=10).status_code == 404


def test_panel_large_body_refused():
    r = requests.post(PANEL + "/v1/auth/login", data=b"{" + b" " * (5 << 20) + b"}", verify=CA, timeout=30,
                      headers={"X-Eq-Panel": "1", "Content-Type": "application/json"})
    assert r.status_code in (400, 413), r.status_code
