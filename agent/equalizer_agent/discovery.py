"""One discovery, from the job to the result."""

import urllib.parse

from . import browser, crawler, fingerprint, mapbuild, rules
from .fetch import Fetcher, ReachError
from .llm import LLMError, provider
from .llm.refine import refine


def seeds(job: dict, cms: str | None) -> list:
    out = ["/", "/robots.txt", "/sitemap.xml"]
    for base in (job.get("current_map"), job.get("template")):
        for r in (base or {}).get("routes", []):
            p = r.get("path", "")
            if "{" not in p:
                out.append(p)
    for u in job.get("unmapped", []):
        p = u.get("path", "")
        if u.get("method") == "GET" and u.get("clients", 0) >= 2 and p.startswith("/") and not rules.excluded(p, cms):
            out.append(p)
    return list(dict.fromkeys(out))


def accounts(job: dict) -> list:
    """The test accounts: the list, or the single account of an older panel."""
    out = [a for a in job.get("accounts") or [] if a.get("username")]
    if not out and (job.get("account") or {}).get("username"):
        out = [dict(job["account"], label="account")]
    return out[:3]


def use_browser(cfg, log) -> bool:
    """The real browser, when it is wanted and installed."""
    want = getattr(cfg, "browser", "off")
    if want == "off":
        return False
    if not browser.available():
        if want == "on":
            log.warn("EQ_BROWSER=on but Playwright is not installed: plain requests only")
        return False
    return True


def run(job: dict, cfg, panel, log) -> None:
    jid = job["id"]
    try:
        f = Fetcher(job["base_url"], job["reach"], job["tenant"], cfg.relay_url, cfg.relay_token, cfg.direct, cfg.rate)
    except ReachError as e:
        panel.result(jid, error=str(e))
        return
    cms = job.get("kind") if job.get("kind") not in (None, "generic") else None
    c = crawler.Crawler(f, cms, int(job.get("max_pages", 500)), log, progress=lambda p: panel.progress(jid, p))
    if use_browser(cfg, log):
        c.browser = {"executable": getattr(cfg, "browser_path", "")}
    fp = fingerprint.Fingerprint(cms=cms)
    try:
        result = c.anonymous(seeds(job, cms))
        fingerprint.detect(fp, result.generator, result.urls)
        c.cms = fp.cms  # its admin rules and login page from now on
        for account in accounts(job):
            result = c.logged_in(account, account["label"])
            fingerprint.detect(fp, result.generator, result.urls)
    except ReachError as e:
        panel.result(jid, error=str(e))
        return
    doc, report, samples = mapbuild.build(job, result, fp)
    try:
        llm = provider(job.get("llm"), cfg.allow_private_llm)
    except LLMError as e:
        llm, report["ai"] = None, {"used": False, "error": str(e)}
    if llm is not None:
        report["ai"] = refine(llm, doc, report, samples)
        report["coverage_note"] = "after AI review"
    elif "ai" not in report:
        report["ai"] = {"used": False, "reason": "no AI model configured for this tenant" if not job.get("llm") else "off"}
    log.info("discovery finished", discovery=jid, app=job["app"], pages=report["pages_found"],
             coverage=report["coverage"], routes_added=len(report["routes_added"]), ai=report["ai"].get("used"))
    panel.result(jid, report=report, document=doc)


def host_of(url: str) -> str:
    return urllib.parse.urlsplit(url).netloc
