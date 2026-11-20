"""TYPO3."""
import re

from .base import Profile

PROFILE = Profile(
    name="typo3", title="TYPO3",
    generator=re.compile(r"typo3\s*(?:cms\s*)?([\d.]+)?", re.I),
    paths=re.compile(r"/(typo3conf|typo3temp|fileadmin)/|/_assets/[0-9a-f]{32}/"),
    components=re.compile(r"/typo3conf/(ext)/([a-z0-9_]+)/", re.I),
    login="",  # the front-end login (felogin) lives on a page each site chooses
    admin=(r"^/typo3(/|$)", r"^/typo3/install", r"^/typo3conf(/|$)"),
    admin_home=("/typo3/",),
    admin_login="/typo3/",  # the backend has a login of its own
    never=(r"^/typo3/install(\.php|/|$)", r"^/typo3/sysext/install/", r"[?&]logintype=logout"),
    note="Administration closed until the lab proves that viewing it changes nothing.",
)
