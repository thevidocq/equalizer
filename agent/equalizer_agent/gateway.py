"""The gateway's tries (POST /maps/try, schema/api/gateway-admin.yaml): a
draft served in dry-run (verdict only, nothing forwarded) or preview (GET
and HEAD through the connector). The agent's only way to see what the
gateway would do with a version."""

import base64
import json
import urllib.error
import urllib.request

TIMEOUT = 30


class TryError(Exception):
    pass


class Gateway:
    def __init__(self, admin_url: str, token: str, try_map: dict):
        self.url, self.token, self.map = admin_url.rstrip("/") + "/maps/try", token, try_map

    def _try(self, mode: str, method: str, path: str, headers=None, body: bytes = b"") -> dict:
        req = {"method": method, "path": path, "headers": headers or {}}
        if body:
            req["body"] = base64.b64encode(body).decode()
        data = json.dumps({"map": self.map, "mode": mode, "request": req}).encode()
        r = urllib.request.Request(self.url, data=data, method="POST", headers={
            "Authorization": "Bearer " + self.token, "Content-Type": "application/json"})
        try:
            with urllib.request.urlopen(r, timeout=TIMEOUT) as resp:
                return json.loads(resp.read())
        except urllib.error.HTTPError as e:
            raise TryError("the gateway refused the try: HTTP %d %s" % (e.code, e.read()[:200].decode("utf-8", "replace")))
        except (urllib.error.URLError, OSError, ValueError) as e:
            raise TryError("the gateway is unreachable: %s" % e)

    def preview(self, path: str, headers=None) -> dict:
        res = self._try("preview", "GET", path, headers)
        if res.get("response"):
            res["response"]["body"] = base64.b64decode(res["response"].get("body") or "")
        return res

    def dry_run(self, method: str, path: str, headers=None, body: bytes = b"") -> dict:
        return self._try("dry_run", method, path, headers, body)


def cookie_header(set_cookies: list) -> str:
    """The Cookie header a browser would send back for these Set-Cookie
    values (name=value only)."""
    return "; ".join(c.split(";", 1)[0].strip() for c in set_cookies if "=" in c.split(";", 1)[0])
