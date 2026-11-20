"""PrestaShop."""
import re

from .base import Profile

PROFILE = Profile(
    name="prestashop", title="PrestaShop",
    generator=re.compile(r"prestashop\s*([\d.]+)?", re.I),
    paths=re.compile(r"/modules/ps_[a-z0-9_]+/|/themes/(classic|default-bootstrap)/|[?&]controller=(product|category|cms)"),
    components=re.compile(r"/(modules)/([a-z0-9_]+)/", re.I),
    login="/index.php?controller=authentication",
    # The administration folder is renamed at install (/admin123abc): the
    # usual names are closed, the real one stays with the site area.
    admin=(r"^/admin[a-z0-9_]*(/|$)", r"^/api(/|$)", r"^/install(/|$)", r"^/install-dev(/|$)"),
    never=(r"[?&]controller=cart\b.*[?&](add|delete|update)=", r"[?&]mylogout", r"^/install(/|$)",
           r"[?&]controller=(order|order-confirmation|password)\b"),
    tested="Lab (MariaDB, demo products, plain URLs): catalogue, customer login; studied with a customer account, database unchanged; 220 of 220 pages identical through the draft.",
    note="Administration closed: its folder is renamed at installation. Cart and order links are never opened.",
)
