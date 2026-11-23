"""The attacks the study sends, and how it tells what reached the legacy
site: every value carries a marker, and the legacy site logs what it got
(record.php). An attack "passed" when its marker and its dangerous part
arrived; "neutralised" when the marker arrived without it; "dropped" when
the request was served without the marker; "blocked" when it was refused
and nothing reached the site."""
import base64
import json
import urllib.parse

PAYLOADS = {
    "sqli": ["1' OR '1'='1", "1 UNION SELECT password FROM users-- -", "1;SELECT SLEEP(5)"],
    "xss": ["<script>alert(1)</script>", "\"><img src=x onerror=alert(1)>", "<svg/onload=alert(1)>"],
    "traversal": ["../../etc/passwd", "..%2f..%2fconfig.php", "....//....//etc/passwd"],
    "cmd": ["; cat /etc/passwd", "|id", "$(id)"],
    "ssti": ["{{7*7}}", "${7*7}", "<%= 7*7 %>"],
    "php_object": ['O:8:"stdClass":0:{}', 'a:1:{i:0;O:1:"X":0:{}}'],
    "crlf": ["x\r\nSet-Cookie: pwn=1", "x%0d%0aLocation:%20http://evil.test"],
    "oversize": ["A" * 6000],
}
# The dangerous part of each payload, as the site would read it decoded.
CORE = {"sqli": ["'", "UNION", "SLEEP"], "xss": ["<", ">"], "traversal": ["..", "/etc/passwd"],
        "cmd": [";", "|", "$("], "ssti": ["{{", "${", "<%"], "php_object": ["O:", "a:1:{"],
        "crlf": ["\r\n", "%0d%0a"], "oversize": ["A" * 6000]}


def decoded(entry: dict) -> str:
    """What the site got, as sent and decoded once and twice (a payload that
    is itself encoded, x%0d%0a, is found as it arrived)."""
    u = entry["u"]
    b = base64.b64decode(entry["b"]).decode("utf-8", "replace")
    once = urllib.parse.unquote_plus(u) + "\n" + urllib.parse.unquote_plus(b)
    return "\n".join([u, b, once, urllib.parse.unquote_plus(once), json.dumps(entry.get("h", {}))])


def verdict(status, legacy: list, marker: str, payload: str, cls: str) -> str:
    if status is None:
        return "blocked"
    seen = [decoded(e) for e in legacy]
    hit = next((s for s in seen if marker in s), None)
    if hit is None:
        if status == 429:
            return "rate_limited"
        return "blocked" if status >= 400 and not legacy else "dropped"
    core = [c for c in CORE[cls] if c in payload]
    return "passed" if payload in hit or any(c in hit for c in core) else "neutralised"
