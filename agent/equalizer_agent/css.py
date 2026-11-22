"""What a stylesheet loads: fonts, background images, other stylesheets.

Read from stylesheet files, <style> blocks and style="" attributes, so that
files a page only names in its CSS are studied like those in its HTML."""

import re
import urllib.parse

# url(x), url('x'), url("x") and @import "x" / @import 'x'
_URL = re.compile(r"""url\(\s*(['"]?)([^'")\s]{1,500})\1\s*\)|@import\s+(['"])([^'"]{1,500})\3""", re.I)
_COMMENT = re.compile(r"/\*.*?\*/", re.S)

MAX_REFS = 500


def refs(text: str, base: str) -> list:
    """Absolute URLs named by a piece of CSS, in order, without repeats."""
    out = []
    for m in _URL.finditer(_COMMENT.sub("", text[:2_000_000])):
        ref = (m.group(2) or m.group(4) or "").strip()
        if not ref or ref.startswith(("data:", "#", "about:", "javascript:")):
            continue
        out.append(urllib.parse.urljoin(base, ref))
        if len(out) >= MAX_REFS:
            break
    return list(dict.fromkeys(out))
