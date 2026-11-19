"""Interchangeable AI providers, configured by each tenant (never by the
platform). Every provider answers complete(system, user) -> text."""

import http.client
import ipaddress
import json
import socket
import ssl
import urllib.parse


class LLMError(Exception):
    pass


def _post(url: str, headers: dict, body: dict, timeout: float, public_only: bool = True) -> dict:
    """POSTs JSON over https. Redirects are never followed, and with
    public_only the address actually connected to must be public: a tenant's
    server could otherwise send the agent into the platform's own networks
    (a redirect to http://redis:6379, or a DNS answer that changes between
    the check and the connection)."""
    u = urllib.parse.urlsplit(url)
    if u.scheme != "https" or not u.hostname:
        raise LLMError("the provider must be an https address")
    conn = http.client.HTTPSConnection(u.hostname, u.port or 443, timeout=timeout, context=ssl.create_default_context())
    try:
        conn.connect()
        if public_only and not ipaddress.ip_address(conn.sock.getpeername()[0]).is_global:
            raise LLMError("the provider's address is not public")
        path = (u.path or "/") + ("?" + u.query if u.query else "")
        conn.request("POST", path, body=json.dumps(body).encode(),
                     headers={"Content-Type": "application/json", **headers})
        resp = conn.getresponse()
        if 300 <= resp.status < 400:
            raise LLMError(f"provider answered {resp.status} (redirects are not followed)")
        if resp.status >= 400:
            raise LLMError(f"provider answered {resp.status}")
        return json.loads(resp.read(4 << 20))
    except LLMError:
        raise
    except (OSError, ValueError, http.client.HTTPException) as e:
        raise LLMError(f"provider unreachable: {type(e).__name__}") from None
    finally:
        conn.close()


class Anthropic:
    def __init__(self, model: str, api_key: str, timeout: float = 120):
        self.model, self.key, self.timeout = model, api_key, timeout

    def complete(self, system: str, user: str) -> str:
        out = _post("https://api.anthropic.com/v1/messages", {"x-api-key": self.key, "anthropic-version": "2023-06-01"},
                    {"model": self.model, "max_tokens": 8000, "system": system,
                     "messages": [{"role": "user", "content": user}]}, self.timeout)
        return "".join(b.get("text", "") for b in out.get("content", []) if b.get("type") == "text")


class OpenAICompatible:
    """OpenAI, or any server with the same API (Ollama, vLLM)."""

    def __init__(self, model: str, api_key: str, endpoint: str = "https://api.openai.com/v1", timeout: float = 120,
                 public_only: bool = True):
        self.model, self.key, self.base, self.timeout = model, api_key, endpoint.rstrip("/"), timeout
        self.public_only = public_only

    def complete(self, system: str, user: str) -> str:
        headers = {"Authorization": "Bearer " + self.key} if self.key else {}
        out = _post(self.base + "/chat/completions", headers,
                    {"model": self.model, "temperature": 0,
                     "messages": [{"role": "system", "content": system}, {"role": "user", "content": user}]}, self.timeout,
                    self.public_only)
        try:
            return out["choices"][0]["message"]["content"] or ""
        except (KeyError, IndexError, TypeError):
            raise LLMError("unexpected answer from the provider") from None


def public_endpoint(url: str) -> bool:
    """A tenant's endpoint must not point at the platform's own networks."""
    host = urllib.parse.urlsplit(url).hostname or ""
    try:
        infos = socket.getaddrinfo(host, 443)
    except OSError:
        return False
    for info in infos:
        ip = ipaddress.ip_address(info[4][0])
        if not ip.is_global:
            return False
    return True


def provider(cfg: dict | None, allow_private: bool = False):
    """The tenant's provider, or None (discovery runs without AI)."""
    if not cfg:
        return None
    kind, model, key = cfg.get("provider"), cfg.get("model") or "", cfg.get("api_key") or ""
    if kind == "anthropic":
        return Anthropic(model, key)
    if kind == "openai":
        return OpenAICompatible(model, key)
    if kind == "local":
        endpoint = cfg.get("endpoint") or ""
        if not endpoint.startswith("https://") or (not allow_private and not public_endpoint(endpoint)):
            raise LLMError("the tenant's model endpoint is not a public https address")
        return OpenAICompatible(model, key, endpoint, public_only=not allow_private)
    return None
