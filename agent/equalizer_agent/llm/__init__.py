"""Interchangeable AI providers, configured by each tenant (never by the
platform). Every provider answers complete(system, user) -> text."""

import ipaddress
import json
import socket
import urllib.parse
import urllib.request


class LLMError(Exception):
    pass


def _post(url: str, headers: dict, body: dict, timeout: float) -> dict:
    req = urllib.request.Request(url, data=json.dumps(body).encode(), method="POST",
                                 headers={"Content-Type": "application/json", **headers})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read(4 << 20))
    except urllib.error.HTTPError as e:
        raise LLMError(f"provider answered {e.code}") from None
    except (OSError, ValueError) as e:
        raise LLMError(f"provider unreachable: {type(e).__name__}") from None


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

    def __init__(self, model: str, api_key: str, endpoint: str = "https://api.openai.com/v1", timeout: float = 120):
        self.model, self.key, self.base, self.timeout = model, api_key, endpoint.rstrip("/"), timeout

    def complete(self, system: str, user: str) -> str:
        headers = {"Authorization": "Bearer " + self.key} if self.key else {}
        out = _post(self.base + "/chat/completions", headers,
                    {"model": self.model, "temperature": 0,
                     "messages": [{"role": "system", "content": system}, {"role": "user", "content": user}]}, self.timeout)
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
        return OpenAICompatible(model, key, endpoint)
    return None
