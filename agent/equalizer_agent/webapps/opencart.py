"""OpenCart."""
import re

from .base import Profile

PROFILE = Profile(
    name="opencart", title="OpenCart",
    generator=re.compile(r"opencart\s*([\d.]+)?", re.I),
    paths=re.compile(r"[?&]route=(common/home|product/|information/)|/catalog/view/(theme|javascript)/|/image/cache/"),
    components=re.compile(r"/(extension)/([a-z0-9_]+)/", re.I),
    login="/index.php?route=account/login",
    admin=(r"^/admin(/|$)", r"^/install(/|$)", r"^/system(/|$)", r"^/storage(/|$)"),
    never=(r"[?&]route=(account/logout|checkout/cart\.(add|remove|edit)|checkout/|account/wishlist\.(add|remove))",
           r"^/install(/|$)"),
    tested="Lab (MariaDB, demo products): catalogue, search, customer login (in script, the session cookie kept); studied with a customer account, database unchanged; 163 of 165 pages identical through the corrected draft.",
    note="Administration closed until the lab proves that viewing it changes nothing. Cart and checkout links "
         "are never opened.",
)
