"""The slug: one place for what a slug is, for the map the agent writes and
for the agent's own estimate of what the gateway accepts.

A slug is letters and digits of any alphabet, with combining marks, `-` and
`_` (WordPress writes /tag/επίπεδο/). Never quotes, spaces, dots or slashes,
and never the compatibility look-alikes a legacy stack could fold into
ASCII (fullwidth ａｄｍｉｎ, the modifier apostrophe ʼ, ⒈): the gateway's
`no_lookalikes` rule refuses them (gateway/internal/validate/rules.go,
same ranges as LOOKALIKES below).
"""

import functools
import re
import sys
import unicodedata

# The type's pattern, for Go's RE2 (the gateway checks the decoded value).
PATTERN = r"^[\p{L}\p{Nd}][\p{L}\p{Nd}\p{Mn}\p{Mc}_-]{0,199}$"
RULES = ["no_lookalikes"]

# Code points that look like ASCII or fold to it under NFKC (ligatures,
# fullwidth, mathematical and enclosed letters, modifier letters, ſ), the
# invisible Hangul fillers, variation selectors, the combining grapheme
# joiner and the solidus overlays.
LOOKALIKES = (
    (0x00AA, 0x00AA),  # feminine ordinal ª
    (0x00BA, 0x00BA),  # masculine ordinal º
    (0x0132, 0x0133),  # ligature Ĳ ĳ
    (0x013F, 0x0140),  # Ŀ ŀ
    (0x017F, 0x017F),  # long s ſ
    (0x01C4, 0x01CC),  # digraphs Ǆ…ǌ
    (0x01F1, 0x01F3),  # digraphs Ǳ ǲ ǳ
    (0x02B0, 0x02FF),  # spacing modifier letters (ʼ ʹ ˮ)
    (0x0337, 0x0338),  # combining solidus overlays
    (0x034F, 0x034F),  # combining grapheme joiner
    (0x115F, 0x1160),  # Hangul fillers (invisible)
    (0x1D2C, 0x1D6A),  # phonetic modifier letters (ᴬ ᵃ)
    (0x1D78, 0x1D78),  # modifier letter ᵸ
    (0x1D9B, 0x1DBF),  # modifier letters (ᶜ ᶻ)
    (0x1E9A, 0x1E9A),  # ẚ
    (0x2070, 0x209F),  # superscripts and subscripts (ⁱ ₐ)
    (0x20D0, 0x20FF),  # combining marks for symbols (enclosing)
    (0x2100, 0x214F),  # letterlike symbols (ℌ ℓ K)
    (0x2460, 0x24FF),  # enclosed alphanumerics (⒈ ⑴)
    (0x3164, 0x3164),  # Hangul filler (invisible)
    (0x3200, 0x33FF),  # enclosed CJK letters, CJK compatibility
    (0xFB00, 0xFB06),  # Latin ligatures (ﬁ)
    (0xFE00, 0xFE0F),  # variation selectors
    (0xFF00, 0xFFEF),  # halfwidth and fullwidth forms (ａｄｍｉｎ)
    (0x1D400, 0x1D7FF),  # mathematical alphanumerics (𝐚𝐝𝐦𝐢𝐧 𝟏)
    (0x1F100, 0x1F1FF),  # enclosed alphanumeric supplement
    (0x1FBF0, 0x1FBF9),  # segmented digits
    (0xE0100, 0xE01EF),  # variation selectors supplement
)


def lookalike(value: str) -> bool:
    """Whether the value holds a character the no_lookalikes rule refuses."""
    return any(lo <= ord(c) <= hi for c in value for lo, hi in LOOKALIKES)


def is_slug(value: str) -> bool:
    """Whether a decoded path segment is a slug as the gateway checks it."""
    return re2_search(PATTERN, value) is True and not lookalike(value)


# --- RE2 patterns in Python -------------------------------------------------
# Map patterns are written for Go's RE2; Python's re lacks \p{…}. The agent
# only estimates (the gateway decides), but the estimate must not be looser
# or narrower than the gateway on the classes the agent writes.

_CATEGORY = re.compile(r"\\([pP])\{(L[ultmo]?|N[dlo]?|M[nce]?)\}")


@functools.lru_cache(maxsize=None)
def _table() -> dict:
    """{two-letter category: [(first, last) code points]}, in one pass."""
    table, cur, start = {}, None, 0
    for cp in range(sys.maxunicode + 2):
        cat = unicodedata.category(chr(cp)) if cp <= sys.maxunicode else None
        if cat != cur:
            if cur is not None:
                table.setdefault(cur, []).append((start, cp - 1))
            cur, start = cat, cp
    return table


@functools.lru_cache(maxsize=None)
def _ranges(cat: str) -> str:
    """The code points of a Unicode general category (or its group, "L"), as
    the inside of a Python character class."""
    spans = sorted(r for c, rs in _table().items() if c.startswith(cat) for r in rs)
    return "".join(_esc(a) if a == b else _esc(a) + "-" + _esc(b) for a, b in spans)


def _esc(cp: int) -> str:
    return "\\U%08x" % cp


@functools.lru_cache(maxsize=256)
def translate(rx: str):
    """The pattern for Python's re, or None when it uses RE2 syntax this does
    not translate (scripts such as \\p{Greek}, a negated class inside a
    class). `$` means the end of the text, as in RE2; \\d, \\w, \\s are ASCII
    as in RE2 (re2_search passes re.ASCII)."""
    out, i, in_class = [], 0, False
    while i < len(rx):
        c = rx[i]
        if c == "\\":
            m = _CATEGORY.match(rx, i)
            if m:
                body = _ranges(m.group(2))
                if m.group(1) == "P":
                    if in_class:
                        return None
                    out.append("[^" + body + "]")
                else:
                    out.append(body if in_class else "[" + body + "]")
                i = m.end()
                continue
            if rx[i + 1:i + 2] in ("p", "P"):
                return None  # \pL, \p{Greek}…
            out.append(rx[i:i + 2])
            i += 2
            continue
        if in_class:
            if c == "[":
                return None  # [[:alpha:]] and the like
            if c == "]":
                in_class = False
            out.append(c)
        elif c == "[":
            in_class = True
            out.append(c)
            if rx[i + 1:i + 2] == "^":
                out.append("^")
                i += 1
            if rx[i + 1:i + 2] == "]":  # a literal ] first in the class
                out.append("\\]")
                i += 1
        elif c == "$":
            out.append(r"\Z")
        else:
            out.append(c)
        i += 1
    return "".join(out)


def re2_search(rx: str, value: str):
    """re.search with an RE2 map pattern: True or False, or None when the
    pattern cannot be read here (the gateway decides)."""
    py = translate(rx)
    if py is None:
        return None
    try:
        return re.search(py, value, re.ASCII) is not None
    except re.error:
        return None
