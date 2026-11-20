"""The compatibility matrix (docs/compatibility.md), generated from the
profiles so that it never says more than the code does:

    python3 -m equalizer_agent.webapps.matrix > ../docs/compatibility.md

The agent's tests fail when the file is not the one this prints."""

import os
import sys

from . import ALL, GENERIC

ROOT = os.path.join(os.path.dirname(__file__), "..", "..", "..")
TEMPLATES = os.path.join(ROOT, "panel-api", "internal", "templates", "maps")


def _yes(b: bool) -> str:
    return "yes" if b else "no"


def _recognised(p) -> str:
    how = [w for w, ok in (("generator tag", p.generator), ("paths", p.paths), ("extensions", p.components)) if ok]
    return ", ".join(how) or "not recognised: the fallback"


def render() -> str:
    rows = []
    for p in ALL + (GENERIC,):
        template = os.path.exists(os.path.join(TEMPLATES, p.name + ".json"))
        admin = "read-only (proven in the lab)" if p.admin_read_only else "closed"
        if p.admin_read_only and p.admin_login:
            admin += ", own login"
        rows.append("| %s | `%s` | %s | %s | %s | %d | %s | %s |" % (
            p.title, p.name, _recognised(p), f"`{p.login}`" if p.login else "first login form found",
            admin, len(p.never), _yes(template), "yes" if p.tested else "not yet"))
    notes = []
    for p in ALL + (GENERIC,):
        text = " ".join(t for t in (p.tested, p.note) if t)
        if text:
            notes.append(f"- **{p.title}**: {text}")
    return "\n".join([
        "# Compatibility",
        "",
        "Generated from the agent's web application profiles",
        "(`agent/equalizer_agent/webapps/`) by",
        "`python3 -m equalizer_agent.webapps.matrix`; do not edit by hand.",
        "",
        "Every site is protected the same way whatever it is built with: the",
        "gateway rebuilds every request and the whole site stays served by the",
        "map's `/` area. What changes per application is how much the discovery",
        "agent can study and narrow (docs/agent.md), and the map template",
        "offered for the first version.",
        "",
        "- **Recognised by**: what tells the agent which application it is.",
        "- **Test account login**: where a test account logs in.",
        "- **Administration**: whether a test administrator's pass studies it.",
        "  It opens only where the lab proved that viewing it changes nothing.",
        "- **Pages never opened**: rules for pages that act when only opened",
        "  (on top of the rules every site shares: action links, one-time",
        "  tokens, sensitive files).",
        "- **Lab**: whether `lab/e2e` runs this application; \"not yet\" means",
        "  the profile is written from the application's documentation and",
        "  behaviour, not proven.",
        "",
        "| Application | Site kind | Recognised by | Test account login | Administration | Pages never opened"
        " (rules) | Map template | Lab |",
        "| --- | --- | --- | --- | --- | --- | --- | --- |",
        *rows,
        "",
        "## Notes",
        "",
        *notes,
        "",
    ])


if __name__ == "__main__":
    sys.stdout.write(render())
