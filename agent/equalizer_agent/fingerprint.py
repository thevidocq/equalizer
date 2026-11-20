"""Which CMS, which version, which plugins, themes and libraries."""

import json
import os
import re
from dataclasses import dataclass, field

from . import webapps

# From the profiles (webapps/): one place per application.
GENERATORS = [(p.name, p.generator) for p in webapps.ALL if p.generator]
PATH_HINTS = [(p.name, p.paths) for p in webapps.ALL if p.paths]
COMPONENT = {p.name: p.components for p in webapps.ALL if p.components}
LIBS = re.compile(r"/(jquery|bootstrap|angular|vue|react)[.\-]?([\d.]+)?(?:\.min)?\.js", re.I)


@dataclass
class Fingerprint:
    cms: str | None = None
    version: str | None = None
    components: dict = field(default_factory=dict)  # "plugin:contact-form-7" -> version or ""

    def as_report(self, advisories: list) -> dict:
        flagged = []
        for name, ver in sorted(self.components.items()):
            for a in advisories:
                if a.get("component") == name and ver and _lower(ver, a.get("fixed", "")):
                    flagged.append({"component": name, "version": ver, "fixed": a["fixed"], "note": a.get("note", "")})
        if self.cms and self.version:
            for a in advisories:
                if a.get("component") == self.cms and _lower(self.version, a.get("fixed", "")):
                    flagged.append({"component": self.cms, "version": self.version, "fixed": a["fixed"], "note": a.get("note", "")})
        return {"cms": self.cms, "version": self.version,
                "components": [{"name": k, "version": v or None} for k, v in sorted(self.components.items())][:200],
                "known_issues": flagged}


def _lower(v: str, fixed: str) -> bool:
    def nums(s):
        return [int(x) for x in re.findall(r"\d+", s)[:4]]
    return bool(fixed) and nums(v) < nums(fixed)


def detect(fp: Fingerprint, generator: str, urls: list):
    if generator and not fp.version:
        for cms, rx in GENERATORS:
            m = rx.search(generator)
            if m and fp.cms in (None, cms):
                fp.cms, fp.version = cms, (m.group(1) or None)
                break
    if not fp.cms:
        votes = {}
        for u in urls:
            for cms, rx in PATH_HINTS:
                if rx.search(u):
                    votes[cms] = votes.get(cms, 0) + 1
        if votes:
            fp.cms = max(votes, key=votes.get)
    rx = COMPONENT.get(fp.cms or "")
    for u in urls:
        if rx:
            m = rx.search(u)
            if m:
                kind = (m.group(1) or "module").rstrip("s").lower()
                ver = m.group(3) if m.lastindex and m.lastindex >= 3 else None
                key = f"{kind}:{m.group(2).lower()}"
                if ver or key not in fp.components:
                    fp.components[key] = ver or fp.components.get(key, "")
        m = LIBS.search(u)
        if m:
            key = "library:" + m.group(1).lower()
            if m.group(2) or key not in fp.components:
                fp.components[key] = (m.group(2) or "").strip(".")


def advisories() -> list:
    path = os.environ.get("EQ_ADVISORIES_FILE") or os.path.join(os.path.dirname(__file__), "data", "advisories.json")
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f).get("entries", [])
    except (OSError, ValueError):
        return []
