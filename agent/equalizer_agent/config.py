"""Configuration from environment variables (no local state)."""

import os
from dataclasses import dataclass, field

from .log import Logger


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


def _number(name: str, default: float, low: float, high: float) -> float:
    """A number setting, kept within its bounds; a value that is not a
    number is the default, said in the agent's log at start."""
    raw = os.environ.get(name, "").strip()
    try:
        v = float(raw) if raw else default
    except ValueError:
        Logger(os.environ.get("HOSTNAME", "agent")).warn("a setting is not a number: its default is used", setting=name)
        v = default
    return min(high, max(low, v))


def from_env() -> Config:
    c = Config(
        panel_url=os.environ.get("EQ_PANEL_URL", Config.panel_url).rstrip("/"),
        panel_token=_secret("EQ_INTERNAL_TOKEN"),
        relay_url=os.environ.get("EQ_RELAY_URL", Config.relay_url).rstrip("/"),
        relay_token=_secret("EQ_RELAY_TOKEN"),
        poll_seconds=_number("EQ_POLL_SECONDS", 10.0, 1, 3600),
        rate=_number("EQ_RATE", 2.0, 0.1, 100),
        direct={d.strip().lower() for d in os.environ.get("EQ_DIRECT_DESTINATIONS", "").split(",") if d.strip()},
        allow_private_llm=os.environ.get("EQ_AGENT_ALLOW_PRIVATE_LLM", "") == "true",
        log_level=os.environ.get("EQ_LOG_LEVEL", "info"),
        instance=os.environ.get("HOSTNAME", "agent"),
        browser=os.environ.get("EQ_BROWSER", "auto").strip().lower(),
        browser_path=os.environ.get("EQ_BROWSER_PATH", "").strip(),
        browser_page_seconds=int(_number("EQ_BROWSER_PAGE_SECONDS", 20, 5, 120)),
        gateway_url=os.environ.get("EQ_GATEWAY_ADMIN_URL", Config.gateway_url).rstrip("/"),
    )
    if len(c.panel_token) < 32:
        raise SystemExit("agent: EQ_INTERNAL_TOKEN or EQ_INTERNAL_TOKEN_FILE is required")
    return c
