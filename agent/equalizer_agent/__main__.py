"""python -m equalizer_agent: takes discoveries from the panel and runs
them one at a time."""

import signal
import time

from . import config, discovery, review
from .log import Logger
from .panel import Panel


def main():
    cfg = config.from_env()
    log = Logger(cfg.instance, cfg.log_level)
    panel = Panel(cfg.panel_url, cfg.panel_token)
    stop = {"now": False}
    signal.signal(signal.SIGTERM, lambda *_: stop.update(now=True))
    log.info("agent started", panel=cfg.panel_url, rate=cfg.rate)
    while not stop["now"]:
        try:
            job = panel.claim()
        except OSError as e:
            log.warn("panel unreachable", error=e)
            job = None
        if job is None:
            try:
                rv = panel.claim_review()
            except OSError:
                rv = None
            if rv is not None:
                try:
                    review.run(rv, cfg, panel, log)
                except Exception as e:  # never leave a review running
                    log.error("review failed", review=rv["id"], error=type(e).__name__)
                    panel.review_result(rv["id"], error="the agent failed: " + type(e).__name__)
                continue
            time.sleep(cfg.poll_seconds)
            continue
        log.info("discovery started", discovery=job["id"], app=job["app"], ai=bool(job.get("llm")),
                 with_account=bool(job.get("account")))
        try:
            discovery.run(job, cfg, panel, log)
        except Exception as e:  # never leave a job running
            log.error("discovery failed", discovery=job["id"], error=type(e).__name__)
            panel.result(job["id"], error="the agent failed: " + type(e).__name__)
    log.info("agent stopped")


if __name__ == "__main__":
    main()
