"""The agent corrects its own draft (docs/agent.md): it widens what breaks a
page, narrows what the values it saw allow, and verifies again.

The rules:
- working comes before protecting: a correction that breaks a page that
  worked is dropped;
- narrowing uses only values seen on the site, never a guess;
- every change is reported with its reason; the result is a new draft,
  approved by a person like any other."""

import copy
import re
import urllib.parse

from . import mapbuild

WIDE = {"type": "string", "max_length": 1000, "rules": ["no_html", "no_template_meta"]}
INT = re.compile(r"^-?\d{1,9}$")
PLAIN = re.compile(r"^[A-Za-z0-9_.,\-]{1,200}$")
ROUNDS = 3


def _entries(doc):
    for kind in ("routes", "forms"):
        for e in doc.get(kind, []):
            yield kind, e


def _find(doc, entry_id):
    return next(((k, e) for k, e in _entries(doc) if e.get("id") == entry_id), (None, None))


def _specs(entry):
    """name -> (container dict, spec) for every input of an entry."""
    out = {}
    for key in ("query", "params", "fields"):
        for name, spec in (entry.get(key) or {}).items():
            out[name] = (entry[key], spec)
    return out


def _describe(spec) -> str:
    if spec.get("type") == "enum":
        return "one of " + ", ".join(spec.get("values", [])[:5])
    return spec.get("type", "string") + (" with a pattern" if spec.get("pattern") else "")


def widen(doc: dict, m) -> list:
    """Changes doc so the pages m found broken work again; the corrections."""
    out, areas = [], {a.get("id") for a in doc.get("areas", [])}
    for b in m.broken:
        eid = b.get("entry")
        if not eid or eid in areas:
            continue  # served by the area: nothing in the map to widen
        kind, e = _find(doc, eid)
        if e is None:
            continue
        fields = {f for f in m.refused_fields.get(eid, set()) if f} if b["difference"] == "refused" else set()
        specs = _specs(e)
        widenable = [f for f in fields if f in specs and specs[f][1] != dict(WIDE, required=specs[f][1].get("required", False))]
        if widenable:
            for f in widenable:
                box, spec = specs[f]
                box[f] = dict(WIDE, required=spec.get("required", False))
                out.append({"entry": f"{eid}.{f}", "change": "widened", "detail": _describe(spec) + " → text",
                            "reason": f"{b['path']} was refused through Equalizer"})
        else:
            doc[kind] = [x for x in doc[kind] if x.get("id") != eid]
            out.append({"entry": eid, "change": "removed", "detail": "back to the site area",
                        "reason": f"{b['path']}: {b.get('detail') or b['difference']}"})
    return out


def narrow(doc: dict, m, pages: list) -> list:
    """Changes doc where the values seen on the site allow a narrower type
    than the one an attack got through; the corrections."""
    out = []
    for o in m.open:
        if o["kind"] != "input" or "." not in o["target"]:
            continue
        eid, name = o["target"].rsplit(".", 1)
        kind, e = _find(doc, eid)
        if kind != "routes" or name not in (e.get("query") or {}):
            continue
        spec = e["query"][name]
        if spec.get("type") != "string" or spec.get("pattern"):
            continue
        seen = [v for pq in pages if mapbuild.fits(doc, e, pq.split("?")[0])
                for k, v in urllib.parse.parse_qsl(pq.partition("?")[2], keep_blank_values=True) if k == name and v]
        if len(seen) < 3:
            continue  # too few values to say what the field holds
        if all(INT.match(v) for v in seen):
            e["query"][name] = {"type": "int", "min": -999999999 if any(v.startswith("-") for v in seen) else 0,
                                "max": 999999999, "required": spec.get("required", False)}
            out.append({"entry": o["target"], "change": "narrowed", "detail": "string → int",
                        "reason": f"every value seen ({len(seen)}) is a number"})
        elif all(PLAIN.match(v) for v in seen):
            longest = max(len(v) for v in seen)
            e["query"][name] = dict(spec, pattern="^[A-Za-z0-9_.,\\-]{1,%d}$" % min(200, max(64, longest * 2)))
            out.append({"entry": o["target"], "change": "narrowed", "detail": "string → letters, digits and . , _ -",
                        "reason": f"every value seen ({len(seen)}) is made of them"})
    return out


def loop(job, doc, first, run, panel, log):
    """Rounds of widen, verify, narrow, verify; keeps only what does not
    break a page that worked. Returns the document, its last measure and the
    corrections."""
    pages = list(job["pages"]) + [p for ps in (job.get("pages_by_account") or {}).values() for p in ps]
    current, measure, corrections = copy.deepcopy(doc), first, []
    for _ in range(ROUNDS):
        changed = False
        candidate = copy.deepcopy(current)
        widened = widen(candidate, measure)
        if widened:
            tm = panel.try_map(job["id"], candidate)
            m2 = run.measure(candidate, tm) if tm else None
            if m2 is not None and m2.identical > measure.identical:  # it must mend a page
                current, measure, changed = candidate, m2, True
                corrections += widened
            else:
                log.warn("correction dropped", verification=job["id"], kind="widen")
        candidate = copy.deepcopy(current)
        narrowed = narrow(candidate, measure, pages)
        if narrowed:
            tm = panel.try_map(job["id"], candidate)
            m2 = run.measure(candidate, tm) if tm else None
            if m2 is not None and m2.identical >= measure.identical and m2.protected > measure.protected:
                current, measure, changed = candidate, m2, True
                corrections += narrowed
            else:
                log.warn("correction dropped", verification=job["id"], kind="narrow")
        if not changed:
            break
    return current, measure, corrections
