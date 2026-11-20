"""Fixed safety rules: what is never crawled and never mapped. They do not
depend on the AI, which can only add exclusions. What is specific to an
application lives in its profile (webapps/); this module applies them all."""

import re

from . import webapps
from .webapps import is_action  # noqa: F401  (the crawler's rule for links that act)

_PROFILES = webapps.ALL + (webapps.GENERIC,)


def excluded(path_query: str, cms: str | None, logged_in: bool = False) -> str:
    """Returns why a URL is not opened ('' when it is). Every profile's
    never rules apply whatever the site is (a site is not always
    recognised); administrations are those of the site's profile, or all of
    them while it is unknown. A test account's pass opens the administration
    of an application whose profile the lab proved read-only."""
    for p in _PROFILES:
        for rx in p.never:
            if re.search(rx, path_query, re.I):
                return "never"
    for rx in webapps.SENSITIVE:
        if re.search(rx, path_query, re.I):
            return "sensitive"
    prof = webapps.get(cms)
    if logged_in and prof.admin_read_only:
        return ""
    # a recognised site: its own administration and the usual paths; not
    # yet recognised: every profile's, to be safe
    for p in ((prof, webapps.GENERIC) if prof is not webapps.GENERIC else _PROFILES):
        for rx in p.admin:
            if re.search(rx, path_query, re.I):
                return "admin"
    return ""
