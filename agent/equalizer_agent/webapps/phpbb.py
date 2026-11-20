"""phpBB."""
import re

from .base import Profile

PROFILE = Profile(
    name="phpbb", title="phpBB",
    generator=re.compile(r"phpbb\s*([\d.]+)?", re.I),
    paths=re.compile(r"/(viewtopic|viewforum|memberlist|ucp)\.php|/styles/(prosilver|subsilver2)/"),
    components=re.compile(r"/(ext)/([a-z0-9_]+/[a-z0-9_]+)/", re.I),
    login="/ucp.php?mode=login",
    admin=(r"^/adm(/|$)", r"^/install(/|$)", r"^/mcp\.php", r"^/ucp\.php\?.*mode=(register|activate|delete_cookies)"),
    never=(r"[?&]mode=(logout|delete_cookies|delete|bump|bookmark|subscribe|unsubscribe|watch|unwatch)\b",
           r"^/posting\.php", r"^/mcp\.php", r"^/install(/|$)", r"[?&]hash="),
    note="Moderation and administration closed until the lab proves that viewing them changes nothing; posting "
         "and moderation links are never opened.",
)
