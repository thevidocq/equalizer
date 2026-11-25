"""Efficacy study: how much of what Equalizer is meant to stop it stops, on
every lab site the study can run (docs/efficacy.md). For each site the real
agent makes the map, the gateway of this checkout serves it, and the legacy
site logs what it receives.

    python3 lab/e2e/efficacy/efficacy.py [site ...]

Writes .run/efficacy/<site>/*.json and .run/efficacy/summary.md, and exits 1
when a guarantee does not hold: every page the same as on the site, every
attack on a typed value stopped (with an extra key too), no attack on the
protocol through."""
import collections
import json
import sys

import measure
import stack
from equalizer_agent.llm import study as ai_study
from sites import OUT, SITES, available

GUARANTEES = {
    "fidelity": "every page the same as on the site",
    "typed": "every attack on a typed value stopped",
    "extra_key:typed": "the same with an undeclared key added",
    "protocol": "no attack on the protocol through",
}


def tally(rows, key):
    t = collections.defaultdict(collections.Counter)
    for r in rows:
        t[r[key]][r["verdict"]] += 1
    return t


def study(name):
    s = SITES[name]
    out = OUT / name
    out.mkdir(parents=True, exist_ok=True)
    procs = [stack.legacy(name, s)]
    try:
        draft, report = stack.discover(name, s)
        procs.append(stack.gateway(name, s, draft))
        site = measure.Site(name, s, out)
        res = {"parity": parity(draft, report),
               "visits": measure.visits(site, report), "attacks": measure.attacks(site, draft, report),
               "protocol": measure.protocol(site), "map": {"routes": len(draft.get("routes", [])),
                                                          "forms": len(draft.get("forms", [])), "pages": report.get("pages_found")}}
        (out / "results.json").write_text(json.dumps(res, indent=1))
        return res
    finally:
        for p in procs:
            p.terminate()


def parity(draft, report):
    """What the agent made of the site, the same measures on every kind: the
    inputs of the map and how many are typed, what the code added, what the
    AI security study would be given (no model needed to count it)."""
    fields = [spec for r in draft.get("routes", []) for part in ("params", "query") for spec in (r.get(part) or {}).values()]
    fields += [spec for f in draft.get("forms", []) for spec in (f.get("fields") or {}).values()]
    kinds = collections.Counter(measure.kind_of(draft, spec) for spec in fields)
    src = report.get("source") or {}
    ev = ai_study.evidence(draft, report)
    items = collections.Counter(i[0] for i in ev.items)
    return {"pages": report.get("pages_found"), "routes": len(draft.get("routes", [])), "forms": len(draft.get("forms", [])),
            "inputs": len(fields), "typed": kinds["typed"], "patterned": kinds["patterned"], "free": kinds["free"],
            "code": {"stack": src.get("stack"), "files": src.get("files", 0), "typed": len(src.get("typed") or []),
                     "unseen": src.get("unseen_total", len(src.get("unseen") or [])),
                     "risks": src.get("risks_total", len(src.get("risks") or [])),
                     "components": len(report.get("components") or [])},
            "study": dict(sorted(items.items()))}


def parity_table(results):
    """One line per site: the same measures side by side."""
    lines = ["## Parity", "", "| Site | Pages | Precise | Inputs | Typed | Pattern | Free | Code files | Typed by code | "
             "Code-only entries | Risky calls | Components | Study evidence |", "| --- " * 13 + "|"]
    for name, res in results.items():
        p, c = res["parity"], res["parity"]["code"]
        v = [r for r in res["visits"] if r["direct"] is not None]
        precise = [r for r in v if r["entry"] and not r["entry"].startswith("site")]
        ev = ", ".join(f"{k} {n}" for k, n in p["study"].items())
        lines.append(f"| {name} | {p['pages']} | {len(precise)}/{len(v)} | {p['inputs']} | {p['typed']} | {p['patterned']} | "
                     f"{p['free']} | {c['files']} | {c['typed']} | {c['unseen']} | {c['risks']} | {c['components']} | {ev} |")
    return lines + [""]


def summary(results, missing=()):
    lines, failed = ["# Efficacy study", ""], []
    if missing:
        lines += ["Not studied (not built in this lab): " + ", ".join(missing) + ".", ""]
    for name, res in results.items():
        v = [r for r in res["visits"] if r["direct"] is not None]
        same = [r for r in v if r["direct"] == r["via"] and r.get("text", 1.0) >= 0.97]
        precise = [r for r in v if r["entry"] and not r["entry"].startswith("site")]
        lines += [f"## {name}", "", f"Map: {res['map']['routes']} routes, {res['map']['forms']} forms, "
                  f"{res['map']['pages']} pages found.", "",
                  f"Visits: {len(same)}/{len(v)} the same as on the site; {len(precise)}/{len(v)} served by a precise entry.", "",
                  "| Input | Requests | Blocked | Dropped | Neutralised | Passed |", "| --- | --- | --- | --- | --- | --- |"]
        if len(same) < len(v):
            failed.append((name, "fidelity", [r["url"] for r in v if r not in same][:5]))
        unmeasured = sorted({r["point"] for r in res["attacks"] if r["verdict"] == "unmeasured"})
        attacks = [r for r in res["attacks"] if r["verdict"] != "unmeasured"]
        for surface, c in sorted(tally(attacks, "surface").items()):
            lines.append(f"| {surface} | {sum(c.values())} | {c['blocked']} | {c['dropped']} | {c['neutralised']} | {c['passed']} |")
            if surface in ("typed", "extra_key:typed") and c["passed"]:
                failed.append((name, surface, [r["point"] for r in res["attacks"] if r["surface"] == surface and r["verdict"] == "passed"][:5]))
        p = collections.Counter(r["verdict"] for r in res["protocol"])
        if unmeasured:
            lines += ["", "Not measured (a plain value does not reach the site): " + ", ".join(unmeasured)]
        lines += ["", f"Protocol: {len(res['protocol'])} attacks, {p['blocked']} blocked, {p['neutralised']} neutralised, "
                  f"{p['passed']} passed.", ""]
        if p["passed"]:
            failed.append((name, "protocol", [r["test"] for r in res["protocol"] if r["verdict"] == "passed"]))
    lines += parity_table(results)
    lines += ["## Guarantees", ""] + ([f"- FAILED {n}: {GUARANTEES[g]} ({', '.join(map(str, x))})" for n, g, x in failed]
                                     or ["- all hold"])
    (OUT / "summary.md").write_text("\n".join(lines) + "\n")
    return failed


def main(names):
    stack.build()
    names = names or [n for n in SITES if available(n)]
    missing = [n for n in SITES if n not in names]
    results = {}
    for n in names:
        print(f"studying {n}...", flush=True)
        results[n] = study(n)
    failed = summary(results, missing)
    print((OUT / "summary.md").read_text())
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
