"""From a form field's HTML to the narrowest type the gateway checks.

Read-only: the agent never submits a form, so it cannot see the values
people type. What it has is what the page declares: the field's type, its
limits (maxlength, min, max, step, pattern), the options of a select, the
values of radio buttons and checkboxes, and the field's name. Each field
gets the narrowest specification those allow; a field that must still
accept characters an attack needs is reported, with the reason, instead of
being guessed narrower (docs/agent.md, step 2)."""

import re

INT_LIMIT = 999999999

# Names of single-line fields that hold structured data (a name, a code, a
# phone): no SQL delimiters (an apostrophe stays allowed between letters,
# as in O'Brien).
STRUCTURED = re.compile(r"(^|_|-)(user(name)?|login|log|name|first_?name|last_?name|surname|nick|city|town|zip|"
                        r"postcode|postal|cap|code|phone|tel|mobile|company|vat|piva|fiscal|cf|ssn|iban|account|"
                        r"id|number|num|qty|quantity|amount|price)($|_|-)", re.I)
SEARCH = re.compile(r"^(s|q|query|search|keyword|keywords|term|terms|find|searchword|search_api_fulltext)$", re.I)

# HTML patterns use the browser's regular expressions; the gateway's are
# RE2. Only patterns that mean the same in both are kept.
UNSAFE_PATTERN = re.compile(r"\(\?|\\[1-9kpPuxc]|\\[bB]|[\x00-\x1f]")

TEXT_RULES = ["no_html", "no_template_meta"]


def _int(v: str):
    try:
        return int(v)
    except (TypeError, ValueError):
        return None


def _pattern(p: str) -> str:
    """The HTML pattern anchored like the browser does, or "" when it cannot
    be kept."""
    if not p or len(p) > 200 or UNSAFE_PATTERN.search(p):
        return ""
    counts = [int(n) for pair in re.findall(r"\{(\d+)(?:,(\d*))?\}", p) for n in pair if n]
    if any(n > 1000 for n in counts):
        return ""  # RE2 refuses repeats over 1000
    try:
        re.compile(p)
    except re.error:
        return ""
    return "^(?:" + p + ")$"


def _choices(values: list) -> list:
    return sorted({v for v in values if v != "" and len(v) <= 100})[:100]


def spec(i) -> tuple:
    """(specification, open reason or "") for one visible field."""
    t = i.type
    if t in ("select", "radio", "checkbox"):
        vals = _choices(i.options)
        if vals:
            return {"type": "enum", "values": vals}, ""
        return {"type": "string", "max_length": 100, "rules": list(TEXT_RULES)}, "cannot_narrow"
    if t in ("number", "range"):
        lo, hi, step = _int(i.min), _int(i.max), i.step
        if step == "" or (_int(step) or 0) > 0:
            return {"type": "int", "min": lo if lo is not None else -INT_LIMIT,
                    "max": hi if hi is not None else INT_LIMIT}, ""
        return {"type": "string", "max_length": 30, "pattern": r"^-?[0-9]{1,15}([.,][0-9]{1,10})?$"}, ""
    if t == "email":
        return {"type": "email"}, ""
    if t == "date":
        return {"type": "date"}, ""
    if t == "url":
        return {"type": "string", "max_length": 2000, "pattern": r"^https?://[^\s<>\"'`]+$"}, ""
    if t == "tel":
        return {"type": "string", "max_length": 30, "pattern": r"^[0-9+() .\-]{3,30}$"}, ""
    if t == "color":
        return {"type": "string", "max_length": 7, "pattern": r"^#[0-9A-Fa-f]{6}$"}, ""
    if t == "time":
        return {"type": "string", "max_length": 8, "pattern": r"^[0-9]{2}:[0-9]{2}(:[0-9]{2})?$"}, ""
    if t in ("datetime-local", "month", "week"):
        return {"type": "string", "max_length": 20, "pattern": r"^[0-9]{4}-[0-9W:T\-]{2,15}$"}, ""
    if t == "password":
        return {"type": "password", "max_length": 128, "rules": ["no_template_meta"]}, "accepts_sql"
    if t == "textarea":
        return {"type": "text", "max_length": i.maxlength or 5000, "rules": list(TEXT_RULES)}, "accepts_sql"
    # text, search, and anything a browser shows as a text box
    length = min(i.maxlength or 200, 1000)
    pat = _pattern(i.pattern)
    if pat:
        return {"type": "string", "max_length": length, "pattern": pat}, ""
    if STRUCTURED.search(i.name) and not SEARCH.match(i.name):
        return {"type": "string", "max_length": length, "rules": TEXT_RULES + ["no_sql_meta"]}, ""
    return {"type": "string", "max_length": length, "rules": list(TEXT_RULES)}, "accepts_sql"
