"""Configuration from environment variables (no local state)."""

import os
from dataclasses import dataclass, field


def _secret(name: str) -> str:
    value = os.environ.get(name, "").strip()
    if value:
        return value
    path = os.environ.get(name + "_FILE", "")
    if path:
        with open(path, encoding="utf-8") as f:
            return f.read().strip()
    return ""


@dataclass
class Config:
    panel_url: str = "http://panel-api:8080"
    panel_token: str = ""
    relay_url: str = "http://tunnel-relay:8082"
    relay_token: str = ""
    poll_seconds: float = 10.0
    rate: float = 2.0  # requests per second towards the legacy site
    direct: set = field(default_factory=set)  # host:port allowed in direct mode (lab)
    allow_private_llm: bool = False  # on-prem installations with a model on the LAN
    log_level: str = "info"
    instance: str = ""
    browser: str = "off"  # from EQ_BROWSER: auto (when Playwright is installed, the default), on, off
    browser_path: str = ""  # a Chromium to use instead of Playwright's own
    browser_page_seconds: int = 20  # EQ_BROWSER_PAGE_SECONDS: one page in the browser at most, then skipped
    gateway_url: str = "http://gateway:9090"  # the gateway's admin endpoint, for verifications


def from_env() -> Config:
    c = Config(
        panel_url=os.environ.get("EQ_PANEL_URL", Config.panel_url).rstrip("/"),
        panel_token=_secret("EQ_INTERNAL_TOKEN"),
        relay_url=os.environ.get("EQ_RELAY_URL", Config.relay_url).rstrip("/"),
        relay_token=_secret("EQ_RELAY_TOKEN"),
        poll_seconds=float(os.environ.get("EQ_POLL_SECONDS", "10")),
        rate=float(os.environ.get("EQ_RATE", "2")),
        direct={d.strip().lower() for d in os.environ.get("EQ_DIRECT_DESTINATIONS", "").split(",") if d.strip()},
        allow_private_llm=os.environ.get("EQ_AGENT_ALLOW_PRIVATE_LLM", "") == "true",
        log_level=os.environ.get("EQ_LOG_LEVEL", "info"),
        instance=os.environ.get("HOSTNAME", "agent"),
        browser=os.environ.get("EQ_BROWSER", "auto").strip().lower(),
        browser_path=os.environ.get("EQ_BROWSER_PATH", "").strip(),
        browser_page_seconds=min(120, max(5, int(os.environ.get("EQ_BROWSER_PAGE_SECONDS", "20") or 20))),
        gateway_url=os.environ.get("EQ_GATEWAY_ADMIN_URL", Config.gateway_url).rstrip("/"),
    )
    if len(c.panel_token) < 32:
        raise SystemExit("agent: EQ_INTERNAL_TOKEN or EQ_INTERNAL_TOKEN_FILE is required")
    return c
