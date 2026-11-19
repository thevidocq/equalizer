"""What a hostile client does to a public gateway: requests the browser
would never send, written byte by byte on the wire. Only against the lab."""
import socket
import ssl
import time

import requests

from lab import CA, RUN, SITE

HOST = "www.acme.test"
GATEWAY = ("127.0.0.1", 18080)  # behind the front, as behind Caddy
SECRETS = (b"DB_NAME", b"DB_PASSWORD", b"root:x:0:0", b"AUTH_KEY")


def raw(request: bytes, tls=True, timeout=20) -> bytes:
    """Sends bytes as written (no client library cleans them) and returns
    everything the server answers."""
    s = socket.create_connection(("127.0.0.1", 443) if tls else GATEWAY, timeout=timeout)
    if tls:
        s = ssl.create_default_context(cafile=CA).wrap_socket(s, server_hostname=HOST)
    try:
        s.sendall(request)
        out = b""
        while True:
            try:
                chunk = s.recv(65536)
            except (socket.timeout, ConnectionResetError):
                break
            if not chunk:
                break
            out += chunk
        return out
    finally:
        s.close()


def status(answer: bytes) -> int:
    try:
        return int(answer.split(b" ", 2)[1])
    except (IndexError, ValueError):
        return 0


def get(path: str, **headers) -> bytes:
    lines = [f"GET {path} HTTP/1.1", f"Host: {HOST}", "Connection: close"] + [f"{k}: {v}" for k, v in headers.items()]
    return raw(("\r\n".join(lines) + "\r\n\r\n").encode("latin-1"))


def test_paths_that_climb_on_some_servers():
    for path in ["/wp-content/..;/wp-config.php", "/wp-content/..%3b/wp-config.php", "/wp-content/..%5cwp-config.php",
                 "/wp-content/..\\wp-config.php", "/wp-content/.. /wp-config.php".replace(" ", "%20"),
                 "/wp-content/.../wp-config.php", "/%c0%ae%c0%ae/wp-config.php", "/wp-content/%2e%2e/wp-config.php",
                 "/wp-config.php%00.png", "/wp-content/uploads/..%2f..%2fwp-config.php"]:
        answer = get(path)
        assert status(answer) in (400, 404), (path, answer[:80])
        assert not any(s in answer for s in SECRETS), path


def test_request_smuggling_refused():
    # Both lengths: the classic desync between a proxy and the server behind.
    answer = raw(b"POST / HTTP/1.1\r\nHost: " + HOST.encode() + b"\r\nContent-Length: 4\r\nTransfer-Encoding: chunked\r\n"
                 b"Connection: close\r\n\r\n0\r\n\r\nGET /wp-admin/ HTTP/1.1\r\nHost: " + HOST.encode() + b"\r\n\r\n")
    # Chunked wins (RFC 9112), one answer, connection closed: the request
    # hidden in the body is never served.
    assert answer.count(b"HTTP/1.1 ") == 1 and b"wp-admin-bar" not in answer and b"Dashboard" not in answer, answer[:120]
    answer = raw(b"GET / HTTP/1.1\r\nHost: " + HOST.encode() + b"\r\nTransfer-Encoding: xchunked\r\nConnection: close\r\n\r\n")
    assert status(answer) in (400, 501), answer[:120]


def test_oversized_headers_refused():
    answer = get("/", **{"X-Filler": "a" * 70000})
    assert status(answer) in (400, 431), answer[:80]


def test_other_hosts_and_absolute_targets():
    assert status(raw(b"GET / HTTP/1.1\r\nHost: evil.example\r\nConnection: close\r\n\r\n")) in (0, 404, 421)
    answer = raw(b"GET http://evil.example/ HTTP/1.1\r\nHost: " + HOST.encode() + b"\r\nConnection: close\r\n\r\n")
    assert b"evil.example" not in answer.split(b"\r\n\r\n")[0] or status(answer) in (400, 404, 421), answer[:200]


def test_form_with_too_many_fields():
    r = requests.post(SITE + "/wp-comments-post.php", data="a=1&" * 20000, verify=CA, timeout=30,
                      headers={"Content-Type": "application/x-www-form-urlencoded"}, allow_redirects=False)
    assert r.status_code == 400, r.status_code


def test_forged_client_address_ignored():
    before = (RUN / "logs" / "gateway.log").read_text(errors="replace").count("\n")
    get("/?forged-address-check=1", **{"X-Forwarded-For": "6.6.6.6", "X-Real-IP": "6.6.6.6", "CF-Connecting-IP": "6.6.6.6"})
    time.sleep(0.5)
    new = (RUN / "logs" / "gateway.log").read_text(errors="replace").splitlines()[before:]
    assert new and not any('"src_ip":"6.6.6.6"' in line for line in new)


def test_session_flood_from_one_address():
    """Thousands of visitors without cookies on a page that sets one would
    fill the session store: a new anonymous session lives five minutes until
    its visitor comes back, and one address only gets so many."""
    import hashlib
    import subprocess
    redis = ["redis-cli", "-p", "16379", "--raw"]
    s = requests.Session()
    s.verify = CA
    ids, start, sent = [], time.time(), 0
    while sent < 1000 and sent - len(ids) < 50:  # until 50 are refused
        r = s.get(SITE + "/wp-login.php", timeout=20, allow_redirects=False)
        ids += [c.value for c in r.cookies if c.name == "__Host-eq_session"]
        s.cookies.clear()
        sent += 1
    elapsed = time.time() - start
    # A burst of 300, then 10 a second.
    assert 0 < len(ids) <= 300 + 10 * elapsed + 5 and sent - len(ids) >= 50, (len(ids), sent, elapsed)
    ttl = int(subprocess.check_output(redis + ["TTL", "eq:sess:" + hashlib.sha256(ids[-1].encode()).hexdigest()]))
    assert 0 < ttl <= 300, ttl
    # The page itself is still served.
    assert s.get(SITE + "/wp-login.php", timeout=20).status_code == 200


def test_body_sent_a_byte_at_a_time_is_cut():
    """A client that sends its body at a drip holds a connection: past the
    grace period the gateway closes it."""
    s = socket.create_connection(GATEWAY, timeout=90)
    s.sendall(b"POST /wp-comments-post.php HTTP/1.1\r\nHost: " + HOST.encode() +
              b"\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 100000\r\n\r\n")
    start, closed = time.time(), False
    try:
        while time.time() - start < 75:
            s.sendall(b"a")
            time.sleep(2)
    except (BrokenPipeError, ConnectionResetError, socket.timeout):
        closed = True
    finally:
        s.close()
    assert closed, "connection still open after 75 s at one byte every 2 s"


def test_site_still_answers():
    assert requests.get(SITE + "/", verify=CA, timeout=20).status_code == 200
